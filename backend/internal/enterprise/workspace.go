package enterprise

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/api"
	"github.com/soren/backend/internal/cache"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/connectors"
	"github.com/soren/backend/internal/group"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/memory"
	"github.com/soren/backend/internal/models"
	"github.com/soren/backend/internal/repository"
	"github.com/soren/backend/internal/store"
	"gorm.io/gorm"
)

// One immutable PostgreSQL search_path per workspace pool. No request can select
// a schema, and raw Table("bots") queries and background jobs stay in the same scope.
// Version 1 runs as a single server, enforced by a database advisory lock in cmd.
type Workspaces struct {
	Config              Config
	Root                *gorm.DB
	Provider            llm.Provider
	Model, ProviderName string
	Active              func(string) error
	mu                  sync.Mutex
	handlers            map[string]http.Handler
	slots               chan struct{}
}

func NewWorkspaces(c Config, root *gorm.DB, p llm.Provider, model, name string) *Workspaces {
	return &Workspaces{Config: c, Root: root, Provider: p, Model: model, ProviderName: name, handlers: map[string]http.Handler{}, slots: make(chan struct{}, c.Concurrency)}
}
func (ws *Workspaces) Handler(u User) (http.Handler, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if h := ws.handlers[u.ID]; h != nil {
		return h, nil
	}
	id, err := uuid.Parse(u.ID)
	if err != nil || id.String() != u.ID {
		return nil, errors.New("空间身份无效")
	}
	schema := "workspace_" + strings.ReplaceAll(u.ID, "-", "")
	if err = ws.Root.Exec(`CREATE SCHEMA IF NOT EXISTS "` + schema + `"`).Error; err != nil {
		return nil, err
	}
	dsn, err := url.Parse(ws.Config.DatabaseURL)
	if err != nil {
		return nil, err
	}
	q := dsn.Query()
	q.Set("search_path", schema)
	dsn.RawQuery = q.Encode()
	db, err := store.OpenDB(&config.Config{StorageDriver: config.StorageDriverPostgres, DatabaseURL: dsn.String()})
	if err != nil {
		return nil, err
	}
	sql, err := db.DB()
	if err != nil {
		return nil, err
	}
	sql.SetMaxOpenConns(2)
	sql.SetMaxIdleConns(0)
	sql.SetConnMaxLifetime(5 * time.Minute)
	dir := filepath.Join(ws.Config.DataDir, "workspaces", u.ID)
	if err = os.MkdirAll(dir, 0700); err != nil {
		_ = sql.Close()
		return nil, err
	}
	authorize := func() error {
		if ws.Active == nil {
			return errors.New("身份服务未就绪")
		}
		return ws.Active(u.ID)
	}
	p := &limitedProvider{inner: ws.Provider, slots: ws.slots, model: ws.Model, active: authorize}
	h, err := WorkspaceHandler(db, dir, ws.Config.Key, u.ID, p, ws.Model, ws.ProviderName, authorize)
	if err != nil {
		_ = sql.Close()
		return nil, err
	}
	ws.handlers[u.ID] = h
	return h, nil
}

// WorkspaceHandler accepts a workspace-scoped database, never the shared root DB.
// Tests use distinct temporary SQLite databases to exercise all cross-user routes.
func WorkspaceHandler(db *gorm.DB, dir string, key []byte, id string, p llm.Provider, model, providerName string, active func() error) (http.Handler, error) {
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}, &models.GroupChat{}, &models.GroupMember{}, &models.GroupTurn{}); err != nil {
		return nil, err
	}
	cached := cache.NewMemoryCache(0)
	opts := memory.DefaultManagerOptions()
	opts.Enabled = true
	opts.Cache = cached
	mem, err := memory.NewMemoryManager(db, cached, opts)
	if err != nil {
		return nil, err
	}
	a, err := agent.New(db, p, mem, model, providerName == "local-echo")
	if err != nil {
		return nil, err
	}
	v, err := newVault(db, key, id)
	if err != nil {
		return nil, err
	}
	connections, err := connectors.New(db, v, dir)
	if err != nil {
		return nil, err
	}
	a.AttachTools(connections)
	previous := connections.Authorize
	connections.Authorize = func(run string) error {
		if err := active(); err != nil {
			return err
		}
		return previous(run)
	}
	repo := repository.NewConversationRepository(db)
	groups := repository.NewGroupRepository(db)
	orch := group.New(a, repo, groups, p, mem)
	if err = orch.RecoverInterrupted(context.Background()); err != nil {
		return nil, err
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/identity") || strings.HasPrefix(path, "/api/connections/gmail") || strings.HasPrefix(path, "/api/template") || strings.HasPrefix(path, "/api/bot-templates") || strings.HasSuffix(path, "/template") {
			c.AbortWithStatusJSON(409, gin.H{"error": "此功能暂不支持公司空间，请使用个人空间；内部 Bot 分享将单独开放"})
			return
		}
		c.Next()
	})
	g := r.Group("/api")
	g.Use(api.BotContext(a))
	api.RegisterAgents(g, a)
	api.RegisterBotConfiguration(g, a)
	api.RegisterConnectors(g, connections, "")
	chat := api.NewChatHandler(repo, p, mem).WithAgents(a)
	g.POST("/chat", chat.SendMessage)
	g.GET("/conversations", chat.GetConversations)
	g.GET("/conversations/:id", chat.GetConversation)
	g.DELETE("/conversations/:id", chat.DeleteConversation)
	m := api.NewMemoryHandler(mem)
	g.GET("/memories", m.ListMemories)
	g.GET("/memories/search", m.SearchMemories)
	g.GET("/memories/stats", m.GetMemoryStats)
	g.DELETE("/memories/:id", m.DeleteMemory)
	g.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "storageDriver": db.Dialector.Name(), "cacheDriver": "memory", "memoryEnabled": true, "llmProvider": providerName, "demoMode": providerName == "local-echo", "mode": "enterprise", "model": model})
	})
	g.GET("/channels", func(c *gin.Context) { c.JSON(200, gin.H{"channels": []any{}}) })
	api.RegisterGroups(r, api.NewGroupHandler(orch, groups, repo, a))
	return r, nil
}

type limitedProvider struct {
	inner  llm.Provider
	slots  chan struct{}
	model  string
	active func() error
}

func (p *limitedProvider) acquire(ctx context.Context) error {
	if err := p.active(); err != nil {
		return err
	}
	select {
	case p.slots <- struct{}{}:
		if err := p.active(); err != nil {
			<-p.slots
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *limitedProvider) options(in *llm.Options) *llm.Options {
	out := llm.Options{}
	if in != nil {
		out = *in
	}
	out.Model = p.model
	if out.MaxTokens <= 0 || out.MaxTokens > 8192 {
		out.MaxTokens = 8192
	}
	return &out
}
func (p *limitedProvider) Chat(ctx context.Context, m []llm.Message, o *llm.Options) (*llm.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := p.acquire(ctx); err != nil {
		return nil, err
	}
	defer func() { <-p.slots }()
	return p.inner.Chat(ctx, m, p.options(o))
}
func (p *limitedProvider) StreamChat(ctx context.Context, m []llm.Message, o *llm.Options, ch chan<- llm.StreamChunk) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := p.acquire(ctx); err != nil {
		return err
	}
	defer func() { <-p.slots }()
	return p.inner.StreamChat(ctx, m, p.options(o), ch)
}
