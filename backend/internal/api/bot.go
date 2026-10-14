package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/memory"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"time"
)

// BotContext scopes the local single-owner API. Remote identity is not inferred
// from client supplied X-User-ID; an authenticated gateway is required separately.
func BotContext(s *agent.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Bot-ID")
		if id == "" {
			id = agent.DefaultBotID
		}
		b, err := s.GetBot(id)
		if err != nil {
			c.AbortWithStatusJSON(404, gin.H{"error": "Bot 不存在"})
			return
		}
		c.Set("bot", b)
		c.Set("memoryOwner", agent.Owner(id))
		c.Next()
	}
}
func currentBot(c *gin.Context) *agent.Bot {
	if v, ok := c.Get("bot"); ok {
		return v.(*agent.Bot)
	}
	return nil
}
func botID(c *gin.Context) string {
	if b := currentBot(c); b != nil {
		return b.ID
	}
	return agent.DefaultBotID
}
func RegisterAgents(g *gin.RouterGroup, s *agent.Service) {
	g.POST("/bots/draft", func(c *gin.Context) {
		if s.Demo {
			c.JSON(409, gin.H{"error": "请先在设置中连接模型，再生成职责草稿"})
			return
		}
		var input struct {
			Name         string `json:"name"`
			Instructions string `json:"instructions"`
		}
		if c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.Instructions) == "" || len([]rune(input.Instructions)) > 8000 {
			c.JSON(400, gin.H{"error": "请先描述 Bot 的职责"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()
		res, err := s.Provider.Chat(ctx, []llm.Message{{Role: "system", Content: "根据用户要求生成一个 Bot 的职责草稿。只返回 JSON 对象，字段 name（40 字内）、instructions（2000 字内）。写明负责事项、工作方式、输出标准。不承诺未配置的外部工具，不加入权限授权或定时执行。"}, {Role: "user", Content: input.Name + "\n" + input.Instructions}}, &llm.Options{Model: s.DefaultModel, Temperature: .4, MaxTokens: 1500})
		if err != nil {
			c.JSON(502, gin.H{"error": "生成失败：" + err.Error()})
			return
		}
		raw := strings.TrimSpace(res.Content)
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		var output struct {
			Name         string `json:"name"`
			Instructions string `json:"instructions"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(raw)), &output) != nil || strings.TrimSpace(output.Name) == "" || len([]rune(output.Name)) > 40 || strings.TrimSpace(output.Instructions) == "" || len([]rune(output.Instructions)) > 8000 {
			c.JSON(502, gin.H{"error": "模型未返回有效的职责草稿，请重试"})
			return
		}
		c.JSON(200, output)
	})

	g.GET("/bots", func(c *gin.Context) {
		bots := []agent.Bot{}
		if err := s.DB.Order("pinned DESC").Order("pinned_at DESC").Order("created_at ASC").Order("id ASC").Find(&bots).Error; err != nil {
			c.JSON(500, gin.H{"error": "无法加载 Bot"})
			return
		}
		c.JSON(200, bots)
	})
	g.POST("/bots", func(c *gin.Context) {
		b := agent.Bot{CollaborationEnabled: true, CollaborationAll: true}
		if c.ShouldBindJSON(&b) != nil {
			c.JSON(400, gin.H{"error": "配置无效"})
			return
		}
		if err := s.SaveBot(&b, true); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, b)
	})
	g.PUT("/bots/:id", func(c *gin.Context) {
		var b agent.Bot
		if c.ShouldBindJSON(&b) != nil {
			c.JSON(400, gin.H{"error": "配置无效"})
			return
		}
		b.ID = c.Param("id")
		if err := s.SaveBot(&b, false); err != nil {
			code := 400
			if errors.Is(err, agent.ErrConflict) {
				code = 409
			}
			c.JSON(code, gin.H{"error": err.Error()})
			return
		}
		b2, _ := s.GetBot(b.ID)
		c.JSON(200, b2)
	})
	g.PATCH("/bots/:id", func(c *gin.Context) {
		var patch agent.BotNavigationPatch
		if c.ShouldBindJSON(&patch) != nil {
			c.JSON(400, gin.H{"error": "参数无效"})
			return
		}
		bot, err := s.UpdateBotNavigation(c.Param("id"), patch)
		if err != nil {
			code := 400
			if errors.Is(err, agent.ErrConflict) {
				code = 409
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				code = 404
			}
			c.JSON(code, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, bot)
	})
	// Shared notes are explicitly written by the local owner, never by model output.
	g.GET("/shared-notes", func(c *gin.Context) {
		notes := []agent.SharedNote{}
		if s.DB.Order("updated_at DESC").Find(&notes).Error != nil {
			c.JSON(500, gin.H{"error": "无法加载共享资料"})
			return
		}
		c.JSON(200, notes)
	})
	save := func(c *gin.Context) {
		var n agent.SharedNote
		if c.ShouldBindJSON(&n) != nil {
			c.JSON(400, gin.H{"error": "资料无效"})
			return
		}
		n.Title = strings.TrimSpace(n.Title)
		n.Content = strings.TrimSpace(n.Content)
		if n.Title == "" || n.Content == "" || len([]rune(n.Title)) > 100 || len([]rune(n.Content)) > 8000 || len(n.AllowedBots) > 100 {
			c.JSON(400, gin.H{"error": "请填写标题和内容，内容最多 8000 字"})
			return
		}
		for _, id := range n.AllowedBots {
			if _, err := s.GetBot(id); err != nil {
				c.JSON(400, gin.H{"error": "授权 Bot 不存在"})
				return
			}
		}
		if c.Param("id") == "" {
			n.ID = uuid.NewString()
			n.Version = 1
			if s.DB.Create(&n).Error != nil {
				c.JSON(500, gin.H{"error": "保存失败"})
				return
			}
		} else {
			n.ID = c.Param("id")
			expected := n.Version
			n.Version++
			n.UpdatedAt = time.Now()
			r := s.DB.Model(&agent.SharedNote{}).Where("id = ? AND version = ?", n.ID, expected).Select("title", "content", "allowed_bots", "version", "updated_at").Updates(&n)
			if r.Error != nil {
				c.JSON(500, gin.H{"error": "保存失败"})
				return
			}
			if r.RowsAffected != 1 {
				c.JSON(409, gin.H{"error": "资料已更新，请刷新后重试"})
				return
			}
		}
		c.JSON(200, n)
	}
	g.POST("/shared-notes", save)
	g.PUT("/shared-notes/:id", save)
	g.DELETE("/shared-notes/:id", func(c *gin.Context) {
		r := s.DB.Delete(&agent.SharedNote{}, "id = ?", c.Param("id"))
		if r.Error != nil {
			c.JSON(500, gin.H{"error": "删除失败"})
			return
		}
		if r.RowsAffected == 0 {
			c.JSON(404, gin.H{"error": "资料不存在"})
			return
		}
		c.Status(204)
	})
	g.GET("/runs", func(c *gin.Context) {
		runs := []agent.Run{}
		if s.DB.Where("owner_bot_id = ?", botID(c)).Order("created_at DESC").Limit(100).Find(&runs).Error != nil {
			c.JSON(500, gin.H{"error": "无法加载任务"})
			return
		}
		c.JSON(200, runs)
	})
	g.POST("/runs", func(c *gin.Context) {
		var req struct {
			RequestID    string   `json:"requestId"`
			Goal         string   `json:"goal"`
			Participants []string `json:"participants"`
		}
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(400, gin.H{"error": "任务无效"})
			return
		}
		run, err := s.Start(botID(c), req.RequestID, req.Goal, req.Participants)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, run)
	})
	g.POST("/runs/:id/cancel", func(c *gin.Context) {
		if err := s.Cancel(c.Param("id"), botID(c)); err != nil {
			c.JSON(409, gin.H{"error": err.Error()})
			return
		}
		c.Status(204)
	})
	g.POST("/runs/:id/resume", func(c *gin.Context) {
		if err := s.Resume(c.Param("id"), botID(c)); err != nil {
			c.JSON(409, gin.H{"error": err.Error()})
			return
		}
		c.Status(204)
	})
	// Human-authored additions and corrections to the active bot's private memory.
	g.POST("/memories", func(c *gin.Context) {
		if s.Memory == nil {
			c.JSON(503, gin.H{"error": "记忆未启用"})
			return
		}
		var input struct {
			Content string `json:"content"`
		}
		if c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.Content) == "" || len([]rune(input.Content)) > 8000 {
			c.JSON(400, gin.H{"error": "请输入 1–8000 字的内容"})
			return
		}
		if err := s.Memory.StoreFact(c.Request.Context(), resolveUserID(c), input.Content); err != nil {
			c.JSON(500, gin.H{"error": "保存失败"})
			return
		}
		c.Status(201)
	})
	g.PUT("/memories/:id", func(c *gin.Context) {
		if s.Memory == nil {
			c.JSON(503, gin.H{"error": "记忆未启用"})
			return
		}
		var input struct {
			Content string `json:"content"`
		}
		if c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.Content) == "" || len([]rune(input.Content)) > 8000 {
			c.JSON(400, gin.H{"error": "请输入 1–8000 字的内容"})
			return
		}
		var m memory.Memory
		if s.DB.First(&m, "id = ? AND user_id = ?", c.Param("id"), resolveUserID(c)).Error != nil {
			c.JSON(404, gin.H{"error": "记忆不存在"})
			return
		}
		if err := s.Memory.Edit(c.Request.Context(), m.ID, input.Content); err != nil {
			c.JSON(500, gin.H{"error": "修改失败"})
			return
		}
		c.Status(http.StatusNoContent)
	})
}
