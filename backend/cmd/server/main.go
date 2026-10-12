package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/api"
	"github.com/soren/backend/internal/cache"
	"github.com/soren/backend/internal/channel"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/connectors"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/memory"
	"github.com/soren/backend/internal/middleware"
	"github.com/soren/backend/internal/mobile"
	"github.com/soren/backend/internal/models"
	"github.com/soren/backend/internal/repository"
	"github.com/soren/backend/internal/store"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 打开数据库：默认嵌入式 SQLite（~/.soren/soren.db），零配置开箱即用；
	// 设置 STORAGE_DRIVER=postgres 可切换到 PostgreSQL 作为服务端部署的升级路径。
	db, err := store.OpenDB(cfg)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Back up the existing SQLite database before adding Bot ownership.
	if cfg.StorageDriver == config.StorageDriverSQLite && db.Migrator().HasTable(&models.Conversation{}) && !db.Migrator().HasColumn(&models.Conversation{}, "bot_id") {
		backup := cfg.SQLitePath + ".before-bots-" + time.Now().Format("20060102-150405")
		if err := db.Exec("VACUUM INTO ?", backup).Error; err != nil {
			log.Fatalf("Cannot back up database before migration: %v", err)
		}
		_ = os.Chmod(backup, 0600)
	}
	// 自动迁移数据库表
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}, &models.LLMConfig{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// 初始化缓存：默认进程内缓存（TTL + LRU），无需 Redis；CACHE_DRIVER=redis 可切换
	cacheClient, err := cache.New(cfg.CacheDriver, cfg.RedisURL, 0)
	if err != nil {
		if cfg.CacheDriver == config.CacheDriverRedis {
			log.Fatalf("Failed to connect to Redis: %v", err)
		}
		log.Printf("Cache driver %q unavailable (%v), falling back to in-memory cache", cfg.CacheDriver, err)
		cacheClient = cache.NewMemoryCache(0)
	}
	defer func() {
		if err := cacheClient.Close(); err != nil {
			log.Printf("Failed to close cache: %v", err)
		}
	}()
	log.Printf("Cache driver: %s", cfg.CacheDriver)

	// 初始化长期记忆系统（内部自动迁移 memories/tags/memory_tags/user_preferences 表）
	var memoryManager *memory.MemoryManager
	if cfg.MemoryEnabled {
		opts := memory.DefaultManagerOptions()
		opts.Enabled = true
		opts.RecallLimit = cfg.MemoryRecallLimit
		opts.Cache = cacheClient

		memoryManager, err = memory.NewMemoryManager(db, cacheClient, opts)
		if err != nil {
			// 记忆系统初始化失败不应导致服务不可用
			log.Printf("Memory system disabled: %v", err)
			memoryManager = nil
		} else {
			log.Printf("Long-term memory system enabled (recall limit %d)", cfg.MemoryRecallLimit)
		}
	} else {
		log.Println("Long-term memory system disabled by configuration")
	}

	// 初始化仓储
	convRepo := repository.NewConversationRepository(db)

	// 初始化 LLM 提供商
	//
	// 可用的服务商按优先级选择：Anthropic > OpenAI 兼容接口。
	// 若都没有配置 API Key，则退回本地回显提供商而不是直接退出：
	// 桌面端首次启动不应该因为缺少凭据而崩溃，用户可以在设置页补齐配置。
	var provider llm.Provider
	providerName := "local-echo"
	switch {
	case cfg.AnthropicAPIKey != "":
		provider = llm.NewAnthropicProvider(cfg.AnthropicAPIKey, cfg.AnthropicBaseURL)
		providerName = "anthropic"
		log.Println("LLM provider: Anthropic")
	case cfg.OpenAIAPIKey != "":
		provider = llm.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL)
		providerName = "openai-compatible"
		log.Println("LLM provider: OpenAI-compatible")
	default:
		provider = llm.NewMockProvider()
		log.Println("LLM provider: local echo (no API key configured — set OPENAI_API_KEY or ANTHROPIC_API_KEY in Settings)")
	}

	// 初始化处理器
	agents, err := agent.New(db, provider, memoryManager, defaultModel(cfg), providerName == "local-echo")
	if err != nil {
		log.Fatalf("Cannot start Bot runtime: %v", err)
	}
	tools, err := connectors.New(db, connectors.NewKeychain(cfg.DataDir), cfg.DataDir)
	if err != nil {
		log.Fatalf("Cannot start connectors: %v", err)
	}
	agents.AttachTools(tools)
	chatHandler := api.NewChatHandler(convRepo, provider, memoryManager).WithAgents(agents)

	// 初始化消息渠道（全部可选：未配置的渠道保持未启用状态，不影响服务启动）
	channelManager := channel.NewManager()

	feishuChannel := channel.NewFeishuChannel(cfg.Feishu)
	channelManager.Register(feishuChannel, feishuChannel.Status())

	dingtalkChannel := channel.NewDingTalkChannel(cfg.DingTalk)
	channelManager.Register(dingtalkChannel, dingtalkChannel.Status())

	// 消息路由：平台消息 → LLM → 回复平台
	router := channel.NewRouter(channelManager, convRepo, provider, &llm.Options{
		Model:       defaultModel(cfg),
		Temperature: 0.7,
		MaxTokens:   2000,
	})
	router.Start()

	// 启动配置完整的渠道，单个渠道启动失败不影响整体服务
	startChannels(channelManager, cfg)

	channelHandler := api.NewChannelHandler(channelManager)

	// 初始化Gin引擎
	r := gin.New()
	r.Use(gin.Recovery(), middleware.LocalOnly())

	// 中间件
	r.Use(middleware.Logger())
	r.Use(cors.New(appCORSConfig()))

	// 路由
	apiGroup := r.Group("/api")
	apiGroup.Use(api.BotContext(agents))
	api.RegisterAgents(apiGroup, agents)
	api.RegisterConnectors(apiGroup, tools, "http://127.0.0.1:"+cfg.ServerPort+"/oauth/google/callback")
	api.RegisterOAuthCallback(r, tools)
	{
		// 聊天相关
		apiGroup.POST("/chat", chatHandler.SendMessage)
		apiGroup.GET("/conversations", chatHandler.GetConversations)
		apiGroup.GET("/conversations/:id", chatHandler.GetConversation)
		apiGroup.DELETE("/conversations/:id", chatHandler.DeleteConversation)

		// 健康检查
		apiGroup.GET("/health", func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()

			status := gin.H{
				"status":        "ok",
				"storageDriver": cfg.StorageDriver,
				"cacheDriver":   cfg.CacheDriver,
				"memoryEnabled": cfg.MemoryEnabled,
				"llmProvider":   providerName,
				"demoMode":      providerName == "local-echo",
			}
			if cacheClient != nil {
				status["cacheHealthy"] = cacheClient.Ping(ctx) == nil
			}
			c.JSON(200, status)
		})

		// 记忆相关：让用户能查看和管理 AI 记住的内容
		if memoryManager != nil {
			memoryHandler := api.NewMemoryHandler(memoryManager)

			apiGroup.GET("/memories", memoryHandler.ListMemories)
			apiGroup.GET("/memories/search", memoryHandler.SearchMemories)
			apiGroup.GET("/memories/stats", memoryHandler.GetMemoryStats)
			apiGroup.DELETE("/memories/:id", memoryHandler.DeleteMemory)
		}

		// 消息渠道
		apiGroup.GET("/channels", channelHandler.ListChannels)
		apiGroup.GET("/channels/:name", channelHandler.GetChannel)
		apiGroup.POST("/channels/:name/enable", channelHandler.EnableChannel)
		apiGroup.POST("/channels/:name/disable", channelHandler.DisableChannel)
	}

	relayURL, mobileURL := mobile.DefaultRelayURL, mobile.DefaultMobileURL
	if value := os.Getenv("SOREN_RELAY_URL"); value != "" {
		relayURL = value
	}
	if value := os.Getenv("SOREN_MOBILE_URL"); value != "" {
		mobileURL = value
	}
	bridge, bridgeErr := mobile.New(db, cfg.DataDir, r, relayURL, mobileURL, os.Getenv("SOREN_MOBILE_DEV") == "1")
	if bridgeErr != nil {
		log.Printf("Mobile bridge unavailable: %v", bridgeErr)
		r.GET("/api/mobile", func(c *gin.Context) {
			c.JSON(200, gin.H{"available": false, "enabled": false, "online": false, "devices": []any{}, "pending": nil})
		})
	} else {
		bridge.Register(r)
		go bridge.Run(context.Background())
	}

	// 优雅退出：停止消息路由与各渠道的长连接
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down channels...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		router.Stop()
		channelManager.StopAll(ctx)
		os.Exit(0)
	}()

	// 启动服务器
	address := net.JoinHostPort(cfg.ServerHost, cfg.ServerPort)
	log.Printf("Starting server on %s", address)
	if err := r.Run(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// appCORSConfig restricts browser access to the desktop app and its dev server.
// Loopback binding and CORS protect the local default; remote deployments still
// need authentication and a trusted gateway before exposing these APIs.
func appCORSConfig() cors.Config {
	return cors.Config{
		AllowOriginFunc: func(origin string) bool {
			switch origin {
			case "http://localhost:1420", "http://127.0.0.1:1420",
				"tauri://localhost", "http://tauri.localhost", "https://tauri.localhost":
				return true
			default:
				return false
			}
		},
		AllowMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Accept", "Authorization", "X-User-ID", "X-Bot-ID"},
		ExposeHeaders: []string{"Content-Length"},
	}
}

// startChannels 启动所有配置完整的渠道
//
// 渠道是可选项：未配置或启动失败时只记录日志，不影响服务本身启动。
func startChannels(manager *channel.Manager, cfg *config.Config) {
	defer func() {
		// 渠道启动过程中的异常不应影响主服务
		if r := recover(); r != nil {
			log.Printf("[channel] panic while starting channels: %v", r)
		}
	}()

	if cfg.Feishu.Ready() {
		if err := manager.Enable(context.Background(), channel.FeishuChannelName); err != nil {
			log.Printf("Failed to start feishu channel: %v", err)
		}
	}

	if cfg.DingTalk.Ready() {
		if err := manager.Enable(context.Background(), channel.DingTalkChannelName); err != nil {
			log.Printf("Failed to start dingtalk channel: %v", err)
		}
	}
}

// defaultModel 按当前提供商返回默认模型名
func defaultModel(cfg *config.Config) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	if cfg.AnthropicAPIKey != "" && cfg.OpenAIAPIKey == "" {
		return "claude-sonnet-4-5"
	}
	return "gpt-4o-mini"
}
