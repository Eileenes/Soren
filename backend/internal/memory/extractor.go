package memory

import (
	"regexp"
	"strings"
)

// 偏好与事实的启发式模式
//
// 桌面端单用户场景下，逐条调用 LLM 提取记忆既慢又贵，
// 这里采用规则抽取兜底，保证"至少存对话摘要、用户偏好"这一要求
// 在不增加额外模型调用的前提下成立。
var (
	// 偏好表达
	preferencePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)我(?:很|更|最|比较|非常)?(?:喜欢|喜欢上|爱|偏好|倾向于|习惯(?:于)?|希望|想要|需要)\s*([^。！？!?\n；;]{1,60})`),
		regexp.MustCompile(`(?i)(?:请|以后|下次|今后)\s*(?:一直|都|总是|尽量)?\s*(?:用|使用|采用|按|以)\s*([^。！？!?\n；;]{1,60})`),
		regexp.MustCompile(`(?i)(?:不要|别|不喜欢|讨厌|反感)\s*([^。！？!?\n；;]{1,60})`),
	}

	// 事实表达
	factPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)我(?:是|叫|的名字是|是一名?|在|住在|来自)\s*([^。！？!?\n；;]{1,60})`),
		regexp.MustCompile(`(?i)我的([^。！？!?\n；;]{1,30}?)(?:是|叫|为)\s*([^。！？!?\n；;]{1,40})`),
		regexp.MustCompile(`(?i)我(?:正在|打算|计划)(?:学习|研究|开发|做)\s*([^。！？!?\n；;]{1,60})`),
	}

	// 任务表达
	taskPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:帮我|请|需要你)\s*([^。！？!?\n；;]{1,60})`),
		regexp.MustCompile(`(?i)(?:记得|提醒我|待办|todo)\s*[:：]?\s*([^。！？!?\n；;]{1,60})`),
	}

	// 话题标签：英文单词与中文词组
	englishWordRegex = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_+#.\-]{1,24}`)

	// 明确强调的语气词
	explicitMarkers = []string{
		"记住", "请记住", "一定要", "务必", "非常重要", "很重要", "以后都", "永远",
	}
)

// ExtractPreferences 从用户消息中抽取偏好
//
// 返回 key/value 对，key 为归一化后的主题词（用作摘要与标签），
// value 为偏好内容原文。
func ExtractPreferences(message string) []Preference {
	if strings.TrimSpace(message) == "" {
		return nil
	}

	var preferences []Preference
	seen := make(map[string]struct{})

	for _, pattern := range preferencePatterns {
		for _, match := range pattern.FindAllStringSubmatch(message, -1) {
			if len(match) < 2 {
				continue
			}
			// Keep the complete statement, including negation and the requested
			// action. Saving only match[1] would turn “不要用英文” into “用英文”.
			value := cleanFragment(match[0])
			if value == "" {
				continue
			}

			key := preferenceKey(value)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			preferences = append(preferences, Preference{
				Key:   key,
				Value: value,
			})
		}
	}

	return preferences
}

// Preference 抽取出的用户偏好
type Preference struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ExtractFacts 从用户消息中抽取事实
func ExtractFacts(message string) []string {
	if strings.TrimSpace(message) == "" {
		return nil
	}

	var facts []string
	seen := make(map[string]struct{})

	for _, pattern := range factPatterns {
		for _, match := range pattern.FindAllStringSubmatch(message, -1) {
			// 最后一个捕获组为事实内容
			fragment := cleanFragment(match[len(match)-1])
			if fragment == "" {
				continue
			}
			if _, ok := seen[fragment]; ok {
				continue
			}
			seen[fragment] = struct{}{}
			facts = append(facts, fragment)
		}
	}

	return facts
}

// ExtractTasks 从用户消息中抽取任务
func ExtractTasks(message string) []string {
	if strings.TrimSpace(message) == "" {
		return nil
	}

	var tasks []string
	seen := make(map[string]struct{})

	for _, pattern := range taskPatterns {
		for _, match := range pattern.FindAllStringSubmatch(message, -1) {
			fragment := cleanFragment(match[1])
			if fragment == "" {
				continue
			}
			if _, ok := seen[fragment]; ok {
				continue
			}
			seen[fragment] = struct{}{}
			tasks = append(tasks, fragment)
		}
	}

	return tasks
}

// IsExplicitlyStated 判断消息是否包含明确强调
func IsExplicitlyStated(message string) bool {
	lowered := strings.ToLower(message)
	for _, marker := range explicitMarkers {
		if strings.Contains(lowered, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

// GenerateSummary 生成记忆摘要（50-100 字，超出则截断）
//
// 单句内容直接返回；多句内容取首句，保证摘要保留核心信息。
func GenerateSummary(content string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 100
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}

	// 压缩空白
	trimmed = strings.Join(strings.Fields(trimmed), " ")

	// 取第一句
	for _, sep := range []string{"。", "！", "？", "\n", ". ", "! ", "? "} {
		if idx := strings.Index(trimmed, sep); idx > 0 {
			trimmed = trimmed[:idx]
			break
		}
	}

	runes := []rune(trimmed)
	if len(runes) <= maxLen {
		return trimmed
	}
	return string(runes[:maxLen]) + "…"
}

// ExtractTags 从记忆内容中提取标签（话题/实体）
func ExtractTags(content string) []string {
	keywords := ExtractKeywords(content)

	tags := make([]string, 0, 8)
	seen := make(map[string]struct{})

	// 英文单词优先（实体、技术名词）
	for _, word := range englishWordRegex.FindAllString(content, -1) {
		word = strings.ToLower(strings.Trim(word, ".-"))
		if len(word) < 2 {
			continue
		}
		if _, ok := stopWords[word]; ok {
			continue
		}
		if _, ok := seen[word]; ok {
			continue
		}
		seen[word] = struct{}{}
		tags = append(tags, word)
		if len(tags) >= 5 {
			break
		}
	}

	// 再用中文关键词补足
	for _, keyword := range keywords {
		if len(tags) >= 8 {
			break
		}
		// 只保留中文词/二元组，长度控制在 2-6 之间
		if !containsCJK(keyword) {
			continue
		}
		runes := []rune(keyword)
		if len(runes) < 2 || len(runes) > 6 {
			continue
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		tags = append(tags, keyword)
	}

	return tags
}

// containsCJK 判断字符串是否包含中文字符
func containsCJK(s string) bool {
	for _, r := range s {
		if isCJK(r) {
			return true
		}
	}
	return false
}

// cleanFragment 清洗抽取出的片段
func cleanFragment(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	fragment = strings.Trim(fragment, "，,。.、:：;；!！?？\"'“”‘’()（）[]【】 \t")
	fragment = strings.Join(strings.Fields(fragment), " ")
	if len([]rune(fragment)) < 2 {
		return ""
	}
	return fragment
}

// preferenceTopics 偏好主题识别规则
//
// 键应当表达「这是关于什么的偏好」，而不是偏好内容的截断——
// 否则短偏好会出现 key == value，界面渲染成「X: X」的重复。
// 按主题归并还有额外收益：同一主题的新偏好会覆盖旧偏好，
// 而不是让「回答要简洁」和「别太啰嗦」长期并存互相矛盾。
var preferenceTopics = []struct {
	topic    string
	keywords []string
}{
	{"回答风格", []string{"简洁", "简短", "啰嗦", "详细", "详细点", "精简", "长", "短", "直接", "重点"}},
	{"语言偏好", []string{"中文", "英文", "英语", "日文", "语言", "翻译"}},
	{"表达形式", []string{"表情", "emoji", "符号", "代码块", "表格", "列表", "分点", "格式", "markdown"}},
	{"技术栈", []string{"go", "golang", "rust", "python", "java", "typescript", "vue", "react", "docker", "k8s"}},
	{"工作节奏", []string{"提醒", "通知", "提前", "每天", "每周", "定时", "计划"}},
	{"沟通方式", []string{"示例", "例子", "解释", "原理", "背景", "原因", "为什么"}},
}

// preferenceKey 从偏好内容推导主题键名
//
// 先按主题规则归类（覆盖绝大多数日常偏好），无法归类时退回
// 内容摘要作为键，保证仍然能稳定归并同主题偏好。
func preferenceKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	lowered := strings.ToLower(value)
	for _, rule := range preferenceTopics {
		for _, kw := range rule.keywords {
			if strings.Contains(lowered, kw) {
				return rule.topic
			}
		}
	}

	// 未能归入已知主题：用截断内容作为兜底键，并加前缀以区分
	// 「主题键」与「内容键」，避免界面上出现纯重复的 key: value
	runes := []rune(value)
	if len(runes) > 12 {
		value = string(runes[:12]) + "…"
	}
	return "其他偏好（" + value + "）"
}
