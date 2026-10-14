package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/cache"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/group"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/memory"
	"github.com/soren/backend/internal/middleware"
	"github.com/soren/backend/internal/models"
	"github.com/soren/backend/internal/repository"
	"github.com/soren/backend/internal/store"
	"gorm.io/gorm"
)

// groupTestProvider 记录每次调用的上下文并按脚本作答
type groupTestProvider struct {
	mu     sync.Mutex
	calls  [][]llm.Message
	models []string

	// reply 按调用序号（第 0 次是第一次）返回内容或错误
	reply func(call int) (string, error)
	// release 非 nil 时，从第 blockFrom 次调用起阻塞到它被关闭或请求结束
	release   chan struct{}
	blockFrom int
}

func (p *groupTestProvider) Chat(ctx context.Context, messages []llm.Message, options *llm.Options) (*llm.Response, error) {
	p.mu.Lock()
	index := len(p.calls)
	p.calls = append(p.calls, append([]llm.Message(nil), messages...))
	p.models = append(p.models, options.Model)
	p.mu.Unlock()

	if p.release != nil && index >= p.blockFrom {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.release:
		}
	}
	if p.reply != nil {
		content, err := p.reply(index)
		if err != nil {
			return nil, err
		}
		return &llm.Response{Content: content, Model: "test"}, nil
	}
	return &llm.Response{Content: fmt.Sprintf("发言%d", index+1), Model: "test"}, nil
}

func (p *groupTestProvider) StreamChat(ctx context.Context, messages []llm.Message, options *llm.Options, stream chan<- llm.StreamChunk) error {
	return nil
}

func (p *groupTestProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

func (p *groupTestProvider) callMessages(index int) []llm.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	if index >= len(p.calls) {
		return nil
	}
	return p.calls[index]
}

func (p *groupTestProvider) callModel(index int) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if index >= len(p.models) {
		return ""
	}
	return p.models[index]
}

// hasContent 判断某次调用的上下文里是否出现过指定文本
func (p *groupTestProvider) hasContent(index int, needle string) bool {
	for _, message := range p.callMessages(index) {
		if strings.Contains(message.Content, needle) {
			return true
		}
	}
	return false
}

type groupHarness struct {
	router   *gin.Engine
	db       *gorm.DB
	agents   *agent.Service
	orch     *group.Orchestrator
	groups   *repository.GroupRepository
	convRepo *repository.ConversationRepository
	memory   *memory.MemoryManager
	provider *groupTestProvider
}

// setupGroupHarness 搭建群聊路由
//
// 数据库走 store.OpenDB，会把连接池压到 SetMaxOpenConns(1)——
// 群落库如果误用嵌套事务，这里会直接暴露成阻塞。
// 路由挂在没有任何中间件的 gin 引擎上，因此"不带 X-Bot-ID 也能返回"
// 才能真正证明绕过了 BotContext，而不是被中间件兜底。
func setupGroupHarness(t *testing.T, provider *groupTestProvider, withMemory bool) *groupHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := store.OpenDB(&config.Config{
		StorageDriver: config.StorageDriverSQLite,
		SQLitePath:    filepath.Join(t.TempDir(), "group_test.db"),
	})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{},
		&models.GroupChat{}, &models.GroupMember{}, &models.GroupTurn{}); err != nil {
		t.Fatalf("迁移测试数据库失败: %v", err)
	}

	var manager *memory.MemoryManager
	if withMemory {
		cacheClient := cache.NewMemoryCache(100)
		opts := memory.DefaultManagerOptions()
		opts.Cache = cacheClient
		if manager, err = memory.NewMemoryManager(db, cacheClient, opts); err != nil {
			t.Fatalf("创建记忆管理器失败: %v", err)
		}
	}

	agents, err := agent.New(db, provider, manager, "test-model", false)
	if err != nil {
		t.Fatalf("创建 Bot 运行时失败: %v", err)
	}

	convRepo := repository.NewConversationRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	orch := group.New(agents, convRepo, groupRepo, provider, manager)

	router := gin.New()
	RegisterGroups(router, NewGroupHandler(orch, groupRepo, convRepo, agents))

	return &groupHarness{
		router: router, db: db, agents: agents, orch: orch,
		groups: groupRepo, convRepo: convRepo, memory: manager, provider: provider,
	}
}

// createGroupBot 建一个可用 Bot
func createGroupBot(t *testing.T, h *groupHarness, name string, mutate func(*agent.Bot)) string {
	t.Helper()
	bot := agent.Bot{
		Name:          name,
		Avatar:        "angler",
		Instructions:  name + " 负责的部分",
		MemoryEnabled: true,
		Collaborators: []string{},
	}
	if mutate != nil {
		mutate(&bot)
	}
	if err := h.agents.SaveBot(&bot, true); err != nil {
		t.Fatalf("创建 Bot %q 失败: %v", name, err)
	}
	return bot.ID
}

// createGroup 走真实接口建群，返回群 ID
func createGroup(t *testing.T, h *groupHarness, title string, botIDs []string, mode string, rounds int) string {
	t.Helper()
	recorder := callGroup(t, h, http.MethodPost, "/api/groups", map[string]any{
		"title": title, "botIds": botIDs, "mode": mode, "maxRounds": rounds,
	})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("创建群聊失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var created GroupView
	decodeGroup(t, recorder, &created)
	return created.ID
}

// setGroupPinned 只提交 pinned 字段（局部更新），返回更新后的群
func setGroupPinned(t *testing.T, h *groupHarness, groupID string, pinned bool) GroupView {
	t.Helper()
	recorder := callGroup(t, h, http.MethodPatch, "/api/groups/"+groupID, map[string]any{"pinned": pinned})
	if recorder.Code != http.StatusOK {
		t.Fatalf("更新置顶状态失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var updated GroupView
	decodeGroup(t, recorder, &updated)
	return updated
}

// listGroupIDs 按接口返回顺序取出群 ID，顺序本身就是被断言的对象
func listGroupIDs(t *testing.T, h *groupHarness) []string {
	t.Helper()
	recorder := callGroup(t, h, http.MethodGet, "/api/groups", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("拉群列表失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var views []GroupView
	decodeGroup(t, recorder, &views)
	ids := make([]string, 0, len(views))
	for _, view := range views {
		ids = append(ids, view.ID)
	}
	return ids
}

// storedGroup 读回落库的群行
//
// 断言不能只看接口响应：置顶时间由服务端写库，响应里的值可能来自内存中的结构体。
func storedGroup(t *testing.T, h *groupHarness, groupID string) models.GroupChat {
	t.Helper()
	var chat models.GroupChat
	if err := h.db.First(&chat, "id = ?", groupID).Error; err != nil {
		t.Fatalf("读取群记录失败: %v", err)
	}
	return chat
}

// sameInstant 两个可空时间是否表示同一时刻
//
// 置顶时间来自可空指针，直接调 Equal 会在其一为 nil 时 panic，
// 而"nil 变 nil"恰恰是取消置顶要断言的场景。
func sameInstant(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

// alignGroupTimestamps 直接改写群行的时间列
//
// 用 UpdateColumn 而非 Updates：后者会被 GORM 自动刷新 updated_at，
// 把这里刻意构造的时间关系抹掉。排序测试需要确定的时间，不能依赖驱动的时间精度。
func alignGroupTimestamps(t *testing.T, h *groupHarness, groupID string, values map[string]any) {
	t.Helper()
	for column, value := range values {
		if err := h.db.Model(&models.GroupChat{}).Where("id = ?", groupID).
			UpdateColumn(column, value).Error; err != nil {
			t.Fatalf("改写群 %s 的 %s 失败: %v", groupID, column, err)
		}
	}
}

// callGroup 发请求并设超时上限
//
// 单连接 SQLite 下若落库代码借了第二条连接，测试会以可读的失败结束而不是挂死。
func callGroup(t *testing.T, h *groupHarness, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		request := httptest.NewRequest(method, path, bytes.NewReader(payload))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		h.router.ServeHTTP(recorder, request)
		done <- recorder
	}()
	select {
	case recorder := <-done:
		return recorder
	case <-time.After(30 * time.Second):
		t.Fatalf("请求 %s %s 超时，落库可能存在事务嵌套", method, path)
		return nil
	}
}

// callGroupAsync 在后台发请求，用于并发与中止场景
func callGroupAsync(h *groupHarness, method, path string, body any) <-chan *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		request := httptest.NewRequest(method, path, bytes.NewReader(payload))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		h.router.ServeHTTP(recorder, request)
		done <- recorder
	}()
	return done
}

func speakBody(content string, requestID string, mentions []string, mode string) map[string]any {
	return map[string]any{
		"content": content, "requestId": requestID, "mentions": mentions, "mode": mode,
	}
}

func decodeGroup(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("解析响应失败: %v (%s)", err, recorder.Body.String())
	}
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待超时：%s", what)
}

// storedMessages 读回群对话里已落库的消息
func storedMessages(t *testing.T, h *groupHarness, groupID string) []models.Message {
	t.Helper()
	var conv models.Conversation
	if err := h.db.Where("bot_id = ?", models.GroupBotID(uuid.MustParse(groupID))).First(&conv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		t.Fatalf("读取群对话失败: %v", err)
	}
	messages, err := h.convRepo.GetMessages(context.Background(), conv.ID, 0)
	if err != nil {
		t.Fatalf("读取群消息失败: %v", err)
	}
	return messages
}

func botMemories(t *testing.T, manager *memory.MemoryManager, botID string) []*memory.Memory {
	t.Helper()
	memories, _, err := manager.List(context.Background(), memory.ListOptions{UserID: agent.Owner(botID), Limit: 50})
	if err != nil {
		t.Fatalf("读取记忆失败: %v", err)
	}
	return memories
}

func TestGroupRoutesWorkWithoutBotHeader(t *testing.T) {
	h := setupGroupHarness(t, &groupTestProvider{}, false)
	first := createGroupBot(t, h, "产品", nil)
	second := createGroupBot(t, h, "研发", nil)

	recorder := callGroup(t, h, http.MethodPost, "/api/groups", map[string]any{
		"title": "路线讨论", "botIds": []string{first, second}, "mode": "mention",
	})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("不带 X-Bot-ID 建群失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var created GroupView
	decodeGroup(t, recorder, &created)

	// 路由上若挂着 BotContext，这里的 X-Bot-ID 缺失会直接 404
	if recorder := callGroup(t, h, http.MethodGet, "/api/groups", nil); recorder.Code != http.StatusOK {
		t.Fatalf("不带 X-Bot-ID 拉群列表失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder := callGroup(t, h, http.MethodGet, "/api/groups/"+created.ID, nil); recorder.Code != http.StatusOK {
		t.Fatalf("不带 X-Bot-ID 拉群详情失败 %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestGroupMentionRepliesOncePerMentionedBot(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	first := createGroupBot(t, h, "产品", nil)
	second := createGroupBot(t, h, "研发", nil)
	third := createGroupBot(t, h, "设计", nil)
	id := createGroup(t, h, "接口方案", []string{first, second, third}, "mention", 3)

	recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("@研发 @设计 这个接口怎么设计", uuid.NewString(), []string{third, second}, "mention"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("提及发言失败 %d: %s", recorder.Code, recorder.Body.String())
	}

	var result group.SpeakResult
	decodeGroup(t, recorder, &result)
	if result.Status != "done" {
		t.Fatalf("期望 done，收到 %s", result.Status)
	}

	// 被点名的两个 Bot 各一条，发言人身份互不相同
	speakers := map[string]string{}
	replies := 0
	for _, message := range result.Messages {
		if message.Role != "assistant" {
			continue
		}
		replies++
		if previous, seen := speakers[message.SpeakerBotID]; seen {
			t.Fatalf("Bot %s 重复发言（先前为 %s）", message.SpeakerBotID, previous)
		}
		speakers[message.SpeakerBotID] = message.SpeakerName
	}
	if replies != 2 {
		t.Fatalf("期望 2 条回复，收到 %d", replies)
	}
	if _, ok := speakers[first]; ok {
		t.Error("未被点名的 Bot 不应发言")
	}
	for botID, name := range map[string]string{second: "研发", third: "设计"} {
		if speakers[botID] != name {
			t.Errorf("发言人 %q 记录错误：得到 %q", botID, speakers[botID])
		}
	}
	if provider.callCount() != 2 {
		t.Errorf("提及模式应只调用 2 次模型，实际 %d 次", provider.callCount())
	}
}

func TestGroupAutoModeCallsLLMOncePerBotPerRound(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	ids := []string{
		createGroupBot(t, h, "产品", nil),
		createGroupBot(t, h, "研发", nil),
		createGroupBot(t, h, "设计", nil),
	}
	id := createGroup(t, h, "要不要先做移动端", ids, "auto", 3)

	recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("这个功能要不要先做移动端", uuid.NewString(), nil, "auto"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("自主发言失败 %d: %s", recorder.Code, recorder.Body.String())
	}

	// 3 Bot × 3 轮 = 9 次调用；这里也顺带验证了单连接 SQLite 下不发生事务嵌套
	if got := provider.callCount(); got != 9 {
		t.Fatalf("期望 9 次模型调用，实际 %d 次", got)
	}
	var result group.SpeakResult
	decodeGroup(t, recorder, &result)
	if len(result.Messages) != 1+9 {
		t.Fatalf("期望 10 条消息（用户 1 + 发言 9），收到 %d", len(result.Messages))
	}
	if got := len(storedMessages(t, h, id)); got != 10 {
		t.Errorf("期望落库 10 条消息，实际 %d", got)
	}
}

func TestGroupAutoModeSecondRoundSeesFirstRound(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "排期", ids, "auto", 2)

	recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("先做哪个功能", uuid.NewString(), nil, "auto"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("自主发言失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	if provider.callCount() != 4 {
		t.Fatalf("期望 4 次模型调用，实际 %d", provider.callCount())
	}

	// 第 2 轮第 1 次调用必须能看到第 1 轮两个 Bot 的全部发言
	if !provider.hasContent(2, "发言1") || !provider.hasContent(2, "发言2") {
		t.Errorf("第 2 轮未拿到第 1 轮内容：%v", provider.callMessages(2))
	}
	if !provider.hasContent(3, "发言1") || !provider.hasContent(3, "发言2") {
		t.Errorf("第 2 轮第 2 次未拿到第 1 轮内容：%v", provider.callMessages(3))
	}
}

func TestGroupCancelKeepsProducedMessagesAndExplains(t *testing.T) {
	provider := &groupTestProvider{release: make(chan struct{}), blockFrom: 2}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "长讨论", ids, "auto", 3)

	pending := callGroupAsync(h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("我们来讨论一下", uuid.NewString(), nil, "auto"))
	waitFor(t, "讨论进入第 2 轮", func() bool { return provider.callCount() >= 2 })

	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/cancel", nil); recorder.Code != http.StatusOK {
		t.Fatalf("中止失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	close(provider.release)

	var result group.SpeakResult
	decodeGroup(t, <-pending, &result)
	if result.Status != "cancelled" {
		t.Fatalf("期望 cancelled，收到 %s", result.Status)
	}

	// 已产生的发言必须落库，末尾追加一条系统说明
	stored := storedMessages(t, h, id)
	if len(stored) < 4 {
		t.Fatalf("期望至少保留 3 条发言与 1 条中止说明，实际 %d 条", len(stored))
	}
	last := stored[len(stored)-1]
	if last.Role != "system" || !strings.Contains(last.Content, "中止") {
		t.Errorf("末尾应为中止说明，实际 role=%q content=%q", last.Role, last.Content)
	}
	if last.SpeakerBotID != "" {
		t.Error("中止说明不应带发言人")
	}

	// 响应里的消息与库里一致
	if len(result.Messages) != len(stored) {
		t.Errorf("返回消息数 %d 与落库 %d 不一致", len(result.Messages), len(stored))
	}
	if tail := result.Messages[len(result.Messages)-1]; tail.Role != "system" {
		t.Errorf("响应末尾应为中止说明，实际 role=%q", tail.Role)
	}

	// 没有进行中的讨论时，中止应明确告知而不是静默成功
	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/cancel", nil); recorder.Code != http.StatusConflict {
		t.Errorf("没有进行中的讨论时应返回 409，实际 %d", recorder.Code)
	}
}

func TestGroupResolvesModelPerBot(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	// 群成员可以跨不同服务商，模型解析不能被统一成一个
	first := createGroupBot(t, h, "产品", func(bot *agent.Bot) { bot.Model = "model-for-product" })
	second := createGroupBot(t, h, "研发", nil) // 未配置模型，走默认
	id := createGroup(t, h, "多模型", []string{first, second}, "mention", 3)

	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("各自用什么模型", uuid.NewString(), nil, "mention")); recorder.Code != http.StatusOK {
		t.Fatalf("发言失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := provider.callModel(0); got != "model-for-product" {
		t.Errorf("产品应使用自己的模型，实际 %q", got)
	}
	if got := provider.callModel(1); got != "test-model" {
		t.Errorf("未配置模型的 Bot 应使用默认模型，实际 %q", got)
	}
}

func TestGroupDeleteStopsRunningDiscussionFirst(t *testing.T) {
	provider := &groupTestProvider{release: make(chan struct{})}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "边聊边删", ids, "auto", 3)

	pending := callGroupAsync(h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("先聊起来", uuid.NewString(), nil, "auto"))
	waitFor(t, "讨论进入模型调用", func() bool { return provider.callCount() >= 1 })

	// 编排器还在写消息时删除：必须先中止再清理，否则会往已删除的对话里继续写
	if recorder := callGroup(t, h, http.MethodDelete, "/api/groups/"+id, nil); recorder.Code != http.StatusOK {
		t.Fatalf("删除进行中的群失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	close(provider.release)

	select {
	case recorder := <-pending:
		if recorder.Code >= http.StatusInternalServerError {
			t.Errorf("中止后的请求不应以 5xx 结束，实际 %d: %s", recorder.Code, recorder.Body.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("中止后的发言请求没有结束")
	}

	var conversations int64
	h.db.Model(&models.Conversation{}).Where("bot_id = ?", models.GroupBotID(uuid.MustParse(id))).Count(&conversations)
	if conversations != 0 {
		t.Errorf("删除后应清空群对话，实际 %d 条", conversations)
	}
	var chatRows int64
	h.db.Model(&models.GroupChat{}).Where("id = ?", id).Count(&chatRows)
	if chatRows != 0 {
		t.Errorf("群记录应被删除，实际仍有 %d 条", chatRows)
	}
}

func TestGroupAutoModeSurvivesSingleBotFailure(t *testing.T) {
	provider := &groupTestProvider{}
	provider.reply = func(call int) (string, error) {
		if call == 1 {
			return "", errors.New("上游限流")
		}
		return fmt.Sprintf("发言%d", call+1), nil
	}
	h := setupGroupHarness(t, provider, false)
	ids := []string{
		createGroupBot(t, h, "产品", nil),
		createGroupBot(t, h, "研发", nil),
		createGroupBot(t, h, "设计", nil),
	}
	id := createGroup(t, h, "容错", ids, "auto", 1)

	recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("继续", uuid.NewString(), nil, "auto"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("单个 Bot 失败不应让整场请求失败，实际 %d: %s", recorder.Code, recorder.Body.String())
	}

	var result group.SpeakResult
	decodeGroup(t, recorder, &result)
	if result.Status != "done" {
		t.Fatalf("期望 done，收到 %s", result.Status)
	}
	if result.Error == "" || !strings.Contains(result.Error, "研发") {
		t.Errorf("应在 error 中说明失败的 Bot，实际 %q", result.Error)
	}
	if provider.callCount() != 3 {
		t.Fatalf("失败后仍应继续调用其余 Bot，实际 %d 次", provider.callCount())
	}
	// 第三个 Bot 必须照常发言
	speakers := map[string]bool{}
	for _, message := range result.Messages {
		if message.Role == "assistant" {
			speakers[message.SpeakerBotID] = true
		}
	}
	if !speakers[ids[2]] {
		t.Error("失败之后的设计 Bot 仍应发言")
	}
}

func TestGroupAllBotsFailingReturnsServerErrorWithMessages(t *testing.T) {
	provider := &groupTestProvider{}
	provider.reply = func(call int) (string, error) { return "", errors.New("全部不可用") }
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "全失败", ids, "mention", 3)

	recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("试一下", uuid.NewString(), nil, "mention"))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("全部 Bot 失败应返回 500，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	decodeGroup(t, recorder, &payload)
	if payload["status"] != "failed" {
		t.Errorf("期望 status=failed，实际 %v", payload["status"])
	}
	messages, _ := payload["messages"].([]any)
	if len(messages) != 1 {
		t.Errorf("500 响应也应带回已产生的消息，实际 %d 条", len(messages))
	}
}

func TestGroupSkipsBotArchivedMidDiscussion(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	ids := []string{
		createGroupBot(t, h, "产品", nil),
		createGroupBot(t, h, "研发", nil),
		createGroupBot(t, h, "设计", nil),
	}
	id := createGroup(t, h, "归档", ids, "auto", 1)

	// 第 2 次调用（研发）返回时把设计归档，保证设计的轮次被跳过
	provider.reply = func(call int) (string, error) {
		if call == 1 {
			if err := h.db.Model(&agent.Bot{}).Where("id = ?", ids[2]).
				Update("status", "archived").Error; err != nil {
				t.Errorf("归档 Bot 失败: %v", err)
			}
		}
		return fmt.Sprintf("发言%d", call+1), nil
	}

	recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("讨论一下", uuid.NewString(), nil, "auto"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("归档不应让整场请求失败，实际 %d: %s", recorder.Code, recorder.Body.String())
	}

	var result group.SpeakResult
	decodeGroup(t, recorder, &result)
	if provider.callCount() != 2 {
		t.Errorf("被归档的 Bot 不应再调用模型，实际 %d 次", provider.callCount())
	}
	if !strings.Contains(result.Error, "设计") {
		t.Errorf("应在 error 中记录被跳过的 Bot，实际 %q", result.Error)
	}
	for _, message := range result.Messages {
		if message.SpeakerBotID == ids[2] {
			t.Error("被归档的 Bot 不应留下发言")
		}
	}
}

func TestGroupValidationRejectsOutOfRangeInput(t *testing.T) {
	h := setupGroupHarness(t, &groupTestProvider{}, false)
	first := createGroupBot(t, h, "产品", nil)
	second := createGroupBot(t, h, "研发", nil)
	more := []string{
		createGroupBot(t, h, "设计", nil),
		createGroupBot(t, h, "测试", nil),
		createGroupBot(t, h, "运营", nil),
		createGroupBot(t, h, "法务", nil),
	}

	cases := []struct {
		name string
		body map[string]any
	}{
		{"轮次超上限", map[string]any{"title": "群", "botIds": []string{first, second}, "mode": "auto", "maxRounds": 7}},
		{"成员超上限", map[string]any{"title": "群", "botIds": append([]string{first, second}, more...), "mode": "mention"}},
		{"成员不足", map[string]any{"title": "群", "botIds": []string{first}, "mode": "mention"}},
		{"成员重复", map[string]any{"title": "群", "botIds": []string{first, first}, "mode": "mention"}},
		{"Bot 不存在", map[string]any{"title": "群", "botIds": []string{first, "not-a-bot"}, "mode": "mention"}},
		{"模式无效", map[string]any{"title": "群", "botIds": []string{first, second}, "mode": "shout"}},
		{"缺群名", map[string]any{"botIds": []string{first, second}, "mode": "mention"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if recorder := callGroup(t, h, http.MethodPost, "/api/groups", testCase.body); recorder.Code != http.StatusBadRequest {
				t.Fatalf("期望 400，实际 %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}

	// SaveBot 建档时强制置为 active，paused 只能在建档后改出来
	paused := createGroupBot(t, h, "停用", nil)
	if err := h.db.Model(&agent.Bot{}).Where("id = ?", paused).Update("status", "paused").Error; err != nil {
		t.Fatalf("设置 Bot 状态失败: %v", err)
	}
	if recorder := callGroup(t, h, http.MethodPost, "/api/groups", map[string]any{
		"title": "群", "botIds": []string{first, paused}, "mode": "mention",
	}); recorder.Code != http.StatusBadRequest {
		t.Errorf("非 active 的 Bot 应被拒绝，实际 %d", recorder.Code)
	}

	id := createGroup(t, h, "正常群", []string{first, second}, "mention", 3)
	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("@不在群里的人", uuid.NewString(), []string{"not-a-bot"}, "mention")); recorder.Code != http.StatusBadRequest {
		t.Errorf("提及非群成员应返回 400，实际 %d", recorder.Code)
	}
	if recorder := callGroup(t, h, http.MethodGet, "/api/groups/not-a-uuid", nil); recorder.Code != http.StatusBadRequest {
		t.Errorf("非法群 ID 应返回 400，实际 %d", recorder.Code)
	}
	if recorder := callGroup(t, h, http.MethodGet, "/api/groups/"+uuid.NewString(), nil); recorder.Code != http.StatusNotFound {
		t.Errorf("群不存在应返回 404，实际 %d", recorder.Code)
	}
}

func TestGroupRejectsConcurrentDiscussion(t *testing.T) {
	provider := &groupTestProvider{release: make(chan struct{})}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "并发", ids, "auto", 2)

	first := callGroupAsync(h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("第一次", uuid.NewString(), nil, "auto"))
	waitFor(t, "首个请求进入模型调用", func() bool { return provider.callCount() >= 1 })

	second := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("第二次", uuid.NewString(), nil, "auto"))
	if second.Code != http.StatusConflict {
		t.Fatalf("同群并发发言应返回 409，实际 %d: %s", second.Code, second.Body.String())
	}

	close(provider.release)
	if recorder := <-first; recorder.Code != http.StatusOK {
		t.Fatalf("首个请求应成功，实际 %d: %s", recorder.Code, recorder.Body.String())
	}

	// 讨论结束后互斥释放
	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("第三次", uuid.NewString(), nil, "auto")); recorder.Code != http.StatusOK {
		t.Fatalf("讨论结束后应能再次发言，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestGroupIdempotencyReplaysWithoutCallingLLMAgain(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "幂等", ids, "mention", 3)
	requestID := uuid.NewString()
	body := speakBody("同一个问题", requestID, nil, "mention")

	firstRecorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages", body)
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("首次发言失败 %d: %s", firstRecorder.Code, firstRecorder.Body.String())
	}
	var first group.SpeakResult
	decodeGroup(t, firstRecorder, &first)
	if calls := provider.callCount(); calls != 2 {
		t.Fatalf("首次应调用 2 次模型，实际 %d", calls)
	}

	secondRecorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages", body)
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("重放失败 %d: %s", secondRecorder.Code, secondRecorder.Body.String())
	}
	var second group.SpeakResult
	decodeGroup(t, secondRecorder, &second)

	if provider.callCount() != 2 {
		t.Errorf("重放不应再次调用模型，实际 %d 次", provider.callCount())
	}
	if second.ConversationID != first.ConversationID || second.Status != first.Status {
		t.Errorf("重放结果应与首次一致：%s/%s vs %s/%s", second.ConversationID, second.Status, first.ConversationID, first.Status)
	}
	if len(second.Messages) != len(first.Messages) {
		t.Fatalf("重放消息数不一致：%d vs %d", len(second.Messages), len(first.Messages))
	}
	for i := range first.Messages {
		if second.Messages[i].ID != first.Messages[i].ID {
			t.Errorf("第 %d 条消息 ID 不一致：%s vs %s", i, second.Messages[i].ID, first.Messages[i].ID)
		}
	}

	// 同键不同参数必须 409
	mismatch := speakBody("换了内容", requestID, nil, "mention")
	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages", mismatch); recorder.Code != http.StatusConflict {
		t.Errorf("同键不同参数应返回 409，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestGroupMemoryHonorsPerBotSwitch(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, true)
	remembering := createGroupBot(t, h, "产品", nil)
	forgetting := createGroupBot(t, h, "研发", func(bot *agent.Bot) { bot.MemoryEnabled = false })
	id := createGroup(t, h, "记忆", []string{remembering, forgetting}, "mention", 3)

	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("我喜欢用 Go 写后端，请记住这个偏好", uuid.NewString(), nil, "mention")); recorder.Code != http.StatusOK {
		t.Fatalf("发言失败 %d: %s", recorder.Code, recorder.Body.String())
	}

	// 记忆提取是异步的，先等开启记忆的 Bot 落库
	waitFor(t, "产品 Bot 写入私有记忆", func() bool {
		return len(botMemories(t, h.memory, remembering)) > 0
	})
	// 同一批 goroutine 已经跑完，关掉记忆的 Bot 必须仍然为空
	if got := botMemories(t, h.memory, forgetting); len(got) != 0 {
		t.Errorf("MemoryEnabled=false 的 Bot 不应写入群聊内容，实际 %d 条", len(got))
	}

	// 记忆按 Bot 隔离，而不是落到请求头对应的单一 owner 下
	for _, mem := range botMemories(t, h.memory, remembering) {
		if mem.UserID != agent.Owner(remembering) {
			t.Errorf("记忆应归属该 Bot 的私有空间，实际 %q", mem.UserID)
		}
	}
}

func TestGroupDeleteCleansConversationsButKeepsPrivateMemories(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, true)
	first := createGroupBot(t, h, "产品", nil)
	second := createGroupBot(t, h, "研发", nil)
	id := createGroup(t, h, "待删除", []string{first, second}, "mention", 3)

	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("我喜欢用 Go 写后端", uuid.NewString(), nil, "mention")); recorder.Code != http.StatusOK {
		t.Fatalf("发言失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	waitFor(t, "写入 Bot 私有记忆", func() bool { return len(botMemories(t, h.memory, first)) > 0 })

	before := storedMessages(t, h, id)
	if len(before) < 3 {
		t.Fatalf("期望至少 3 条群消息，实际 %d", len(before))
	}
	if recorder := callGroup(t, h, http.MethodDelete, "/api/groups/"+id, nil); recorder.Code != http.StatusOK {
		t.Fatalf("删除群失败 %d: %s", recorder.Code, recorder.Body.String())
	}

	// 没有数据库级联，对话与消息必须被显式清理
	var conversations, messages int64
	h.db.Model(&models.Conversation{}).Where("bot_id = ?", models.GroupBotID(uuid.MustParse(id))).Count(&conversations)
	h.db.Model(&models.Message{}).Where("conversation_id IN (?)",
		h.db.Model(&models.Conversation{}).Select("id").Where("bot_id = ?", models.GroupBotID(uuid.MustParse(id)))).Count(&messages)
	if conversations != 0 || messages != 0 {
		t.Errorf("删除群后应清空对话与消息，实际对话 %d 条、消息 %d 条", conversations, messages)
	}
	var members int64
	h.db.Model(&models.GroupMember{}).Where("group_id = ?", id).Count(&members)
	if members != 0 {
		t.Errorf("删除群后应清空成员，实际 %d 条", members)
	}

	// Bot 的私有记忆不属于群的数据，必须保留
	if got := botMemories(t, h.memory, first); len(got) == 0 {
		t.Error("删除群不应清空 Bot 的私有记忆")
	}
}

func TestGroupConversationKeepsExternalKeyEmpty(t *testing.T) {
	h := setupGroupHarness(t, &groupTestProvider{}, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "渠道隔离", ids, "mention", 3)

	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("群里的发言", uuid.NewString(), nil, "mention")); recorder.Code != http.StatusOK {
		t.Fatalf("发言失败 %d: %s", recorder.Code, recorder.Body.String())
	}

	var conv models.Conversation
	if err := h.db.Where("bot_id = ?", models.GroupBotID(uuid.MustParse(id))).First(&conv).Error; err != nil {
		t.Fatalf("群对话缺失: %v", err)
	}
	if conv.ExternalKey != "" {
		t.Errorf("群对话的 ExternalKey 必须为空，实际 %q", conv.ExternalKey)
	}

	// 渠道会话仍按 external_key 复用同一条历史，不与群对话串号
	channelConv, err := h.convRepo.GetOrCreateConversation(context.Background(), "feishu:chat_id:oc_group", "飞书会话")
	if err != nil {
		t.Fatalf("创建渠道会话失败: %v", err)
	}
	again, err := h.convRepo.GetOrCreateConversation(context.Background(), "feishu:chat_id:oc_group", "飞书会话")
	if err != nil || again.ID != channelConv.ID {
		t.Errorf("渠道会话应复用同一条记录：%v", err)
	}
	if again.ID == conv.ID {
		t.Error("渠道会话与群对话串号了")
	}
}

func TestGroupRecoversInterruptedDiscussionOnStartup(t *testing.T) {
	h := setupGroupHarness(t, &groupTestProvider{}, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "重启", ids, "auto", 3)

	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("重启前的问题", uuid.NewString(), nil, "mention")); recorder.Code != http.StatusOK {
		t.Fatalf("发言失败 %d: %s", recorder.Code, recorder.Body.String())
	}

	// 伪造一条上次进程被杀留下的 running 记录
	var conversation models.Conversation
	h.db.Where("bot_id = ?", models.GroupBotID(uuid.MustParse(id))).First(&conversation)
	orphan := &models.GroupTurn{
		ID: uuid.New(), RequestID: uuid.NewString(), GroupID: uuid.MustParse(id),
		Mode: "auto", Content: "被打断的讨论", Status: "running",
	}
	if err := h.db.Create(orphan).Error; err != nil {
		t.Fatalf("构造孤儿发言失败: %v", err)
	}

	if err := h.orch.RecoverInterrupted(context.Background()); err != nil {
		t.Fatalf("收敛中断发言失败: %v", err)
	}

	var turn models.GroupTurn
	if err := h.db.First(&turn, "id = ?", orphan.ID).Error; err != nil {
		t.Fatalf("读取发言记录失败: %v", err)
	}
	if turn.Status != "interrupted" {
		t.Errorf("期望 interrupted，实际 %q", turn.Status)
	}
	stored := storedMessages(t, h, id)
	if last := stored[len(stored)-1]; last.Role != "system" || !strings.Contains(last.Content, "中断") {
		t.Errorf("重启后应在末尾追加中断说明，实际 role=%q content=%q", last.Role, last.Content)
	}

	// 互斥锁释放后该群可以继续发言
	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("重启后的新问题", uuid.NewString(), nil, "mention")); recorder.Code != http.StatusOK {
		t.Fatalf("收敛后应能继续发言，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestGroupListReturnsMembersAndPreview(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "列表展示", ids, "mention", 3)

	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("先说重点", uuid.NewString(), nil, "mention")); recorder.Code != http.StatusOK {
		t.Fatalf("发言失败 %d: %s", recorder.Code, recorder.Body.String())
	}

	recorder := callGroup(t, h, http.MethodGet, "/api/groups", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("拉群列表失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var views []GroupView
	decodeGroup(t, recorder, &views)
	if len(views) != 1 {
		t.Fatalf("期望 1 个群，实际 %d", len(views))
	}
	if len(views[0].Members) != 2 || views[0].Members[0].Name == "" {
		t.Errorf("群列表应带成员摘要，实际 %+v", views[0].Members)
	}
	// 预览由 GroupChat 的冗余字段提供，列表接口不该再查一次对话
	if !strings.HasPrefix(views[0].LastMessagePreview, "研发：") {
		t.Errorf("预览应包含发言人名，实际 %q", views[0].LastMessagePreview)
	}
	if views[0].LastMessageAt == nil {
		t.Error("预览时间应随消息更新")
	}
}

func TestGroupUpdateChangesTitleAndMembers(t *testing.T) {
	h := setupGroupHarness(t, &groupTestProvider{}, false)
	first := createGroupBot(t, h, "产品", nil)
	second := createGroupBot(t, h, "研发", nil)
	third := createGroupBot(t, h, "设计", nil)
	id := createGroup(t, h, "旧群名", []string{first, second}, "mention", 3)

	recorder := callGroup(t, h, http.MethodPatch, "/api/groups/"+id, map[string]any{
		"title": "新群名", "botIds": []string{first, third}, "maxRounds": 5,
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("更新群失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var updated GroupView
	decodeGroup(t, recorder, &updated)
	if updated.Title != "新群名" || updated.MaxRounds != 5 {
		t.Errorf("更新未生效：%s / %d", updated.Title, updated.MaxRounds)
	}
	if len(updated.Members) != 2 || updated.Members[0].BotID != first || updated.Members[1].BotID != third {
		t.Errorf("成员未更新：%+v", updated.Members)
	}
	// 移出群的 Bot 不能再被点名
	if recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("被移出群的人", uuid.NewString(), []string{second}, "mention")); recorder.Code != http.StatusBadRequest {
		t.Errorf("提及已移出群的 Bot 应返回 400，实际 %d", recorder.Code)
	}
}

func TestGroupPartialUpdateKeepsUntouchedFields(t *testing.T) {
	h := setupGroupHarness(t, &groupTestProvider{}, false)
	first := createGroupBot(t, h, "产品", nil)
	second := createGroupBot(t, h, "研发", nil)
	id := createGroup(t, h, "局部更新", []string{first, second}, "auto", 3)

	// 只改轮次：模式、标题、成员都不能被动到
	recorder := callGroup(t, h, http.MethodPatch, "/api/groups/"+id, map[string]any{"maxRounds": 5})
	if recorder.Code != http.StatusOK {
		t.Fatalf("局部更新失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var updated GroupView
	decodeGroup(t, recorder, &updated)
	if updated.MaxRounds != 5 {
		t.Errorf("轮次未更新，实际 %d", updated.MaxRounds)
	}
	if updated.Mode != "auto" {
		t.Errorf("未提供的模式被覆盖，实际 %q", updated.Mode)
	}
	if updated.Title != "局部更新" {
		t.Errorf("未提供的群名被覆盖，实际 %q", updated.Title)
	}
	if len(updated.Members) != 2 {
		t.Errorf("未提供的成员被覆盖，实际 %d 人", len(updated.Members))
	}

	// 只改主题：群名为空不应导致 400
	recorder = callGroup(t, h, http.MethodPatch, "/api/groups/"+id, map[string]any{"topic": "新的讨论主题"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("只改主题失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	decodeGroup(t, recorder, &updated)
	if updated.Topic != "新的讨论主题" || updated.Title != "局部更新" {
		t.Errorf("主题更新结果不对：topic=%q title=%q", updated.Topic, updated.Title)
	}
	if updated.Mode != "auto" || updated.MaxRounds != 5 {
		t.Errorf("模式/轮次被覆盖：mode=%q rounds=%d", updated.Mode, updated.MaxRounds)
	}

	// 越界的局部更新仍要拦住
	if recorder := callGroup(t, h, http.MethodPatch, "/api/groups/"+id, map[string]any{"maxRounds": 9}); recorder.Code != http.StatusBadRequest {
		t.Errorf("越界轮次应返回 400，实际 %d", recorder.Code)
	}
}

func TestGroupAutoModeEndsEarlyWhenAllBotsSkip(t *testing.T) {
	provider := &groupTestProvider{}
	// 带首尾空白，顺带验证 §4 的解析规则是 TrimSpace 后严格相等
	provider.reply = func(call int) (string, error) { return "  [SKIP]  ", nil }
	h := setupGroupHarness(t, provider, false)
	ids := []string{
		createGroupBot(t, h, "产品", nil),
		createGroupBot(t, h, "研发", nil),
		createGroupBot(t, h, "设计", nil),
	}
	id := createGroup(t, h, "全员弃权", ids, "auto", 3)

	recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("大家看看这个方案", uuid.NewString(), nil, "auto"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("全员弃权不应报错，实际 %d: %s", recorder.Code, recorder.Body.String())
	}

	// 精确计数：3 个成员 × 3 轮上限，但全员弃权让讨论在第 1 轮后立即结束
	if got := provider.callCount(); got != 3 {
		t.Errorf("全员弃权后应恰好调用 3 次模型（成员数），实际 %d 次", got)
	}

	var result group.SpeakResult
	decodeGroup(t, recorder, &result)
	if result.Status != "done" {
		t.Errorf("全员弃权属于正常结束，期望 done，收到 %s", result.Status)
	}
	if result.Error != "" {
		t.Errorf("弃权不是失败，error 应为空，实际 %q", result.Error)
	}
	if len(result.Messages) != 1 || result.Messages[0].Role != "user" {
		t.Errorf("响应里只应有用户那一条，实际 %d 条", len(result.Messages))
	}

	// [SKIP] 不落库
	stored := storedMessages(t, h, id)
	if len(stored) != 1 || stored[0].Role != "user" {
		t.Fatalf("期望只落库 1 条用户消息，实际 %d 条", len(stored))
	}
	for _, message := range stored {
		if message.Role == "assistant" || message.SpeakerBotID != "" {
			t.Errorf("弃权不应留下发言：role=%q speaker=%q", message.Role, message.SpeakerBotID)
		}
	}
}

func TestGroupAutoModeWithPartialSkip(t *testing.T) {
	newProvider := func(pattern map[int]string) *groupTestProvider {
		provider := &groupTestProvider{}
		provider.reply = func(call int) (string, error) {
			if content, ok := pattern[call]; ok {
				return content, nil
			}
			return "[SKIP]", nil
		}
		return provider
	}

	t.Run("单轮内部分弃权", func(t *testing.T) {
		provider := newProvider(map[int]string{1: "只有研发有话说"})
		h := setupGroupHarness(t, provider, false)
		ids := []string{
			createGroupBot(t, h, "产品", nil),
			createGroupBot(t, h, "研发", nil),
			createGroupBot(t, h, "设计", nil),
		}
		id := createGroup(t, h, "部分弃权", ids, "auto", 1)

		recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
			speakBody("先听听研发的意见", uuid.NewString(), nil, "auto"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("部分弃权不应报错，实际 %d: %s", recorder.Code, recorder.Body.String())
		}
		if got := provider.callCount(); got != 3 {
			t.Errorf("期望恰好调用 3 次模型（成员数），实际 %d 次", got)
		}

		stored := storedMessages(t, h, id)
		if len(stored) != 2 {
			t.Fatalf("期望用户消息 1 条 + 发言 1 条 = 2，实际 %d 条", len(stored))
		}
		if stored[1].SpeakerBotID != ids[1] || stored[1].SpeakerName != "研发" {
			t.Errorf("只有非弃权的 Bot 发言应落库，实际 speaker=%q name=%q", stored[1].SpeakerBotID, stored[1].SpeakerName)
		}
	})

	t.Run("多轮在全员弃权的那一轮提前结束", func(t *testing.T) {
		// 第 1 轮：产品、设计弃权，研发发言；第 2 轮起全员弃权
		provider := newProvider(map[int]string{1: "研发第一轮的观点"})
		h := setupGroupHarness(t, provider, false)
		ids := []string{
			createGroupBot(t, h, "产品", nil),
			createGroupBot(t, h, "研发", nil),
			createGroupBot(t, h, "设计", nil),
		}
		id := createGroup(t, h, "多轮弃权", ids, "auto", 3)

		recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
			speakBody("继续讨论", uuid.NewString(), nil, "auto"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("多轮弃权不应报错，实际 %d: %s", recorder.Code, recorder.Body.String())
		}
		// 第 1 轮有人发言，讨论继续；第 2 轮全员弃权才收尾，因此是 2 轮 × 3 人 = 6 次
		if got := provider.callCount(); got != 6 {
			t.Errorf("期望恰好调用 6 次模型（2 轮 × 3 人），实际 %d 次", got)
		}

		stored := storedMessages(t, h, id)
		if len(stored) != 2 {
			t.Fatalf("期望只有 1 条发言落库，实际共 %d 条", len(stored))
		}
		if stored[1].Content != "研发第一轮的观点" {
			t.Errorf("落库内容不对：%q", stored[1].Content)
		}
	})
}

func TestGroupDoesNotSkipWhenTokenIsOnlySubstring(t *testing.T) {
	// §4：只有 TrimSpace 后严格等于 [SKIP] 才跳过，
	// 正文里出现 [SKIP] 字样不能被吞掉
	provider := &groupTestProvider{}
	provider.reply = func(call int) (string, error) { return "我认为应该 [SKIP] 掉这句废话", nil }
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "含标记的正文", ids, "auto", 1)

	recorder := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages",
		speakBody("讨论一下", uuid.NewString(), nil, "auto"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("发言失败 %d: %s", recorder.Code, recorder.Body.String())
	}

	stored := storedMessages(t, h, id)
	if len(stored) != 3 {
		t.Fatalf("期望用户消息 1 条 + 2 条发言 = 3，实际 %d 条", len(stored))
	}
	for _, message := range stored[1:] {
		if strings.TrimSpace(message.Content) == "[SKIP]" {
			t.Error("包含 [SKIP] 字样的正文被误判为弃权")
		}
	}
}

// TestGroupRoutesStayBehindLocalOnly 验证群路由挂到根 r 之后仍受本机边界保护
//
// 群路由必须避开 apiGroup 的 BotContext（那里会强制解析 X-Bot-ID），
// 但 LocalOnly 是另一个维度的保护：只允许本机可信界面访问，同样不能被绕开。
// 这里复刻 main.go 的装配方式——中间件与群路由都挂在同一个根引擎上。
func TestGroupRoutesStayBehindLocalOnly(t *testing.T) {
	h := setupGroupHarness(t, &groupTestProvider{}, false)
	first := createGroupBot(t, h, "产品", nil)
	second := createGroupBot(t, h, "研发", nil)
	id := createGroup(t, h, "安全边界", []string{first, second}, "mention", 3)

	router := gin.New()
	router.Use(gin.Recovery(), middleware.LocalOnly())
	RegisterGroups(router, NewGroupHandler(h.orch, h.groups, h.convRepo, h.agents))

	// 模拟桌面端 / 浏览器发起的本机可信请求
	localRequest := func(method, path, contentType string) *http.Request {
		request := httptest.NewRequest(method, path, bytes.NewReader(nil))
		request.Host = "127.0.0.1:1420"
		request.RemoteAddr = "127.0.0.1:52344"
		if contentType != "" {
			request.Header.Set("Content-Type", contentType)
		}
		return request
	}
	serve := func(request *http.Request) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder
	}

	t.Run("非 GET 缺少 Content-Type 与 X-Bot-ID 被拦下", func(t *testing.T) {
		if recorder := serve(localRequest(http.MethodDelete, "/api/groups/"+id, "")); recorder.Code != http.StatusUnsupportedMediaType {
			t.Errorf("期望 415，实际 %d: %s", recorder.Code, recorder.Body.String())
		}
		if recorder := serve(localRequest(http.MethodPost, "/api/groups/"+id+"/messages", "")); recorder.Code != http.StatusUnsupportedMediaType {
			t.Errorf("发言接口期望 415，实际 %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("非本机来源被拦下", func(t *testing.T) {
		request := localRequest(http.MethodDelete, "/api/groups/"+id, "application/json")
		request.RemoteAddr = "203.0.113.9:51000"
		if recorder := serve(request); recorder.Code != http.StatusForbidden {
			t.Errorf("非回环来源期望 403，实际 %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("GET 不受 Content-Type 限制", func(t *testing.T) {
		if recorder := serve(localRequest(http.MethodGet, "/api/groups", "")); recorder.Code != http.StatusOK {
			t.Errorf("本机 GET 应放行，实际 %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("带 Content-Type 的请求可以真正到达处理器", func(t *testing.T) {
		// 放行标记：前面若是 415/403，这里就会拿到非 200
		recorder := serve(localRequest(http.MethodDelete, "/api/groups/"+id, "application/json"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("合法请求期望 200，实际 %d: %s", recorder.Code, recorder.Body.String())
		}
		if recorder := serve(localRequest(http.MethodGet, "/api/groups/"+id, "")); recorder.Code != http.StatusNotFound {
			t.Errorf("群确已删除（说明 DELETE 真的执行了），期望 404，实际 %d", recorder.Code)
		}
	})
}

func TestGroupCreateDefaultsToUnpinned(t *testing.T) {
	h := setupGroupHarness(t, &groupTestProvider{}, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}

	recorder := callGroup(t, h, http.MethodPost, "/api/groups", map[string]any{
		"title": "默认群", "botIds": ids, "mode": "mention", "maxRounds": 3,
	})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("建群失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var created GroupView
	decodeGroup(t, recorder, &created)
	if created.Pinned {
		t.Error("新建群默认应为未置顶")
	}
	if created.PinnedAt != nil {
		t.Errorf("未置顶的群不应带置顶时间，实际 %v", created.PinnedAt)
	}

	// 落库同样必须是未置顶：AutoMigrate 新列的默认值不能只在内存里成立
	stored := storedGroup(t, h, created.ID)
	if stored.Pinned || stored.PinnedAt != nil {
		t.Errorf("落库应为未置顶，实际 pinned=%v pinnedAt=%v", stored.Pinned, stored.PinnedAt)
	}

	// 详情接口读出来的也是同一状态，避免列表与详情对不上
	if recorder := callGroup(t, h, http.MethodGet, "/api/groups/"+created.ID, nil); recorder.Code != http.StatusOK {
		t.Fatalf("拉群详情失败 %d: %s", recorder.Code, recorder.Body.String())
	} else {
		var detail GroupDetailView
		decodeGroup(t, recorder, &detail)
		if detail.Pinned || detail.PinnedAt != nil {
			t.Errorf("详情的置顶状态不对：pinned=%v pinnedAt=%v", detail.Pinned, detail.PinnedAt)
		}
	}
}

func TestGroupPinAndUnpinMaintainsPinnedAt(t *testing.T) {
	h := setupGroupHarness(t, &groupTestProvider{}, false)
	first := createGroupBot(t, h, "产品", nil)
	second := createGroupBot(t, h, "研发", nil)
	id := createGroup(t, h, "置顶", []string{first, second}, "mention", 3)

	pinned := setGroupPinned(t, h, id, true)
	if !pinned.Pinned || pinned.PinnedAt == nil {
		t.Fatalf("置顶后应为 pinned=true 且带置顶时间，实际 pinned=%v pinnedAt=%v", pinned.Pinned, pinned.PinnedAt)
	}
	stored := storedGroup(t, h, id)
	if !stored.Pinned || stored.PinnedAt == nil {
		t.Fatalf("置顶未落库：pinned=%v pinnedAt=%v", stored.Pinned, stored.PinnedAt)
	}

	// 重复置顶保留原时刻：否则连点两次置顶就能把自己不断往后推，
	// 置顶区的先后会被同一个人反复刷出来
	again := setGroupPinned(t, h, id, true)
	if !sameInstant(again.PinnedAt, pinned.PinnedAt) {
		t.Errorf("重复置顶不应刷新置顶时间：%v → %v", pinned.PinnedAt, again.PinnedAt)
	}

	// 取消置顶必须把时间清空，不能留下一个没有意义的旧时刻
	unpinned := setGroupPinned(t, h, id, false)
	if unpinned.Pinned {
		t.Error("取消置顶后 pinned 应为 false")
	}
	if unpinned.PinnedAt != nil {
		t.Errorf("取消置顶后置顶时间应为空，实际 %v", unpinned.PinnedAt)
	}
	if row := storedGroup(t, h, id); row.PinnedAt != nil {
		t.Errorf("取消置顶未落库，置顶时间仍是 %v", row.PinnedAt)
	}

	// PinnedAt 由服务端推导，客户端自带的值一律忽略
	forged := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	forgedGroup := createGroup(t, h, "伪造置顶时间", []string{first, second}, "mention", 3)
	recorder := callGroup(t, h, http.MethodPatch, "/api/groups/"+forgedGroup, map[string]any{
		"pinned": true, "pinnedAt": forged.Format(time.RFC3339),
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("置顶失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var forgedView GroupView
	decodeGroup(t, recorder, &forgedView)
	if forgedView.PinnedAt == nil || forgedView.PinnedAt.Equal(forged) {
		t.Errorf("客户端传入的置顶时间应被忽略，实际 %v", forgedView.PinnedAt)
	}
	if row := storedGroup(t, h, forgedGroup); row.PinnedAt == nil || row.PinnedAt.Equal(forged) {
		t.Errorf("落库的置顶时间被客户端覆盖了，实际 %v", row.PinnedAt)
	}
}

func TestGroupPinnedPatchKeepsUntouchedFields(t *testing.T) {
	h := setupGroupHarness(t, &groupTestProvider{}, false)
	first := createGroupBot(t, h, "产品", nil)
	second := createGroupBot(t, h, "研发", nil)
	third := createGroupBot(t, h, "设计", nil)
	id := createGroup(t, h, "局部置顶", []string{first, second}, "auto", 4)

	// 先把主题改掉，待会儿断言的是"没被动过"，而不是"本来就是这个值"
	if recorder := callGroup(t, h, http.MethodPatch, "/api/groups/"+id,
		map[string]any{"topic": "保留的主题"}); recorder.Code != http.StatusOK {
		t.Fatalf("改主题失败 %d: %s", recorder.Code, recorder.Body.String())
	}

	// 只传 pinned：标题、主题、模式、轮次、成员都不能被动到
	pinned := setGroupPinned(t, h, id, true)
	if pinned.Title != "局部置顶" || pinned.Topic != "保留的主题" {
		t.Errorf("未提供的标题/主题被覆盖：title=%q topic=%q", pinned.Title, pinned.Topic)
	}
	if pinned.Mode != "auto" || pinned.MaxRounds != 4 {
		t.Errorf("未提供的模式/轮次被覆盖：mode=%q rounds=%d", pinned.Mode, pinned.MaxRounds)
	}
	if len(pinned.Members) != 2 || pinned.Members[0].BotID != first || pinned.Members[1].BotID != second {
		t.Errorf("未提供的成员被覆盖：%+v", pinned.Members)
	}
	stored := storedGroup(t, h, id)
	if stored.Title != "局部置顶" || stored.Topic != "保留的主题" ||
		stored.Mode != "auto" || stored.MaxRounds != 4 {
		t.Errorf("落库被覆盖：title=%q topic=%q mode=%q rounds=%d",
			stored.Title, stored.Topic, stored.Mode, stored.MaxRounds)
	}
	var members int64
	h.db.Model(&models.GroupMember{}).Where("group_id = ?", id).Count(&members)
	if members != 2 {
		t.Errorf("只改置顶不应动成员，实际落库 %d 人", members)
	}

	// 反向：置顶之后只改标题，置顶状态与置顶时间都得留住
	recorder := callGroup(t, h, http.MethodPatch, "/api/groups/"+id, map[string]any{"title": "改过名字"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("改标题失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var renamed GroupView
	decodeGroup(t, recorder, &renamed)
	if renamed.Title != "改过名字" {
		t.Errorf("标题未更新，实际 %q", renamed.Title)
	}
	if !renamed.Pinned || !sameInstant(renamed.PinnedAt, pinned.PinnedAt) {
		t.Errorf("改标题不应回踩置顶：pinned=%v pinnedAt=%v", renamed.Pinned, renamed.PinnedAt)
	}
	if renamed.Mode != "auto" || renamed.MaxRounds != 4 {
		t.Errorf("改标题时模式/轮次被覆盖：mode=%q rounds=%d", renamed.Mode, renamed.MaxRounds)
	}

	// 换一个成员名单也不能顺带把置顶清掉
	recorder = callGroup(t, h, http.MethodPatch, "/api/groups/"+id,
		map[string]any{"botIds": []string{first, third}})
	if recorder.Code != http.StatusOK {
		t.Fatalf("换成员失败 %d: %s", recorder.Code, recorder.Body.String())
	}
	var swapped GroupView
	decodeGroup(t, recorder, &swapped)
	if !swapped.Pinned || !sameInstant(swapped.PinnedAt, pinned.PinnedAt) {
		t.Errorf("换成员不应回踩置顶：pinned=%v pinnedAt=%v", swapped.Pinned, swapped.PinnedAt)
	}
	if len(swapped.Members) != 2 || swapped.Members[1].BotID != third {
		t.Errorf("成员未更新：%+v", swapped.Members)
	}
}

func TestGroupListOrdersPinnedGroupsFirst(t *testing.T) {
	t.Run("置顶群排在未置顶群前面", func(t *testing.T) {
		h := setupGroupHarness(t, &groupTestProvider{}, false)
		bots := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
		plain := createGroup(t, h, "普通群", bots, "mention", 3)
		pinned := createGroup(t, h, "置顶群", bots, "mention", 3)
		createGroup(t, h, "另一个普通群", bots, "mention", 3)
		setGroupPinned(t, h, pinned, true)

		ids := listGroupIDs(t, h)
		if len(ids) != 3 {
			t.Fatalf("期望 3 个群，实际 %d", len(ids))
		}
		if ids[0] != pinned {
			t.Errorf("置顶群应排最前，实际顺序 %v（置顶的是 %s，普通的是 %s）", ids, pinned, plain)
		}
		for index, id := range ids {
			if id == pinned && index != 0 {
				t.Errorf("置顶群出现在第 %d 位，实际顺序 %v", index, ids)
			}
		}
	})

	t.Run("多个置顶群按置顶时间倒序", func(t *testing.T) {
		h := setupGroupHarness(t, &groupTestProvider{}, false)
		bots := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
		older := createGroup(t, h, "先置顶", bots, "mention", 3)
		plain := createGroup(t, h, "普通群", bots, "mention", 3)
		newer := createGroup(t, h, "后置顶", bots, "mention", 3)
		setGroupPinned(t, h, older, true)
		setGroupPinned(t, h, newer, true)

		// 把两个置顶时刻固定成确定的先后：排序规则要在确定输入下断言，
		// 不能寄望于驱动写出的时间精度刚好能区分两次置顶
		base := time.Now().Add(-2 * time.Hour)
		alignGroupTimestamps(t, h, older, map[string]any{"pinned_at": base})
		alignGroupTimestamps(t, h, newer, map[string]any{"pinned_at": base.Add(time.Hour)})

		ids := listGroupIDs(t, h)
		if len(ids) != 3 {
			t.Fatalf("期望 3 个群，实际 %d", len(ids))
		}
		if ids[0] != newer || ids[1] != older || ids[2] != plain {
			t.Errorf("期望 [后置顶 先置顶 普通群]，实际 %v", ids)
		}
	})

	t.Run("置顶时间相同时按 id 稳定排序", func(t *testing.T) {
		h := setupGroupHarness(t, &groupTestProvider{}, false)
		bots := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
		left := createGroup(t, h, "甲", bots, "mention", 3)
		right := createGroup(t, h, "乙", bots, "mention", 3)
		setGroupPinned(t, h, left, true)
		setGroupPinned(t, h, right, true)

		// 两个群同时刻置顶。updated_at 的高低按 id 排好序后反向赋值，
		// 这样"兜底用 id"与"兜底用 updated_at"必然给出不同顺序——否则
		// 随机 UUID 有五成概率让错误实现也蒙对，测试就成了摆设。
		sameInstant := time.Now().Add(-time.Hour)
		updatedBase := time.Now().Add(-3 * time.Hour)
		expectedLow, expectedHigh := left, right
		if expectedLow > expectedHigh {
			expectedLow, expectedHigh = expectedHigh, expectedLow
		}
		alignGroupTimestamps(t, h, expectedLow, map[string]any{
			"pinned_at": sameInstant, "updated_at": updatedBase,
		})
		alignGroupTimestamps(t, h, expectedHigh, map[string]any{
			"pinned_at": sameInstant, "updated_at": updatedBase.Add(time.Hour),
		})
		expected := fmt.Sprintf("[%s %s]", expectedLow, expectedHigh)

		ids := listGroupIDs(t, h)
		if len(ids) != 2 {
			t.Fatalf("期望 2 个群，实际 %d", len(ids))
		}
		if got := fmt.Sprintf("[%s %s]", ids[0], ids[1]); got != expected {
			t.Errorf("置顶时间相同时应按 id 排序，期望 %s，实际 %s", expected, got)
		}
		// 两次完全相同的请求必须给出同一个顺序，否则列表项会在界面上跳位
		if again := listGroupIDs(t, h); fmt.Sprint(again) != fmt.Sprint(ids) {
			t.Errorf("两次相同请求返回了不同顺序：%v vs %v", ids, again)
		}
	})

	t.Run("未置顶群按更新时间倒序", func(t *testing.T) {
		h := setupGroupHarness(t, &groupTestProvider{}, false)
		bots := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
		stale := createGroup(t, h, "久未更新", bots, "mention", 3)
		fresh := createGroup(t, h, "刚更新", bots, "mention", 3)
		pinned := createGroup(t, h, "置顶群", bots, "mention", 3)
		setGroupPinned(t, h, pinned, true)

		base := time.Now().Add(-2 * time.Hour)
		alignGroupTimestamps(t, h, stale, map[string]any{"updated_at": base})
		alignGroupTimestamps(t, h, fresh, map[string]any{"updated_at": base.Add(time.Hour)})

		ids := listGroupIDs(t, h)
		if len(ids) != 3 {
			t.Fatalf("期望 3 个群，实际 %d", len(ids))
		}
		if ids[0] != pinned || ids[1] != fresh || ids[2] != stale {
			t.Errorf("期望 [置顶群 刚更新 久未更新]，实际 %v", ids)
		}
	})
}
