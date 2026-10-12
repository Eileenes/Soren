package channel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"github.com/soren/backend/internal/config"
)

// 飞书渠道常量
const (
	// FeishuChannelName 渠道名称
	FeishuChannelName = "feishu"

	// feishuMaxTextLength 单条文本消息的最大字符数，超出则分片发送
	feishuMaxTextLength = 3000

	// feishuMessageBuffer 内部消息队列长度
	feishuMessageBuffer = 64
)

// FeishuChannel 飞书渠道
//
// 支持两种事件接入方式：
//   - WebSocket 长连接（默认）：桌面端 / 内网环境无需公网 IP
//   - Webhook 事件订阅：需要公网回调地址，支持 Verification Token / Encrypt Key 校验
type FeishuChannel struct {
	cfg      config.FeishuConfig
	api      *lark.Client
	wsClient *larkws.Client

	handler *dispatcher.EventDispatcher

	messages chan *IncomingMessage
	httpSrv  *http.Server

	mu      sync.Mutex
	running bool
}

// NewFeishuChannel 创建飞书渠道
func NewFeishuChannel(cfg config.FeishuConfig) *FeishuChannel {
	// 日志级别统一设为 Error，避免 SDK 的 Info 日志在桌面端造成噪音
	opts := []lark.ClientOptionFunc{
		lark.WithEnableTokenCache(true),
		lark.WithLogLevel(larkcore.LogLevelError),
	}
	// 允许指向私有化部署的开放平台地址
	if cfg.BaseURL != "" && cfg.BaseURL != "https://open.feishu.cn" {
		opts = append(opts, lark.WithOpenBaseUrl(cfg.BaseURL))
	}

	return &FeishuChannel{
		cfg:      cfg,
		api:      lark.NewClient(cfg.AppID, cfg.AppSecret, opts...),
		messages: make(chan *IncomingMessage, feishuMessageBuffer),
	}
}

// Name 渠道名称
func (c *FeishuChannel) Name() string { return FeishuChannelName }

// Messages 返回消息流
func (c *FeishuChannel) Messages() <-chan *IncomingMessage { return c.messages }

// Status 返回渠道状态
func (c *FeishuChannel) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	mode := "websocket"
	if !c.wsMode() {
		mode = "webhook"
	}

	return Status{
		Name:        FeishuChannelName,
		Type:        "feishu",
		Enabled:     c.cfg.Enabled,
		Running:     c.running,
		Mode:        mode,
		Configured:  c.cfg.AppID != "" && c.cfg.AppSecret != "",
		Description: "飞书开放平台机器人（长连接或 Webhook 事件订阅）",
	}
}

// wsMode 是否使用长连接模式（默认）
func (c *FeishuChannel) wsMode() bool {
	return c.cfg.TransportMode() == config.ChannelModeWS
}

// Start 启动飞书渠道
func (c *FeishuChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	if c.cfg.AppID == "" || c.cfg.AppSecret == "" {
		c.mu.Unlock()
		return fmt.Errorf("feishu channel requires FEISHU_APP_ID and FEISHU_APP_SECRET")
	}
	if !c.wsMode() && (c.cfg.VerificationToken == "" || c.cfg.EncryptKey == "") {
		c.mu.Unlock()
		return fmt.Errorf("feishu webhook mode requires FEISHU_VERIFICATION_TOKEN and FEISHU_ENCRYPT_KEY")
	}
	c.running = true
	c.mu.Unlock()

	if c.wsMode() {
		if err := c.startWS(); err != nil {
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
			return err
		}
		log.Println("[feishu] channel started (websocket mode)")
		return nil
	}

	if err := c.startWebhook(); err != nil {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		return err
	}
	log.Println("[feishu] channel started (webhook mode)")
	return nil
}

// startWS 建立长连接
func (c *FeishuChannel) startWS() error {
	c.handler = dispatcher.NewEventDispatcher(c.cfg.VerificationToken, c.cfg.EncryptKey)
	// 仅在长连接模式下注册业务处理器；Webhook 模式在请求处理时按需注册
	c.registerMessageHandler(c.handler)

	c.wsClient = larkws.NewClient(c.cfg.AppID, c.cfg.AppSecret,
		larkws.WithEventHandler(c.handler),
		larkws.WithLogLevel(larkcore.LogLevelError),
		larkws.WithAutoReconnect(true),
	)

	// Start 会阻塞直到连接结束，因此放到独立协程中运行
	go func() {
		if err := c.wsClient.Start(context.Background()); err != nil {
			log.Printf("[feishu] websocket client stopped: %v", err)
		}
	}()

	return nil
}

// startWebhook 启动 Webhook 事件订阅服务
func (c *FeishuChannel) startWebhook() error {
	c.handler = dispatcher.NewEventDispatcher(c.cfg.VerificationToken, c.cfg.EncryptKey)
	c.registerMessageHandler(c.handler)

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/feishu", c.handleWebhook)

	// 监听独立端口，避免与 API 服务端口冲突
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", c.cfg.WebhookPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	c.httpSrv = srv

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[feishu] webhook server stopped: %v", err)
		}
	}()

	return nil
}

// registerMessageHandler 注册 im.message.receive_v1 事件处理器
func (c *FeishuChannel) registerMessageHandler(d *dispatcher.EventDispatcher) {
	d.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		c.onMessageReceived(event)
		return nil
	})
}

// onMessageReceived 处理收到的飞书消息
func (c *FeishuChannel) onMessageReceived(event *larkim.P2MessageReceiveV1) {
	defer func() {
		// 单个事件处理失败不应影响 SDK 的连接与后续事件
		if r := recover(); r != nil {
			log.Printf("[feishu] panic while handling message: %v", r)
		}
	}()

	if event == nil || event.Event == nil || event.Event.Message == nil {
		return
	}

	msg := event.Event.Message

	// 忽略机器人自己发送的消息，避免自问自答死循环
	if event.Event.Sender != nil && event.Event.Sender.SenderType != nil &&
		*event.Event.Sender.SenderType == "bot" {
		return
	}

	text := extractFeishuText(msg)
	if strings.TrimSpace(text) == "" {
		return
	}

	incoming := &IncomingMessage{
		Channel:        FeishuChannelName,
		Type:           derefString(msg.MessageType, "text"),
		ConversationID: feishuConversationID(msg),
		MessageID:      derefString(msg.MessageId, ""),
		Text:           text,
		Raw:            event,
		ReceivedAt:     time.Now(),
	}

	if msg.ChatType != nil && *msg.ChatType == "p2p" {
		incoming.ReplyTarget = "p2p"
	}
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		incoming.SenderID = derefString(event.Event.Sender.SenderId.OpenId, "")
	}

	c.publish(incoming)
}

// extractFeishuText 从消息体中提取文本
//
// 飞书不同消息类型的 content 结构不同，这里只处理最常用的文本与富文本，
// 其他类型（图片、文件等）返回空字符串，由上层忽略。
func extractFeishuText(msg *larkim.EventMessage) string {
	content := derefString(msg.Content, "")
	if content == "" {
		return ""
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return ""
	}

	// 群聊中用户 @机器人 会产生 @_user_1 占位符，需要清理
	return cleanFeishuMentions(payload.Text, msg.Mentions)
}

// cleanFeishuMentions 去除文本中的 @ 占位符
func cleanFeishuMentions(text string, mentions []*larkim.MentionEvent) string {
	for _, m := range mentions {
		if m == nil || m.Key == nil {
			continue
		}
		text = strings.ReplaceAll(text, *m.Key, "")
	}
	return strings.TrimSpace(text)
}

// feishuConversationID 生成会话标识
//
// 统一带上 ID 类型前缀，发送时可直接还原 receive_id_type：
// 群聊使用 chat_id，单聊使用 open_id。
func feishuConversationID(msg *larkim.EventMessage) string {
	if msg.ChatType != nil && *msg.ChatType == "p2p" {
		// 单聊优先用 chat_id：机器人始终位于该会话中，权限更稳妥
		if msg.ChatId != nil && *msg.ChatId != "" {
			return "chat_id:" + *msg.ChatId
		}
	}
	if msg.ChatId != nil && *msg.ChatId != "" {
		return "chat_id:" + *msg.ChatId
	}
	return ""
}

// publish 投递消息到内部队列，队列满时丢弃以避免阻塞 SDK 的连接读循环
func (c *FeishuChannel) publish(msg *IncomingMessage) {
	select {
	case c.messages <- msg:
	default:
		log.Printf("[feishu] message queue is full, dropping message %s", msg.MessageID)
	}
}

// Stop 停止飞书渠道
func (c *FeishuChannel) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	if c.wsClient != nil {
		c.wsClient.Close()
		c.wsClient = nil
	}
	if c.httpSrv != nil {
		_ = c.httpSrv.Shutdown(ctx)
		c.httpSrv = nil
	}

	c.running = false
	log.Println("[feishu] channel stopped")
	return nil
}

// Send 发送消息
func (c *FeishuChannel) Send(ctx context.Context, reply *Reply) error {
	if reply == nil {
		return fmt.Errorf("feishu: empty reply")
	}

	receiveIDType, receiveID := parseFeishuConversationID(reply.ConversationID)
	if receiveID == "" {
		return fmt.Errorf("feishu: empty conversation id")
	}

	// 先尝试以「引用回复」发送，保留话题上下文；失败则退回普通发送
	if reply.ReplyToMessageID != "" {
		if err := c.sendReplyToMessage(ctx, reply.ReplyToMessageID, reply); err == nil {
			return nil
		} else {
			log.Printf("[feishu] reply to message failed, fallback to normal send: %v", err)
		}
	}

	msgType, content := feishuMessageContent(reply)

	// 文本过长时分片发送，避免超过开放平台的单条消息上限
	chunks := splitText(content, feishuMaxTextLength)
	for _, chunk := range chunks {
		body := larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType(msgType).
			Content(chunk).
			Build()

		req := larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(receiveIDType).
			Body(body).
			Build()

		resp, err := c.api.Im.V1.Message.Create(ctx, req)
		if err != nil {
			return fmt.Errorf("feishu: send message failed: %w", err)
		}
		if !resp.Success() {
			return fmt.Errorf("feishu: send message rejected, code=%d msg=%s", resp.Code, resp.Msg)
		}
	}

	return nil
}

// sendReplyToMessage 以引用消息的方式回复
func (c *FeishuChannel) sendReplyToMessage(ctx context.Context, messageID string, reply *Reply) error {
	msgType, content := feishuMessageContent(reply)

	body := larkim.NewReplyMessageReqBodyBuilder().
		MsgType(msgType).
		Content(content).
		Build()

	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(body).
		Build()

	resp, err := c.api.Im.V1.Message.Reply(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// feishuMessageContent 组装消息类型与内容
//
// 纯文本走 text 消息；带 Markdown 时使用交互卡片（interactive），
// 以获得更好的排版效果（代码块、列表等）。
func feishuMessageContent(reply *Reply) (msgType string, content string) {
	if reply.Markdown != "" {
		card := map[string]any{
			"config": map[string]any{"wide_screen_mode": true},
			"elements": []map[string]any{
				{
					"tag":  "div",
					"text": map[string]any{"tag": "lark_md", "content": reply.Markdown},
				},
			},
		}
		if data, err := json.Marshal(card); err == nil {
			return "interactive", string(data)
		}
	}

	text := map[string]any{"text": reply.Text}
	if data, err := json.Marshal(text); err == nil {
		return "text", string(data)
	}
	return "text", `{"text":""}`
}

// parseFeishuConversationID 解析会话标识，返回 receive_id_type 与实际 ID
func parseFeishuConversationID(conversationID string) (receiveIDType, receiveID string) {
	if prefix, id, found := strings.Cut(conversationID, ":"); found {
		switch prefix {
		case "chat_id", "open_id", "user_id", "union_id", "email":
			return prefix, id
		}
	}
	// 无前缀时按 open_id 处理
	return "open_id", conversationID
}

// handleWebhook 处理 Webhook 事件订阅请求
//
// 校验流程：签名校验 → 解密 → URL 验证挑战 / 事件分发。
// URL 验证挑战与 token 校验由 SDK 的 dispatcher 统一处理。
func (c *FeishuChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if c.cfg.VerificationToken == "" || c.cfg.EncryptKey == "" {
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

	// Encrypt Key 存在时校验签名，防止伪造回调
	if c.cfg.EncryptKey != "" {
		if err := verifyFeishuSignature(r, body, c.cfg.EncryptKey); err != nil {
			log.Printf("[feishu] webhook signature verification failed: %v", err)
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	resp := c.handler.Handle(r.Context(), &larkevent.EventReq{
		Header:     r.Header,
		Body:       body,
		RequestURI: r.RequestURI,
	})

	// SDK 对校验失败统一返回 500，这里改写为 401：
	// 认证类失败不应表现为服务端故障，避免平台持续重推
	if resp.StatusCode == http.StatusInternalServerError {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(resp.Body)
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}

// verifyFeishuSignature 校验飞书回调签名
//
// 签名算法：SHA256(timestamp + nonce + encryptKey + body)
func verifyFeishuSignature(r *http.Request, body []byte, encryptKey string) error {
	timestamp := r.Header.Get("X-Lark-Request-Timestamp")
	nonce := r.Header.Get("X-Lark-Request-Nonce")
	signature := r.Header.Get("X-Lark-Signature")
	if timestamp == "" || nonce == "" {
		return fmt.Errorf("missing signature headers")
	}

	h := sha256.New()
	h.Write([]byte(timestamp))
	h.Write([]byte(nonce))
	h.Write([]byte(encryptKey))
	h.Write(body)
	expected := hex.EncodeToString(h.Sum(nil))

	if !strings.EqualFold(expected, signature) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// splitText 按最大长度切分文本
func splitText(text string, maxLen int) []string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return []string{text}
	}

	chunks := make([]string, 0, len(runes)/maxLen+1)
	for len(runes) > maxLen {
		chunks = append(chunks, string(runes[:maxLen]))
		runes = runes[maxLen:]
	}
	if len(runes) > 0 {
		chunks = append(chunks, string(runes))
	}
	return chunks
}

// derefString 安全解引用字符串指针
func derefString(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}
