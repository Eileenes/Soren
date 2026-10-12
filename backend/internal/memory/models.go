package memory

import (
	"time"

	"gorm.io/gorm"
)

// MemoryType 记忆类型
type MemoryType string

const (
	// MemoryTypeConversation 对话记忆：用户说过的话、AI 的回复
	MemoryTypeConversation MemoryType = "conversation"
	// MemoryTypePreference 用户偏好：明确表达或推断出的倾向
	MemoryTypePreference MemoryType = "preference"
	// MemoryTypeTask 任务记录：完成的任务、待办事项、项目信息
	MemoryTypeTask MemoryType = "task"
	// MemoryTypeFact 事实知识：用户提供的个人信息、领域知识
	MemoryTypeFact MemoryType = "fact"
)

// ValidMemoryTypes 全部合法记忆类型
var ValidMemoryTypes = []MemoryType{
	MemoryTypeConversation,
	MemoryTypePreference,
	MemoryTypeTask,
	MemoryTypeFact,
}

// IsValidMemoryType 校验记忆类型
func IsValidMemoryType(t string) bool {
	for _, valid := range ValidMemoryTypes {
		if string(valid) == t {
			return true
		}
	}
	return false
}

// TagType 标签类型
type TagType string

const (
	// TagTypePreference 偏好标签
	TagTypePreference TagType = "preference"
	// TagTypeTopic 话题标签
	TagTypeTopic TagType = "topic"
	// TagTypeEntity 实体标签
	TagTypeEntity TagType = "entity"
	// TagTypeSkill 技能标签
	TagTypeSkill TagType = "skill"
)

// Memory 记忆模型（对应设计文档 memories 表）
//
// 说明：字段名与设计文档 3.1.1 一致；CreatedAt/DeletedAt 由 GORM 自动维护，
// 时间列统一使用 DATETIME，保证 SQLite 与 PostgreSQL 行为一致。
type Memory struct {
	ID       int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID   string `gorm:"type:varchar(64);not null;index:idx_memories_user_time,priority:1" json:"userId"`
	SessionID string `gorm:"type:varchar(64);index:idx_memories_session,priority:1" json:"sessionId"`

	// 记忆内容
	ContentType MemoryType `gorm:"type:varchar(32);not null;index:idx_memories_user_type_time,priority:2" json:"contentType"`
	Content     string     `gorm:"type:text;not null" json:"content"`
	Summary     string     `gorm:"type:text" json:"summary"`

	// 元数据
	ImportanceScore int        `gorm:"default:50;index:idx_memories_importance,priority:2" json:"importanceScore"`
	AccessCount     int        `gorm:"default:0" json:"accessCount"`
	LastAccessedAt  *time.Time `json:"lastAccessedAt"`

	// 时序信息
	CreatedAt time.Time  `gorm:"index:idx_memories_user_time,priority:2,sort:desc" json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	ExpiresAt *time.Time `json:"expiresAt"`

	// 上下文关联
	ParentMemoryID   *int64 `json:"parentMemoryId,omitempty"`
	ConversationTurn int    `json:"conversationTurn"`

	// 元数据标记：是否被用户明确陈述（影响重要性评分）
	IsExplicitlyStated bool `gorm:"default:false" json:"isExplicitlyStated"`

	// 软删除
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联标签（查询时按需预加载）
	Tags []Tag `gorm:"many2many:memory_tags;" json:"tags,omitempty"`
}

// TableName 指定表名
func (Memory) TableName() string {
	return "memories"
}

// Tag 标签模型（对应设计文档 tags 表）
type Tag struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    string    `gorm:"type:varchar(64);not null;uniqueIndex:uniq_user_tag,priority:1" json:"userId"`
	Name      string    `gorm:"type:varchar(64);not null;uniqueIndex:uniq_user_tag,priority:2" json:"name"`
	TagType   TagType   `gorm:"type:varchar(32);not null;index:idx_tags_user_type,priority:2" json:"tagType"`
	Metadata  string    `gorm:"type:text" json:"metadata,omitempty"` // JSON 字符串，兼容 SQLite
	CreatedAt time.Time `json:"createdAt"`
}

// TableName 指定表名
func (Tag) TableName() string {
	return "tags"
}

// MemoryTag 记忆-标签关联（对应设计文档 memory_tags 表）
type MemoryTag struct {
	MemoryID       int64     `gorm:"primaryKey;index:idx_memory_tags_memory" json:"memoryId"`
	TagID          int       `gorm:"primaryKey;index:idx_memory_tags_tag,priority:1" json:"tagId"`
	RelevanceScore int       `gorm:"default:100;index:idx_memory_tags_tag,priority:2,sort:desc" json:"relevanceScore"`
	CreatedAt      time.Time `json:"createdAt"`
}

// TableName 指定表名
func (MemoryTag) TableName() string {
	return "memory_tags"
}

// UserPreference 用户偏好表（对应设计文档 3.1.4）
type UserPreference struct {
	ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          string    `gorm:"type:varchar(64);not null;uniqueIndex:uniq_user_preference,priority:1" json:"userId"`
	PreferenceKey   string    `gorm:"type:varchar(128);not null;uniqueIndex:uniq_user_preference,priority:2" json:"preferenceKey"`
	PreferenceValue string    `gorm:"type:text;not null" json:"preferenceValue"`
	ConfidenceScore int       `gorm:"default:50" json:"confidenceScore"`
	EvidenceCount   int       `gorm:"default:1" json:"evidenceCount"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	LastConfirmedAt *time.Time `json:"lastConfirmedAt"`
}

// TableName 指定表名
func (UserPreference) TableName() string {
	return "user_preferences"
}

// AllModels 返回需要自动迁移的全部记忆相关模型
func AllModels() []interface{} {
	return []interface{}{
		&Memory{},
		&Tag{},
		&MemoryTag{},
		&UserPreference{},
	}
}

// RecallRequest 召回请求
type RecallRequest struct {
	UserID         string
	SessionID      string
	CurrentMessage string
	MaxResults     int
	TimeWindow     *time.Duration
	RequiredTags   []string
}

// RecallResult 召回结果
type RecallResult struct {
	Memories      []*Memory `json:"memories"`
	TotalRecalled int       `json:"totalRecalled"`
	LatencyMS     int64     `json:"latencyMs"`
}

// ListOptions 记忆列表查询条件
type ListOptions struct {
	UserID      string
	ContentType string // 为空表示不过滤
	SessionID   string
	Keyword     string
	MinImportance int
	Offset      int
	Limit       int
}

// Stats 记忆统计
type Stats struct {
	Total              int64            `json:"total"`
	ByType             map[string]int64 `json:"byType"`
	HighImportance     int64            `json:"highImportance"`
	AccessedRecently   int64            `json:"accessedRecently"`
	TotalTags          int64            `json:"totalTags"`
	OldestCreatedAt    *time.Time       `json:"oldestCreatedAt,omitempty"`
	NewestCreatedAt    *time.Time       `json:"newestCreatedAt,omitempty"`
}
