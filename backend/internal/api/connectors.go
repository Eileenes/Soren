package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/connectors"
	"gorm.io/gorm"
)

func RegisterConnectors(g *gin.RouterGroup, s *connectors.Service, callbackURL string) {
	fail := func(c *gin.Context, e error) { c.JSON(400, gin.H{"error": e.Error()}) }
	validateBots := func(ids []string) error {
		if len(ids) > 100 {
			return errors.New("Bot 数量过多")
		}
		for _, id := range ids {
			var n int64
			if s.DB.Table("bots").Where("id = ?", id).Count(&n).Error != nil || n != 1 {
				return errors.New("Bot 不存在")
			}
		}
		return nil
	}
	g.GET("/connections", func(c *gin.Context) {
		rows := []connectors.Connection{}
		if e := s.DB.Order("created_at ASC").Find(&rows).Error; e != nil {
			fail(c, e)
			return
		}
		c.JSON(200, rows)
	})
	g.POST("/connections", func(c *gin.Context) {
		var input struct {
			Kind        string   `json:"kind"`
			Name        string   `json:"name"`
			Endpoint    string   `json:"endpoint"`
			Token       string   `json:"token"`
			AllowedBots []string `json:"allowedBots"`
		}
		if c.ShouldBindJSON(&input) != nil || len(input.Name) > 100 || len(input.Token) > 16000 {
			fail(c, errors.New("连接配置无效"))
			return
		}
		if e := validateBots(input.AllowedBots); e != nil {
			fail(c, e)
			return
		}
		row := connectors.Connection{ID: uuid.NewString(), Kind: input.Kind, Name: strings.TrimSpace(input.Name), Endpoint: strings.TrimSpace(input.Endpoint), AllowedBots: input.AllowedBots, Enabled: true, Version: 1, Tools: []connectors.RemoteTool{}}
		if row.Name == "" {
			row.Name = input.Kind
		}
		if row.AllowedBots == nil {
			row.AllowedBots = []string{}
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Second)
		defer cancel()
		switch input.Kind {
		case "notion":
			if input.Token == "" {
				fail(c, errors.New("请填写 Notion 连接令牌"))
				return
			}
			req, _ := http.NewRequestWithContext(ctx, "GET", s.NotionAPI+"/users/me", nil)
			req.Header.Set("Authorization", "Bearer "+input.Token)
			req.Header.Set("Notion-Version", "2022-06-28")
			res, e := s.HTTP.Do(req)
			if e != nil {
				fail(c, errors.New("无法连接 Notion"))
				return
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				fail(c, errors.New("Notion 令牌无效或权限不足"))
				return
			}
			var profile struct {
				Name string `json:"name"`
			}
			if json.NewDecoder(http.MaxBytesReader(c.Writer, res.Body, 1<<20)).Decode(&profile) != nil {
				fail(c, errors.New("Notion 返回无效数据"))
				return
			}
			row.Account = profile.Name
			row.Endpoint = ""
		case "mcp":
			list, e := s.Discover(ctx, row.Endpoint, input.Token)
			if e != nil {
				fail(c, e)
				return
			}
			if len(list) == 0 {
				fail(c, errors.New("MCP 服务没有提供工具"))
				return
			}
			row.Tools = list
			row.Account = row.Endpoint
		default:
			fail(c, errors.New("连接类型无效"))
			return
		}
		if e := s.Vault.Set(row.ID, connectors.Credential{APIKey: input.Token}); e != nil {
			fail(c, errors.New("系统凭据库不可用，连接未保存"))
			return
		}
		if e := s.DB.Create(&row).Error; e != nil {
			_ = s.Vault.Delete(row.ID)
			fail(c, e)
			return
		}
		c.JSON(201, row)
	})
	g.PUT("/connections/:id", func(c *gin.Context) {
		var input struct {
			AllowedBots []string `json:"allowedBots"`
			Enabled     bool     `json:"enabled"`
			Version     int      `json:"version"`
		}
		if c.ShouldBindJSON(&input) != nil {
			fail(c, errors.New("参数无效"))
			return
		}
		if e := validateBots(input.AllowedBots); e != nil {
			fail(c, e)
			return
		}
		if input.AllowedBots == nil {
			input.AllowedBots = []string{}
		}
		// GORM's JSON serializer must be applied through a typed model, not a slice map value.
		row := connectors.Connection{AllowedBots: input.AllowedBots, Enabled: input.Enabled, Version: input.Version + 1}
		res := s.DB.Model(&row).Where("id = ? AND version = ?", c.Param("id"), input.Version).Select("allowed_bots", "enabled", "version").Updates(&row)
		if res.Error != nil {
			fail(c, res.Error)
			return
		}
		if res.RowsAffected != 1 {
			c.JSON(409, gin.H{"error": "连接已变更，请刷新后重试"})
			return
		}
		c.Status(204)
	})
	g.DELETE("/connections/:id", func(c *gin.Context) {
		id := c.Param("id")
		if e := s.DB.Model(&connectors.Connection{}).Where("id = ?", id).Updates(map[string]any{"enabled": false, "version": gorm.Expr("version + 1")}).Error; e != nil {
			fail(c, e)
			return
		}
		if e := s.Vault.Delete(id); e != nil {
			fail(c, errors.New("连接已停用，但系统凭据删除失败，请重试"))
			return
		}
		if e := s.DB.Delete(&connectors.Connection{}, "id = ?", id).Error; e != nil {
			fail(c, e)
			return
		}
		c.Status(204)
	})
	g.POST("/connections/gmail/authorize", func(c *gin.Context) {
		var input struct {
			ClientID     string   `json:"clientId"`
			ClientSecret string   `json:"clientSecret"`
			AllowedBots  []string `json:"allowedBots"`
		}
		if c.ShouldBindJSON(&input) != nil {
			fail(c, errors.New("参数无效"))
			return
		}
		if e := validateBots(input.AllowedBots); e != nil {
			fail(c, e)
			return
		}
		target, e := s.BeginGoogle(connectors.Credential{ClientID: input.ClientID, ClientSecret: input.ClientSecret}, input.AllowedBots, callbackURL)
		if e != nil {
			fail(c, e)
			return
		}
		c.JSON(200, gin.H{"url": target})
	})
	g.GET("/identity", func(c *gin.Context) {
		var identity connectors.Identity
		e := s.DB.First(&identity, 1).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			c.JSON(200, gin.H{"identity": nil})
			return
		}
		if e != nil {
			fail(c, e)
			return
		}
		c.JSON(200, gin.H{"identity": identity})
	})
	g.POST("/identity/github", func(c *gin.Context) {
		var input struct {
			ClientID string `json:"clientId"`
		}
		if c.ShouldBindJSON(&input) != nil {
			fail(c, errors.New("参数无效"))
			return
		}
		code, e := s.BeginGitHub(c.Request.Context(), input.ClientID)
		if e != nil {
			fail(c, e)
			return
		}
		c.JSON(200, code)
	})
	g.POST("/identity/github/:id/poll", func(c *gin.Context) {
		identity, e := s.PollGitHub(c.Request.Context(), c.Param("id"))
		if e != nil {
			fail(c, e)
			return
		}
		c.JSON(200, gin.H{"identity": identity})
	})
	ownRun := func(c *gin.Context) bool {
		var n int64
		s.DB.Table("runs").Where("id = ? AND owner_bot_id = ?", c.Param("id"), botID(c)).Count(&n)
		if n != 1 {
			c.JSON(404, gin.H{"error": "任务不存在"})
			return false
		}
		return true
	}
	g.GET("/runs/:id/tools", func(c *gin.Context) {
		if !ownRun(c) {
			return
		}
		rows := []connectors.Invocation{}
		if e := s.DB.Where("run_id = ?", c.Param("id")).Order("created_at ASC").Find(&rows).Error; e != nil {
			fail(c, e)
			return
		}
		c.JSON(200, rows)
	})
	g.GET("/runs/:id/artifacts", func(c *gin.Context) {
		if !ownRun(c) {
			return
		}
		rows := []connectors.Artifact{}
		if e := s.DB.Where("run_id = ?", c.Param("id")).Order("created_at ASC").Find(&rows).Error; e != nil {
			fail(c, e)
			return
		}
		c.JSON(200, rows)
	})
	g.POST("/tool-invocations/:id/decision", func(c *gin.Context) {
		var input struct {
			Approve *bool `json:"approve"`
		}
		if c.ShouldBindJSON(&input) != nil || input.Approve == nil {
			fail(c, errors.New("请选择批准或拒绝"))
			return
		}
		if e := s.Decide(c.Param("id"), botID(c), *input.Approve); e != nil {
			fail(c, e)
			return
		}
		c.Status(204)
	})
}
func RegisterOAuthCallback(r *gin.Engine, s *connectors.Service) {
	r.GET("/oauth/google/callback", func(c *gin.Context) {
		e := s.CompleteGoogle(c.Request.Context(), c.Query("state"), c.Query("code"))
		title := "Gmail 已连接"
		message := "返回 Soren，在应用连接中查看账号。"
		status := 200
		if e != nil {
			title = "授权未完成"
			message = "请返回 Soren 重新发起连接，检查应用凭据和授权范围。"
			status = 400
		}
		c.Header("Cache-Control", "no-store")
		c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		c.Data(status, "text/html; charset=utf-8", []byte(fmt.Sprintf("<!doctype html><html lang=zh><meta name=viewport content='width=device-width,initial-scale=1'><title>Soren</title><body style='font-family:system-ui;background:#f2f5ec;color:#264b3a;padding:12vh 8vw'><h1>%s</h1><p>%s</p></body></html>", title, message)))
	})
}
