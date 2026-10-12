package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/cache"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/memory"
	"github.com/soren/backend/internal/store"
	"gorm.io/gorm"
)

// setupMemoryRouter 搭建只挂载记忆路由的测试服务器
func setupMemoryRouter(t *testing.T) (*gin.Engine, *memory.MemoryManager, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := store.OpenDB(&config.Config{
		StorageDriver: config.StorageDriverSQLite,
		SQLitePath:    filepath.Join(t.TempDir(), "api_test.db"),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	cacheClient := cache.NewMemoryCache(100)

	opts := memory.DefaultManagerOptions()
	opts.Cache = cacheClient

	manager, err := memory.NewMemoryManager(db, cacheClient, opts)
	if err != nil {
		t.Fatalf("failed to create memory manager: %v", err)
	}

	handler := NewMemoryHandler(manager)
	router := gin.New()
	router.GET("/api/memories", handler.ListMemories)
	router.GET("/api/memories/search", handler.SearchMemories)
	router.GET("/api/memories/stats", handler.GetMemoryStats)
	router.DELETE("/api/memories/:id", handler.DeleteMemory)

	return router, manager, db
}

// do 执行一次请求并返回响应
func do(t *testing.T, router *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestListMemoriesEndpoint(t *testing.T) {
	router, manager, _ := setupMemoryRouter(t)

	if err := manager.StorePreference(context.Background(), "default-user", "咖啡", "喜欢拿铁"); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}
	if err := manager.StoreConversation(context.Background(), "default-user", "s1", "随便聊聊", 1); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}

	recorder := do(t, router, http.MethodGet, "/api/memories")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var body struct {
		Items    []map[string]interface{} `json:"items"`
		Total    int64                    `json:"total"`
		Page     int                      `json:"page"`
		PageSize int                      `json:"pageSize"`
		Enabled  bool                     `json:"enabled"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Total != 2 || len(body.Items) != 2 {
		t.Errorf("expected 2 memories, got total=%d items=%d", body.Total, len(body.Items))
	}
	if body.Page != 1 || body.PageSize != 20 || !body.Enabled {
		t.Errorf("unexpected pagination metadata: %+v", body)
	}
}

func TestListMemoriesFiltersByType(t *testing.T) {
	router, manager, _ := setupMemoryRouter(t)

	if err := manager.StorePreference(context.Background(), "default-user", "咖啡", "喜欢拿铁"); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}
	if err := manager.StoreConversation(context.Background(), "default-user", "s1", "随便聊聊", 1); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}

	recorder := do(t, router, http.MethodGet, "/api/memories?type=preference")
	var body struct {
		Items []map[string]interface{} `json:"items"`
		Total int64                    `json:"total"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)

	if body.Total != 1 || len(body.Items) != 1 {
		t.Errorf("expected only the preference memory, got total=%d", body.Total)
	}
	if body.Items[0]["contentType"] != "preference" {
		t.Errorf("unexpected content type: %v", body.Items[0]["contentType"])
	}
}

func TestListMemoriesRejectsInvalidType(t *testing.T) {
	router, _, _ := setupMemoryRouter(t)

	recorder := do(t, router, http.MethodGet, "/api/memories?type=bogus")
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid type, got %d", recorder.Code)
	}
}

func TestListMemoriesPagination(t *testing.T) {
	router, manager, _ := setupMemoryRouter(t)

	for i := 0; i < 5; i++ {
		if err := manager.StoreConversation(context.Background(), "default-user", "s1", "对话内容", i); err != nil {
			t.Fatalf("failed to seed memory: %v", err)
		}
	}

	recorder := do(t, router, http.MethodGet, "/api/memories?page=2&pageSize=2")
	var body struct {
		Items    []map[string]interface{} `json:"items"`
		Total    int64                    `json:"total"`
		Page     int                      `json:"page"`
		PageSize int                      `json:"pageSize"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)

	if body.Total != 5 {
		t.Errorf("expected total 5, got %d", body.Total)
	}
	if len(body.Items) != 2 {
		t.Errorf("expected 2 items on page 2, got %d", len(body.Items))
	}
	if body.Page != 2 {
		t.Errorf("expected page 2, got %d", body.Page)
	}
}

func TestSearchMemoriesEndpoint(t *testing.T) {
	router, manager, _ := setupMemoryRouter(t)

	if err := manager.StoreFact(context.Background(), "default-user", "用户的项目叫 Soren"); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}

	recorder := do(t, router, http.MethodGet, "/api/memories/search?q=Soren")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var body struct {
		Items []map[string]interface{} `json:"items"`
		Query string                   `json:"query"`
		Total int                      `json:"total"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)

	if body.Total != 1 || body.Query != "Soren" {
		t.Errorf("unexpected search result: %+v", body)
	}
}

func TestDeleteMemoryEndpoint(t *testing.T) {
	router, manager, _ := setupMemoryRouter(t)

	if err := manager.StoreFact(context.Background(), "default-user", "待删除的记忆"); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}

	memories, _, _ := manager.List(context.Background(), memory.ListOptions{})
	if len(memories) != 1 {
		t.Fatalf("expected 1 seeded memory, got %d", len(memories))
	}
	id := memories[0].ID

	recorder := do(t, router, http.MethodDelete, "/api/memories/"+itoa(id))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	// 再次删除应返回 404
	recorder = do(t, router, http.MethodDelete, "/api/memories/"+itoa(id))
	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected 404 on second delete, got %d", recorder.Code)
	}
}

func TestDeleteMemoryRejectsInvalidID(t *testing.T) {
	router, _, _ := setupMemoryRouter(t)

	recorder := do(t, router, http.MethodDelete, "/api/memories/not-a-number")
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid ID, got %d", recorder.Code)
	}
}

func TestMemoryStatsEndpoint(t *testing.T) {
	router, manager, _ := setupMemoryRouter(t)

	if err := manager.StorePreference(context.Background(), "default-user", "咖啡", "喜欢拿铁"); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}

	recorder := do(t, router, http.MethodGet, "/api/memories/stats")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var body struct {
		Enabled   bool             `json:"enabled"`
		Total     int64            `json:"total"`
		ByType    map[string]int64 `json:"byType"`
		TotalTags int64            `json:"totalTags"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode stats: %v", err)
	}
	if !body.Enabled || body.Total != 1 {
		t.Errorf("unexpected stats: %+v", body)
	}
	if body.ByType["preference"] != 1 {
		t.Errorf("expected 1 preference memory in breakdown, got %v", body.ByType)
	}
}

func TestMemoryEndpointsHonorUserHeader(t *testing.T) {
	router, manager, _ := setupMemoryRouter(t)

	if err := manager.StorePreference(context.Background(), "alice", "咖啡", "喜欢拿铁"); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}
	if err := manager.StorePreference(context.Background(), "bob", "咖啡", "喜欢美式"); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/memories", nil)
	req.Header.Set("X-User-ID", "alice")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var body struct {
		Items []map[string]interface{} `json:"items"`
		Total int64                    `json:"total"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)

	if body.Total != 1 {
		t.Fatalf("expected 1 memory for alice, got %d", body.Total)
	}
	if body.Items[0]["userId"] != "alice" {
		t.Errorf("expected alice's memory, got %v", body.Items[0]["userId"])
	}
}

func TestListMemoriesWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	opts := memory.DefaultManagerOptions()
	opts.Enabled = false

	db, err := store.OpenDB(&config.Config{
		StorageDriver: config.StorageDriverSQLite,
		SQLitePath:    filepath.Join(t.TempDir(), "disabled.db"),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	manager, err := memory.NewMemoryManager(db, nil, opts)
	if err != nil {
		t.Fatalf("failed to create disabled manager: %v", err)
	}

	handler := NewMemoryHandler(manager)
	router := gin.New()
	router.GET("/api/memories", handler.ListMemories)
	router.DELETE("/api/memories/:id", handler.DeleteMemory)

	recorder := do(t, router, http.MethodGet, "/api/memories")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from disabled list, got %d", recorder.Code)
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	if body.Enabled {
		t.Error("expected enabled=false in the response")
	}

	// 禁用状态下删除应返回 503 而非 500
	recorder = do(t, router, http.MethodDelete, "/api/memories/1")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when memory system is disabled, got %d", recorder.Code)
	}
}

// itoa 将 int64 转为字符串（避免为测试引入额外依赖）
func itoa(id int64) string {
	if id == 0 {
		return "0"
	}
	negative := id < 0
	if negative {
		id = -id
	}

	var buf [20]byte
	pos := len(buf)
	for id > 0 {
		pos--
		buf[pos] = byte('0' + id%10)
		id /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
