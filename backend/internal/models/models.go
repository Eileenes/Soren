package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Message 消息模型
//
// 注意：UUID 与时间戳统一由 BeforeCreate 钩子生成，不依赖数据库函数，
// 以保证同一套模型可以同时跑在 SQLite（桌面端默认）和 PostgreSQL 上。
type Message struct {
	ToolRunID      string    `gorm:"index" json:"toolRunId,omitempty"`
	ID             uuid.UUID `gorm:"type:varchar(36);primary_key" json:"id"`
	ConversationID uuid.UUID `gorm:"type:varchar(36);not null;index" json:"conversationId"`
	Role           string    `gorm:"type:varchar(20);not null" json:"role"` // user, assistant, system
	Content        string    `gorm:"type:text;not null" json:"content"`
	Timestamp      time.Time `gorm:"not null" json:"timestamp"`
	// SpeakerBotID 群聊中该条消息的发言 Bot。群聊一次发言会产生多条 assistant
	// 消息而 X-Bot-ID 请求头只有一个值，身份只能随消息落库；单聊留空。
	SpeakerBotID string `gorm:"type:varchar(36);index" json:"speakerBotId,omitempty"`
	// SpeakerName 冗余存储发言人名称。运行时 join Bot 表会让历史消息在用户
	// 改名或删除 Bot 之后错认身份。
	SpeakerName string    `gorm:"type:varchar(100)" json:"speakerName,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Conversation 对话模型
type Conversation struct {
	BotID string    `gorm:"not null;default:soren;index" json:"botId"`
	ID    uuid.UUID `gorm:"type:varchar(36);primary_key" json:"id"`
	Title string    `gorm:"type:varchar(255);not null" json:"title"`
	// ExternalKey 外部渠道会话标识（如 feishu:chat_id:oc_xxx）
	// 桌面端本地对话为空字符串，渠道会话据此复用同一份对话历史
	ExternalKey string    `gorm:"type:varchar(255);index" json:"externalKey,omitempty"`
	Messages    []Message `gorm:"foreignKey:ConversationID" json:"messages"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// BeforeCreate 创建前钩子
func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// BeforeCreate 创建前钩子
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}
	return nil
}

// LLMConfig LLM配置模型
type LLMConfig struct {
	ID          uint      `gorm:"primary_key" json:"id"`
	Provider    string    `gorm:"type:varchar(20);not null" json:"provider"` // openai, anthropic
	Model       string    `gorm:"type:varchar(100);not null" json:"model"`
	APIKey      string    `gorm:"type:varchar(255)" json:"apiKey,omitempty"`
	BaseURL     string    `gorm:"type:varchar(255)" json:"baseURL,omitempty"`
	Temperature float32   `gorm:"type:decimal(3,2);default:0.7" json:"temperature"`
	MaxTokens   int       `gorm:"default:2000" json:"maxTokens"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
