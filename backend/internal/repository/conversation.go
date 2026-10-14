package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/soren/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConversationRepository 对话仓储
type ConversationRepository struct {
	db *gorm.DB
}

// NewConversationRepository 创建对话仓储
func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

// Create 创建对话
func (r *ConversationRepository) Create(ctx context.Context, conv *models.Conversation) error {
	return r.db.WithContext(ctx).Create(conv).Error
}

// GetByID 根据ID获取对话
func (r *ConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Conversation, error) {
	var conv models.Conversation
	err := r.db.WithContext(ctx).Preload("Messages").First(&conv, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

// GetAll 获取所有对话
func (r *ConversationRepository) GetAll(ctx context.Context) ([]models.Conversation, error) {
	var convs []models.Conversation
	err := r.db.WithContext(ctx).Order("updated_at DESC").Find(&convs).Error
	return convs, err
}

// Update 更新对话
func (r *ConversationRepository) Update(ctx context.Context, conv *models.Conversation) error {
	return r.db.WithContext(ctx).Save(conv).Error
}

// Delete 删除对话
func (r *ConversationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Messages reference the conversation through a foreign key. Delete both
	// in one transaction, while deliberately retaining independent memories.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", id).Delete(&models.Message{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Conversation{}, "id = ?", id).Error
	})
}

// DeleteConversationsForBot 删除某个 Bot 名下的全部对话及其消息
//
// 消息与对话之间没有数据库级联约束（AutoMigrate 不生成 OnDelete:CASCADE），
// 清理必须显式完成。这里不复用单条 Delete：它自带事务，套在外层事务里
// 会向单连接池再借一条连接并把请求挂死。
func (r *ConversationRepository) DeleteConversationsForBot(ctx context.Context, bot string) (int, error) {
	var ids []uuid.UUID
	if err := r.db.WithContext(ctx).Model(&models.Conversation{}).
		Where("bot_id = ?", bot).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id IN ?", ids).Delete(&models.Message{}).Error; err != nil {
			return err
		}
		return tx.Where("bot_id = ?", bot).Delete(&models.Conversation{}).Error
	}); err != nil {
		return 0, err
	}
	return len(ids), nil
}

// AddMessage 添加消息
func (r *ConversationRepository) AddMessage(ctx context.Context, message *models.Message) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		return tx.Model(&models.Conversation{}).Where("id = ?", message.ConversationID).Update("updated_at", time.Now()).Error
	})
}

// GetOrCreateConversation 按外部渠道标识获取或创建对话
//
// 用于消息渠道（飞书、钉钉）：同一个平台会话始终映射到同一条对话，
// 这样渠道消息与桌面端界面共享同一份历史记录。
func (r *ConversationRepository) GetOrCreateConversation(ctx context.Context, key, title string) (*models.Conversation, error) {
	var conv models.Conversation
	// 首次进入某个会话时查不到记录属于预期情况，用静默 logger 避免误报错误日志
	err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: r.db.Logger.LogMode(logger.Silent)}).
		Where("external_key = ?", key).
		First(&conv).Error
	if err == nil {
		return &conv, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	conv = models.Conversation{
		Title:       title,
		ExternalKey: key,
	}
	if err := r.db.WithContext(ctx).Create(&conv).Error; err != nil {
		// 并发场景下可能已被其他协程创建，回查一次
		var existing models.Conversation
		if findErr := r.db.WithContext(ctx).Where("external_key = ?", key).First(&existing).Error; findErr == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &conv, nil
}

// GetMessages 获取对话的历史消息（按时间正序）
func (r *ConversationRepository) GetMessages(ctx context.Context, conversationID uuid.UUID, limit int) ([]models.Message, error) {
	var messages []models.Message

	// 取最近 limit 条，再反转为正序，保证长对话不会把上下文窗口撑满
	query := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("timestamp DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&messages).Error; err != nil {
		return nil, err
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// GetByExternalKey 按外部渠道标识获取对话
func (r *ConversationRepository) GetByExternalKey(ctx context.Context, key string) (*models.Conversation, error) {
	var conv models.Conversation
	err := r.db.WithContext(ctx).Preload("Messages").Where("external_key = ?", key).First(&conv).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *ConversationRepository) GetAllForBot(ctx context.Context, bot string) ([]models.Conversation, error) {
	convs := []models.Conversation{}
	err := r.db.WithContext(ctx).Where("bot_id = ?", bot).Order("updated_at DESC").Find(&convs).Error
	return convs, err
}
func (r *ConversationRepository) GetByIDForBot(ctx context.Context, id uuid.UUID, bot string) (*models.Conversation, error) {
	var conv models.Conversation
	err := r.db.WithContext(ctx).Preload("Messages", func(db *gorm.DB) *gorm.DB { return db.Order("timestamp ASC") }).First(&conv, "id = ? AND bot_id = ?", id, bot).Error
	return &conv, err
}
