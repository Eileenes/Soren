package channel

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/models"
)

// 路由相关常量
const (
	// routerHistoryLimit 每次请求携带的历史消息条数
	routerHistoryLimit = 20

	// routerReplyTimeout 单条消息的处理超时
	routerReplyTimeout = 90 * time.Second

	// routerSystemPrompt 系统提示词
	routerSystemPrompt = "你是 Soren，一个运行在用户桌面上的 AI 助手。" +
		"当前对话通过即时通讯渠道进行，回复应当简洁、直接、适合在聊天窗口中阅读。" +
		"除非用户明确要求，否则避免使用过度冗长的排版。"
)

// ConversationStore 路由所需的会话存储能力
//
// 抽象为接口而不是直接依赖 *repository.ConversationRepository，
// 便于测试以及后续替换存储实现。
type ConversationStore interface {
	// GetOrCreateConversation 按外部标识获取或创建对话
	GetOrCreateConversation(ctx context.Context, key, title string) (*models.Conversation, error)
	// GetMessages 获取对话的历史消息（按时间正序）
	GetMessages(ctx context.Context, conversationID uuid.UUID, limit int) ([]models.Message, error)
	// AddMessage 追加一条消息
	AddMessage(ctx context.Context, message *models.Message) error
}

// Router 消息路由
//
// 负责把各渠道收到的消息交给 LLM 处理，再把回复发回原渠道，
// 同时把对话写入存储，使渠道消息与桌面端界面共享同一份历史。
type Router struct {
	manager  *Manager
	store    ConversationStore
	provider llm.Provider
	options  *llm.Options

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
}

// NewRouter 创建消息路由
func NewRouter(manager *Manager, store ConversationStore, provider llm.Provider, options *llm.Options) *Router {
	if options == nil {
		options = &llm.Options{
			Model:       "gpt-4o-mini",
			Temperature: 0.7,
			MaxTokens:   2000,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Router{
		manager:  manager,
		store:    store,
		provider: provider,
		options:  options,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 为所有已注册的渠道启动消息消费协程
//
// 需要与渠道的生命周期解耦：渠道可能随时启停，消费协程常驻即可。
func (r *Router) Start() {
	for _, name := range r.manager.ChannelNames() {
		ch, err := r.manager.Get(name)
		if err != nil {
			continue
		}
		r.consume(ch)
	}
}

// Stop 停止所有消费协程
func (r *Router) Stop() {
	r.once.Do(func() {
		r.cancel()
	})
	r.wg.Wait()
}

// consume 启动单个渠道的消息消费协程
func (r *Router) consume(ch Channel) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		messages := ch.Messages()
		for {
			select {
			case <-r.ctx.Done():
				return
			case msg, ok := <-messages:
				if !ok {
					return
				}
				// 每条消息独立处理，避免慢请求阻塞后续消息
				r.handleAsync(ch, msg)
			}
		}
	}()
}

// handleAsync 异步处理单条消息
func (r *Router) handleAsync(ch Channel, msg *IncomingMessage) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		ctx, cancel := context.WithTimeout(r.ctx, routerReplyTimeout)
		defer cancel()

		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[router] panic while handling message from %s: %v", msg.Channel, rec)
			}
		}()

		if err := r.HandleMessage(ctx, ch, msg); err != nil {
			log.Printf("[router] handle message from %s failed: %v", msg.Channel, err)
		}
	}()
}

// HandleMessage 处理单条消息：调用 LLM 并回复渠道
func (r *Router) HandleMessage(ctx context.Context, ch Channel, msg *IncomingMessage) error {
	if msg == nil || strings.TrimSpace(msg.Text) == "" {
		return nil
	}

	// 定位该会话对应的对话记录
	conv, err := r.store.GetOrCreateConversation(ctx, conversationKey(msg), conversationTitle(msg))
	if err != nil {
		return err
	}

	// 保存用户消息
	userMsg := &models.Message{
		ConversationID: conv.ID,
		Role:           "user",
		Content:        msg.Text,
	}
	if err := r.store.AddMessage(ctx, userMsg); err != nil {
		return err
	}

	// 构建 LLM 消息：系统提示词 + 历史 + 当前消息
	history, err := r.store.GetMessages(ctx, conv.ID, routerHistoryLimit)
	if err != nil {
		return err
	}

	prompt := make([]llm.Message, 0, len(history)+2)
	prompt = append(prompt, llm.Message{Role: "system", Content: routerSystemPrompt})
	for _, m := range history {
		prompt = append(prompt, llm.Message{Role: m.Role, Content: m.Content})
	}

	resp, err := r.provider.Chat(ctx, prompt, r.options)
	if err != nil {
		return err
	}
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		return errors.New("empty response from LLM")
	}

	// 保存助手回复
	assistantMsg := &models.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        resp.Content,
	}
	if err := r.store.AddMessage(ctx, assistantMsg); err != nil {
		return err
	}

	return ch.Send(ctx, &Reply{
		ConversationID:   msg.ConversationID,
		ReplyTarget:      msg.ReplyTarget,
		Text:             resp.Content,
		Markdown:         formatMarkdown(ch.Name(), resp.Content),
		ReplyToMessageID: msg.MessageID,
	})
}

// conversationKey 生成对话的外部唯一标识
func conversationKey(msg *IncomingMessage) string {
	if msg.ConversationID == "" {
		return msg.Channel + ":default"
	}
	return msg.Channel + ":" + msg.ConversationID
}

// conversationTitle 生成对话标题
func conversationTitle(msg *IncomingMessage) string {
	title := "会话"
	switch msg.Channel {
	case FeishuChannelName:
		title = "飞书会话"
	case DingTalkChannelName:
		title = "钉钉会话"
	}
	if msg.SenderName != "" {
		title += " · " + msg.SenderName
	}
	return title
}

// formatMarkdown 按渠道特性决定是否使用 markdown 富文本
//
// 钉钉的 markdown 渲染稳定，始终启用；飞书使用交互卡片渲染 markdown，
// 仅在内容确实包含结构化排版时才启用，避免普通对话被塞进卡片中。
func formatMarkdown(channelName, content string) string {
	switch channelName {
	case DingTalkChannelName:
		return content
	case FeishuChannelName:
		if looksStructured(content) {
			return content
		}
	}
	return ""
}

// looksStructured 判断内容是否包含需要富文本渲染的结构
func looksStructured(content string) bool {
	return strings.Contains(content, "```") ||
		strings.Contains(content, "\n- ") ||
		strings.Contains(content, "\n* ") ||
		strings.Contains(content, "\n1. ") ||
		strings.Contains(content, "\n#")
}
