# Soren AI 长期记忆系统设计文档

## 1. 设计理念与目标

### 1.1 Muse 记忆系统的核心理念

Muse 的记忆系统采用了与传统向量数据库方案不同的设计思路：

- **时序优先**：专注于上下文的时间连续性，记忆按时间线组织
- **简单高效**：使用时序数据库 + 用户偏好标签，而非复杂的向量搜索
- **低延迟**：避免向量相似度计算的开销，查询响应快
- **成本可控**：不需要大规模向量索引和计算资源

### 1.2 Soren 记忆系统的设计目标

- **延迟目标**：记忆查询延迟 < 50ms (P95)
- **准确性**：相关记忆召回率 > 85%
- **可扩展性**：支持单用户百万级记忆条目
- **成本效益**：基于 PostgreSQL + Redis，无需额外向量数据库

## 2. 系统架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                     Soren AI Core                        │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ├─── Memory Manager (记忆管理器)
                    │    ├── Writer: 写入新记忆
                    │    ├── Reader: 读取和召回记忆
                    │    └── Indexer: 索引和标签管理
                    │
┌───────────────────┴─────────────────────────────────────┐
│                   Storage Layer                          │
├──────────────────────────────────────┬──────────────────┤
│   PostgreSQL (持久化存储)             │   Redis (缓存层)   │
│   - 记忆主表                          │   - 热数据缓存     │
│   - 标签索引表                        │   - 用户上下文     │
│   - 时序索引                          │   - 最近记忆      │
└──────────────────────────────────────┴──────────────────┘
```

### 2.2 核心组件

#### 2.2.1 Memory Manager
- 统一的记忆管理接口
- 负责记忆的 CRUD 操作
- 协调 PostgreSQL 和 Redis 的数据同步

#### 2.2.2 Storage Layer
- **PostgreSQL**：持久化存储，支持复杂查询和时序索引
- **Redis**：高速缓存，存储最近的记忆和用户会话上下文

## 3. 数据模型设计

### 3.1 PostgreSQL 表结构

#### 3.1.1 记忆主表 (memories)

```sql
CREATE TABLE memories (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    session_id VARCHAR(64),

    -- 记忆内容
    content_type VARCHAR(32) NOT NULL, -- 'conversation', 'preference', 'task', 'fact'
    content TEXT NOT NULL,
    summary TEXT,  -- 记忆的简短摘要（50-100字）

    -- 元数据
    importance_score SMALLINT DEFAULT 50, -- 0-100，重要性评分
    access_count INT DEFAULT 0,  -- 访问次数
    last_accessed_at TIMESTAMPTZ,

    -- 时序信息
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,  -- 可选的过期时间

    -- 上下文关联
    parent_memory_id BIGINT REFERENCES memories(id),  -- 关联的父记忆
    conversation_turn INT,  -- 对话轮次

    -- 软删除
    deleted_at TIMESTAMPTZ,

    -- 索引
    CONSTRAINT valid_importance CHECK (importance_score BETWEEN 0 AND 100)
);

-- 时序索引（核心索引）
CREATE INDEX idx_memories_user_time ON memories(user_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- 类型+时间复合索引
CREATE INDEX idx_memories_user_type_time ON memories(user_id, content_type, created_at DESC)
    WHERE deleted_at IS NULL;

-- 会话索引
CREATE INDEX idx_memories_session ON memories(session_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- 重要性索引
CREATE INDEX idx_memories_importance ON memories(user_id, importance_score DESC, created_at DESC)
    WHERE deleted_at IS NULL AND importance_score >= 70;
```

#### 3.1.2 标签表 (tags)

```sql
CREATE TABLE tags (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    name VARCHAR(64) NOT NULL,
    tag_type VARCHAR(32) NOT NULL, -- 'preference', 'topic', 'entity', 'skill'
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_user_tag UNIQUE(user_id, name)
);

CREATE INDEX idx_tags_user_type ON tags(user_id, tag_type);
```

#### 3.1.3 记忆-标签关联表 (memory_tags)

```sql
CREATE TABLE memory_tags (
    memory_id BIGINT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    tag_id INT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    relevance_score SMALLINT DEFAULT 100, -- 0-100，标签相关性
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (memory_id, tag_id)
);

CREATE INDEX idx_memory_tags_tag ON memory_tags(tag_id, relevance_score DESC);
CREATE INDEX idx_memory_tags_memory ON memory_tags(memory_id);
```

#### 3.1.4 用户偏好表 (user_preferences)

```sql
CREATE TABLE user_preferences (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    preference_key VARCHAR(128) NOT NULL,
    preference_value TEXT NOT NULL,
    confidence_score SMALLINT DEFAULT 50, -- 0-100，置信度
    evidence_count INT DEFAULT 1,  -- 支持证据数量

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_confirmed_at TIMESTAMPTZ,

    CONSTRAINT unique_user_preference UNIQUE(user_id, preference_key)
);

CREATE INDEX idx_user_preferences_user ON user_preferences(user_id, confidence_score DESC);
```

### 3.2 Redis 数据结构

#### 3.2.1 会话上下文缓存

```
# 当前会话的最近记忆（有序集合，按时间排序）
session:{session_id}:recent_memories
  - Score: timestamp
  - Value: memory_id

# TTL: 24小时
```

#### 3.2.2 用户热数据缓存

```
# 用户的高频访问记忆（有序集合，按访问次数排序）
user:{user_id}:hot_memories
  - Score: access_count
  - Value: memory_id

# 用户的重要记忆列表
user:{user_id}:important_memories
  - List: [memory_id1, memory_id2, ...]

# TTL: 7天
```

#### 3.2.3 标签索引缓存

```
# 标签到记忆的映射（集合）
tag:{tag_id}:memories
  - Set: {memory_id1, memory_id2, ...}

# TTL: 3天
```

## 4. 记忆存储策略

### 4.1 记忆分类与重要性评分

#### 4.1.1 记忆类型

- **conversation**：对话记忆
  - 用户说过的话
  - AI 的回复
  - 对话中的关键信息

- **preference**：用户偏好
  - 明确表达的偏好（"我喜欢..."）
  - 推断出的偏好（多次选择表明的倾向）
  - 工作风格、沟通方式

- **task**：任务记录
  - 完成的任务
  - 待办事项
  - 项目信息

- **fact**：事实知识
  - 用户提供的个人信息
  - 领域知识
  - 上下文事实

#### 4.1.2 重要性评分算法

```go
type ImportanceScorer struct {
    // 权重配置
    ExplicitMentionWeight    float64 // 明确提及的权重
    RecencyWeight            float64 // 时间新近度权重
    FrequencyWeight          float64 // 访问频率权重
    UserConfirmationWeight   float64 // 用户确认权重
}

func (s *ImportanceScorer) CalculateScore(memory *Memory) int {
    score := 50.0 // 基础分

    // 1. 类型权重
    switch memory.ContentType {
    case "preference":
        score += 20
    case "fact":
        score += 15
    case "task":
        score += 10
    case "conversation":
        score += 5
    }

    // 2. 明确性（是否被用户明确强调）
    if memory.IsExplicitlyStated {
        score += 15
    }

    // 3. 访问频率
    if memory.AccessCount > 0 {
        score += math.Min(10, float64(memory.AccessCount)*2)
    }

    // 4. 时间衰减（30天内的记忆加分）
    daysSinceCreation := time.Since(memory.CreatedAt).Hours() / 24
    if daysSinceCreation < 30 {
        score += (30 - daysSinceCreation) / 3
    }

    // 5. 归一化到 0-100
    return int(math.Max(0, math.Min(100, score)))
}
```

### 4.2 记忆写入策略

#### 4.2.1 实时写入

```go
type MemoryWriter struct {
    db    *sql.DB
    redis *redis.Client
}

func (w *MemoryWriter) WriteMemory(ctx context.Context, memory *Memory) error {
    // 1. 计算重要性分数
    memory.ImportanceScore = w.scorer.CalculateScore(memory)

    // 2. 写入 PostgreSQL
    tx, err := w.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 插入记忆
    err = w.insertMemory(tx, memory)
    if err != nil {
        return err
    }

    // 提取并插入标签
    tags := w.extractTags(memory)
    err = w.insertTags(tx, memory.ID, tags)
    if err != nil {
        return err
    }

    if err = tx.Commit(); err != nil {
        return err
    }

    // 3. 异步写入 Redis 缓存
    go w.cacheMemory(memory)

    return nil
}
```

#### 4.2.2 批量归档

定期（如每小时）对低重要性、低访问的记忆进行归档处理：

```go
func (w *MemoryWriter) ArchiveOldMemories(ctx context.Context) error {
    // 归档条件：
    // - 创建时间 > 90天
    // - 重要性分数 < 30
    // - 最近30天访问次数 = 0

    query := `
        UPDATE memories
        SET expires_at = NOW() + INTERVAL '90 days'
        WHERE created_at < NOW() - INTERVAL '90 days'
          AND importance_score < 30
          AND (last_accessed_at IS NULL OR last_accessed_at < NOW() - INTERVAL '30 days')
          AND expires_at IS NULL
    `

    _, err := w.db.ExecContext(ctx, query)
    return err
}
```

### 4.3 记忆去重与合并

```go
type MemoryDeduplicator struct {
    db *sql.DB
}

func (d *MemoryDeduplicator) FindDuplicates(memory *Memory) ([]*Memory, error) {
    // 使用摘要相似度检测重复
    // 1. 查找同一用户、同一类型、相近时间的记忆
    // 2. 计算摘要的相似度（简单字符串相似度或更复杂的语义相似度）
    // 3. 返回相似度 > 80% 的记忆

    query := `
        SELECT * FROM memories
        WHERE user_id = $1
          AND content_type = $2
          AND created_at BETWEEN $3 AND $4
          AND deleted_at IS NULL
    `

    // ... 查询和相似度计算逻辑
    return duplicates, nil
}

func (d *MemoryDeduplicator) MergeMemories(primary, secondary *Memory) error {
    // 合并策略：
    // 1. 保留重要性分数更高的作为主记忆
    // 2. 合并标签
    // 3. 累加访问次数
    // 4. 软删除次要记忆

    // ... 合并逻辑
    return nil
}
```

## 5. 记忆召回机制

### 5.1 召回策略

Soren 的记忆召回采用多路召回 + 重排序的两阶段架构：

```
┌─────────────────────────────────────────────────────────┐
│                   召回阶段 (Recall)                       │
├─────────────────────────────────────────────────────────┤
│  1. 时序召回 (时间窗口)                                   │
│  2. 标签召回 (相关标签)                                   │
│  3. 会话召回 (同会话)                                     │
│  4. 重要记忆召回 (高分记忆)                               │
└────────────────────┬────────────────────────────────────┘
                     │ 候选记忆池 (100-200条)
                     ↓
┌─────────────────────────────────────────────────────────┐
│                 重排序阶段 (Re-rank)                      │
├─────────────────────────────────────────────────────────┤
│  1. 相关性评分                                            │
│  2. 时间衰减                                              │
│  3. 重要性加权                                            │
│  4. 多样性调整                                            │
└────────────────────┬────────────────────────────────────┘
                     │ Top K (5-20条)
                     ↓
                返回给 AI 作为上下文
```

### 5.2 召回实现

#### 5.2.1 时序召回

```go
type TimeWindowRecaller struct {
    db *sql.DB
}

func (r *TimeWindowRecaller) Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error) {
    // 召回最近 N 天的记忆
    query := `
        SELECT * FROM memories
        WHERE user_id = $1
          AND created_at >= NOW() - INTERVAL '7 days'
          AND deleted_at IS NULL
        ORDER BY created_at DESC
        LIMIT 50
    `

    // ... 查询逻辑
    return memories, nil
}
```

#### 5.2.2 标签召回

```go
type TagBasedRecaller struct {
    db    *sql.DB
    redis *redis.Client
}

func (r *TagBasedRecaller) Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error) {
    // 1. 从当前对话中提取关键词/实体
    tags := r.extractTags(req.CurrentMessage)

    // 2. 查找带有这些标签的记忆
    query := `
        SELECT DISTINCT m.*
        FROM memories m
        JOIN memory_tags mt ON m.id = mt.memory_id
        JOIN tags t ON mt.tag_id = t.id
        WHERE m.user_id = $1
          AND t.name = ANY($2)
          AND m.deleted_at IS NULL
        ORDER BY mt.relevance_score DESC, m.importance_score DESC
        LIMIT 50
    `

    // ... 查询逻辑
    return memories, nil
}
```

#### 5.2.3 会话召回

```go
type SessionRecaller struct {
    redis *redis.Client
    db    *sql.DB
}

func (r *SessionRecaller) Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error) {
    // 1. 先从 Redis 获取会话的最近记忆
    memoryIDs, err := r.redis.ZRevRange(
        ctx,
        fmt.Sprintf("session:%s:recent_memories", req.SessionID),
        0,
        20,
    ).Result()

    if err != nil || len(memoryIDs) == 0 {
        // 2. Redis 未命中，从数据库查询
        return r.recallFromDB(ctx, req.SessionID)
    }

    // 3. 根据 ID 批量查询记忆详情
    return r.getMemoriesByIDs(ctx, memoryIDs)
}
```

#### 5.2.4 重要记忆召回

```go
type ImportantMemoryRecaller struct {
    db    *sql.DB
    redis *redis.Client
}

func (r *ImportantMemoryRecaller) Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error) {
    // 召回用户的高重要性记忆
    query := `
        SELECT * FROM memories
        WHERE user_id = $1
          AND importance_score >= 70
          AND deleted_at IS NULL
        ORDER BY importance_score DESC, created_at DESC
        LIMIT 30
    `

    // ... 查询逻辑
    return memories, nil
}
```

### 5.3 重排序算法

```go
type MemoryReranker struct {
    config RerankerConfig
}

type RerankerConfig struct {
    RelevanceWeight  float64 // 相关性权重
    RecencyWeight    float64 // 时间新近度权重
    ImportanceWeight float64 // 重要性权重
    DiversityWeight  float64 // 多样性权重
}

func (r *MemoryReranker) Rerank(memories []*Memory, query string) []*Memory {
    // 1. 计算每条记忆的综合得分
    scored := make([]ScoredMemory, len(memories))
    for i, mem := range memories {
        scored[i] = ScoredMemory{
            Memory: mem,
            Score:  r.calculateScore(mem, query),
        }
    }

    // 2. 按得分排序
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })

    // 3. 多样性调整（避免返回过于相似的记忆）
    result := r.diversityFilter(scored)

    return result
}

func (r *MemoryReranker) calculateScore(memory *Memory, query string) float64 {
    score := 0.0

    // 1. 相关性得分（基于关键词匹配）
    relevance := r.calculateRelevance(memory, query)
    score += relevance * r.config.RelevanceWeight

    // 2. 时间衰减（越新越好）
    daysSince := time.Since(memory.CreatedAt).Hours() / 24
    recencyScore := math.Exp(-daysSince / 30) // 30天半衰期
    score += recencyScore * r.config.RecencyWeight

    // 3. 重要性
    importanceScore := float64(memory.ImportanceScore) / 100.0
    score += importanceScore * r.config.ImportanceWeight

    return score
}

func (r *MemoryReranker) calculateRelevance(memory *Memory, query string) float64 {
    // 简单的关键词匹配
    // 生产环境可以使用更复杂的语义相似度计算
    queryTerms := extractKeywords(query)
    memoryTerms := extractKeywords(memory.Content + " " + memory.Summary)

    matches := 0
    for _, qt := range queryTerms {
        for _, mt := range memoryTerms {
            if strings.Contains(strings.ToLower(mt), strings.ToLower(qt)) {
                matches++
                break
            }
        }
    }

    if len(queryTerms) == 0 {
        return 0.0
    }

    return float64(matches) / float64(len(queryTerms))
}
```

## 6. Go 语言接口设计

### 6.1 核心数据结构

```go
package memory

import (
    "context"
    "time"
)

// Memory 表示一条记忆
type Memory struct {
    ID              int64
    UserID          string
    SessionID       string
    ContentType     MemoryType
    Content         string
    Summary         string
    ImportanceScore int
    AccessCount     int
    LastAccessedAt  *time.Time
    CreatedAt       time.Time
    ExpiresAt       *time.Time
    ParentMemoryID  *int64
    ConversationTurn int
    DeletedAt       *time.Time
    Tags            []Tag
}

type MemoryType string

const (
    MemoryTypeConversation MemoryType = "conversation"
    MemoryTypePreference   MemoryType = "preference"
    MemoryTypeTask         MemoryType = "task"
    MemoryTypeFact         MemoryType = "fact"
)

// Tag 表示一个标签
type Tag struct {
    ID       int
    UserID   string
    Name     string
    TagType  TagType
    Metadata map[string]interface{}
}

type TagType string

const (
    TagTypePreference TagType = "preference"
    TagTypeTopic      TagType = "topic"
    TagTypeEntity     TagType = "entity"
    TagTypeSkill      TagType = "skill"
)

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
    Memories      []*Memory
    TotalRecalled int
    LatencyMS     int64
}
```

### 6.2 存储接口

```go
// MemoryStore 记忆存储接口
type MemoryStore interface {
    // 写入记忆
    WriteMemory(ctx context.Context, memory *Memory) error

    // 批量写入
    WriteBatch(ctx context.Context, memories []*Memory) error

    // 读取单条记忆
    GetMemory(ctx context.Context, memoryID int64) (*Memory, error)

    // 更新记忆
    UpdateMemory(ctx context.Context, memory *Memory) error

    // 软删除记忆
    DeleteMemory(ctx context.Context, memoryID int64) error

    // 增加访问计数
    IncrementAccessCount(ctx context.Context, memoryID int64) error
}

// TagStore 标签存储接口
type TagStore interface {
    // 创建或获取标签
    GetOrCreateTag(ctx context.Context, userID, name string, tagType TagType) (*Tag, error)

    // 为记忆添加标签
    AddTagToMemory(ctx context.Context, memoryID int64, tagID int) error

    // 获取记忆的所有标签
    GetMemoryTags(ctx context.Context, memoryID int64) ([]Tag, error)

    // 根据标签查找记忆
    FindMemoriesByTag(ctx context.Context, userID string, tagNames []string, limit int) ([]*Memory, error)
}
```

### 6.3 召回接口

```go
// MemoryRecaller 记忆召回接口
type MemoryRecaller interface {
    // 召回相关记忆
    Recall(ctx context.Context, req *RecallRequest) (*RecallResult, error)
}

// Recaller 召回器（单个召回策略）
type Recaller interface {
    Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error)
    Name() string
}

// MultiRecaller 多路召回实现
type MultiRecaller struct {
    recallers []Recaller
    reranker  *MemoryReranker
}

func NewMultiRecaller(recallers []Recaller, reranker *MemoryReranker) *MultiRecaller {
    return &MultiRecaller{
        recallers: recallers,
        reranker:  reranker,
    }
}

func (m *MultiRecaller) Recall(ctx context.Context, req *RecallRequest) (*RecallResult, error) {
    startTime := time.Now()

    // 1. 并发执行多路召回
    memoriesMap := make(map[int64]*Memory)
    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, recaller := range m.recallers {
        wg.Add(1)
        go func(r Recaller) {
            defer wg.Done()
            memories, err := r.Recall(ctx, req)
            if err != nil {
                // 日志记录错误，但不中断其他召回
                return
            }

            mu.Lock()
            for _, mem := range memories {
                if _, exists := memoriesMap[mem.ID]; !exists {
                    memoriesMap[mem.ID] = mem
                }
            }
            mu.Unlock()
        }(recaller)
    }

    wg.Wait()

    // 2. 转换为切片
    memories := make([]*Memory, 0, len(memoriesMap))
    for _, mem := range memoriesMap {
        memories = append(memories, mem)
    }

    // 3. 重排序
    ranked := m.reranker.Rerank(memories, req.CurrentMessage)

    // 4. 限制返回数量
    if len(ranked) > req.MaxResults {
        ranked = ranked[:req.MaxResults]
    }

    latency := time.Since(startTime).Milliseconds()

    return &RecallResult{
        Memories:      ranked,
        TotalRecalled: len(memoriesMap),
        LatencyMS:     latency,
    }, nil
}
```

### 6.4 记忆管理器

```go
// MemoryManager 记忆管理器（门面模式）
type MemoryManager struct {
    store    MemoryStore
    tagStore TagStore
    recaller MemoryRecaller
    scorer   *ImportanceScorer
    writer   *MemoryWriter
}

func NewMemoryManager(
    store MemoryStore,
    tagStore TagStore,
    recaller MemoryRecaller,
    scorer *ImportanceScorer,
) *MemoryManager {
    return &MemoryManager{
        store:    store,
        tagStore: tagStore,
        recaller: recaller,
        scorer:   scorer,
        writer:   NewMemoryWriter(store, tagStore, scorer),
    }
}

// StoreConversation 存储对话记忆
func (m *MemoryManager) StoreConversation(
    ctx context.Context,
    userID, sessionID, content string,
    turn int,
) error {
    memory := &Memory{
        UserID:           userID,
        SessionID:        sessionID,
        ContentType:      MemoryTypeConversation,
        Content:          content,
        Summary:          generateSummary(content),
        ConversationTurn: turn,
        CreatedAt:        time.Now(),
    }

    return m.writer.WriteMemory(ctx, memory)
}

// StorePreference 存储用户偏好
func (m *MemoryManager) StorePreference(
    ctx context.Context,
    userID, preferenceKey, preferenceValue string,
) error {
    memory := &Memory{
        UserID:      userID,
        ContentType: MemoryTypePreference,
        Content:     fmt.Sprintf("%s: %s", preferenceKey, preferenceValue),
        Summary:     preferenceKey,
        CreatedAt:   time.Now(),
    }

    return m.writer.WriteMemory(ctx, memory)
}

// RecallRelevantMemories 召回相关记忆
func (m *MemoryManager) RecallRelevantMemories(
    ctx context.Context,
    userID, sessionID, currentMessage string,
    maxResults int,
) ([]*Memory, error) {
    req := &RecallRequest{
        UserID:         userID,
        SessionID:      sessionID,
        CurrentMessage: currentMessage,
        MaxResults:     maxResults,
    }

    result, err := m.recaller.Recall(ctx, req)
    if err != nil {
        return nil, err
    }

    // 更新访问计数
    for _, mem := range result.Memories {
        go m.store.IncrementAccessCount(context.Background(), mem.ID)
    }

    return result.Memories, nil
}
```

## 7. 使用示例

### 7.1 初始化记忆系统

```go
package main

import (
    "context"
    "database/sql"
    "github.com/redis/go-redis/v9"
    "soren/memory"
)

func main() {
    // 1. 初始化数据库连接
    db, _ := sql.Open("postgres", "postgresql://user:pass@localhost/soren")
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    // 2. 创建存储实例
    memStore := memory.NewPostgresMemoryStore(db)
    tagStore := memory.NewPostgresTagStore(db)

    // 3. 创建召回器
    timeRecaller := memory.NewTimeWindowRecaller(db)
    tagRecaller := memory.NewTagBasedRecaller(db, redisClient)
    sessionRecaller := memory.NewSessionRecaller(db, redisClient)
    importantRecaller := memory.NewImportantMemoryRecaller(db, redisClient)

    reranker := memory.NewMemoryReranker(memory.RerankerConfig{
        RelevanceWeight:  0.4,
        RecencyWeight:    0.2,
        ImportanceWeight: 0.3,
        DiversityWeight:  0.1,
    })

    multiRecaller := memory.NewMultiRecaller(
        []memory.Recaller{
            timeRecaller,
            tagRecaller,
            sessionRecaller,
            importantRecaller,
        },
        reranker,
    )

    // 4. 创建记忆管理器
    scorer := memory.NewImportanceScorer(memory.ScorerConfig{})
    manager := memory.NewMemoryManager(memStore, tagStore, multiRecaller, scorer)

    // 5. 使用
    ctx := context.Background()

    // 存储对话
    err := manager.StoreConversation(ctx, "user123", "session456",
        "我喜欢喝咖啡，尤其是拿铁", 1)

    // 召回记忆
    memories, err := manager.RecallRelevantMemories(ctx,
        "user123", "session456", "给我推荐一杯饮料", 10)

    for _, mem := range memories {
        fmt.Printf("记忆: %s (重要性: %d)\n", mem.Summary, mem.ImportanceScore)
    }
}
```

### 7.2 与 AI 对话集成

```go
type SorenAI struct {
    memoryManager *memory.MemoryManager
    llmClient     *LLMClient
}

func (s *SorenAI) Chat(ctx context.Context, userID, sessionID, message string) (string, error) {
    // 1. 召回相关记忆
    memories, err := s.memoryManager.RecallRelevantMemories(
        ctx, userID, sessionID, message, 10,
    )
    if err != nil {
        return "", err
    }

    // 2. 构建包含记忆的提示词
    prompt := s.buildPromptWithMemories(message, memories)

    // 3. 调用 LLM
    response, err := s.llmClient.Complete(ctx, prompt)
    if err != nil {
        return "", err
    }

    // 4. 存储本轮对话
    go func() {
        s.memoryManager.StoreConversation(context.Background(),
            userID, sessionID, message, getCurrentTurn())
        s.memoryManager.StoreConversation(context.Background(),
            userID, sessionID, response, getCurrentTurn()+1)
    }()

    return response, nil
}

func (s *SorenAI) buildPromptWithMemories(message string, memories []*memory.Memory) string {
    var builder strings.Builder

    builder.WriteString("## 相关记忆\n")
    for _, mem := range memories {
        builder.WriteString(fmt.Sprintf("- %s\n", mem.Summary))
    }
    builder.WriteString("\n## 当前对话\n")
    builder.WriteString(message)

    return builder.String()
}
```

## 8. 性能优化建议

### 8.1 索引优化

- 为高频查询路径建立复合索引
- 使用部分索引过滤已删除记忆
- 定期 VACUUM 和 ANALYZE

### 8.2 缓存策略

- Redis 缓存热点记忆（访问次数 > 10）
- 会话级缓存最近 20 条记忆
- 用户级缓存重要记忆列表

### 8.3 查询优化

- 使用 LIMIT 限制召回数量
- 并发执行多路召回
- 使用连接池管理数据库连接

### 8.4 存储优化

- 对长文本内容进行压缩
- 定期归档旧记忆
- 使用分区表管理大量数据

## 9. 监控与指标

### 9.1 关键指标

- 召回延迟 P50/P95/P99
- 召回准确率（需要人工标注）
- 记忆存储量增长速率
- Redis 缓存命中率
- 数据库慢查询

### 9.2 告警规则

- 召回延迟 P95 > 100ms
- Redis 内存使用 > 80%
- PostgreSQL 连接数 > 80%
- 记忆存储增长异常

## 10. 总结与展望

### 10.1 设计优势

1. **低延迟**：基于时序索引和简单标签，避免向量计算开销
2. **成本可控**：使用 PostgreSQL + Redis，无需额外向量数据库
3. **易于实现**：架构简单，便于快速迭代
4. **可扩展**：支持水平扩展和功能扩展

### 10.2 未来优化方向

1. **语义理解**：引入轻量级语义相似度（如 Sentence Embeddings）
2. **主动遗忘**：根据用户反馈主动删除错误记忆
3. **记忆图谱**：建立记忆之间的关联关系
4. **个性化调优**：根据用户行为动态调整召回策略

### 10.3 与 Muse 的对比

| 维度 | Muse | Soren |
|------|------|-------|
| 存储引擎 | 时序数据库 | PostgreSQL + Redis |
| 索引方式 | 时序 + 标签 | 时序 + 标签 + 重要性 |
| 召回策略 | 时间窗口为主 | 多路召回 + 重排序 |
| 语言生态 | - | Go 语言实现 |
| 扩展性 | 中等 | 高（PostgreSQL 扩展性强） |

---

**文档版本**: v1.0
**创建日期**: 2026-10-10
**作者**: Claude for Soren AI
**审核状态**: 待审核
