package channel

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// ErrNotSupported 渠道不支持该操作
var ErrNotSupported = errors.New("operation not supported by this channel")

// ErrChannelNotFound 渠道不存在或未注册
var ErrChannelNotFound = errors.New("channel not found")

// Channel 统一的消息渠道接口
//
// 飞书与钉钉都实现该接口，上层（路由、HTTP API）只依赖接口，
// 不关心具体平台的协议差异。启动/停止必须可重入：重复调用应是安全的。
type Channel interface {
	// Name 渠道名称，作为唯一标识，如 "feishu"、"dingtalk"
	Name() string

	// Start 启动渠道（建立长连接或开启 Webhook 监听）
	Start(ctx context.Context) error

	// Stop 停止渠道并释放资源
	Stop(ctx context.Context) error

	// Send 向平台发送消息
	Send(ctx context.Context, reply *Reply) error

	// Messages 返回接收到的消息流，由路由消费
	Messages() <-chan *IncomingMessage

	// Status 返回当前状态，用于 /api/channels 展示
	Status() Status
}

// Status 渠道运行状态
type Status struct {
	Name        string    `json:"name"`                // 渠道名称
	Type        string    `json:"type"`                // 渠道类型，如 feishu、dingtalk
	Enabled     bool      `json:"enabled"`             // 是否已启用
	Running     bool      `json:"running"`             // 是否正在运行
	Mode        string    `json:"mode"`                // 接入模式：websocket / webhook / stream
	Configured  bool      `json:"configured"`          // 配置是否完整
	Description string    `json:"description"`         // 说明信息
	LastError   string    `json:"lastError,omitempty"` // 最近一次错误
	StartedAt   time.Time `json:"startedAt,omitempty"` // 启动时间
}

// IncomingMessage 从平台收到的标准化消息
type IncomingMessage struct {
	Channel string `json:"channel"` // 来源渠道
	Type    string `json:"type"`    // 消息类型：text、markdown 等

	// ConversationID 渠道内的会话标识
	// 飞书为 chat_id（群）或 open_id（单聊）；钉钉为 conversationId
	ConversationID string `json:"conversationId"`

	// SenderID 发送者标识（飞书 open_id，钉钉 senderStaffId/senderId）
	SenderID   string `json:"senderId"`
	SenderName string `json:"senderName"`

	MessageID string `json:"messageId"` // 平台消息 ID
	Text      string `json:"text"`      // 文本内容（已去除 @机器人 前缀）
	Raw       any    `json:"-"`         // 平台原始消息体，供扩展使用

	// ReplyTarget 回复目标所需的平台特定信息
	// 飞书为空；钉钉为 sessionWebhook 等临时回复地址
	ReplyTarget string `json:"-"`

	ReceivedAt time.Time `json:"receivedAt"`
}

// Reply 待发送的回复
type Reply struct {
	// ConversationID 目标会话标识（与 IncomingMessage.ConversationID 对应）
	ConversationID string

	// ReceiverID 可选的目标接收者，飞书用于指定 receive_id_type
	ReceiverID string

	// ReplyTarget 平台特定的回复地址，如钉钉 sessionWebhook
	ReplyTarget string

	Text string // 纯文本内容
	// Markdown 为可选的结构化内容，渠道支持时优先按富文本发送
	Markdown string

	// ReplyToMessageID 需要以“引用回复”方式发送时填写
	ReplyToMessageID string
}

// Manager 渠道管理器，负责注册、启动、停止与状态查询
type Manager struct {
	mu       sync.RWMutex
	channels map[string]Channel
	status   map[string]*Status // 运行状态（运行时状态与渠道实例分离，便于重启）
}

// NewManager 创建渠道管理器
func NewManager() *Manager {
	return &Manager{
		channels: make(map[string]Channel),
		status:   make(map[string]*Status),
	}
}

// Register 注册渠道
func (m *Manager) Register(ch Channel, status Status) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.channels[ch.Name()] = ch
	s := status
	m.status[ch.Name()] = &s
}

// Get 获取渠道
func (m *Manager) Get(name string) (Channel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, ok := m.channels[name]
	if !ok {
		return nil, ErrChannelNotFound
	}
	return ch, nil
}

// ChannelNames 返回已注册的渠道名称，按名称排序保证输出稳定
func (m *Manager) ChannelNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Statuses 返回所有渠道状态
func (m *Manager) Statuses() []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.status))
	for name := range m.status {
		names = append(names, name)
	}
	sort.Strings(names)

	list := make([]Status, 0, len(names))
	for _, name := range names {
		list = append(list, *m.status[name])
	}
	return list
}

// Status 返回单个渠道状态
func (m *Manager) Status(name string) (Status, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.status[name]
	if !ok {
		return Status{}, ErrChannelNotFound
	}
	return *s, nil
}

// Enable 启用渠道（已运行时直接返回）
func (m *Manager) Enable(ctx context.Context, name string) error {
	m.mu.Lock()
	ch, ok := m.channels[name]
	if !ok {
		m.mu.Unlock()
		return ErrChannelNotFound
	}
	s := m.status[name]
	if s.Running {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	if err := ch.Start(ctx); err != nil {
		m.recordError(name, err)
		return err
	}

	m.mu.Lock()
	s = m.status[name]
	s.Enabled = true
	s.Running = true
	s.LastError = ""
	s.StartedAt = time.Now()
	m.mu.Unlock()
	return nil
}

// Disable 禁用渠道（未运行时直接返回）
func (m *Manager) Disable(ctx context.Context, name string) error {
	m.mu.Lock()
	ch, ok := m.channels[name]
	if !ok {
		m.mu.Unlock()
		return ErrChannelNotFound
	}
	s := m.status[name]
	if !s.Running {
		s.Enabled = false
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	if err := ch.Stop(ctx); err != nil {
		m.recordError(name, err)
		return err
	}

	m.mu.Lock()
	s = m.status[name]
	s.Enabled = false
	s.Running = false
	s.StartedAt = time.Time{}
	m.mu.Unlock()
	return nil
}

// StopAll 停止所有渠道，用于服务退出时清理
func (m *Manager) StopAll(ctx context.Context) {
	for _, name := range m.ChannelNames() {
		ch, err := m.Get(name)
		if err != nil {
			continue
		}
		if err := ch.Stop(ctx); err != nil {
			m.recordError(name, err)
		}
	}
}

// recordError 记录渠道最近一次错误
func (m *Manager) recordError(name string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.status[name]; ok {
		s.LastError = err.Error()
	}
}
