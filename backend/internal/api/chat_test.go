package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/cache"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/memory"
	"github.com/soren/backend/internal/models"
	"github.com/soren/backend/internal/repository"
	"github.com/soren/backend/internal/store"
)

// stubProvider 记录收到的消息并返回固定回复
type stubProvider struct {
	received []llm.Message
	reply    string
	model    string
}

func (s *stubProvider) Chat(ctx context.Context, messages []llm.Message, options *llm.Options) (*llm.Response, error) {
	s.received = messages
	s.model = options.Model
	reply := s.reply
	if reply == "" {
		reply = "我推荐你继续用 Go。"
	}
	return &llm.Response{Content: reply, Model: "stub"}, nil
}

func (s *stubProvider) StreamChat(ctx context.Context, messages []llm.Message, options *llm.Options, stream chan<- llm.StreamChunk) error {
	return nil
}

// systemPrompt 返回本次调用中第一条 system 消息
func (s *stubProvider) systemPrompt() string {
	for _, m := range s.received {
		if m.Role == "system" {
			return m.Content
		}
	}
	return ""
}

// setupChatRouter 搭建带记忆系统的完整聊天路由
func setupChatRouter(t *testing.T, provider llm.Provider) (*gin.Engine, *memory.MemoryManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := store.OpenDB(&config.Config{
		StorageDriver: config.StorageDriverSQLite,
		SQLitePath:    filepath.Join(t.TempDir(), "chat_test.db"),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	cacheClient := cache.NewMemoryCache(100)

	opts := memory.DefaultManagerOptions()
	opts.Cache = cacheClient

	manager, err := memory.NewMemoryManager(db, cacheClient, opts)
	if err != nil {
		t.Fatalf("failed to create memory manager: %v", err)
	}

	handler := NewChatHandler(repository.NewConversationRepository(db), provider, manager)

	router := gin.New()
	router.POST("/api/chat", handler.SendMessage)
	return router, manager
}

// postChat 发送一条聊天消息
func postChat(t *testing.T, router *gin.Engine, conversationID, message string) map[string]interface{} {
	t.Helper()

	payload, _ := json.Marshal(map[string]interface{}{
		"conversationId": conversationID,
		"message":        message,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("chat request failed with %d: %s", recorder.Code, recorder.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	return body
}

func TestChatStoresMemoriesAsynchronously(t *testing.T) {
	provider := &stubProvider{}
	router, manager := setupChatRouter(t, provider)

	postChat(t, router, "", "我喜欢用 Go 写后端，请帮我记住这个偏好")

	// 记忆提取是异步的，轮询等待写入完成
	deadline := time.Now().Add(5 * time.Second)
	var count int
	for time.Now().Before(deadline) {
		memories, _, err := manager.List(context.Background(), memory.ListOptions{Limit: 50})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		count = len(memories)
		if count > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if count == 0 {
		t.Fatal("expected memories to be extracted and stored after the turn")
	}

	memories, _, _ := manager.List(context.Background(), memory.ListOptions{Limit: 50})
	hasConversation, hasPreference := false, false
	for _, mem := range memories {
		switch mem.ContentType {
		case memory.MemoryTypeConversation:
			hasConversation = true
		case memory.MemoryTypePreference:
			hasPreference = true
		}
	}
	if !hasConversation {
		t.Error("expected a conversation summary to be stored")
	}
	if !hasPreference {
		t.Error("expected the user preference to be stored")
	}
}

func TestChatInjectsRecalledMemoriesIntoSystemPrompt(t *testing.T) {
	provider := &stubProvider{}
	router, manager := setupChatRouter(t, provider)

	// 预置一条与本次提问相关的偏好记忆
	if err := manager.StorePreference(context.Background(), memory.DefaultUserID, "咖啡", "喜欢拿铁咖啡"); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}

	body := postChat(t, router, "", "推荐一杯咖啡给我")

	// system prompt 中应包含召回的记忆
	prompt := provider.systemPrompt()
	if prompt == "" {
		t.Fatal("expected a system message to be sent to the LLM")
	}
	if !strings.Contains(prompt, "长期记忆") {
		t.Errorf("system prompt missing memory header:\n%s", prompt)
	}
	if !strings.Contains(prompt, "喜欢拿铁咖啡") {
		t.Errorf("system prompt missing the recalled memory:\n%s", prompt)
	}

	// 响应中应返回本次用到的记忆
	used, ok := body["memoriesUsed"].([]interface{})
	if !ok || len(used) == 0 {
		t.Errorf("expected memoriesUsed in the response, got %v", body["memoriesUsed"])
	}
}

func TestChatWithoutMemoriesSendsNoSystemPrompt(t *testing.T) {
	provider := &stubProvider{}
	router, _ := setupChatRouter(t, provider)

	postChat(t, router, "", "你好")

	if prompt := provider.systemPrompt(); prompt != "" {
		t.Errorf("expected no system prompt when nothing was recalled, got %q", prompt)
	}
}

func TestChatContinuesConversationAndRecallsSessionHistory(t *testing.T) {
	provider := &stubProvider{}
	router, _ := setupChatRouter(t, provider)

	// 第一轮：建立会话
	first := postChat(t, router, "", "我们在讨论 Go 语言的内存管理")
	conversationID, _ := first["conversationId"].(string)
	if conversationID == "" {
		t.Fatal("expected a conversation ID in the response")
	}

	// 等待第一轮记忆落库
	time.Sleep(500 * time.Millisecond)

	// 第二轮：复用同一会话
	body := postChat(t, router, conversationID, "继续说这个话题")
	if got, _ := body["conversationId"].(string); got != conversationID {
		t.Errorf("expected the same conversation ID %q, got %q", conversationID, got)
	}

	// 历史消息应被带入，system prompt 应包含同会话记忆
	if len(provider.received) < 3 {
		t.Errorf("expected prior turns plus the new message, got %d messages", len(provider.received))
	}
	if prompt := provider.systemPrompt(); !strings.Contains(prompt, "Go") {
		t.Errorf("expected session memory about Go in the system prompt:\n%s", prompt)
	}
}

func TestChatSurvivesRecalledMemoriesFailure(t *testing.T) {
	// 禁用记忆系统后聊天仍应正常工作
	gin.SetMode(gin.TestMode)

	db, err := store.OpenDB(&config.Config{
		StorageDriver: config.StorageDriverSQLite,
		SQLitePath:    filepath.Join(t.TempDir(), "nodb.db"),
	})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	provider := &stubProvider{}
	handler := NewChatHandler(repository.NewConversationRepository(db), provider, nil)

	router := gin.New()
	router.POST("/api/chat", handler.SendMessage)

	body := postChat(t, router, "", "没有记忆系统时也应正常回复")
	if body["message"] == nil {
		t.Error("expected an assistant message even without the memory system")
	}
	if _, present := body["memoriesUsed"]; present {
		t.Error("memoriesUsed should be omitted when the memory system is unavailable")
	}
}

func TestChatPartialConfigKeepsConfiguredModel(t *testing.T) {
	_, _, db := setupMemoryRouter(t)
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}); err != nil {
		t.Fatal(err)
	}
	provider := &stubProvider{}
	agents, err := agent.New(db, provider, nil, "configured-model", false)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewChatHandler(repository.NewConversationRepository(db), provider, nil).WithAgents(agents)
	router := gin.New()
	router.POST("/api/chat", handler.SendMessage)
	for _, config := range []string{`{}`, `{"temperature":0.2}`, `{"maxTokens":100}`} {
		req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"message":"hello","config":`+config+`}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != 200 || provider.model != "configured-model" {
			t.Fatalf("status %d model %s", res.Code, provider.model)
		}
	}
}
