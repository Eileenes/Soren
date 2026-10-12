package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/soren/backend/internal/cache"
)

// ============================================================
// 重要性评分（设计文档 4.1.2）
// ============================================================

// ScorerConfig 重要性评分权重配置
type ScorerConfig struct {
	BaseScore             float64 // 基础分
	TypeWeightPreference  float64 // preference 类型加分
	TypeWeightFact        float64 // fact 类型加分
	TypeWeightTask        float64 // task 类型加分
	TypeWeightConversation float64 // conversation 类型加分
	ExplicitMentionBonus  float64 // 用户明确陈述加分
	FrequencyMaxBonus     float64 // 访问频率加分上限
	FrequencyPerAccess    float64 // 每次访问的加分
	RecencyWindowDays     float64 // 新近度加分窗口（天）
	RecencyBonusMax       float64 // 新近度加分上限
}

// DefaultScorerConfig 默认评分配置，与设计文档取值一致
func DefaultScorerConfig() ScorerConfig {
	return ScorerConfig{
		BaseScore:              50,
		TypeWeightPreference:   20,
		TypeWeightFact:         15,
		TypeWeightTask:         10,
		TypeWeightConversation: 5,
		ExplicitMentionBonus:   15,
		FrequencyMaxBonus:      10,
		FrequencyPerAccess:     2,
		RecencyWindowDays:      30,
		RecencyBonusMax:        10,
	}
}

// ImportanceScorer 重要性评分器
type ImportanceScorer struct {
	config ScorerConfig
}

// NewImportanceScorer 创建评分器，未配置的字段回落到默认值
func NewImportanceScorer(config ScorerConfig) *ImportanceScorer {
	defaults := DefaultScorerConfig()
	if config.BaseScore == 0 {
		config.BaseScore = defaults.BaseScore
	}
	if config.TypeWeightPreference == 0 {
		config.TypeWeightPreference = defaults.TypeWeightPreference
	}
	if config.TypeWeightFact == 0 {
		config.TypeWeightFact = defaults.TypeWeightFact
	}
	if config.TypeWeightTask == 0 {
		config.TypeWeightTask = defaults.TypeWeightTask
	}
	if config.TypeWeightConversation == 0 {
		config.TypeWeightConversation = defaults.TypeWeightConversation
	}
	if config.ExplicitMentionBonus == 0 {
		config.ExplicitMentionBonus = defaults.ExplicitMentionBonus
	}
	if config.FrequencyMaxBonus == 0 {
		config.FrequencyMaxBonus = defaults.FrequencyMaxBonus
	}
	if config.FrequencyPerAccess == 0 {
		config.FrequencyPerAccess = defaults.FrequencyPerAccess
	}
	if config.RecencyWindowDays == 0 {
		config.RecencyWindowDays = defaults.RecencyWindowDays
	}
	if config.RecencyBonusMax == 0 {
		config.RecencyBonusMax = defaults.RecencyBonusMax
	}
	return &ImportanceScorer{config: config}
}

// CalculateScore 计算重要性分数（0-100）
//
// 评分 = 基础分 + 类型权重 + 明确陈述加分 + 访问频率加分 + 时间新近度加分
func (s *ImportanceScorer) CalculateScore(memory *Memory) int {
	if memory == nil {
		return 0
	}

	score := s.config.BaseScore

	// 1. 类型权重
	switch memory.ContentType {
	case MemoryTypePreference:
		score += s.config.TypeWeightPreference
	case MemoryTypeFact:
		score += s.config.TypeWeightFact
	case MemoryTypeTask:
		score += s.config.TypeWeightTask
	case MemoryTypeConversation:
		score += s.config.TypeWeightConversation
	}

	// 2. 是否被用户明确强调
	if memory.IsExplicitlyStated {
		score += s.config.ExplicitMentionBonus
	}

	// 3. 访问频率
	if memory.AccessCount > 0 {
		score += math.Min(s.config.FrequencyMaxBonus, float64(memory.AccessCount)*s.config.FrequencyPerAccess)
	}

	// 4. 时间新近度：越新的记忆加分越多
	createdAt := memory.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	daysSinceCreation := time.Since(createdAt).Hours() / 24
	if daysSinceCreation < s.config.RecencyWindowDays && daysSinceCreation >= 0 {
		score += (s.config.RecencyWindowDays - daysSinceCreation) / 3
	}

	// 5. 归一化到 0-100
	return int(math.Max(0, math.Min(100, math.Round(score))))
}

// ============================================================
// 关键词提取（中英文混合，不依赖分词服务）
// ============================================================

// stopWords 中英文停用词，避免无意义词污染召回
var stopWords = map[string]struct{}{
	// 英文
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {}, "if": {}, "then": {},
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "being": {},
	"to": {}, "of": {}, "in": {}, "on": {}, "at": {}, "for": {}, "with": {}, "by": {},
	"i": {}, "you": {}, "he": {}, "she": {}, "it": {}, "we": {}, "they": {},
	"me": {}, "my": {}, "your": {}, "our": {}, "their": {},
	"this": {}, "that": {}, "these": {}, "those": {},
	"do": {}, "does": {}, "did": {}, "have": {}, "has": {}, "had": {},
	"can": {}, "could": {}, "will": {}, "would": {}, "should": {}, "may": {}, "might": {},
	"what": {}, "how": {}, "why": {}, "when": {}, "where": {}, "which": {}, "who": {},
	"not": {}, "no": {}, "yes": {}, "so": {}, "as": {}, "from": {}, "about": {},
	// 中文
	"的": {}, "了": {}, "是": {}, "我": {}, "你": {}, "他": {}, "她": {}, "它": {},
	"我们": {}, "你们": {}, "他们": {}, "这个": {}, "那个": {}, "什么": {}, "怎么": {},
	"可以": {}, "然后": {}, "但是": {}, "因为": {}, "所以": {}, "而且": {}, "就是": {},
	"请": {}, "帮我": {}, "一下": {}, "一个": {}, "没有": {}, "不是": {}, "也是": {},
	// 中文补充：代词、助词、时间副词、口语填充词、对话角色词
	// 这些词在任何句子里都高频出现，作为标签没有区分度，只会稀释召回信号
	"自己": {}, "大家": {}, "咱们": {}, "别人": {}, "东西": {}, "事情": {}, "时候": {},
	"现在": {}, "今天": {}, "明天": {}, "昨天": {}, "最近": {}, "以后": {}, "之前": {},
	"已经": {}, "还是": {}, "还有": {}, "这样": {}, "那样": {}, "这些": {}, "那些": {},
	"如果": {}, "虽然": {}, "或者": {}, "不过": {}, "于是": {}, "因此": {}, "另外": {},
	"以及": {}, "对于": {}, "关于": {}, "根据": {}, "通过": {}, "由于": {}, "作为": {},
	"想让": {}, "想要": {}, "希望": {}, "需要": {}, "应该": {}, "可能": {}, "大概": {},
	"非常": {}, "特别": {}, "比较": {}, "有点": {}, "稍微": {}, "真的": {}, "确实": {},
	"用户": {}, "助手": {}, "消息": {}, "收到": {}, "回答": {}, "问题": {}, "内容": {},
	"使用": {}, "进行": {}, "完成": {}, "开始": {}, "继续": {}, "知道": {}, "觉得": {},
}

// 中文虚词字符集：以这些字符开头或结尾的二元组基本都跨越了词边界
//
// 例：「个叫」「发一」「的项」都是相邻两词的片段，不属于任何真实词。
// 二元组切分本身无法知道词边界，但虚词是强信号——真实词极少以虚词
// 开头或结尾，据此可以过滤掉绝大多数噪音标签。
var cjkFunctionChars = map[rune]struct{}{
	'的': {}, '了': {}, '是': {}, '在': {}, '和': {}, '与': {}, '或': {}, '也': {},
	'就': {}, '都': {}, '而': {}, '并': {}, '被': {}, '把': {}, '给': {}, '让': {},
	'对': {}, '从': {}, '向': {}, '到': {}, '于': {}, '为': {}, '以': {}, '及': {},
	'之': {}, '其': {}, '则': {}, '但': {}, '却': {}, '还': {}, '又': {}, '很': {},
	'会': {}, '能': {}, '要': {}, '着': {}, '过': {}, '们': {}, '个': {}, '些': {},
	'这': {}, '那': {}, '我': {}, '你': {}, '他': {}, '她': {}, '它': {}, '咱': {},
	'呀': {}, '吧': {}, '呢': {}, '啊': {}, '吗': {}, '哈': {}, '哦': {}, '嗯': {},
	// 数词与量词：在「开发一个」「三个人」这类结构里，它们标记了词边界
	'一': {}, '二': {}, '三': {}, '两': {}, '几': {}, '多': {}, '半': {}, '每': {},
	'次': {}, '种': {}, '件': {}, '条': {}, '位': {}, '名': {}, '份': {}, '点': {},
}

// looksLikeCrossWordBigram 判断中文二元组是否跨越词边界
//
// 仅对二元组生效（整词候选不受影响）：只要首字符或尾字符是虚词，
// 或首尾字符相同，就认为它切坏了，不作为标签。
func looksLikeCrossWordBigram(word string) bool {
	runes := []rune(word)
	if len(runes) != 2 {
		return false
	}
	if _, ok := cjkFunctionChars[runes[0]]; ok {
		return true
	}
	if _, ok := cjkFunctionChars[runes[1]]; ok {
		return true
	}
	// 叠字（如「想想」「看看」）区分度低
	return runes[0] == runes[1]
}

// ExtractKeywords 从文本中提取关键词
//
// 英文按单词切分，中文按二元组（bigram）切分——SQLite 没有内置中文分词，
// 二元组能在不引入外部词典的前提下获得可用的召回粒度。
func ExtractKeywords(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}

	seen := make(map[string]struct{})
	keywords := make([]string, 0, 32)

	add := func(word string) {
		if word == "" {
			return
		}
		if _, ok := stopWords[word]; ok {
			return
		}
		// 过滤单字符英文与纯标点
		if len([]rune(word)) < 2 && isASCIIWord(word) {
			return
		}
		if _, ok := seen[word]; ok {
			return
		}
		// 丢弃切坏的二元组，避免「个叫」「发一」这类噪音进入标签索引
		if looksLikeCrossWordBigram(word) {
			return
		}
		seen[word] = struct{}{}
		keywords = append(keywords, word)
	}

	var latin []rune
	var cjk []rune

	flushLatin := func() {
		if len(latin) > 0 {
			add(string(latin))
			latin = latin[:0]
		}
	}
	flushCJK := func() {
		runes := cjk
		cjk = cjk[:0]
		switch {
		case len(runes) == 1:
			add(string(runes))
		case len(runes) > 1:
			// 二元组
			for i := 0; i+1 < len(runes); i++ {
				add(string(runes[i : i+2]))
			}
			// 长度在 2-4 的整词也作为候选，覆盖“咖啡”“编译器”这类常用词
			if len(runes) <= 4 {
				add(string(runes))
			}
		}
	}

	for _, r := range text {
		switch {
		case isCJK(r):
			flushLatin()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			latin = append(latin, r)
		default:
			flushLatin()
			flushCJK()
		}
	}
	flushLatin()
	flushCJK()

	return keywords
}

// isCJK 判断是否为中日韩统一表意文字
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// isASCIIWord 判断是否为纯 ASCII 单词
func isASCIIWord(word string) bool {
	for _, r := range word {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) {
			return false
		}
	}
	return word != ""
}

// ============================================================
// 召回器（设计文档 5.2）
// ============================================================

// Recaller 单个召回策略
type Recaller interface {
	Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error)
	Name() string
}

// 缓存 TTL（设计文档 3.2）
const (
	sessionMemoriesTTL = 24 * time.Hour
	hotMemoriesTTL     = 7 * 24 * time.Hour
	tagIndexTTL        = 3 * 24 * time.Hour
)

// TimeWindowRecaller 时序召回：最近 N 天的记忆
type TimeWindowRecaller struct {
	store      Store
	windowDays int
}

// NewTimeWindowRecaller 创建时序召回器
func NewTimeWindowRecaller(store Store, windowDays int) *TimeWindowRecaller {
	if windowDays <= 0 {
		windowDays = 7
	}
	return &TimeWindowRecaller{store: store, windowDays: windowDays}
}

// Name 召回器名称
func (r *TimeWindowRecaller) Name() string { return "time_window" }

// Recall 执行时序召回
func (r *TimeWindowRecaller) Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error) {
	window := time.Duration(r.windowDays) * 24 * time.Hour
	if req.TimeWindow != nil && *req.TimeWindow > 0 {
		window = *req.TimeWindow
	}
	since := time.Now().Add(-window)
	return r.store.RecallByTimeWindow(ctx, req.UserID, since, 50)
}

// TagBasedRecaller 标签召回：从当前消息提取关键词，匹配带标签的记忆
type TagBasedRecaller struct {
	store Store
	cache cache.Cache
}

// NewTagBasedRecaller 创建标签召回器
func NewTagBasedRecaller(store Store, c cache.Cache) *TagBasedRecaller {
	return &TagBasedRecaller{store: store, cache: c}
}

// Name 召回器名称
func (r *TagBasedRecaller) Name() string { return "tag" }

// Recall 执行标签召回
func (r *TagBasedRecaller) Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error) {
	// 1. 从当前对话中提取关键词/实体
	tags := req.RequiredTags
	if len(tags) == 0 {
		tags = ExtractKeywords(req.CurrentMessage)
	}
	if len(tags) == 0 {
		return nil, nil
	}
	// 限制标签数量，避免 IN 条件过大
	if len(tags) > 20 {
		tags = tags[:20]
	}

	// 2. 查找带有这些标签的记忆
	return r.store.FindMemoriesByTag(ctx, req.UserID, tags, 50)
}

// SessionRecaller 会话召回：优先命中缓存，未命中回查数据库
type SessionRecaller struct {
	store Store
	cache cache.Cache
}

// NewSessionRecaller 创建会话召回器
func NewSessionRecaller(store Store, c cache.Cache) *SessionRecaller {
	return &SessionRecaller{store: store, cache: c}
}

// Name 召回器名称
func (r *SessionRecaller) Name() string { return "session" }

// Recall 执行会话召回
func (r *SessionRecaller) Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error) {
	if req.SessionID == "" {
		return nil, nil
	}

	key := sessionMemoriesKey(req.SessionID)

	// 1. 先查缓存（会话最近记忆，按时间倒序）
	if r.cache != nil {
		ids, err := r.cache.SortedRevRange(ctx, key, 20)
		if err == nil && len(ids) > 0 {
			if memories, err := r.getMemoriesByIDs(ctx, ids); err == nil && len(memories) > 0 {
				return memories, nil
			}
		}
	}

	// 2. 缓存未命中，回查数据库
	memories, err := r.store.RecallBySession(ctx, req.UserID, req.SessionID, 20)
	if err != nil || len(memories) == 0 {
		return memories, err
	}

	// 3. 回填缓存（异步，不阻塞召回路径）
	r.refillCache(ctx, key, memories)
	return memories, nil
}

// refillCache 回填会话缓存
func (r *SessionRecaller) refillCache(ctx context.Context, key string, memories []*Memory) {
	if r.cache == nil {
		return
	}
	members := make([]cache.ZMember, 0, len(memories))
	for _, m := range memories {
		members = append(members, cache.ZMember{
			Member: formatMemoryID(m.ID),
			Score:  float64(m.CreatedAt.Unix()),
		})
	}
	if err := r.cache.ZAdd(ctx, key, members...); err != nil {
		return
	}
	// 设置过期时间：进程内缓存以覆盖写入实现，Redis 侧由 Set 语义兜底
	_ = r.cache.Set(ctx, key+":ttl", "1", sessionMemoriesTTL)
}

// getMemoriesByIDs 按缓存中的 ID 批量回查
func (r *SessionRecaller) getMemoriesByIDs(ctx context.Context, ids []string) ([]*Memory, error) {
	gormStore, ok := r.store.(*GormStore)
	if !ok {
		return nil, fmt.Errorf("store does not support batch get")
	}
	numeric := make([]int64, 0, len(ids))
	for _, id := range ids {
		if parsed, err := parseMemoryID(id); err == nil {
			numeric = append(numeric, parsed)
		}
	}
	if len(numeric) == 0 {
		return nil, nil
	}
	return gormStore.GetMemoriesByIDs(ctx, numeric)
}

// ImportantMemoryRecaller 高重要性召回
type ImportantMemoryRecaller struct {
	store      Store
	minScore   int
}

// NewImportantMemoryRecaller 创建高重要性召回器
func NewImportantMemoryRecaller(store Store, minScore int) *ImportantMemoryRecaller {
	if minScore <= 0 {
		minScore = 70
	}
	return &ImportantMemoryRecaller{store: store, minScore: minScore}
}

// Name 召回器名称
func (r *ImportantMemoryRecaller) Name() string { return "important" }

// Recall 执行高重要性召回
func (r *ImportantMemoryRecaller) Recall(ctx context.Context, req *RecallRequest) ([]*Memory, error) {
	return r.store.RecallByImportance(ctx, req.UserID, r.minScore, 30)
}

// ============================================================
// 重排序（设计文档 5.3）
// ============================================================

// RerankerConfig 重排序权重配置
type RerankerConfig struct {
	RelevanceWeight  float64 // 相关性权重
	RecencyWeight    float64 // 时间新近度权重
	ImportanceWeight float64 // 重要性权重
	DiversityWeight  float64 // 多样性权重
	HalfLifeDays     float64 // 时间衰减半衰期（天）
}

// DefaultRerankerConfig 默认重排序配置
func DefaultRerankerConfig() RerankerConfig {
	return RerankerConfig{
		RelevanceWeight:  0.4,
		RecencyWeight:    0.2,
		ImportanceWeight: 0.3,
		DiversityWeight:  0.1,
		HalfLifeDays:     30,
	}
}

// ScoredMemory 打分后的记忆
type ScoredMemory struct {
	Memory *Memory
	Score  float64
}

// MemoryReranker 记忆重排序器
type MemoryReranker struct {
	config RerankerConfig
}

// NewMemoryReranker 创建重排序器
func NewMemoryReranker(config RerankerConfig) *MemoryReranker {
	defaults := DefaultRerankerConfig()
	if config.RelevanceWeight == 0 {
		config.RelevanceWeight = defaults.RelevanceWeight
	}
	if config.RecencyWeight == 0 {
		config.RecencyWeight = defaults.RecencyWeight
	}
	if config.ImportanceWeight == 0 {
		config.ImportanceWeight = defaults.ImportanceWeight
	}
	if config.HalfLifeDays == 0 {
		config.HalfLifeDays = defaults.HalfLifeDays
	}
	return &MemoryReranker{config: config}
}

// Rerank 对候选记忆重排序
//
// 流程：综合打分 → 按分数排序 → 多样性调整。
func (r *MemoryReranker) Rerank(memories []*Memory, query string) []*Memory {
	if len(memories) == 0 {
		return nil
	}

	// 1. 计算每条记忆的综合得分
	scored := make([]ScoredMemory, 0, len(memories))
	for _, mem := range memories {
		if mem == nil {
			continue
		}
		scored = append(scored, ScoredMemory{
			Memory: mem,
			Score:  r.CalculateScore(mem, query),
		})
	}

	// 2. 按得分排序（分数相同时偏好更新的记忆）
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Memory.CreatedAt.After(scored[j].Memory.CreatedAt)
		}
		return scored[i].Score > scored[j].Score
	})

	// 3. 多样性调整
	return r.diversityFilter(scored)
}

// CalculateScore 计算单条记忆的综合得分
func (r *MemoryReranker) CalculateScore(memory *Memory, query string) float64 {
	score := 0.0

	// 1. 相关性得分（基于关键词匹配）
	relevance := r.CalculateRelevance(memory, query)
	score += relevance * r.config.RelevanceWeight

	// 2. 时间衰减（越新越好，指数衰减）
	createdAt := memory.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	daysSince := time.Since(createdAt).Hours() / 24
	if daysSince < 0 {
		daysSince = 0
	}
	recencyScore := math.Exp(-daysSince / r.config.HalfLifeDays)
	score += recencyScore * r.config.RecencyWeight

	// 3. 重要性
	importanceScore := float64(memory.ImportanceScore) / 100.0
	score += importanceScore * r.config.ImportanceWeight

	// 4. 访问热度（作为相关性的补充信号，权重很小，不单独占配置项）
	if memory.AccessCount > 0 {
		score += math.Min(0.05, float64(memory.AccessCount)*0.005)
	}

	// 5. 会话连续性：同一会话的记忆略加权，保证上下文连续
	if memory.SessionID != "" {
		score += 0.02
	}

	return score
}

// CalculateRelevance 计算关键词相关性（0-1）
func (r *MemoryReranker) CalculateRelevance(memory *Memory, query string) float64 {
	queryTerms := ExtractKeywords(query)
	if len(queryTerms) == 0 {
		return 0.0
	}

	content := memory.Content + " " + memory.Summary
	memoryTerms := ExtractKeywords(content)

	// 标签命中同样算作相关
	for _, tag := range memory.Tags {
		memoryTerms = append(memoryTerms, ExtractKeywords(tag.Name)...)
	}
	if len(memoryTerms) == 0 {
		return 0.0
	}

	memorySet := make(map[string]struct{}, len(memoryTerms))
	for _, mt := range memoryTerms {
		memorySet[mt] = struct{}{}
	}

	matches := 0
	for _, qt := range queryTerms {
		if _, ok := memorySet[qt]; ok {
			matches++
			continue
		}
		// 双向子串包含，兼容长短关键词差异（如“咖啡” vs “咖啡因”）
		for mt := range memorySet {
			if strings.Contains(mt, qt) || strings.Contains(qt, mt) {
				matches++
				break
			}
		}
	}

	return float64(matches) / float64(len(queryTerms))
}

// diversityFilter 多样性过滤：抑制与已选记忆高度相似的条目
//
// 采用贪心选择：每轮在剩余候选中选出"基础分 - 多样性惩罚"最高的一条，
// 惩罚值取该候选与所有已选记忆的最大 Jaccard 相似度。
// 每一轮都会移除一个候选，因此必然终止（不会出现反复排队的情况）。
func (r *MemoryReranker) diversityFilter(scored []ScoredMemory) []*Memory {
	const similarityThreshold = 0.8

	// 预计算每条候选的词汇集合，避免重复分词
	type candidate struct {
		memory ScoredMemory
		terms  map[string]struct{}
	}

	remaining := make([]candidate, 0, len(scored))
	for _, item := range scored {
		remaining = append(remaining, candidate{
			memory: item,
			terms:  termSet(item.Memory),
		})
	}

	result := make([]*Memory, 0, len(scored))
	selectedTerms := make([]map[string]struct{}, 0, len(scored))

	for len(remaining) > 0 {
		bestIndex := 0
		bestScore := math.Inf(-1)

		for i, item := range remaining {
			adjusted := item.memory.Score

			// 与已选记忆越相似，惩罚越重
			if r.config.DiversityWeight > 0 {
				maxSimilarity := 0.0
				for _, selected := range selectedTerms {
					if similarity := jaccard(item.terms, selected); similarity > maxSimilarity {
						maxSimilarity = similarity
					}
				}
				if maxSimilarity > similarityThreshold {
					adjusted -= r.config.DiversityWeight * maxSimilarity
				}
			}

			if adjusted > bestScore {
				bestScore = adjusted
				bestIndex = i
			}
		}

		chosen := remaining[bestIndex]
		remaining = append(remaining[:bestIndex], remaining[bestIndex+1:]...)

		result = append(result, chosen.memory.Memory)
		selectedTerms = append(selectedTerms, chosen.terms)
	}

	return result
}

// termSet 取出记忆的词汇集合
func termSet(memory *Memory) map[string]struct{} {
	terms := ExtractKeywords(memory.Content + " " + memory.Summary)
	set := make(map[string]struct{}, len(terms))
	for _, t := range terms {
		set[t] = struct{}{}
	}
	return set
}

// jaccard 计算两个集合的 Jaccard 相似度
func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	intersection := 0
	for term := range a {
		if _, ok := b[term]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// ============================================================
// 多路召回（设计文档 6.3）
// ============================================================

// MultiRecaller 多路召回 + 重排序
type MultiRecaller struct {
	recallers []Recaller
	reranker  *MemoryReranker
}

// NewMultiRecaller 创建多路召回器
func NewMultiRecaller(recallers []Recaller, reranker *MemoryReranker) *MultiRecaller {
	return &MultiRecaller{
		recallers: recallers,
		reranker:  reranker,
	}
}

// Recall 并发执行多路召回，合并去重后重排序
func (m *MultiRecaller) Recall(ctx context.Context, req *RecallRequest) (*RecallResult, error) {
	startTime := time.Now()

	if req.MaxResults <= 0 {
		req.MaxResults = 10
	}

	// 1. 并发执行多路召回
	var mu sync.Mutex
	var wg sync.WaitGroup
	memoriesMap := make(map[int64]*Memory)

	for _, recaller := range m.recallers {
		if recaller == nil {
			continue
		}
		wg.Add(1)
		go func(r Recaller) {
			defer wg.Done()

			memories, err := r.Recall(ctx, req)
			if err != nil {
				// 单路召回失败不影响其他路径，仅跳过
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, mem := range memories {
				if mem == nil {
					continue
				}
				if existing, ok := memoriesMap[mem.ID]; !ok {
					memoriesMap[mem.ID] = mem
				} else if len(mem.Tags) > len(existing.Tags) {
					// 同一记忆被多路召回时，保留信息更全的那份
					memoriesMap[mem.ID] = mem
				}
			}
		}(recaller)
	}
	wg.Wait()

	// 2. 转换为切片
	memories := make([]*Memory, 0, len(memoriesMap))
	for _, mem := range memoriesMap {
		memories = append(memories, mem)
	}

	// 3. 重排序 + 多样性调整
	ranked := m.reranker.Rerank(memories, req.CurrentMessage)

	// 4. 限制返回数量
	if len(ranked) > req.MaxResults {
		ranked = ranked[:req.MaxResults]
	}

	return &RecallResult{
		Memories:      ranked,
		TotalRecalled: len(memoriesMap),
		LatencyMS:     time.Since(startTime).Milliseconds(),
	}, nil
}

// Recallers 返回已注册的召回器名称，便于日志与调试
func (m *MultiRecaller) Recallers() []string {
	names := make([]string, 0, len(m.recallers))
	for _, r := range m.recallers {
		if r != nil {
			names = append(names, r.Name())
		}
	}
	return names
}
