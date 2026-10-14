package api

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/models"
	"gorm.io/gorm"
)

// The user's first answer becomes the role verbatim. No model call or inferred
// permissions are needed; the role and its conversation commit together.
func (h *ChatHandler) defineBotResponsibilities(c *gin.Context, bot *agent.Bot, req ChatRequest) {
	instructions := strings.TrimSpace(req.Message)
	if len([]rune(instructions)) > 8000 {
		c.JSON(400, gin.H{"error": "职责最多 8000 字，请精简后再发送"})
		return
	}
	conv := models.Conversation{BotID: bot.ID, Title: "定义职责"}
	var reply models.Message
	err := h.agents.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if req.ConversationID != "" {
			id, err := uuid.Parse(req.ConversationID)
			if err != nil {
				return gorm.ErrRecordNotFound
			}
			if err := tx.First(&conv, "id = ? AND bot_id = ?", id, bot.ID).Error; err != nil {
				return err
			}
		} else if err := tx.Create(&conv).Error; err != nil {
			return err
		}
		updated := tx.Model(&agent.Bot{}).Where("id = ? AND version = ? AND instructions = ? AND status = ?", bot.ID, bot.Version, "", "active").Updates(map[string]any{
			"instructions": instructions, "version": bot.Version + 1, "updated_at": time.Now(),
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return agent.ErrConflict
		}
		if err := tx.First(bot, "id = ?", bot.ID).Error; err != nil {
			return err
		}
		snapshot, err := json.Marshal(bot)
		if err != nil {
			return err
		}
		if err := tx.Create(&agent.BotVersion{BotID: bot.ID, Version: bot.Version, Snapshot: string(snapshot)}).Error; err != nil {
			return err
		}
		if req.ConversationID == "" {
			greeting := models.Message{ConversationID: conv.ID, Role: "assistant", Content: "你好，我是 " + bot.Name + "。我的职责是什么？请告诉我工作范围、做事方式，以及你期待的结果。"}
			if err := tx.Create(&greeting).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&models.Message{ConversationID: conv.ID, Role: "user", Content: instructions}).Error; err != nil {
			return err
		}
		reply = models.Message{ConversationID: conv.ID, Role: "assistant", Content: "收到，我已经记住自己的职责：\n\n" + instructions + "\n\n接下来，你想先交给我什么任务？"}
		if err := tx.Create(&reply).Error; err != nil {
			return err
		}
		return tx.Model(&conv).Update("updated_at", time.Now()).Error
	})
	if err != nil {
		status, message := 500, "保存职责失败，请重试"
		if errors.Is(err, agent.ErrConflict) {
			status, message = 409, "Bot 配置已更新，请刷新后重试"
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			status, message = 404, "对话不存在"
		}
		c.JSON(status, gin.H{"error": message})
		return
	}
	c.JSON(200, ChatResponse{ConversationID: conv.ID.String(), Message: reply})
}
