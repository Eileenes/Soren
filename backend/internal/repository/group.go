package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/soren/backend/internal/models"
	"gorm.io/gorm"
)

// ErrTurnRunning 同群已有进行中的讨论
var ErrTurnRunning = errors.New("该群已有讨论正在进行")

// previewLimit 群列表预览的字符上限，与 GroupChat.LastMessagePreview 列宽一致
const previewLimit = 200

// GroupRepository 群聊仓储
type GroupRepository struct {
	db *gorm.DB
}

// NewGroupRepository 创建群聊仓储
func NewGroupRepository(db *gorm.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

// Create 创建群与成员
func (r *GroupRepository) Create(ctx context.Context, g *models.GroupChat, botIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(g).Error; err != nil {
			return err
		}
		rows := memberRows(g.ID, botIDs)
		return tx.Create(&rows).Error
	})
}

// List 获取全部群聊
//
// 排序分两段：置顶群在前、按置顶时刻倒序；未置顶群在后、按更新时间倒序。
//
// 两段各自补一个 id 兜底，否则 pinned_at（或 updated_at）相同的行顺序由存储引擎
// 自行决定，同一份数据两次相同请求可能给出不同列表——列表项在界面上"跳位"就是这么来的。
// 兜底必须用 CASE 分段：若不分，updated_at 会插进置顶段中间，一个置顶群收到新消息
// 就会越过另一个置顶群，置顶顺序随聊天活动漂移。
func (r *GroupRepository) List(ctx context.Context) ([]models.GroupChat, error) {
	groups := []models.GroupChat{}
	err := r.db.WithContext(ctx).
		Order("pinned DESC, " +
			"CASE WHEN pinned THEN pinned_at END DESC, " +
			"CASE WHEN pinned THEN id END ASC, " +
			"updated_at DESC, id ASC").
		Find(&groups).Error
	return groups, err
}

// GetByID 获取单个群聊
func (r *GroupRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GroupChat, error) {
	var g models.GroupChat
	if err := r.db.WithContext(ctx).First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// Update 更新群聊
//
// 用 map 而不是 struct：struct 版会跳过零值，取消置顶时 pinned_at = nil 就写不进去。
func (r *GroupRepository) Update(ctx context.Context, g *models.GroupChat) error {
	return r.db.WithContext(ctx).Model(&models.GroupChat{}).
		Where("id = ?", g.ID).
		Updates(map[string]any{
			"title":      g.Title,
			"topic":      g.Topic,
			"mode":       g.Mode,
			"max_rounds": g.MaxRounds,
			"pinned":     g.Pinned,
			"pinned_at":  g.PinnedAt,
		}).Error
}

// ReplaceMembers 用给定名单整体替换群成员
func (r *GroupRepository) ReplaceMembers(ctx context.Context, groupID uuid.UUID, botIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&models.GroupMember{}).Error; err != nil {
			return err
		}
		rows := memberRows(groupID, botIDs)
		return tx.Create(&rows).Error
	})
}

// MemberIDs 获取群成员 Bot ID（按加入顺序）
func (r *GroupRepository) MemberIDs(ctx context.Context, groupID uuid.UUID) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&models.GroupMember{}).
		Where("group_id = ?", groupID).
		Order("joined_at ASC, bot_id ASC").
		Pluck("bot_id", &ids).Error
	return ids, err
}

// MemberIDsByGroups 一次取出所有群的成员，避免列表接口逐群查询
func (r *GroupRepository) MemberIDsByGroups(ctx context.Context) (map[uuid.UUID][]string, error) {
	var rows []models.GroupMember
	if err := r.db.WithContext(ctx).Order("joined_at ASC, bot_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]string{}
	for _, row := range rows {
		out[row.GroupID] = append(out[row.GroupID], row.BotID)
	}
	return out, nil
}

// ClearDependents 清掉群成员与发言记录
//
// 与删除群行分开，是为了严格遵守"先中止编排器、再清对话、最后删群行"的顺序：
// 中途失败时宁可留下一个历史被清空的群，也不要留下查不到主人的孤儿对话。
func (r *GroupRepository) ClearDependents(ctx context.Context, groupID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&models.GroupMember{}).Error; err != nil {
			return err
		}
		return tx.Where("group_id = ?", groupID).Delete(&models.GroupTurn{}).Error
	})
}

// Delete 删除群
//
// 对话与消息由 ConversationRepository.DeleteConversationsForBot 单独清理，
// Bot 的私有记忆一律保留。
func (r *GroupRepository) Delete(ctx context.Context, groupID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.GroupChat{}, "id = ?", groupID).Error
}

// AddMessage 写入群消息并刷新群列表预览
//
// 必须在单个事务内直接操作 tx：外层再调 ConversationRepository.AddMessage
// 会因后者自带事务而向 SetMaxOpenConns(1) 的连接池借第二条连接，直接挂死。
func (r *GroupRepository) AddMessage(ctx context.Context, message *models.Message, groupID uuid.UUID) error {
	now := time.Now()
	preview := groupPreview(message)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Conversation{}).Where("id = ?", message.ConversationID).
			Update("updated_at", now).Error; err != nil {
			return err
		}
		return tx.Model(&models.GroupChat{}).Where("id = ?", groupID).
			Updates(map[string]any{
				"last_message_at":      now,
				"last_message_preview": preview,
			}).Error
	})
}

// GetTurnByRequestID 按幂等键查询历史发言
func (r *GroupRepository) GetTurnByRequestID(ctx context.Context, requestID string) (*models.GroupTurn, error) {
	var turn models.GroupTurn
	if err := r.db.WithContext(ctx).First(&turn, "request_id = ?", requestID).Error; err != nil {
		return nil, err
	}
	return &turn, nil
}

// LatestTurn 返回最近一轮的状态，供页面恢复及轮询使用。
func (r *GroupRepository) LatestTurn(ctx context.Context, groupID uuid.UUID) (*models.GroupTurn, error) {
	var turn models.GroupTurn
	err := r.db.WithContext(ctx).Where("group_id = ?", groupID).
		Order("created_at DESC, id DESC").Take(&turn).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &turn, nil
}

// BeginTurn 占用同群的发言名额并创建记录
//
// 检查与插入必须同处一个事务：先查后插会留下 TOCTOU 窗口，两个并发请求
// 都能通过检查。SQLite 单连接下事务天然串行，检查因此是可靠的。
func (r *GroupRepository) BeginTurn(ctx context.Context, turn *models.GroupTurn) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var running int64
		if err := tx.Model(&models.GroupTurn{}).
			Where("group_id = ? AND status = ?", turn.GroupID, "running").
			Count(&running).Error; err != nil {
			return err
		}
		if running > 0 {
			return ErrTurnRunning
		}
		return tx.Create(turn).Error
	})
}

// FinishTurn 结束一次发言并写回产出
func (r *GroupRepository) FinishTurn(ctx context.Context, id uuid.UUID, status, failure string, messageIDs []string) error {
	turn := models.GroupTurn{ID: id}
	turn.SetMessageIDs(messageIDs)
	return r.db.WithContext(ctx).Model(&models.GroupTurn{}).Where("id = ?", id).
		Updates(map[string]any{
			"status":      status,
			"error":       failure,
			"message_ids": turn.MessageIDs,
		}).Error
}

// RunningTurns 返回所有处于运行中的发言（用于进程启动时的孤儿收敛）
func (r *GroupRepository) RunningTurns(ctx context.Context) ([]models.GroupTurn, error) {
	turns := []models.GroupTurn{}
	err := r.db.WithContext(ctx).Where("status = ?", "running").Find(&turns).Error
	return turns, err
}

// MarkTurnInterrupted 把一次运行中的发言收敛为 interrupted
func (r *GroupRepository) MarkTurnInterrupted(ctx context.Context, id uuid.UUID, reason string) error {
	return r.db.WithContext(ctx).Model(&models.GroupTurn{}).Where("id = ?", id).
		Updates(map[string]any{"status": "interrupted", "error": reason}).Error
}

// MessagesByIDs 按给定顺序取回消息
func (r *GroupRepository) MessagesByIDs(ctx context.Context, ids []string) ([]models.Message, error) {
	out := []models.Message{}
	if len(ids) == 0 {
		return out, nil
	}
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if value, err := uuid.Parse(id); err == nil {
			parsed = append(parsed, value)
		}
	}
	if len(parsed) == 0 {
		return out, nil
	}
	var rows []models.Message
	if err := r.db.WithContext(ctx).Where("id IN ?", parsed).Find(&rows).Error; err != nil {
		return nil, err
	}
	index := make(map[uuid.UUID]models.Message, len(rows))
	for _, row := range rows {
		index[row.ID] = row
	}
	for _, id := range parsed {
		if row, ok := index[id]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

// MessageCount 统计对话已有消息数，用于推导记忆的轮次编号
func (r *GroupRepository) MessageCount(ctx context.Context, conversationID uuid.UUID) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Message{}).
		Where("conversation_id = ?", conversationID).Count(&count).Error
	return int(count), err
}

// memberRows 构造成员记录
//
// JoinedAt 逐条递增 1 微秒，用来固定群成员的顺序：列表头像叠放、点名的回复顺序
// 都按加入顺序走，才与用户挑选 Bot 的顺序一致。
func memberRows(groupID uuid.UUID, botIDs []string) []models.GroupMember {
	base := time.Now()
	rows := make([]models.GroupMember, 0, len(botIDs))
	for i, id := range botIDs {
		rows = append(rows, models.GroupMember{
			GroupID:  groupID,
			BotID:    id,
			JoinedAt: base.Add(time.Duration(i) * time.Microsecond),
		})
	}
	return rows
}

// groupPreview 生成群列表预览：发言人名：内容
func groupPreview(message *models.Message) string {
	content := strings.TrimSpace(message.Content)
	if speaker := strings.TrimSpace(message.SpeakerName); speaker != "" {
		content = speaker + "：" + content
	}
	runes := []rune(content)
	if len(runes) > previewLimit {
		return string(runes[:previewLimit])
	}
	return content
}
