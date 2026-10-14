package api

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/botconfig"
	"github.com/soren/backend/internal/bottemplate"
	"github.com/soren/backend/internal/templatesclient"
)

func RegisterTemplates(g *gin.RouterGroup, s *templatesclient.Client) {
	reply := func(c *gin.Context, value any, e error) {
		if e != nil {
			c.JSON(400, gin.H{"error": e.Error()})
			return
		}
		c.JSON(200, value)
	}
	decode := func(c *gin.Context, target any) bool {
		data, e := io.ReadAll(io.LimitReader(c.Request.Body, bottemplate.MaxBytes+1))
		if e == nil {
			e = bottemplate.Decode(data, target)
		}
		if e != nil {
			c.JSON(400, gin.H{"error": "请求格式无效或内容过长"})
			return false
		}
		return true
	}
	g.GET("/template-service", func(c *gin.Context) { v, e := s.Get(); reply(c, v, e) })
	g.PUT("/template-service", func(c *gin.Context) {
		var input templatesclient.SettingsInput
		if !decode(c, &input) {
			return
		}
		v, e := s.Save(input)
		reply(c, v, e)
	})
	g.GET("/template-service/identity", func(c *gin.Context) { v, e := s.Identity(c.Request.Context()); reply(c, v, e) })
	g.GET("/bot-templates", func(c *gin.Context) { v, e := s.List(c.Request.Context()); reply(c, v, e) })
	publish := func(c *gin.Context) {
		var input bottemplate.PublishRequest
		if !decode(c, &input) {
			return
		}
		v, e := s.Publish(c.Request.Context(), c.Param("id"), input)
		reply(c, v, e)
	}
	g.POST("/bot-templates", publish)
	g.POST("/bot-templates/:id/versions", publish)
	g.PATCH("/bot-templates/:id", func(c *gin.Context) {
		var input struct {
			Disabled *bool `json:"disabled"`
		}
		if !decode(c, &input) {
			return
		}
		if input.Disabled == nil {
			reply(c, nil, errors.New("请指定模板状态"))
			return
		}
		reply(c, gin.H{"ok": true}, s.Manage(c.Request.Context(), c.Param("id"), input.Disabled))
	})
	g.DELETE("/bot-templates/:id", func(c *gin.Context) { reply(c, gin.H{"ok": true}, s.Manage(c.Request.Context(), c.Param("id"), nil)) })
	g.GET("/bots/:id/template", func(c *gin.Context) {
		b, e := s.Agents.GetBot(c.Param("id"))
		if e != nil {
			reply(c, nil, errors.New("Bot 不存在"))
			return
		}
		config := templatesclient.Export(b)
		if s.Agents.Tools != nil && b.Status == "active" {
			actions, err := s.Agents.Tools.Actions(b.ID)
			if err != nil {
				reply(c, nil, err)
				return
			}
			known := map[string]bool{}
			for _, key := range config.RequiredCapabilities {
				known[key] = true
			}
			for _, action := range actions {
				if _, err := botconfig.Decision(b.Policy, action.Key); err == nil && !known[action.Key] {
					config.RequiredCapabilities = append(config.RequiredCapabilities, action.Key)
				}
			}
		}
		reply(c, config, nil)
	})
	g.POST("/template-imports/preview", func(c *gin.Context) {
		var input struct {
			Link string `json:"link"`
		}
		if !decode(c, &input) {
			return
		}
		v, e := s.Preview(c.Request.Context(), input.Link)
		reply(c, v, e)
	})
	g.POST("/template-imports/confirm", func(c *gin.Context) {
		var input struct {
			PreviewID string `json:"previewId"`
			Name      string `json:"name"`
		}
		if !decode(c, &input) {
			return
		}
		v, e := s.Import(c.Request.Context(), input.PreviewID, input.Name)
		reply(c, v, e)
	})
}

func RegisterBotConfiguration(g *gin.RouterGroup, s *agent.Service) {
	g.GET("/bots/:id/actions", func(c *gin.Context) {
		if s.Tools == nil {
			c.JSON(200, []any{})
			return
		}
		rows, e := s.Tools.Actions(c.Param("id"))
		if e != nil {
			c.JSON(400, gin.H{"error": e.Error()})
			return
		}
		c.JSON(200, rows)
	})
	g.GET("/bots/:id/versions", func(c *gin.Context) {
		var rows []agent.BotVersion
		if e := s.DB.Where("bot_id = ?", c.Param("id")).Order("version DESC").Limit(100).Find(&rows).Error; e != nil {
			c.JSON(500, gin.H{"error": "无法加载版本"})
			return
		}
		out := []gin.H{}
		for _, row := range rows {
			var b agent.Bot
			if json.Unmarshal([]byte(row.Snapshot), &b) != nil {
				continue
			}
			out = append(out, gin.H{"version": row.Version, "createdAt": row.CreatedAt, "bot": b})
		}
		c.JSON(200, out)
	})
	g.POST("/bots/:id/versions/:version/restore", func(c *gin.Context) {
		var input struct {
			Version int `json:"version"`
		}
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(400, gin.H{"error": "参数无效"})
			return
		}
		version, e := strconv.Atoi(c.Param("version"))
		if e != nil {
			c.JSON(400, gin.H{"error": "版本无效"})
			return
		}
		var old agent.BotVersion
		var snapshot agent.Bot
		current, e := s.GetBot(c.Param("id"))
		if e != nil || s.DB.Where("bot_id = ? AND version = ?", c.Param("id"), version).First(&old).Error != nil || json.Unmarshal([]byte(old.Snapshot), &snapshot) != nil {
			c.JSON(404, gin.H{"error": "版本不存在"})
			return
		}
		// Restoring a role never restores old account grants, action permissions or status.
		current.Name = snapshot.Name
		current.Avatar = snapshot.Avatar
		current.Instructions = snapshot.Instructions
		current.Definition = snapshot.Definition
		current.Version = input.Version
		if e = s.SaveBot(current, false); e != nil {
			code := 400
			if errors.Is(e, agent.ErrConflict) {
				code = 409
			}
			c.JSON(code, gin.H{"error": e.Error()})
			return
		}
		c.JSON(200, current)
	})
}
