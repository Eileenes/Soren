package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/group"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/models"
	"github.com/soren/backend/internal/repository"
	"gorm.io/gorm"
)

// deleteWaitTimeout 删除群时等待编排器退出的上限
const deleteWaitTimeout = 15 * time.Second

// GroupHandler 群聊处理器
type GroupHandler struct {
	orch     *group.Orchestrator
	groups   *repository.GroupRepository
	convRepo *repository.ConversationRepository
	agents   *agent.Service
}

// NewGroupHandler 创建群聊处理器
func NewGroupHandler(
	orch *group.Orchestrator,
	groups *repository.GroupRepository,
	convRepo *repository.ConversationRepository,
	agents *agent.Service,
) *GroupHandler {
	return &GroupHandler{orch: orch, groups: groups, convRepo: convRepo, agents: agents}
}

// RegisterGroups 注册群聊路由
//
// 必须挂在根路由上而不是 /api 分组：分组级中间件 api.BotContext 对该分组
// 的所有路由生效，会强制解析 X-Bot-ID，而群聊的身份来自群成员而非单个 Bot。
// 群路由的 context 里不存在 bot，Bot 身份必须由处理器自己逐个解析。
func RegisterGroups(r *gin.Engine, h *GroupHandler) {
	r.GET("/api/groups", h.List)
	r.POST("/api/groups", h.Create)
	r.GET("/api/groups/:id", h.Get)
	r.PATCH("/api/groups/:id", h.Update)
	r.DELETE("/api/groups/:id", h.Delete)
	r.POST("/api/groups/:id/messages", h.SendMessage)
	r.POST("/api/groups/:id/cancel", h.Cancel)
}

// GroupMemberView 群成员展示信息
type GroupMemberView struct {
	BotID  string `json:"botId"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Status string `json:"status"`
}

// GroupView 群聊视图
type GroupView struct {
	ID                 string            `json:"id"`
	Title              string            `json:"title"`
	Topic              string            `json:"topic"`
	Mode               string            `json:"mode"`
	MaxRounds          int               `json:"maxRounds"`
	Pinned             bool              `json:"pinned"`
	PinnedAt           *time.Time        `json:"pinnedAt,omitempty"`
	LastMessageAt      *time.Time        `json:"lastMessageAt,omitempty"`
	LastMessagePreview string            `json:"lastMessagePreview,omitempty"`
	Members            []GroupMemberView `json:"members"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
}

// GroupDetailView 群聊详情
//
// 群对话挂在 Conversation.BotID = "group:<id>" 上，而 apiGroup 上的
// /conversations 接口要求 X-Bot-ID 解析出一个真实 Bot，因此群消息只能从这里取。
type GroupDetailView struct {
	GroupView
	ConversationID string            `json:"conversationId,omitempty"`
	Messages       []models.Message  `json:"messages"`
	LatestTurn     *models.GroupTurn `json:"latestTurn,omitempty"`
}

// List 获取群聊列表
func (h *GroupHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	groups, err := h.groups.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载群聊"})
		return
	}
	members, err := h.groups.MemberIDsByGroups(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载群聊成员"})
		return
	}
	// 一次性取回全部 Bot，避免逐群逐成员查询在单连接 SQLite 下变成 N+1
	lookup := h.botLookup(ctx, allMemberIDs(members))
	views := make([]GroupView, 0, len(groups))
	for _, item := range groups {
		views = append(views, h.view(item, members[item.ID], lookup))
	}
	c.JSON(http.StatusOK, views)
}

// Create 创建群聊
func (h *GroupHandler) Create(c *gin.Context) {
	var req struct {
		Title     string   `json:"title"`
		Topic     string   `json:"topic"`
		BotIDs    []string `json:"botIds"`
		Mode      string   `json:"mode"`
		MaxRounds int      `json:"maxRounds"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群聊配置无效"})
		return
	}
	title, topic, ok := groupTitleAndTopic(c, req.Title, req.Topic)
	if !ok {
		return
	}
	mode, maxRounds, ok := groupModeAndRounds(c, req.Mode, req.MaxRounds)
	if !ok {
		return
	}
	members, ok := h.validateMembers(c, req.BotIDs)
	if !ok {
		return
	}

	item := &models.GroupChat{ID: uuid.New(), Title: title, Topic: topic, Mode: mode, MaxRounds: maxRounds}
	if err := h.groups.Create(c.Request.Context(), item, members); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建群聊失败"})
		return
	}
	lookup := h.botLookup(c.Request.Context(), members)
	c.JSON(http.StatusCreated, h.view(*item, members, lookup))
}

// Get 获取群聊详情（含群历史）
func (h *GroupHandler) Get(c *gin.Context) {
	id, ok := groupID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	item, err := h.groups.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "群聊不存在"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载群聊"})
		return
	}
	members, err := h.groups.MemberIDs(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载群聊成员"})
		return
	}
	// 先读状态再读消息：观察到终态时，随后返回的历史必须包含该轮全部写入。
	turn, err := h.groups.LatestTurn(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载群讨论状态"})
		return
	}
	detail := GroupDetailView{GroupView: h.view(*item, members, h.botLookup(ctx, members)), Messages: []models.Message{}, LatestTurn: turn}
	conversations, err := h.convRepo.GetAllForBot(ctx, models.GroupBotID(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载群对话"})
		return
	}
	if len(conversations) > 0 {
		detail.ConversationID = conversations[0].ID.String()
		messages, err := h.convRepo.GetMessages(ctx, conversations[0].ID, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载群消息"})
			return
		}
		detail.Messages = messages
	}
	c.JSON(http.StatusOK, detail)
}

// Update 更新群聊
func (h *GroupHandler) Update(c *gin.Context) {
	id, ok := groupID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	item, err := h.groups.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "群聊不存在"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载群聊"})
		return
	}

	var req struct {
		Title     *string   `json:"title"`
		Topic     *string   `json:"topic"`
		BotIDs    *[]string `json:"botIds"`
		Mode      *string   `json:"mode"`
		MaxRounds *int      `json:"maxRounds"`
		Pinned    *bool     `json:"pinned"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群聊配置无效"})
		return
	}

	// 未提供的字段一律沿用现值：PATCH 语义是局部更新，
	// 用空值补齐会把 auto 群悄悄改回 mention，或让只改主题的请求因缺群名被拒。
	title, topic := item.Title, item.Topic
	nextTitle, nextTopic := title, topic
	if req.Title != nil {
		nextTitle = *req.Title
	}
	if req.Topic != nil {
		nextTopic = *req.Topic
	}
	if nextTitle != title || nextTopic != topic {
		if title, topic, ok = groupTitleAndTopic(c, nextTitle, nextTopic); !ok {
			return
		}
	}

	mode, maxRounds := item.Mode, item.MaxRounds
	nextMode, nextRounds := mode, maxRounds
	if req.Mode != nil {
		nextMode = strings.TrimSpace(*req.Mode)
	}
	if req.MaxRounds != nil {
		nextRounds = *req.MaxRounds
	}
	if nextMode != mode || nextRounds != maxRounds {
		if mode, maxRounds, ok = groupModeAndRounds(c, nextMode, nextRounds); !ok {
			return
		}
	}

	// 置顶同样走局部更新：请求里没带 pinned 就保持原值。
	// PinnedAt 只由服务端推导，不接受客户端传入——否则客户端可以把自己排到最前面，
	// 也可以伪造一个很旧的置顶时刻抢到置顶区首位。
	pinned, pinnedAt := item.Pinned, item.PinnedAt
	if req.Pinned != nil {
		pinned = *req.Pinned
		switch {
		case !pinned:
			pinnedAt = nil
		case pinnedAt == nil:
			now := time.Now()
			pinnedAt = &now
		}
		// 已经置顶时重复置顶保留原时刻，避免同一次置顶被反复推后
	}

	members := []string{}
	if req.BotIDs != nil {
		members, ok = h.validateMembers(c, *req.BotIDs)
		if !ok {
			return
		}
	} else if members, err = h.groups.MemberIDs(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载群聊成员"})
		return
	}

	item.Title, item.Topic, item.Mode, item.MaxRounds = title, topic, mode, maxRounds
	item.Pinned, item.PinnedAt = pinned, pinnedAt
	if err := h.groups.Update(ctx, item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存群聊失败"})
		return
	}
	if req.BotIDs != nil {
		if err := h.groups.ReplaceMembers(ctx, id, members); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存群成员失败"})
			return
		}
	}
	c.JSON(http.StatusOK, h.view(*item, members, h.botLookup(ctx, members)))
}

// Delete 删除群聊及其对话与消息
func (h *GroupHandler) Delete(c *gin.Context) {
	id, ok := groupID(c)
	if !ok {
		return
	}
	if err := h.orch.Delete(c.Request.Context(), id, deleteWaitTimeout); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "群聊已删除"})
}

// SendMessage 在群里发言
func (h *GroupHandler) SendMessage(c *gin.Context) {
	id, ok := groupID(c)
	if !ok {
		return
	}
	var req struct {
		Content       string       `json:"content"`
		Mentions      []string     `json:"mentions"`
		Mode          string       `json:"mode"`
		RequestID     string       `json:"requestId"`
		ToolExecution bool         `json:"toolExecution"`
		Config        *llm.Options `json:"config,omitempty"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "发言内容无效"})
		return
	}

	result, err := h.orch.Speak(c.Request.Context(), group.SpeakRequest{
		GroupID:       id,
		Content:       req.Content,
		Mentions:      req.Mentions,
		Mode:          req.Mode,
		RequestID:     req.RequestID,
		ToolExecution: req.ToolExecution,
		Config:        req.Config,
	})
	if err != nil {
		code := http.StatusInternalServerError
		var ge *group.Error
		if errors.As(err, &ge) {
			code = ge.Code
		}
		payload := gin.H{"error": err.Error()}
		// 全部 Bot 调用失败时仍要把已产生的消息带回去，前端才能接着显示
		if result != nil {
			payload["conversationId"] = result.ConversationID
			payload["status"] = result.Status
			payload["messages"] = result.Messages
		}
		c.JSON(code, payload)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Cancel 中止进行中的群讨论
func (h *GroupHandler) Cancel(c *gin.Context) {
	id, ok := groupID(c)
	if !ok {
		return
	}
	if err := h.orch.Cancel(id); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelling"})
}

// view 组装群聊视图
func (h *GroupHandler) view(item models.GroupChat, members []string, lookup map[string]agent.Bot) GroupView {
	views := make([]GroupMemberView, 0, len(members))
	for _, id := range members {
		entry := GroupMemberView{BotID: id}
		if bot, ok := lookup[id]; ok {
			entry.Name, entry.Avatar, entry.Status = bot.Name, bot.Avatar, bot.Status
		}
		views = append(views, entry)
	}
	return GroupView{
		ID:                 item.ID.String(),
		Title:              item.Title,
		Topic:              item.Topic,
		Mode:               item.Mode,
		MaxRounds:          item.MaxRounds,
		Pinned:             item.Pinned,
		PinnedAt:           item.PinnedAt,
		LastMessageAt:      item.LastMessageAt,
		LastMessagePreview: item.LastMessagePreview,
		Members:            views,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

// botLookup 一次查回所有 Bot
func (h *GroupHandler) botLookup(ctx context.Context, ids []string) map[string]agent.Bot {
	lookup := map[string]agent.Bot{}
	if len(ids) == 0 {
		return lookup
	}
	var bots []agent.Bot
	if err := h.agents.DB.Where("id IN ?", ids).Find(&bots).Error; err != nil {
		return lookup
	}
	for _, bot := range bots {
		lookup[bot.ID] = bot
	}
	return lookup
}

// validateMembers 校验群成员名单
func (h *GroupHandler) validateMembers(c *gin.Context, botIDs []string) ([]string, bool) {
	if len(botIDs) < group.MinMembers || len(botIDs) > group.MaxMembers {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群成员需为 2–5 个 Bot"})
		return nil, false
	}
	seen := map[string]bool{}
	members := make([]string, 0, len(botIDs))
	for _, id := range botIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "群成员不能为空"})
			return nil, false
		}
		if seen[id] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "群成员不能重复"})
			return nil, false
		}
		seen[id] = true
		bot, err := h.agents.GetBot(id)
		if err != nil || bot.Status != "active" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "群成员必须存在且处于可用状态"})
			return nil, false
		}
		members = append(members, id)
	}
	return members, true
}

func (h *GroupHandler) fail(c *gin.Context, err error) {
	code := http.StatusInternalServerError
	var ge *group.Error
	if errors.As(err, &ge) {
		code = ge.Code
	}
	c.JSON(code, gin.H{"error": err.Error()})
}

func groupID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群聊 ID 无效"})
		return uuid.Nil, false
	}
	return id, true
}

func groupTitleAndTopic(c *gin.Context, title, topic string) (string, string, bool) {
	title = strings.TrimSpace(title)
	topic = strings.TrimSpace(topic)
	if title == "" || len([]rune(title)) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写群名，最多 100 字"})
		return "", "", false
	}
	if len([]rune(topic)) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "主题最多 2000 字"})
		return "", "", false
	}
	return title, topic, true
}

func groupModeAndRounds(c *gin.Context, mode string, maxRounds int) (string, int, bool) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = group.ModeMention
	}
	if mode != group.ModeMention && mode != group.ModeAuto {
		c.JSON(http.StatusBadRequest, gin.H{"error": "发言模式无效"})
		return "", 0, false
	}
	if maxRounds == 0 {
		maxRounds = group.MaxRoundsDef
	}
	if maxRounds < 1 || maxRounds > group.MaxRoundsCap {
		c.JSON(http.StatusBadRequest, gin.H{"error": "讨论轮次需为 1–6"})
		return "", 0, false
	}
	return mode, maxRounds, true
}

func allMemberIDs(members map[uuid.UUID][]string) []string {
	ids := make([]string, 0, len(members))
	for _, groupIDs := range members {
		ids = append(ids, groupIDs...)
	}
	return ids
}
