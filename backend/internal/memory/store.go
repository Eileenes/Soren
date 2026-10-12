package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ErrMemoryNotFound 记忆不存在
var ErrMemoryNotFound = errors.New("memory not found")

// Store 记忆存储接口
//
// 设计文档 6.2 定义了 MemoryStore / TagStore 两个接口，
// 这里合并为统一的 Store，由同一份实现（GORM）同时支撑 SQLite 与 PostgreSQL。
type Store interface {
	// WriteMemory 写入记忆
	WriteMemory(ctx context.Context, memory *Memory) error
	// WriteBatch 批量写入
	WriteBatch(ctx context.Context, memories []*Memory) error
	// GetMemory 读取单条记忆
	GetMemory(ctx context.Context, memoryID int64) (*Memory, error)
	// UpdateMemory 更新记忆
	UpdateMemory(ctx context.Context, memory *Memory) error
	// DeleteMemory 软删除记忆
	DeleteMemory(ctx context.Context, memoryID int64) error
	// IncrementAccessCount 增加访问计数并刷新最近访问时间
	IncrementAccessCount(ctx context.Context, memoryID int64) error

	// GetOrCreateTag 创建或获取标签
	GetOrCreateTag(ctx context.Context, userID, name string, tagType TagType) (*Tag, error)
	// AddTagToMemory 为记忆添加标签
	AddTagToMemory(ctx context.Context, memoryID int64, tagID int) error
	// GetMemoryTags 获取记忆的所有标签
	GetMemoryTags(ctx context.Context, memoryID int64) ([]Tag, error)
	// FindMemoriesByTag 根据标签查找记忆
	FindMemoriesByTag(ctx context.Context, userID string, tagNames []string, limit int) ([]*Memory, error)

	// List 分页列出记忆
	List(ctx context.Context, opts ListOptions) ([]*Memory, int64, error)
	// Search 关键词搜索记忆
	Search(ctx context.Context, userID, keyword string, limit int) ([]*Memory, error)
	// Stats 记忆统计
	Stats(ctx context.Context, userID string) (*Stats, error)

	// RecallByTimeWindow 时序召回
	RecallByTimeWindow(ctx context.Context, userID string, since time.Time, limit int) ([]*Memory, error)
	// RecallBySession 会话召回
	RecallBySession(ctx context.Context, userID, sessionID string, limit int) ([]*Memory, error)
	// RecallByImportance 高重要性召回
	RecallByImportance(ctx context.Context, userID string, minScore, limit int) ([]*Memory, error)
	// CountByUser 统计用户记忆总数
	CountByUser(ctx context.Context, userID string) (int64, error)
}

// GormStore 基于 GORM 的记忆存储实现
//
// 同一份代码同时支持 SQLite（桌面端默认）与 PostgreSQL（可选升级路径），
// 所有 SQL 均使用参数占位符且不依赖数据库特有函数。
type GormStore struct {
	db *gorm.DB
}

// NewGormStore 创建 GORM 存储
func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

// Migrate 自动迁移记忆相关表
func (s *GormStore) Migrate() error {
	return s.db.AutoMigrate(AllModels()...)
}

// WriteMemory 写入记忆
func (s *GormStore) WriteMemory(ctx context.Context, memory *Memory) error {
	if memory == nil {
		return errors.New("memory is nil")
	}
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = time.Now()
	}
	if memory.ContentType == "" {
		memory.ContentType = MemoryTypeConversation
	}
	if memory.ImportanceScore == 0 {
		memory.ImportanceScore = 50
	}

	return s.db.WithContext(ctx).Create(memory).Error
}

// WriteBatch 批量写入
func (s *GormStore) WriteBatch(ctx context.Context, memories []*Memory) error {
	if len(memories) == 0 {
		return nil
	}

	now := time.Now()
	for _, m := range memories {
		if m.CreatedAt.IsZero() {
			m.CreatedAt = now
		}
		if m.ContentType == "" {
			m.ContentType = MemoryTypeConversation
		}
	}

	// SQLite 与 PostgreSQL 都支持批量插入，用事务包一层保证原子性
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.CreateInBatches(memories, 50).Error
	})
}

// GetMemory 读取单条记忆
func (s *GormStore) GetMemory(ctx context.Context, memoryID int64) (*Memory, error) {
	var memory Memory
	err := s.db.WithContext(ctx).Preload("Tags").First(&memory, memoryID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMemoryNotFound
	}
	if err != nil {
		return nil, err
	}
	return &memory, nil
}

// UpdateMemory 更新记忆
func (s *GormStore) UpdateMemory(ctx context.Context, memory *Memory) error {
	if memory == nil || memory.ID == 0 {
		return errors.New("memory or memory ID is empty")
	}
	// 使用 Omit 避免误更新关联与创建时间
	return s.db.WithContext(ctx).
		Omit("Tags", "CreatedAt").
		Model(&Memory{ID: memory.ID}).
		Select("content", "summary", "content_type", "importance_score",
			"access_count", "last_accessed_at", "expires_at", "session_id").
		Updates(memory).Error
}

// DeleteMemory 软删除记忆
func (s *GormStore) DeleteMemory(ctx context.Context, memoryID int64) error {
	result := s.db.WithContext(ctx).Delete(&Memory{}, memoryID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMemoryNotFound
	}
	return nil
}

// IncrementAccessCount 增加访问计数并刷新最近访问时间
func (s *GormStore) IncrementAccessCount(ctx context.Context, memoryID int64) error {
	now := time.Now()
	return s.db.WithContext(ctx).
		Model(&Memory{}).
		Where("id = ?", memoryID).
		Updates(map[string]interface{}{
			"access_count":     gorm.Expr("access_count + 1"),
			"last_accessed_at": now,
			"updated_at":       now,
		}).Error
}

// GetOrCreateTag 创建或获取标签
func (s *GormStore) GetOrCreateTag(ctx context.Context, userID, name string, tagType TagType) (*Tag, error) {
	name = strings.TrimSpace(name)
	if userID == "" || name == "" {
		return nil, errors.New("user ID and tag name are required")
	}
	if tagType == "" {
		tagType = TagTypeTopic
	}

	var tag Tag
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND name = ?", userID, name).
		First(&tag).Error
	if err == nil {
		return &tag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	tag = Tag{
		UserID:    userID,
		Name:      name,
		TagType:   tagType,
		CreatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&tag).Error; err != nil {
		// 并发场景下可能已被其他协程插入，回查一次
		if lookupErr := s.db.WithContext(ctx).
			Where("user_id = ? AND name = ?", userID, name).
			First(&tag).Error; lookupErr == nil {
			return &tag, nil
		}
		return nil, err
	}
	return &tag, nil
}

// AddTagToMemory 为记忆添加标签
func (s *GormStore) AddTagToMemory(ctx context.Context, memoryID int64, tagID int) error {
	return s.AddTagToMemoryWithRelevance(ctx, memoryID, tagID, 100)
}

// AddTagToMemoryWithRelevance 为记忆添加标签并指定相关性分数
func (s *GormStore) AddTagToMemoryWithRelevance(ctx context.Context, memoryID int64, tagID int, relevance int) error {
	if relevance < 0 {
		relevance = 0
	}
	if relevance > 100 {
		relevance = 100
	}

	link := MemoryTag{
		MemoryID:       memoryID,
		TagID:          tagID,
		RelevanceScore: relevance,
		CreatedAt:      time.Now(),
	}

	// 主键冲突时更新相关性分数，保证重复打标幂等
	return s.db.WithContext(ctx).
		Where("memory_id = ? AND tag_id = ?", memoryID, tagID).
		Assign(map[string]interface{}{"relevance_score": relevance}).
		FirstOrCreate(&link).Error
}

// GetMemoryTags 获取记忆的所有标签
func (s *GormStore) GetMemoryTags(ctx context.Context, memoryID int64) ([]Tag, error) {
	var tags []Tag
	err := s.db.WithContext(ctx).
		Model(&Tag{}).
		Joins("JOIN memory_tags ON memory_tags.tag_id = tags.id").
		Where("memory_tags.memory_id = ?", memoryID).
		Order("memory_tags.relevance_score DESC").
		Find(&tags).Error
	return tags, err
}

// FindMemoriesByTag 根据标签查找记忆
func (s *GormStore) FindMemoriesByTag(ctx context.Context, userID string, tagNames []string, limit int) ([]*Memory, error) {
	if len(tagNames) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	var memories []*Memory
	err := s.db.WithContext(ctx).
		Model(&Memory{}).
		Distinct("memories.*").
		Joins("JOIN memory_tags ON memory_tags.memory_id = memories.id").
		Joins("JOIN tags ON tags.id = memory_tags.tag_id").
		Where("memories.user_id = ?", userID).
		Where("tags.name IN ?", tagNames).
		Order("memories.importance_score DESC, memories.created_at DESC").
		Limit(limit).
		Find(&memories).Error
	if err != nil {
		return nil, err
	}
	return memories, nil
}

// List 分页列出记忆
func (s *GormStore) List(ctx context.Context, opts ListOptions) ([]*Memory, int64, error) {
	query := s.db.WithContext(ctx).Model(&Memory{})
	query = s.applyListFilters(query, opts)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	var memories []*Memory
	err := query.
		Preload("Tags").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&memories).Error
	if err != nil {
		return nil, 0, err
	}
	return memories, total, nil
}

// applyListFilters 组装列表过滤条件
func (s *GormStore) applyListFilters(query *gorm.DB, opts ListOptions) *gorm.DB {
	if opts.UserID != "" {
		query = query.Where("user_id = ?", opts.UserID)
	}
	if opts.ContentType != "" {
		query = query.Where("content_type = ?", opts.ContentType)
	}
	if opts.SessionID != "" {
		query = query.Where("session_id = ?", opts.SessionID)
	}
	if opts.MinImportance > 0 {
		query = query.Where("importance_score >= ?", opts.MinImportance)
	}
	if opts.Keyword != "" {
		like := "%" + opts.Keyword + "%"
		query = query.Where("(content LIKE ? OR summary LIKE ?)", like, like)
	}
	return query
}

// Search 关键词搜索记忆
func (s *GormStore) Search(ctx context.Context, userID, keyword string, limit int) ([]*Memory, error) {
	if limit <= 0 {
		limit = 20
	}

	query := s.db.WithContext(ctx).Model(&Memory{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(content LIKE ? OR summary LIKE ?)", like, like)
	}

	var memories []*Memory
	err := query.
		Preload("Tags").
		Order("importance_score DESC, created_at DESC").
		Limit(limit).
		Find(&memories).Error
	if err != nil {
		return nil, err
	}
	return memories, nil
}

// Stats 记忆统计
func (s *GormStore) Stats(ctx context.Context, userID string) (*Stats, error) {
	stats := &Stats{
		ByType: make(map[string]int64),
	}

	base := func() *gorm.DB {
		query := s.db.WithContext(ctx).Model(&Memory{})
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		return query
	}

	if err := base().Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	// 按类型聚合
	type typeCount struct {
		ContentType string
		Total       int64
	}
	var counts []typeCount
	if err := base().
		Select("content_type, COUNT(*) AS total").
		Group("content_type").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	for _, c := range counts {
		stats.ByType[c.ContentType] = c.Total
	}

	if err := base().Where("importance_score >= ?", 70).Count(&stats.HighImportance).Error; err != nil {
		return nil, err
	}

	recent := time.Now().AddDate(0, 0, -7)
	if err := base().Where("last_accessed_at >= ?", recent).Count(&stats.AccessedRecently).Error; err != nil {
		return nil, err
	}

	tagQuery := s.db.WithContext(ctx).Model(&Tag{})
	if userID != "" {
		tagQuery = tagQuery.Where("user_id = ?", userID)
	}
	if err := tagQuery.Count(&stats.TotalTags).Error; err != nil {
		return nil, err
	}

	// 最早/最新记忆时间
	//
	// 说明：SQLite 对 DATETIME 列执行 MIN/MAX 会返回字符串，
	// PostgreSQL 则返回时间类型，因此统一按字符串扫描后再解析。
	var bounds []struct {
		Oldest *string
		Newest *string
	}
	if err := base().
		Select("MIN(created_at) AS oldest, MAX(created_at) AS newest").
		Scan(&bounds).Error; err == nil && len(bounds) > 0 {
		stats.OldestCreatedAt = parseTimePtr(bounds[0].Oldest)
		stats.NewestCreatedAt = parseTimePtr(bounds[0].Newest)
	}

	return stats, nil
}

// parseTimePtr 解析数据库返回的时间字符串
//
// 兼容 SQLite 的多种时间存储格式以及 PostgreSQL 的 RFC3339 输出。
func parseTimePtr(raw *string) *time.Time {
	if raw == nil || *raw == "" {
		return nil
	}

	formats := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, *raw); err == nil {
			return &parsed
		}
	}
	return nil
}

// RecallByTimeWindow 时序召回：取时间窗口内的最新记忆
func (s *GormStore) RecallByTimeWindow(ctx context.Context, userID string, since time.Time, limit int) ([]*Memory, error) {
	if limit <= 0 {
		limit = 50
	}

	var memories []*Memory
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND created_at >= ?", userID, since).
		Order("created_at DESC").
		Limit(limit).
		Find(&memories).Error
	if err != nil {
		return nil, err
	}
	return memories, nil
}

// RecallBySession 会话召回：取同一会话内的记忆
func (s *GormStore) RecallBySession(ctx context.Context, userID, sessionID string, limit int) ([]*Memory, error) {
	if sessionID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	var memories []*Memory
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Order("created_at DESC").
		Limit(limit).
		Find(&memories).Error
	if err != nil {
		return nil, err
	}
	return memories, nil
}

// RecallByImportance 高重要性召回
func (s *GormStore) RecallByImportance(ctx context.Context, userID string, minScore, limit int) ([]*Memory, error) {
	if limit <= 0 {
		limit = 30
	}

	var memories []*Memory
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND importance_score >= ?", userID, minScore).
		Order("importance_score DESC, created_at DESC").
		Limit(limit).
		Find(&memories).Error
	if err != nil {
		return nil, err
	}
	return memories, nil
}

// CountByUser 统计用户记忆总数
func (s *GormStore) CountByUser(ctx context.Context, userID string) (int64, error) {
	var total int64
	err := s.db.WithContext(ctx).
		Model(&Memory{}).
		Where("user_id = ?", userID).
		Count(&total).Error
	return total, err
}

// GetMemoriesByIDs 按 ID 批量读取记忆（供缓存命中后回查）
func (s *GormStore) GetMemoriesByIDs(ctx context.Context, ids []int64) ([]*Memory, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var memories []*Memory
	err := s.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&memories).Error
	if err != nil {
		return nil, err
	}

	// 保持传入顺序，避免召回结果顺序错乱
	byID := make(map[int64]*Memory, len(memories))
	for _, m := range memories {
		byID[m.ID] = m
	}
	ordered := make([]*Memory, 0, len(memories))
	for _, id := range ids {
		if m, ok := byID[id]; ok {
			ordered = append(ordered, m)
		}
	}
	return ordered, nil
}

// ArchiveOldMemories 归档低价值旧记忆（设计文档 4.2.2）
//
// 归档条件：创建时间 > 90 天、重要性 < 30、最近 30 天未访问。
// 归档即设置 90 天后的过期时间，由清理任务真正删除。
func (s *GormStore) ArchiveOldMemories(ctx context.Context, userID string) (int64, error) {
	now := time.Now()
	createdBefore := now.AddDate(0, 0, -90)
	accessedBefore := now.AddDate(0, 0, -30)

	query := s.db.WithContext(ctx).Model(&Memory{}).
		Where("created_at < ?", createdBefore).
		Where("importance_score < ?", 30).
		Where("expires_at IS NULL").
		Where("last_accessed_at IS NULL OR last_accessed_at < ?", accessedBefore)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	result := query.Updates(map[string]interface{}{
		"expires_at": now.AddDate(0, 0, 90),
		"updated_at": now,
	})
	return result.RowsAffected, result.Error
}

// PurgeExpired 清理已过期的记忆（硬删除）
func (s *GormStore) PurgeExpired(ctx context.Context, userID string) (int64, error) {
	query := s.db.WithContext(ctx).
		Unscoped().
		Where("expires_at IS NOT NULL AND expires_at < ?", time.Now())
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	result := query.Delete(&Memory{})
	return result.RowsAffected, result.Error
}

// String 便于日志
func (s *GormStore) String() string {
	return fmt.Sprintf("gorm-store(%s)", s.db.Dialector.Name())
}
