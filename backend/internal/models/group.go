package models

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GroupBotIDPrefix 群对话在 Conversation.BotID 上占用的保留命名空间。
//
// Bot ID 由服务端 uuid.NewString() 生成（agent.Service.SaveBot），
// 不可能以该前缀开头，因此群对话与单 Bot 对话可以共用一张 conversations 表。
const GroupBotIDPrefix = "group:"

// GroupBotID 返回群对话使用的 BotID
func GroupBotID(groupID uuid.UUID) string {
	return GroupBotIDPrefix + groupID.String()
}

// GroupChat 群聊模型
//
// 一轮 = 群内每个 Bot 各发言一次；成员数 × MaxRounds 即单次发言的 LLM 调用次数上限。
type GroupChat struct {
	ID        uuid.UUID `gorm:"type:varchar(36);primary_key" json:"id"`
	Title     string    `gorm:"type:varchar(255);not null" json:"title"`
	Topic     string    `gorm:"type:text" json:"topic"`
	Mode      string    `gorm:"type:varchar(20);not null;default:mention" json:"mode"`
	MaxRounds int       `gorm:"not null;default:3" json:"maxRounds"`
	// Pinned 群列表置顶开关；PinnedAt 是置顶时刻，决定置顶群之间的先后。
	// 两者都只由服务端维护：客户端只能改 Pinned，不能自己写 PinnedAt。
	Pinned   bool       `gorm:"not null;default:false" json:"pinned"`
	PinnedAt *time.Time `json:"pinnedAt,omitempty"`
	// LastMessageAt / LastMessagePreview 是冗余字段。列表接口在
	// SetMaxOpenConns(1) 的 SQLite 下无法对每个群再查一次对话详情，
	// 那会把串行写放大成 N+1 锁竞争。
	LastMessageAt      *time.Time `json:"lastMessageAt,omitempty"`
	LastMessagePreview string     `gorm:"type:varchar(200)" json:"lastMessagePreview,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// GroupMember 群成员
type GroupMember struct {
	GroupID  uuid.UUID `gorm:"type:varchar(36);primary_key" json:"groupId"`
	BotID    string    `gorm:"type:varchar(36);primary_key;index" json:"botId"`
	JoinedAt time.Time `json:"joinedAt"`
}

// GroupTurn 一次群发言的记录
//
// 它同时承担两个职责：幂等键的落点（RequestID 唯一），
// 以及同群并发互斥的锁（status = "running" 的行同一群只允许一条）。
type GroupTurn struct {
	ID        uuid.UUID `gorm:"type:varchar(36);primary_key" json:"id"`
	RequestID string    `gorm:"type:varchar(36);not null;uniqueIndex" json:"requestId"`
	GroupID   uuid.UUID `gorm:"type:varchar(36);index;not null" json:"groupId"`
	Mode      string    `gorm:"type:varchar(20);not null" json:"mode"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	// RequestFingerprint 规范化的提及名单与模型选项；旧记录留空时拒绝不确定的重放。
	RequestFingerprint string `gorm:"type:varchar(64)" json:"-"`
	// Status: running | done | cancelled | failed | interrupted
	Status     string    `gorm:"type:varchar(20);index;not null" json:"status"`
	MessageIDs string    `gorm:"type:text" json:"-"` // JSON 数组，本次产出的消息 ID
	Error      string    `gorm:"type:text" json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// SetMessageIDs 记录本次发言产出的消息 ID
func (t *GroupTurn) SetMessageIDs(ids []string) {
	if len(ids) == 0 {
		t.MessageIDs = ""
		return
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		t.MessageIDs = ""
		return
	}
	t.MessageIDs = string(encoded)
}

// DecodeMessageIDs 解析本次发言产出的消息 ID，顺序与写入时一致
func (t *GroupTurn) DecodeMessageIDs() []string {
	if strings.TrimSpace(t.MessageIDs) == "" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(t.MessageIDs), &ids); err != nil {
		return nil
	}
	return ids
}
