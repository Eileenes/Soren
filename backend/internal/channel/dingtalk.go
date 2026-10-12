package channel

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"

	"github.com/soren/backend/internal/config"
)

// 钉钉渠道常量
const (
	// DingTalkChannelName 渠道名称
	DingTalkChannelName = "dingtalk"

	// dingtalkMaxTextLength 单条 markdown 消息的最大字符数
	dingtalkMaxTextLength = 3000

	// dingtalkMessageBuffer 内部消息队列长度
	dingtalkMessageBuffer = 64

	// dingtalkReplayWindow 加签校验允许的时间戳偏差
	dingtalkReplayWindow = time.Hour
)

// DingTalkChannel 钉钉渠道
//
// 支持两种接入方式：
//   - Stream 长连接（默认）：桌面端 / 内网无需公网 IP，最适合桌面应用
//   - Outgoing Webhook：需要公网回调地址，支持加签校验
type DingTalkChannel struct {
	cfg config.DingTalkConfig

	streamClient *client.StreamClient

	messages chan *IncomingMessage
	httpSrv  *http.Server

	// replyTargets 记录会话最近一次可用的回复地址（sessionWebhook）
	// 钉钉机器人消息的 sessionWebhook 有时效性，收到消息时刷新
	targetMu     sync.RWMutex
	replyTargets map[string]string
	httpClient   *http.Client

	mu      sync.Mutex
	running bool
}

// NewDingTalkChannel 创建钉钉渠道
func NewDingTalkChannel(cfg config.DingTalkConfig) *DingTalkChannel {
	// RobotCode 未单独配置时默认与应用 AppKey 一致
	if cfg.RobotCode == "" {
		cfg.RobotCode = cfg.ClientID
	}

	return &DingTalkChannel{
		cfg:          cfg,
		messages:     make(chan *IncomingMessage, dingtalkMessageBuffer),
		replyTargets: make(map[string]string),
		httpClient:   &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

// Name 渠道名称
func (c *DingTalkChannel) Name() string { return DingTalkChannelName }

// Messages 返回消息流
func (c *DingTalkChannel) Messages() <-chan *IncomingMessage { return c.messages }

// Status 返回渠道状态
func (c *DingTalkChannel) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	mode := "stream"
	if !c.streamMode() {
		mode = "webhook"
	}

	return Status{
		Name:        DingTalkChannelName,
		Type:        "dingtalk",
		Enabled:     c.cfg.Enabled,
		Running:     c.running,
		Mode:        mode,
		Configured:  c.cfg.ClientID != "" && c.cfg.ClientSecret != "",
		Description: "钉钉机器人（Stream 长连接或 Outgoing Webhook）",
	}
}

// streamMode 是否使用 Stream 长连接模式（默认）
func (c *DingTalkChannel) streamMode() bool {
	return c.cfg.TransportMode() == config.ChannelModeWS
}

// Start 启动钉钉渠道
func (c *DingTalkChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	if c.cfg.ClientID == "" || c.cfg.ClientSecret == "" {
		c.mu.Unlock()
		return fmt.Errorf("dingtalk channel requires DINGTALK_CLIENT_ID and DINGTALK_CLIENT_SECRET")
	}
	if !c.streamMode() && (c.cfg.WebhookSecret == "") {
		c.mu.Unlock()
		return fmt.Errorf("dingtalk webhook mode requires DINGTALK_WEBHOOK_SECRET")
	}
	c.running = true
	c.mu.Unlock()

	if c.streamMode() {
		if err := c.startStream(); err != nil {
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
			return err
		}
		log.Println("[dingtalk] channel started (stream mode)")
		return nil
	}

	if err := c.startWebhook(); err != nil {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		return err
	}
	log.Println("[dingtalk] channel started (webhook mode)")
	return nil
}

// startStream 建立 Stream 长连接
func (c *DingTalkChannel) startStream() error {
	c.streamClient = client.NewStreamClient(
		client.WithAppCredential(client.NewAppCredentialConfig(c.cfg.ClientID, c.cfg.ClientSecret)),
		// 断线自动重连，保证桌面端长时间在线
		client.WithAutoReconnect(true),
	)
	// 机器人消息回调
	c.streamClient.RegisterChatBotCallbackRouter(c.onBotMessage)

	// Start 会阻塞直到连接结束，放入独立协程
	go func() {
		if err := c.streamClient.Start(context.Background()); err != nil {
			log.Printf("[dingtalk] stream client stopped: %v", err)
		}
	}()

	return nil
}

// onBotMessage 处理通过 Stream 收到的机器人消息
//
// 返回的字节会被 SDK 作为回调响应内容回传，这里直接进行业务处理并异步回复，
// 因此返回空内容，避免 SDK 等待。
func (c *DingTalkChannel) onBotMessage(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[dingtalk] panic while handling message: %v", r)
		}
	}()

	if data == nil {
		return []byte(""), nil
	}

	// 只处理文本消息
	text := strings.TrimSpace(data.Text.Content)
	if text == "" {
		return []byte(""), nil
	}

	// 回调自带旧版 JSON 结构，sessionWebhook 有时效，这里直接用原始消息里的地址
	incoming := &IncomingMessage{
		Channel:        DingTalkChannelName,
		Type:           "text",
		ConversationID: data.ConversationId,
		SenderID:       firstNonEmpty(data.SenderStaffId, data.SenderId),
		SenderName:     data.SenderNick,
		MessageID:      data.MsgId,
		Text:           text,
		ReplyTarget:    data.SessionWebhook,
		Raw:            data,
		ReceivedAt:     time.Now(),
	}

	if data.SessionWebhook != "" {
		c.rememberReplyTarget(data.ConversationId, data.SessionWebhook)
	}

	c.publish(incoming)
	return []byte(""), nil
}

// startWebhook 启动 Outgoing Webhook 回调服务
func (c *DingTalkChannel) startWebhook() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/dingtalk", c.handleWebhook)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", c.cfg.WebhookPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	c.httpSrv = srv

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[dingtalk] webhook server stopped: %v", err)
		}
	}()

	return nil
}

// dingtalkWebhookPayload Webhook 模式下的机器人消息体
type dingtalkWebhookPayload struct {
	ConversationID string `json:"conversationId"`
	MsgID          string `json:"msgId"`
	SenderNick     string `json:"senderNick"`
	SenderStaffID  string `json:"senderStaffId"`
	SenderID       string `json:"senderId"`
	SessionWebhook string `json:"sessionWebhook"`
	Msgtype        string `json:"msgtype"`
	Text           struct {
		Content string `json:"content"`
	} `json:"text"`
}

// handleWebhook 处理 Outgoing Webhook 回调
//
// 钉钉机器人加签校验：请求头带 timestamp 与 sign，
// sign = Base64(HMAC-SHA256(timestamp + "\n" + secret))
func (c *DingTalkChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if c.cfg.WebhookSecret == "" {
		http.Error(w, "webhook authentication is not configured", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if c.cfg.WebhookSecret != "" {
		if err := verifyDingTalkSignature(r, c.cfg.WebhookSecret); err != nil {
			log.Printf("[dingtalk] webhook signature verification failed: %v", err)
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var payload dingtalkWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(payload.Text.Content)
	if text == "" {
		// 非文本消息，直接回执成功，避免钉钉重推
		writeDingTalkOK(w)
		return
	}

	if payload.SessionWebhook != "" {
		c.rememberReplyTarget(payload.ConversationID, payload.SessionWebhook)
	}

	c.publish(&IncomingMessage{
		Channel:        DingTalkChannelName,
		Type:           "text",
		ConversationID: payload.ConversationID,
		SenderID:       firstNonEmpty(payload.SenderStaffID, payload.SenderID),
		SenderName:     payload.SenderNick,
		MessageID:      payload.MsgID,
		Text:           text,
		ReplyTarget:    payload.SessionWebhook,
		Raw:            payload,
		ReceivedAt:     time.Now(),
	})

	writeDingTalkOK(w)
}

// writeDingTalkOK 返回钉钉要求的成功回执
func writeDingTalkOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"msgtype":"empty"}`))
}

// verifyDingTalkSignature 校验钉钉加签
//
// 钉钉的加签算法：sign = Base64(HMAC-SHA256(timestamp + "\n" + secret))
func verifyDingTalkSignature(r *http.Request, secret string) error {
	timestamp := r.Header.Get("timestamp")
	sign := r.Header.Get("sign")
	if timestamp == "" || sign == "" {
		return fmt.Errorf("missing signature headers")
	}

	// 校验时间戳偏移，防止重放攻击
	ts, err := parseTimestampMillis(timestamp)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	if diff := time.Since(ts); diff > dingtalkReplayWindow || diff < -dingtalkReplayWindow {
		return fmt.Errorf("timestamp expired")
	}

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(timestamp + "\n" + secret))
	expected := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(sign)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// parseTimestampMillis 解析毫秒时间戳
func parseTimestampMillis(value string) (time.Time, error) {
	ms, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms), nil
}

// publish 投递消息到内部队列，队列满时丢弃以避免阻塞 SDK 读循环
func (c *DingTalkChannel) publish(msg *IncomingMessage) {
	select {
	case c.messages <- msg:
	default:
		log.Printf("[dingtalk] message queue is full, dropping message %s", msg.MessageID)
	}
}

// rememberReplyTarget 记录会话的回复地址
func (c *DingTalkChannel) rememberReplyTarget(conversationID, webhook string) {
	if conversationID == "" || webhook == "" {
		return
	}
	c.targetMu.Lock()
	defer c.targetMu.Unlock()
	c.replyTargets[conversationID] = webhook
}

// replyTarget 获取会话的回复地址，优先使用消息自带的地址
func (c *DingTalkChannel) replyTarget(conversationID, fallback string) string {
	if fallback != "" {
		return fallback
	}
	c.targetMu.RLock()
	defer c.targetMu.RUnlock()
	return c.replyTargets[conversationID]
}

// Stop 停止钉钉渠道
func (c *DingTalkChannel) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	if c.streamClient != nil {
		c.streamClient.Close()
		c.streamClient = nil
	}
	if c.httpSrv != nil {
		_ = c.httpSrv.Shutdown(ctx)
		c.httpSrv = nil
	}

	c.running = false
	log.Println("[dingtalk] channel stopped")
	return nil
}

// Send 发送消息
//
// 钉钉机器人回复通过 sessionWebhook 主动推送，支持 text 与 markdown 两种消息类型。
func (c *DingTalkChannel) Send(ctx context.Context, reply *Reply) error {
	if reply == nil {
		return fmt.Errorf("dingtalk: empty reply")
	}

	target := c.replyTarget(reply.ConversationID, reply.ReplyTarget)
	if target == "" {
		return fmt.Errorf("dingtalk: no session webhook available for conversation %s", reply.ConversationID)
	}

	// 回复内容优先使用 markdown 以获得更好的排版
	content := reply.Markdown
	if content == "" {
		content = reply.Text
	}

	chunks := splitText(content, dingtalkMaxTextLength)
	for _, chunk := range chunks {
		var payload map[string]any
		if reply.Markdown != "" {
			payload = map[string]any{
				"msgtype": "markdown",
				"markdown": map[string]any{
					"title": "Soren",
					"text":  chunk,
				},
			}
		} else {
			payload = map[string]any{
				"msgtype": "text",
				"text": map[string]any{
					"content": chunk,
				},
			}
		}

		if err := c.postJSON(ctx, target, payload); err != nil {
			return err
		}
	}

	return nil
}

// postJSON 向钉钉回复地址推送 JSON 消息
func (c *DingTalkChannel) postJSON(ctx context.Context, target string, payload map[string]any) error {
	// 仅允许 HTTPS 回执地址，避免会话数据被推送到不可信地址
	if u, err := url.Parse(target); err != nil || u.Scheme != "https" || u.Hostname() != "oapi.dingtalk.com" || (u.Port() != "" && u.Port() != "443") || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("dingtalk: invalid session webhook")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("dingtalk: marshal reply failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("dingtalk: build request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dingtalk: reply transport failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dingtalk: send message failed, status=%d", resp.StatusCode)
	}

	// 钉钉回执为 JSON，errcode 非 0 表示失败
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.ErrCode != 0 {
		return fmt.Errorf("dingtalk: send message rejected, errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// firstNonEmpty 返回第一个非空字符串
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
