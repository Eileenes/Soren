package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/memory"
)

// MemoryHandler 记忆处理器
//
// 提供记忆的查看与管理能力，让用户能看到并控制 AI 记住了什么。
type MemoryHandler struct {
	manager *memory.MemoryManager
}

// NewMemoryHandler 创建记忆处理器
func NewMemoryHandler(manager *memory.MemoryManager) *MemoryHandler {
	return &MemoryHandler{manager: manager}
}

// ListMemories 列出记忆（支持分页与按类型筛选）
//
// GET /api/memories?page=1&pageSize=20&type=preference&sessionId=xxx&keyword=xxx&minImportance=70
func (h *MemoryHandler) ListMemories(c *gin.Context) {
	if !h.manager.Enabled() {
		c.JSON(http.StatusOK, gin.H{"items": []interface{}{}, "total": 0, "page": 1, "pageSize": 20, "enabled": false})
		return
	}

	ctx := c.Request.Context()

	page := parseQueryInt(c, "page", 1)
	if page < 1 {
		page = 1
	}
	pageSize := parseQueryInt(c, "pageSize", 20)
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	contentType := strings.TrimSpace(c.Query("type"))
	// 同时兼容 type 与 contentType 两种参数名
	if contentType == "" {
		contentType = strings.TrimSpace(c.Query("contentType"))
	}
	if contentType != "" && !memory.IsValidMemoryType(contentType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memory type"})
		return
	}

	opts := memory.ListOptions{
		UserID:        resolveUserID(c),
		ContentType:   contentType,
		SessionID:     strings.TrimSpace(c.Query("sessionId")),
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		MinImportance: parseQueryInt(c, "minImportance", 0),
		Offset:        (page - 1) * pageSize,
		Limit:         pageSize,
	}

	memories, total, err := h.manager.List(ctx, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list memories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":    memories,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"enabled":  true,
	})
}

// SearchMemories 搜索记忆
//
// GET /api/memories/search?q=咖啡&limit=20
func (h *MemoryHandler) SearchMemories(c *gin.Context) {
	if !h.manager.Enabled() {
		c.JSON(http.StatusOK, gin.H{"items": []interface{}{}, "total": 0, "enabled": false})
		return
	}

	ctx := c.Request.Context()

	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		// 兼容 keyword 参数名
		query = strings.TrimSpace(c.Query("keyword"))
	}

	limit := parseQueryInt(c, "limit", 20)
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	memories, err := h.manager.Search(ctx, resolveUserID(c), query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search memories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": memories,
		"query": query,
		"total": len(memories),
	})
}

// DeleteMemory 删除单条记忆
//
// DELETE /api/memories/:id
func (h *MemoryHandler) DeleteMemory(c *gin.Context) {
	if !h.manager.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory system is disabled"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memory ID"})
		return
	}

	ctx := c.Request.Context()
	entry, lookupErr := h.manager.Get(ctx, id)
	if lookupErr != nil || entry.UserID != resolveUserID(c) {
		c.JSON(404, gin.H{"error": "memory not found"})
		return
	}
	if err := h.manager.Delete(ctx, id); err != nil {
		if err == memory.ErrMemoryNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "memory not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete memory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "memory deleted", "id": id})
}

// GetMemoryStats 记忆统计
//
// GET /api/memories/stats
func (h *MemoryHandler) GetMemoryStats(c *gin.Context) {
	if !h.manager.Enabled() {
		c.JSON(http.StatusOK, gin.H{"enabled": false, "total": 0})
		return
	}

	ctx := c.Request.Context()
	stats, err := h.manager.Stats(ctx, resolveUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get memory stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":          true,
		"total":            stats.Total,
		"byType":           stats.ByType,
		"highImportance":   stats.HighImportance,
		"accessedRecently": stats.AccessedRecently,
		"totalTags":        stats.TotalTags,
		"oldestCreatedAt":  stats.OldestCreatedAt,
		"newestCreatedAt":  stats.NewestCreatedAt,
	})
}

// parseQueryInt 读取整型查询参数
func parseQueryInt(c *gin.Context, key string, defaultValue int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return value
}

// resolveUserID 解析用户 ID
//
// 桌面端为单用户场景，优先读取 X-User-ID 请求头，缺省时回落到 default-user。
func resolveUserID(c *gin.Context) string {
	if owner := c.GetString("memoryOwner"); owner != "" {
		return owner
	}
	if userID := strings.TrimSpace(c.GetHeader("X-User-ID")); userID != "" {
		return userID
	}
	if userID := strings.TrimSpace(c.Query("userId")); userID != "" {
		return userID
	}
	return memory.DefaultUserID
}
