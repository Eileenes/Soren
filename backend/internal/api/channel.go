package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/soren/backend/internal/channel"
)

// ChannelHandler 消息渠道处理器
type ChannelHandler struct {
	manager *channel.Manager
}

// NewChannelHandler 创建消息渠道处理器
func NewChannelHandler(manager *channel.Manager) *ChannelHandler {
	return &ChannelHandler{manager: manager}
}

// ListChannels 列出所有渠道及其状态
//
// GET /api/channels
func (h *ChannelHandler) ListChannels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"channels": h.manager.Statuses(),
	})
}

// GetChannel 获取单个渠道状态
//
// GET /api/channels/:name
func (h *ChannelHandler) GetChannel(c *gin.Context) {
	name := c.Param("name")

	status, err := h.manager.Status(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// EnableChannel 启用渠道
//
// POST /api/channels/:name/enable
func (h *ChannelHandler) EnableChannel(c *gin.Context) {
	h.toggle(c, true)
}

// DisableChannel 禁用渠道
//
// POST /api/channels/:name/disable
func (h *ChannelHandler) DisableChannel(c *gin.Context) {
	h.toggle(c, false)
}

// toggle 启用或禁用渠道
func (h *ChannelHandler) toggle(c *gin.Context, enable bool) {
	name := c.Param("name")
	ctx := c.Request.Context()

	var err error
	if enable {
		err = h.manager.Enable(ctx, name)
	} else {
		err = h.manager.Disable(ctx, name)
	}

	if err != nil {
		switch {
		case errors.Is(err, channel.ErrChannelNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		default:
			// 启动失败通常是配置缺失或平台拒绝连接，返回 400 便于前端提示
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	status, _ := h.manager.Status(name)
	c.JSON(http.StatusOK, status)
}
