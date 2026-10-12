package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/soren/backend/internal/cache"
	"gorm.io/gorm"
)

// DefaultUserID 桌面端单用户场景下的默认用户 ID
const DefaultUserID = "default-user"

// MemoryManager 记忆管理器（门面模式，设计文档 6.4）
//
// 对外提供统一接口，内部组合存储、标签、召回与提取四部分能力。
type MemoryManager struct {
	store    *GormStore
	recaller *MultiRecaller
	scorer   *ImportanceScorer
	cache    cache.Cache
	enabled  bool
}

// ManagerOptions 记忆管理器配置
type ManagerOptions struct {
	Enabled        bool           // 是否启用长期记忆
	RecallLimit    int            // 默认召回条数
	TimeWindowDays int            // 时序召回窗口（天）
	MinImportance  int            // 高重要性召回阈值
	RerankerConfig RerankerConfig // 重排序权重
	ScorerConfig   ScorerConfig   // 重要性评分权重
	Cache          cache.Cache    // 可选缓存，为空时降级为无缓存
}

// DefaultManagerOptions 默认配置
func DefaultManagerOptions() ManagerOptions {
	return ManagerOptions{
		Enabled:        true,
		RecallLimit:    10,
		TimeWindowDays: 7,
		MinImportance:  70,
		RerankerConfig: DefaultRerankerConfig(),
		ScorerConfig:   DefaultScorerConfig(),
	}
}

// NewMemoryManager 创建记忆管理器
func NewMemoryManager(db *gorm.DB, c cache.Cache, opts ManagerOptions) (*MemoryManager, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	store := NewGormStore(db)
	if err := store.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate memory tables: %w", err)
	}

	scorer := NewImportanceScorer(opts.ScorerConfig)

	// 组装四路召回器：时序、标签、会话、高重要性
	recallers := []Recaller{
		NewTimeWindowRecaller(store, opts.TimeWindowDays),
		NewTagBasedRecaller(store, c),
		NewSessionRecaller(store, c),
		NewImportantMemoryRecaller(store, opts.MinImportance),
	}

	reranker := NewMemoryReranker(opts.RerankerConfig)

	return &MemoryManager{
		store:    store,
		recaller: NewMultiRecaller(recallers, reranker),
		scorer:   scorer,
		cache:    c,
		enabled:  opts.Enabled,
	}, nil
}

// Enabled 是否启用记忆系统
func (m *MemoryManager) Enabled() bool {
	return m != nil && m.enabled
}

// Store 返回底层存储，供健康检查或高级用法使用
func (m *MemoryManager) Store() *GormStore {
	return m.store
}

// ============================================================
// 写入
// ============================================================

// AddMemory 写入一条记忆（自动计算重要性分数并抽取标签）
func (m *MemoryManager) AddMemory(ctx context.Context, memory *Memory) error {
	if !m.Enabled() {
		return nil
	}
	if memory == nil {
		return errors.New("memory is nil")
	}

	if memory.UserID == "" {
		memory.UserID = DefaultUserID
	}
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = time.Now()
	}
	if memory.Summary == "" {
		memory.Summary = GenerateSummary(memory.Content, 100)
	}

	// 1. 计算重要性分数
	memory.ImportanceScore = m.scorer.CalculateScore(memory)

	// 2. 在同一事务中写入记忆与标签
	err := m.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(memory).Error; err != nil {
			return err
		}

		tags := ExtractTags(memory.Content + " " + memory.Summary)
		for _, name := range tags {
			tag, err := m.getOrCreateTagWithTx(ctx, tx, memory.UserID, name, TagTypeTopic)
			if err != nil {
				return err
			}
			link := MemoryTag{
				MemoryID:       memory.ID,
				TagID:          tag.ID,
				RelevanceScore: 100,
				CreatedAt:      time.Now(),
			}
			if err := tx.Where("memory_id = ? AND tag_id = ?", memory.ID, tag.ID).
				FirstOrCreate(&link).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to write memory: %w", err)
	}

	// 3. 异步写入缓存
	go m.cacheMemory(memory)

	return nil
}

// getOrCreateTagWithTx 在事务内创建或获取标签
func (m *MemoryManager) getOrCreateTagWithTx(ctx context.Context, tx *gorm.DB, userID, name string, tagType TagType) (*Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("tag name is empty")
	}

	var tag Tag
	err := tx.WithContext(ctx).Where("user_id = ? AND name = ?", userID, name).First(&tag).Error
	if err == nil {
		return &tag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	tag = Tag{UserID: userID, Name: name, TagType: tagType, CreatedAt: time.Now()}
	if err := tx.WithContext(ctx).Create(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

// StoreConversation 存储对话记忆
func (m *MemoryManager) StoreConversation(
	ctx context.Context,
	userID, sessionID, content string,
	turn int,
) error {
	if !m.Enabled() || strings.TrimSpace(content) == "" {
		return nil
	}

	memory := &Memory{
		UserID:           normalizeUserID(userID),
		SessionID:        sessionID,
		ContentType:      MemoryTypeConversation,
		Content:          content,
		Summary:          GenerateSummary(content, 100),
		ConversationTurn: turn,
		CreatedAt:        time.Now(),
	}

	return m.AddMemory(ctx, memory)
}

// StorePreference 存储用户偏好
func (m *MemoryManager) StorePreference(
	ctx context.Context,
	userID, preferenceKey, preferenceValue string,
) error {
	if !m.Enabled() {
		return nil
	}
	preferenceKey = strings.TrimSpace(preferenceKey)
	preferenceValue = strings.TrimSpace(preferenceValue)
	if preferenceKey == "" || preferenceValue == "" {
		return nil
	}

	userID = normalizeUserID(userID)

	memory := &Memory{
		UserID:             userID,
		ContentType:        MemoryTypePreference,
		Content:            fmt.Sprintf("%s: %s", preferenceKey, preferenceValue),
		Summary:            fmt.Sprintf("%s: %s", preferenceKey, preferenceValue),
		IsExplicitlyStated: true,
		CreatedAt:          time.Now(),
	}

	if err := m.AddMemory(ctx, memory); err != nil {
		return err
	}

	// 同步维护偏好表：同一 key 累加证据数量并提升置信度
	return m.upsertPreference(ctx, userID, preferenceKey, preferenceValue)
}

// upsertPreference 写入或更新用户偏好表
func (m *MemoryManager) upsertPreference(ctx context.Context, userID, key, value string) error {
	now := time.Now()

	var existing UserPreference
	err := m.store.db.WithContext(ctx).
		Where("user_id = ? AND preference_key = ?", userID, key).
		First(&existing).Error

	switch {
	case err == nil:
		// 已存在：累加证据，置信度按 10 递增，上限 100
		confidence := existing.ConfidenceScore + 10
		if confidence > 100 {
			confidence = 100
		}
		return m.store.db.WithContext(ctx).
			Model(&UserPreference{}).
			Where("id = ?", existing.ID).
			Updates(map[string]interface{}{
				"preference_value":  value,
				"confidence_score":  confidence,
				"evidence_count":    gorm.Expr("evidence_count + 1"),
				"last_confirmed_at": now,
				"updated_at":        now,
			}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		return m.store.db.WithContext(ctx).Create(&UserPreference{
			UserID:          userID,
			PreferenceKey:   key,
			PreferenceValue: value,
			ConfidenceScore: 60,
			EvidenceCount:   1,
			CreatedAt:       now,
			UpdatedAt:       now,
			LastConfirmedAt: &now,
		}).Error
	default:
		return err
	}
}

// StoreFact 存储事实知识
func (m *MemoryManager) StoreFact(ctx context.Context, userID, content string) error {
	if !m.Enabled() || strings.TrimSpace(content) == "" {
		return nil
	}

	return m.AddMemory(ctx, &Memory{
		UserID:             normalizeUserID(userID),
		ContentType:        MemoryTypeFact,
		Content:            content,
		Summary:            GenerateSummary(content, 100),
		IsExplicitlyStated: true,
		CreatedAt:          time.Now(),
	})
}

// StoreTask 存储任务记录
func (m *MemoryManager) StoreTask(ctx context.Context, userID, sessionID, content string) error {
	if !m.Enabled() || strings.TrimSpace(content) == "" {
		return nil
	}

	return m.AddMemory(ctx, &Memory{
		UserID:      normalizeUserID(userID),
		SessionID:   sessionID,
		ContentType: MemoryTypeTask,
		Content:     content,
		Summary:     GenerateSummary(content, 100),
		CreatedAt:   time.Now(),
	})
}

// ProcessTurn 一轮对话结束后异步提取并存储记忆
//
// 提取内容：
//  1. 用户消息中的偏好（preference）
//  2. 用户消息中的事实（fact）
//  3. 本轮的对话摘要（conversation，含用户提问与助手回复要点）
//
// 该方法设计为在 goroutine 中调用，内部使用独立的 context，
// 避免请求结束导致 context 取消而丢失记忆。
func (m *MemoryManager) ProcessTurn(userID, sessionID, userMessage, assistantReply string, turn int) {
	if !m.Enabled() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userID = normalizeUserID(userID)

	// 1. 用户偏好
	for _, pref := range ExtractPreferences(userMessage) {
		if err := m.StorePreference(ctx, userID, pref.Key, pref.Value); err != nil {
			log.Printf("memory: failed to store preference: %v", err)
		}
	}

	// 2. 事实
	for _, fact := range ExtractFacts(userMessage) {
		if err := m.StoreFact(ctx, userID, fact); err != nil {
			log.Printf("memory: failed to store fact: %v", err)
		}
	}

	// 3. 任务
	for _, task := range ExtractTasks(userMessage) {
		if err := m.StoreTask(ctx, userID, sessionID, task); err != nil {
			log.Printf("memory: failed to store task: %v", err)
		}
	}

	// 4. 对话摘要（用户提问 + 助手回复要点）
	summary := buildConversationRecord(userMessage, assistantReply)
	if err := m.StoreConversation(ctx, userID, sessionID, summary, turn); err != nil {
		log.Printf("memory: failed to store conversation: %v", err)
	}

	// 5. 刷新会话缓存
	m.cacheSessionMemories(ctx, userID, sessionID)
}

// buildConversationRecord 组装一轮对话的记忆内容
func buildConversationRecord(userMessage, assistantReply string) string {
	var builder strings.Builder

	builder.WriteString("用户：")
	builder.WriteString(GenerateSummary(userMessage, 120))

	if strings.TrimSpace(assistantReply) != "" {
		builder.WriteString("\n助手：")
		builder.WriteString(GenerateSummary(assistantReply, 120))
	}

	return builder.String()
}

// ============================================================
// 召回
// ============================================================

// RecallRelevantMemories 召回相关记忆
func (m *MemoryManager) RecallRelevantMemories(
	ctx context.Context,
	userID, sessionID, currentMessage string,
	maxResults int,
) ([]*Memory, error) {
	if !m.Enabled() {
		return nil, nil
	}
	if maxResults <= 0 {
		maxResults = 10
	}

	req := &RecallRequest{
		UserID:         normalizeUserID(userID),
		SessionID:      sessionID,
		CurrentMessage: currentMessage,
		MaxResults:     maxResults,
	}

	result, err := m.recaller.Recall(ctx, req)
	if err != nil {
		return nil, err
	}

	// 异步更新访问计数，避免阻塞对话主链路
	for _, mem := range result.Memories {
		go func(id int64) {
			updateCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := m.store.IncrementAccessCount(updateCtx, id); err != nil {
				log.Printf("memory: failed to increment access count: %v", err)
			}
		}(mem.ID)
	}

	return result.Memories, nil
}

// RecallWithDetail 召回并返回完整统计信息
func (m *MemoryManager) RecallWithDetail(ctx context.Context, req *RecallRequest) (*RecallResult, error) {
	if !m.Enabled() {
		return &RecallResult{}, nil
	}
	if req.UserID == "" {
		req.UserID = DefaultUserID
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 10
	}
	return m.recaller.Recall(ctx, req)
}

// BuildContextBlock 将召回的记忆组装为可注入 system prompt 的文本
//
// 返回空字符串表示没有可用记忆，调用方无需插入任何内容。
func (m *MemoryManager) BuildContextBlock(memories []*Memory) string {
	if len(memories) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("## 关于用户的长期记忆\n")
	builder.WriteString("以下是此前与用户交互中积累的记忆，请在回答时自然地参考，不要生硬罗列：\n")

	// 按类型分组输出，偏好与事实优先（对回答风格影响最大）
	groups := []struct {
		title string
		types []MemoryType
	}{
		{"用户偏好", []MemoryType{MemoryTypePreference}},
		{"已知事实", []MemoryType{MemoryTypeFact}},
		{"进行中的任务", []MemoryType{MemoryTypeTask}},
		{"相关历史对话", []MemoryType{MemoryTypeConversation}},
	}

	for _, group := range groups {
		var lines []string
		for _, mem := range memories {
			if !containsType(group.types, mem.ContentType) {
				continue
			}
			text := mem.Summary
			// Older preference records stored only their topic in Summary. Use
			// the original content so both old and new records retain the value.
			if mem.ContentType == MemoryTypePreference && mem.Content != "" {
				text = mem.Content
			}
			if text == "" {
				text = GenerateSummary(mem.Content, 100)
			}
			lines = append(lines, fmt.Sprintf("- %s", text))
		}
		if len(lines) > 0 {
			builder.WriteString("\n### " + group.title + "\n")
			builder.WriteString(strings.Join(lines, "\n"))
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// containsType 判断类型是否在列表中
func containsType(types []MemoryType, target MemoryType) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

// ============================================================
// 管理接口（供 API 层使用）
// ============================================================

// List 分页列出记忆
func (m *MemoryManager) List(ctx context.Context, opts ListOptions) ([]*Memory, int64, error) {
	if opts.UserID == "" {
		opts.UserID = DefaultUserID
	}
	return m.store.List(ctx, opts)
}

// Search 搜索记忆
func (m *MemoryManager) Search(ctx context.Context, userID, keyword string, limit int) ([]*Memory, error) {
	return m.store.Search(ctx, normalizeUserID(userID), keyword, limit)
}

// Get 读取单条记忆
func (m *MemoryManager) Get(ctx context.Context, id int64) (*Memory, error) {
	return m.store.GetMemory(ctx, id)
}

// Delete 删除单条记忆
func (m *MemoryManager) Delete(ctx context.Context, id int64) error {
	memory, err := m.store.GetMemory(ctx, id)
	if err != nil {
		return err
	}
	if err := m.store.DeleteMemory(ctx, id); err != nil {
		return err
	}

	// 缓存失效：尽力而为，失败不影响删除结果
	m.invalidateCache(ctx, memory)
	return nil
}

// invalidateCache 让被删除记忆对应的缓存失效
//
// 热记忆与标签索引都是聚合结构，直接重建代价高于收益，
// 因此仅在条目仍留在缓存时按集合语义移除其 ID 引用。
func (m *MemoryManager) invalidateCache(ctx context.Context, memory *Memory) {
	if m.cache == nil || memory == nil {
		return
	}

	keys := []string{}
	if memory.UserID != "" {
		keys = append(keys, hotMemoriesKey(memory.UserID))
	}
	if memory.SessionID != "" {
		keys = append(keys, sessionMemoriesKey(memory.SessionID))
	}

	for _, key := range keys {
		if err := m.cache.Delete(ctx, key); err != nil {
			log.Printf("memory: failed to invalidate cache %s: %v", key, err)
		}
	}
}

// Stats 记忆统计
func (m *MemoryManager) Stats(ctx context.Context, userID string) (*Stats, error) {
	return m.store.Stats(ctx, normalizeUserID(userID))
}

// ArchiveOldMemories 归档低价值旧记忆
func (m *MemoryManager) ArchiveOldMemories(ctx context.Context, userID string) (int64, error) {
	return m.store.ArchiveOldMemories(ctx, normalizeUserID(userID))
}

// PurgeExpired 清理过期记忆
func (m *MemoryManager) PurgeExpired(ctx context.Context, userID string) (int64, error) {
	return m.store.PurgeExpired(ctx, normalizeUserID(userID))
}

// ============================================================
// 缓存辅助
// ============================================================

// cacheMemory 写入热数据缓存（设计文档 3.2.2）
func (m *MemoryManager) cacheMemory(memory *Memory) {
	if m.cache == nil || memory == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	members := []cache.ZMember{{
		Member: formatMemoryID(memory.ID),
		Score:  float64(memory.ImportanceScore),
	}}
	if err := m.cache.ZAdd(ctx, hotMemoriesKey(memory.UserID), members...); err != nil {
		return
	}
	_ = m.cache.Set(ctx, hotMemoriesKey(memory.UserID)+":ttl", "1", hotMemoriesTTL)
}

// cacheSessionMemories 刷新会话最近记忆缓存
func (m *MemoryManager) cacheSessionMemories(ctx context.Context, userID, sessionID string) {
	if m.cache == nil || sessionID == "" {
		return
	}

	memories, err := m.store.RecallBySession(ctx, userID, sessionID, 20)
	if err != nil || len(memories) == 0 {
		return
	}

	members := make([]cache.ZMember, 0, len(memories))
	for _, mem := range memories {
		members = append(members, cache.ZMember{
			Member: formatMemoryID(mem.ID),
			Score:  float64(mem.CreatedAt.Unix()),
		})
	}
	if err := m.cache.ZAdd(ctx, sessionMemoriesKey(sessionID), members...); err != nil {
		return
	}
	_ = m.cache.Set(ctx, sessionMemoriesKey(sessionID)+":ttl", "1", sessionMemoriesTTL)
}

// sessionMemoriesKey 会话最近记忆缓存键
func sessionMemoriesKey(sessionID string) string {
	return fmt.Sprintf("session:%s:recent_memories", sessionID)
}

// hotMemoriesKey 用户热数据缓存键
func hotMemoriesKey(userID string) string {
	return fmt.Sprintf("user:%s:hot_memories", userID)
}

// formatMemoryID 记忆 ID 转字符串
func formatMemoryID(id int64) string {
	return strconv.FormatInt(id, 10)
}

// parseMemoryID 字符串转记忆 ID
func parseMemoryID(id string) (int64, error) {
	return strconv.ParseInt(id, 10, 64)
}

// normalizeUserID 归一化用户 ID（桌面端单用户场景）
func normalizeUserID(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DefaultUserID
	}
	return userID
}

// EncodeMetadata 将标签元数据编码为 JSON 字符串
//
// SQLite 没有 JSONB 类型，统一以 TEXT 存储 JSON，两种驱动行为一致。
func EncodeMetadata(metadata map[string]interface{}) string {
	if len(metadata) == 0 {
		return ""
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// DecodeMetadata 解析标签元数据
func DecodeMetadata(raw string) map[string]interface{} {
	if raw == "" {
		return nil
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil
	}
	return metadata
}

// Edit keeps ownership unchanged and invalidates recall caches.
func (m *MemoryManager) Edit(ctx context.Context, id int64, content string) error {
	entry, err := m.store.GetMemory(ctx, id)
	if err != nil {
		return err
	}
	entry.Content = content
	entry.Summary = content
	if err = m.store.UpdateMemory(ctx, entry); err != nil {
		return err
	}
	m.invalidateCache(ctx, entry)
	return nil
}
