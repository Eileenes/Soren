package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soren/backend/internal/cache"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/store"
	"gorm.io/gorm"
)

// newTestDB 创建临时 SQLite 数据库用于测试
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := store.OpenDB(&config.Config{
		StorageDriver: config.StorageDriverSQLite,
		SQLitePath:    filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(AllModels()...); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	return db
}

// newTestManager 创建带内存缓存的记忆管理器
func newTestManager(t *testing.T) *MemoryManager {
	t.Helper()

	opts := DefaultManagerOptions()
	opts.Cache = cache.NewMemoryCache(100)

	manager, err := NewMemoryManager(newTestDB(t), opts.Cache, opts)
	if err != nil {
		t.Fatalf("failed to create memory manager: %v", err)
	}
	return manager
}

// ============================================================
// 重要性评分
// ============================================================

func TestImportanceScorerPrefersPreferences(t *testing.T) {
	scorer := NewImportanceScorer(DefaultScorerConfig())
	now := time.Now()

	conversation := &Memory{ContentType: MemoryTypeConversation, CreatedAt: now}
	preference := &Memory{ContentType: MemoryTypePreference, CreatedAt: now}
	explicitPreference := &Memory{
		ContentType:        MemoryTypePreference,
		IsExplicitlyStated: true,
		CreatedAt:          now,
	}

	convScore := scorer.CalculateScore(conversation)
	prefScore := scorer.CalculateScore(preference)
	explicitScore := scorer.CalculateScore(explicitPreference)

	if prefScore <= convScore {
		t.Errorf("preference score (%d) should exceed conversation score (%d)", prefScore, convScore)
	}
	if explicitScore <= prefScore {
		t.Errorf("explicit statement bonus missing: %d vs %d", explicitScore, prefScore)
	}
	if explicitScore > 100 {
		t.Errorf("score must be capped at 100, got %d", explicitScore)
	}
}

func TestImportanceScorerDecaysOverTime(t *testing.T) {
	scorer := NewImportanceScorer(DefaultScorerConfig())

	fresh := &Memory{ContentType: MemoryTypeFact, CreatedAt: time.Now()}
	old := &Memory{ContentType: MemoryTypeFact, CreatedAt: time.Now().AddDate(0, 0, -365)}

	if scorer.CalculateScore(fresh) <= scorer.CalculateScore(old) {
		t.Errorf("fresh memory should score higher than a year-old one")
	}
}

func TestImportanceScorerFrequencyBonusIsCapped(t *testing.T) {
	config := DefaultScorerConfig()
	scorer := NewImportanceScorer(config)

	rare := &Memory{ContentType: MemoryTypeTask, AccessCount: 1, CreatedAt: time.Now()}
	hot := &Memory{ContentType: MemoryTypeTask, AccessCount: 100, CreatedAt: time.Now()}

	difference := scorer.CalculateScore(hot) - scorer.CalculateScore(rare)
	if float64(difference) > config.FrequencyMaxBonus+1 {
		t.Errorf("frequency bonus %d exceeds cap %.1f", difference, config.FrequencyMaxBonus)
	}
}

// ============================================================
// 关键词提取
// ============================================================

func TestExtractKeywordsHandlesChineseAndEnglish(t *testing.T) {
	keywords := ExtractKeywords("我喜欢用 Go 语言写后端服务")

	joined := strings.Join(keywords, "|")
	for _, expected := range []string{"喜欢"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("expected keyword %q in %v", expected, keywords)
		}
	}
	// 英文单词应完整保留
	found := false
	for _, k := range keywords {
		if k == "go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected lowercase english word \"go\" in %v", keywords)
	}
}

func TestExtractKeywordsFiltersStopWords(t *testing.T) {
	keywords := ExtractKeywords("the and 的 了 是")
	for _, k := range keywords {
		if _, isStop := stopWords[k]; isStop {
			t.Errorf("stop word %q should be filtered", k)
		}
	}
}

func TestExtractKeywordsEmptyInput(t *testing.T) {
	if keywords := ExtractKeywords("   "); len(keywords) != 0 {
		t.Errorf("expected no keywords for blank input, got %v", keywords)
	}
}

// ============================================================
// 召回与重排序
// ============================================================

func TestRerankerPrefersRelevantMemory(t *testing.T) {
	reranker := NewMemoryReranker(DefaultRerankerConfig())
	now := time.Now()

	relevant := &Memory{
		ID:              1,
		Content:         "用户喜欢喝拿铁咖啡",
		Summary:         "喜欢拿铁咖啡",
		ContentType:     MemoryTypePreference,
		ImportanceScore: 70,
		CreatedAt:       now,
	}
	irrelevant := &Memory{
		ID:              2,
		Content:         "用户配置了编译参数并部署到测试环境",
		Summary:         "部署配置",
		ContentType:     MemoryTypeFact,
		ImportanceScore: 70,
		CreatedAt:       now,
	}

	ranked := reranker.Rerank([]*Memory{irrelevant, relevant}, "推荐一杯咖啡")
	if len(ranked) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(ranked))
	}
	if ranked[0].ID != relevant.ID {
		t.Errorf("expected relevant memory first, got ID %d", ranked[0].ID)
	}
}

func TestRerankerRelevanceScoreRange(t *testing.T) {
	reranker := NewMemoryReranker(DefaultRerankerConfig())

	memory := &Memory{Content: "用户喜欢咖啡", Summary: "喜欢咖啡"}

	if score := reranker.CalculateRelevance(memory, "咖啡"); score <= 0 {
		t.Errorf("expected positive relevance for matching query, got %.3f", score)
	}
	if score := reranker.CalculateRelevance(memory, "量子力学与天体物理"); score != 0 {
		t.Errorf("expected zero relevance for unrelated query, got %.3f", score)
	}
	if score := reranker.CalculateRelevance(memory, ""); score != 0 {
		t.Errorf("expected zero relevance for empty query, got %.3f", score)
	}
}

func TestRerankerTimeDecay(t *testing.T) {
	reranker := NewMemoryReranker(DefaultRerankerConfig())

	fresh := &Memory{ID: 1, Content: "项目进度更新", ImportanceScore: 50, CreatedAt: time.Now()}
	stale := &Memory{ID: 2, Content: "项目进度更新", ImportanceScore: 50, CreatedAt: time.Now().AddDate(0, 0, -180)}

	ranked := reranker.Rerank([]*Memory{stale, fresh}, "项目进度")
	if ranked[0].ID != fresh.ID {
		t.Errorf("expected recently created memory first, got ID %d", ranked[0].ID)
	}
}

func TestRerankerDiversityPushesUpDuplicates(t *testing.T) {
	// 放大多样性权重，验证近重复记忆会被降权
	config := DefaultRerankerConfig()
	config.DiversityWeight = 5.0
	reranker := NewMemoryReranker(config)

	now := time.Now()
	first := &Memory{ID: 1, Content: "用户喜欢喝拿铁咖啡", ImportanceScore: 50, CreatedAt: now}
	duplicate := &Memory{ID: 2, Content: "用户喜欢喝拿铁咖啡", ImportanceScore: 50, CreatedAt: now}
	distinct := &Memory{ID: 3, Content: "用户偏好深色主题编辑器", ImportanceScore: 50, CreatedAt: now}

	ranked := reranker.Rerank([]*Memory{first, duplicate, distinct}, "")
	if len(ranked) != 3 {
		t.Fatalf("expected 3 memories, got %d", len(ranked))
	}
	if ranked[1].ID != distinct.ID {
		t.Errorf("expected distinct memory at rank 2, got ID %d", ranked[1].ID)
	}
}

func TestRerankerEmptyInput(t *testing.T) {
	reranker := NewMemoryReranker(DefaultRerankerConfig())
	if ranked := reranker.Rerank(nil, "任意查询"); len(ranked) != 0 {
		t.Errorf("expected empty result, got %d", len(ranked))
	}
}

// fakeRecaller 用于测试多路召回合并的假召回器
type fakeRecaller struct {
	name     string
	memories []*Memory
	err      error
}

func (f *fakeRecaller) Name() string { return f.name }

func (f *fakeRecaller) Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error) {
	return f.memories, f.err
}

func TestMultiRecallerMergesAndDeduplicates(t *testing.T) {
	shared := &Memory{ID: 1, Content: "用户喜欢咖啡", ImportanceScore: 80, CreatedAt: time.Now()}
	onlyA := &Memory{ID: 2, Content: "用户在做记忆系统", ImportanceScore: 60, CreatedAt: time.Now()}
	onlyB := &Memory{ID: 3, Content: "用户使用 Go 语言", ImportanceScore: 40, CreatedAt: time.Now()}

	multi := NewMultiRecaller(
		[]Recaller{
			&fakeRecaller{name: "a", memories: []*Memory{shared, onlyA}},
			&fakeRecaller{name: "b", memories: []*Memory{shared, onlyB}},
		},
		NewMemoryReranker(DefaultRerankerConfig()),
	)

	result, err := multi.Recall(context.Background(), &RecallRequest{
		UserID:         "test-user",
		CurrentMessage: "咖啡",
		MaxResults:     10,
	})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}

	// 三路不同 ID 去重后应为 3 条
	if result.TotalRecalled != 3 {
		t.Errorf("expected 3 unique memories, got %d", result.TotalRecalled)
	}
	if len(result.Memories) != 3 {
		t.Errorf("expected 3 ranked memories, got %d", len(result.Memories))
	}

	// 去重后不应出现重复 ID
	seen := make(map[int64]int)
	for _, mem := range result.Memories {
		seen[mem.ID]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Errorf("memory %d appeared %d times after dedup", id, count)
		}
	}
}

func TestMultiRecallerRespectsMaxResults(t *testing.T) {
	memories := make([]*Memory, 0, 5)
	for i := int64(1); i <= 5; i++ {
		memories = append(memories, &Memory{
			ID:              i,
			Content:         "测试记忆内容",
			ImportanceScore: 50,
			CreatedAt:       time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}

	multi := NewMultiRecaller(
		[]Recaller{&fakeRecaller{name: "all", memories: memories}},
		NewMemoryReranker(DefaultRerankerConfig()),
	)

	result, err := multi.Recall(context.Background(), &RecallRequest{MaxResults: 2})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(result.Memories) != 2 {
		t.Errorf("expected 2 memories after truncation, got %d", len(result.Memories))
	}
	if result.TotalRecalled != 5 {
		t.Errorf("expected TotalRecalled=5, got %d", result.TotalRecalled)
	}
}

func TestMultiRecallerSurvivesFailingRecaller(t *testing.T) {
	healthy := &Memory{ID: 1, Content: "用户喜欢咖啡", ImportanceScore: 70, CreatedAt: time.Now()}

	multi := NewMultiRecaller(
		[]Recaller{
			&fakeRecaller{name: "broken", err: context.DeadlineExceeded},
			&fakeRecaller{name: "healthy", memories: []*Memory{healthy}},
		},
		NewMemoryReranker(DefaultRerankerConfig()),
	)

	result, err := multi.Recall(context.Background(), &RecallRequest{MaxResults: 10})
	if err != nil {
		t.Fatalf("one failing path should not fail the whole recall: %v", err)
	}
	if len(result.Memories) != 1 {
		t.Errorf("expected the healthy path result, got %d memories", len(result.Memories))
	}
}

// ============================================================
// 存储与端到端
// ============================================================

func TestManagerStoreAndRecallEndToEnd(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	// 写入三条不同类型的记忆
	if err := manager.StorePreference(ctx, "u1", "咖啡", "喜欢拿铁，不喜欢美式"); err != nil {
		t.Fatalf("failed to store preference: %v", err)
	}
	if err := manager.StoreFact(ctx, "u1", "用户是后端工程师，主要写 Go"); err != nil {
		t.Fatalf("failed to store fact: %v", err)
	}
	if err := manager.StoreConversation(ctx, "u1", "s1", "用户问了关于咖啡豆产地的问题", 1); err != nil {
		t.Fatalf("failed to store conversation: %v", err)
	}

	// 召回：查询与咖啡相关
	memories, err := manager.RecallRelevantMemories(ctx, "u1", "s1", "推荐一杯咖啡", 10)
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(memories) == 0 {
		t.Fatal("expected at least one recalled memory")
	}

	// 与查询最相关的偏好记忆应排在首位
	if memories[0].ContentType != MemoryTypePreference {
		t.Errorf("expected preference memory ranked first, got %s (%q)",
			memories[0].ContentType, memories[0].Summary)
	}

	// 召回后访问计数应被异步更新（等待 goroutine 完成）
	time.Sleep(300 * time.Millisecond)
	stored, err := manager.Get(ctx, memories[0].ID)
	if err != nil {
		t.Fatalf("failed to reload memory: %v", err)
	}
	if stored.AccessCount == 0 {
		t.Error("expected access count to be incremented after recall")
	}
}

func TestManagerIsolatesUsers(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	if err := manager.StorePreference(ctx, "user-a", "主题", "喜欢深色主题"); err != nil {
		t.Fatalf("failed to store preference: %v", err)
	}
	if err := manager.StorePreference(ctx, "user-b", "主题", "喜欢浅色主题"); err != nil {
		t.Fatalf("failed to store preference: %v", err)
	}

	memories, err := manager.RecallRelevantMemories(ctx, "user-a", "", "主题 颜色", 10)
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	for _, mem := range memories {
		if mem.UserID != "user-a" {
			t.Errorf("user isolation broken: recalled memory belongs to %s", mem.UserID)
		}
	}
}

func TestManagerDefaultUserFallback(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	if err := manager.StoreFact(ctx, "", "默认用户的事实"); err != nil {
		t.Fatalf("failed to store fact: %v", err)
	}

	memories, _, err := manager.List(ctx, ListOptions{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}
	if memories[0].UserID != DefaultUserID {
		t.Errorf("expected fallback user %q, got %q", DefaultUserID, memories[0].UserID)
	}
}

func TestManagerListFilterAndPagination(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := manager.StoreConversation(ctx, "u1", "s1", "对话内容", i); err != nil {
			t.Fatalf("failed to store conversation: %v", err)
		}
	}
	if err := manager.StorePreference(ctx, "u1", "编辑器", "喜欢 vim"); err != nil {
		t.Fatalf("failed to store preference: %v", err)
	}

	// 按类型过滤
	prefs, total, err := manager.List(ctx, ListOptions{UserID: "u1", ContentType: "preference"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if total != 1 || len(prefs) != 1 {
		t.Errorf("expected exactly 1 preference memory, got total=%d len=%d", total, len(prefs))
	}

	// 分页
	page, total, err := manager.List(ctx, ListOptions{UserID: "u1", Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(page) != 2 {
		t.Errorf("expected page size 2, got %d", len(page))
	}
	if total != 6 {
		t.Errorf("expected total 6, got %d", total)
	}
}

func TestManagerSearchAndDelete(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	if err := manager.StoreFact(ctx, "u1", "用户的项目叫 Soren，是一个桌面端 AI 助手"); err != nil {
		t.Fatalf("failed to store fact: %v", err)
	}

	results, err := manager.Search(ctx, "u1", "Soren", 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search hit, got %d", len(results))
	}

	if err := manager.Delete(ctx, results[0].ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// 软删除后列表应为空
	remaining, _, err := manager.List(ctx, ListOptions{UserID: "u1"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected no memories after delete, got %d", len(remaining))
	}

	// 重复删除应返回未找到
	if err := manager.Delete(ctx, results[0].ID); err != ErrMemoryNotFound {
		t.Errorf("expected ErrMemoryNotFound on second delete, got %v", err)
	}
}

func TestManagerStats(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	if err := manager.StorePreference(ctx, "u1", "咖啡", "喜欢拿铁"); err != nil {
		t.Fatalf("failed to store preference: %v", err)
	}
	if err := manager.StoreConversation(ctx, "u1", "s1", "普通对话", 1); err != nil {
		t.Fatalf("failed to store conversation: %v", err)
	}

	stats, err := manager.Stats(ctx, "u1")
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats.Total != 2 {
		t.Errorf("expected total 2, got %d", stats.Total)
	}
	if stats.ByType["preference"] != 1 || stats.ByType["conversation"] != 1 {
		t.Errorf("unexpected type breakdown: %v", stats.ByType)
	}
	// 偏好记忆应达到高重要性阈值
	if stats.HighImportance < 1 {
		t.Errorf("expected at least 1 high-importance memory, got %d", stats.HighImportance)
	}
	if stats.TotalTags == 0 {
		t.Error("expected tags to be extracted and stored")
	}
}

func TestManagerTagsAreAttached(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	if err := manager.StoreFact(ctx, "u1", "用户使用 React 和 TypeScript 开发前端"); err != nil {
		t.Fatalf("failed to store fact: %v", err)
	}

	memories, _, err := manager.List(ctx, ListOptions{UserID: "u1"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}
	if len(memories[0].Tags) == 0 {
		t.Error("expected tags to be preloaded on the memory")
	}

	// 标签召回应能命中
	tagRecaller := NewTagBasedRecaller(manager.Store(), nil)
	found, err := tagRecaller.Recall(ctx, &RecallRequest{
		UserID:         "u1",
		CurrentMessage: "React TypeScript",
	})
	if err != nil {
		t.Fatalf("tag recall failed: %v", err)
	}
	if len(found) == 0 {
		t.Error("expected tag recall to find the memory")
	}
}

// ============================================================
// 记忆提取
// ============================================================

func TestExtractPreferencesFromMessage(t *testing.T) {
	preferences := ExtractPreferences("我喜欢用深色主题，而且偏好简洁的界面")
	if len(preferences) == 0 {
		t.Fatal("expected preferences to be extracted")
	}

	joined := ""
	for _, p := range preferences {
		joined += p.Key + "=" + p.Value + ";"
	}
	if !strings.Contains(joined, "深色主题") {
		t.Errorf("expected 深色主题 preference, got %s", joined)
	}
}

func TestExtractPreferencesEmptyForNeutralMessage(t *testing.T) {
	preferences := ExtractPreferences("今天天气怎么样")
	if len(preferences) != 0 {
		t.Errorf("expected no preferences, got %+v", preferences)
	}
}

func TestPreferenceExtractionPreservesNegation(t *testing.T) {
	for _, statement := range []string{
		"不要用英文回答", "别加表情", "不喜欢咖啡", "讨厌冗长的回答", "反感频繁通知",
	} {
		t.Run(statement, func(t *testing.T) {
			preferences := ExtractPreferences(statement)
			if len(preferences) != 1 || preferences[0].Value != statement {
				t.Fatalf("preference must retain the user's negation: %#v", preferences)
			}
		})
	}
}

func TestStoredPreferenceValueReachesRecallContext(t *testing.T) {
	manager := newTestManager(t)
	for _, preference := range ExtractPreferences("不要用英文回答") {
		if err := manager.StorePreference(context.Background(), "alice", preference.Key, preference.Value); err != nil {
			t.Fatal(err)
		}
	}
	memories, err := manager.RecallRelevantMemories(context.Background(), "alice", "new-session", "如何回答我", 10)
	if err != nil {
		t.Fatal(err)
	}
	if block := manager.BuildContextBlock(memories); !strings.Contains(block, "不要用英文回答") {
		t.Fatalf("recalled context lost preference value or negation: %q", block)
	}
}

func TestLegacyPreferenceContextUsesContent(t *testing.T) {
	manager := newTestManager(t)
	block := manager.BuildContextBlock([]*Memory{{
		ContentType: MemoryTypePreference,
		Content:     "语言偏好: 请用中文回答",
		Summary:     "语言偏好",
	}})
	if !strings.Contains(block, "请用中文回答") {
		t.Fatalf("legacy preference value was lost: %q", block)
	}
}

func TestProcessTurnStoresMemories(t *testing.T) {
	manager := newTestManager(t)

	manager.ProcessTurn(
		"u1",
		"s1",
		"我喜欢用 Go 写后端，请帮我记住这个偏好",
		"好的，我记住了。",
		2,
	)

	memories, _, err := manager.List(context.Background(), ListOptions{UserID: "u1", Limit: 50})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	hasConversation := false
	hasPreference := false
	for _, mem := range memories {
		switch mem.ContentType {
		case MemoryTypeConversation:
			hasConversation = true
		case MemoryTypePreference:
			hasPreference = true
		}
	}

	if !hasConversation {
		t.Error("expected a conversation summary memory to be stored")
	}
	if !hasPreference {
		t.Error("expected a preference memory to be stored")
	}
}

func TestBuildContextBlockGroupsByType(t *testing.T) {
	manager := newTestManager(t)

	block := manager.BuildContextBlock([]*Memory{
		{ID: 1, ContentType: MemoryTypePreference, Summary: "喜欢拿铁"},
		{ID: 2, ContentType: MemoryTypeFact, Summary: "是后端工程师"},
		{ID: 3, ContentType: MemoryTypeTask, Summary: "实现记忆系统"},
		{ID: 4, ContentType: MemoryTypeConversation, Summary: "讨论过咖啡"},
	})

	for _, expected := range []string{"用户偏好", "已知事实", "进行中的任务", "相关历史对话", "喜欢拿铁"} {
		if !strings.Contains(block, expected) {
			t.Errorf("expected %q in context block", expected)
		}
	}

	// 空记忆应返回空字符串，避免注入无意义的 system prompt
	if empty := manager.BuildContextBlock(nil); empty != "" {
		t.Errorf("expected empty block for no memories, got %q", empty)
	}
}

func TestDisabledManagerIsInert(t *testing.T) {
	opts := DefaultManagerOptions()
	opts.Enabled = false

	manager, err := NewMemoryManager(newTestDB(t), nil, opts)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	ctx := context.Background()
	if manager.Enabled() {
		t.Fatal("manager should be disabled")
	}
	if err := manager.StorePreference(ctx, "u1", "咖啡", "喜欢拿铁"); err != nil {
		t.Errorf("disabled manager should not error on write: %v", err)
	}

	memories, err := manager.RecallRelevantMemories(ctx, "u1", "s1", "咖啡", 5)
	if err != nil || len(memories) != 0 {
		t.Errorf("disabled manager should return no memories, got %d (err=%v)", len(memories), err)
	}
}

// ============================================================
// 缓存
// ============================================================

func TestMemoryCacheTTL(t *testing.T) {
	c := cache.NewMemoryCache(10)
	ctx := context.Background()

	if err := c.Set(ctx, "short", "value", 20*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	if value, err := c.Get(ctx, "short"); err != nil || value != "value" {
		t.Fatalf("expected cache hit, got value=%q err=%v", value, err)
	}

	time.Sleep(60 * time.Millisecond)
	if _, err := c.Get(ctx, "short"); err != cache.ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss after TTL expiry, got %v", err)
	}
}

func TestMemoryCacheLRUEviction(t *testing.T) {
	c := cache.NewMemoryCache(2)
	ctx := context.Background()

	_ = c.Set(ctx, "a", "1", 0)
	_ = c.Set(ctx, "b", "2", 0)
	// 访问 a 使其成为最近使用
	if _, err := c.Get(ctx, "a"); err != nil {
		t.Fatalf("expected a to be cached: %v", err)
	}
	// 写入 c 触发淘汰，最久未使用的 b 应被移除
	_ = c.Set(ctx, "c", "3", 0)

	if _, err := c.Get(ctx, "b"); err != cache.ErrCacheMiss {
		t.Errorf("expected least-recently-used key b to be evicted, got %v", err)
	}
	if _, err := c.Get(ctx, "a"); err != nil {
		t.Errorf("expected recently used key a to survive, got %v", err)
	}
}

func TestMemoryCacheSortedSet(t *testing.T) {
	c := cache.NewMemoryCache(10)
	ctx := context.Background()

	if err := c.ZAdd(ctx, "z",
		cache.ZMember{Member: "low", Score: 1},
		cache.ZMember{Member: "high", Score: 100},
		cache.ZMember{Member: "mid", Score: 50},
	); err != nil {
		t.Fatalf("zadd failed: %v", err)
	}

	members, err := c.SortedRevRange(ctx, "z", 2)
	if err != nil {
		t.Fatalf("zrevrange failed: %v", err)
	}
	if len(members) != 2 || members[0] != "high" || members[1] != "mid" {
		t.Errorf("unexpected sorted result: %v", members)
	}
}

func TestSessionRecallerUsesCacheOnSecondCall(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	if err := manager.StoreConversation(ctx, "u1", "s1", "会话内的记忆", 1); err != nil {
		t.Fatalf("failed to store conversation: %v", err)
	}

	recaller := NewSessionRecaller(manager.Store(), manager.cache)

	first, err := recaller.Recall(ctx, &RecallRequest{UserID: "u1", SessionID: "s1"})
	if err != nil {
		t.Fatalf("session recall failed: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected session recall to hit the database on first call")
	}

	// 第一次召回后缓存应已回填
	second, err := recaller.Recall(ctx, &RecallRequest{UserID: "u1", SessionID: "s1"})
	if err != nil {
		t.Fatalf("session recall failed: %v", err)
	}
	if len(second) == 0 {
		t.Error("expected session recall to return memories from cache on second call")
	}

	// 无会话 ID 时不应返回任何记忆
	none, err := recaller.Recall(ctx, &RecallRequest{UserID: "u1"})
	if err != nil || len(none) != 0 {
		t.Errorf("expected no memories without session ID, got %d (err=%v)", len(none), err)
	}
}

func TestTimeWindowRecallerRespectsWindow(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	if err := manager.StoreConversation(ctx, "u1", "s1", "最近的记忆", 1); err != nil {
		t.Fatalf("failed to store conversation: %v", err)
	}

	// 手工插入一条 30 天前的记忆
	old := &Memory{
		UserID:          "u1",
		ContentType:     MemoryTypeConversation,
		Content:         "很久以前的记忆",
		Summary:         "很久以前的记忆",
		ImportanceScore: 50,
		CreatedAt:       time.Now().AddDate(0, 0, -30),
	}
	if err := manager.AddMemory(ctx, old); err != nil {
		t.Fatalf("failed to add old memory: %v", err)
	}

	recaller := NewTimeWindowRecaller(manager.Store(), 7)
	window := 7 * 24 * time.Hour

	found, err := recaller.Recall(ctx, &RecallRequest{UserID: "u1", TimeWindow: &window})
	if err != nil {
		t.Fatalf("time window recall failed: %v", err)
	}
	for _, mem := range found {
		if mem.ID == old.ID {
			t.Error("30-day-old memory should be outside the 7-day window")
		}
	}
	if len(found) == 0 {
		t.Error("expected the recent memory to be recalled")
	}
}

func TestImportantMemoryRecallerThreshold(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	// 偏好记忆得分为 50+20+15 = 85，高于 70 阈值
	if err := manager.StorePreference(ctx, "u1", "咖啡", "喜欢拿铁"); err != nil {
		t.Fatalf("failed to store preference: %v", err)
	}
	if err := manager.StoreConversation(ctx, "u1", "s1", "普通寒暄", 1); err != nil {
		t.Fatalf("failed to store conversation: %v", err)
	}

	recaller := NewImportantMemoryRecaller(manager.Store(), 70)
	found, err := recaller.Recall(ctx, &RecallRequest{UserID: "u1"})
	if err != nil {
		t.Fatalf("important recall failed: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("expected at least one high-importance memory")
	}
	for _, mem := range found {
		if mem.ImportanceScore < 70 {
			t.Errorf("memory %d below threshold: score %d", mem.ID, mem.ImportanceScore)
		}
		if mem.ContentType == MemoryTypeConversation {
			t.Error("plain conversation memory should not pass the 70 threshold")
		}
	}
}

func TestArchiveAndPurge(t *testing.T) {
	manager := newTestManager(t)
	ctx := context.Background()

	// 通过底层存储直接写入，绕过 AddMemory 的重要性重算，
	// 以便构造"低重要性 + 长时间未访问"的归档候选
	old := &Memory{
		UserID:          "u1",
		ContentType:     MemoryTypeConversation,
		Content:         "陈旧且低价值的记忆",
		Summary:         "陈旧记忆",
		ImportanceScore: 20,
		CreatedAt:       time.Now().AddDate(0, 0, -120),
	}
	if err := manager.Store().WriteMemory(ctx, old); err != nil {
		t.Fatalf("failed to write memory: %v", err)
	}

	archived, err := manager.ArchiveOldMemories(ctx, "u1")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}
	if archived != 1 {
		t.Errorf("expected 1 archived memory, got %d", archived)
	}

	// 归档只设置过期时间，不应立即删除
	stats, err := manager.Stats(ctx, "u1")
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats.Total != 1 {
		t.Errorf("archived memory should still exist before expiry, total=%d", stats.Total)
	}

	// 高重要性记忆不应被归档
	fresh := &Memory{
		UserID:          "u1",
		ContentType:     MemoryTypePreference,
		Content:         "用户偏好深色主题",
		Summary:         "偏好深色主题",
		ImportanceScore: 90,
		CreatedAt:       time.Now().AddDate(0, 0, -120),
	}
	if err := manager.Store().WriteMemory(ctx, fresh); err != nil {
		t.Fatalf("failed to write memory: %v", err)
	}

	archivedAgain, err := manager.ArchiveOldMemories(ctx, "u1")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}
	if archivedAgain != 0 {
		t.Errorf("high-importance memory should not be archived, got %d", archivedAgain)
	}
}

// TestExtractKeywordsDropsCrossWordBigrams 验证切坏的二元组被过滤
//
// 「开发一个叫」按二元组会切出「发一」「个叫」等片段，它们跨越了词边界，
// 作为标签没有意义，必须被丢弃；而「开发」这类真实词应当保留。
func TestExtractKeywordsDropsCrossWordBigrams(t *testing.T) {
	keywords := ExtractKeywords("开发一个叫 Soren 的桌面端助手")
	joined := strings.Join(keywords, "|")

	// 只断言「至少有一个虚词字符」的跨词片段——这是本方法能高置信度
	// 覆盖的范围（发一：量词一；个叫：量词个；的桌：助词的）。
	//
	// 明确的能力边界：像「端助」（来自「桌面端|助手」）这种两个字符都是
	// 实词的跨词片段，纯字符级启发式抓不到——区分它需要真正的分词词典。
	// 这里选择不做过度设计：这类噪音数量少，会被重要性评分和重排序
	// 进一步稀释，而引入分词词典会显著增加体积与维护成本。
	for _, junk := range []string{"发一", "个叫", "的桌"} {
		for _, k := range keywords {
			if k == junk {
				t.Errorf("cross-word bigram %q should be filtered, got %v", junk, keywords)
			}
		}
	}

	// 真实词应当留下，否则说明过滤过度
	if !strings.Contains(joined, "开发") {
		t.Errorf("expected real word 开发 to survive, got %v", keywords)
	}
}

// TestExtractKeywordsDoesNotOverFilter 验证过滤没有把正常词一并干掉
func TestExtractKeywordsDoesNotOverFilter(t *testing.T) {
	keywords := ExtractKeywords("华阳是后端工程师，喜欢用 Rust 写服务")
	joined := strings.Join(keywords, "|")

	for _, expected := range []string{"华阳", "rust", "后端"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("expected %q in keywords, got %v", expected, keywords)
		}
	}

	if len(keywords) == 0 {
		t.Error("expected non-empty keywords")
	}
}
