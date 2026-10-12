package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// 存储驱动常量
const (
	// StorageDriverSQLite 嵌入式 SQLite（桌面端默认，零配置）
	StorageDriverSQLite = "sqlite"
	// StorageDriverPostgres PostgreSQL（可选升级路径）
	StorageDriverPostgres = "postgres"

	// CacheDriverMemory 进程内缓存（桌面端默认，无需 Redis）
	CacheDriverMemory = "memory"
	// CacheDriverRedis Redis 缓存（可选）
	CacheDriverRedis = "redis"

	// 渠道 Webhook 默认端口
	// 与 API 服务端口（默认 8080）错开，避免端口占用
	defaultFeishuWebhookPort   = 8081
	defaultDingTalkWebhookPort = 8082
)

// Config 应用配置
type Config struct {
	ServerHost string
	ServerPort string

	// 存储配置
	StorageDriver string // sqlite | postgres
	SQLitePath    string // SQLite 数据库文件路径
	DatabaseURL   string // PostgreSQL 连接串（仅当 StorageDriver=postgres 时使用）
	DataDir       string // 数据目录，默认 ~/.soren

	// 缓存配置
	CacheDriver string // memory | redis
	RedisURL    string // Redis 地址（仅当 CacheDriver=redis 时使用）

	// 记忆系统配置
	MemoryEnabled     bool // 是否启用长期记忆
	MemoryRecallLimit int  // 每轮对话注入的召回记忆条数

	// LLM 配置
	Model            string
	OpenAIAPIKey     string
	OpenAIBaseURL    string
	AnthropicAPIKey  string
	AnthropicBaseURL string

	// 消息渠道配置（均为可选，未配置对应项时该渠道不会自动启动，也不影响服务启动）
	Feishu   FeishuConfig
	DingTalk DingTalkConfig
}

// 渠道接入模式
const (
	// ChannelModeWS WebSocket / Stream 长连接模式，桌面端与内网环境无需公网 IP
	ChannelModeWS = "ws"
	// ChannelModeWebhook Webhook 回调模式，需要公网可访问的地址
	ChannelModeWebhook = "webhook"
)

// FeishuConfig 飞书渠道配置
type FeishuConfig struct {
	Enabled           bool   // 是否启用渠道
	AppID             string // 应用 AppID
	AppSecret         string // 应用 AppSecret
	VerificationToken string // 事件订阅 Verification Token，用于校验事件来源
	EncryptKey        string // 事件订阅 Encrypt Key，配置后事件体为加密推送
	Mode              string // 接入模式：ws（默认，长连接）| webhook
	BaseURL           string // 开放平台地址，飞书为 https://open.feishu.cn
	// WebhookPort Webhook 模式下的监听端口
	// 必须与 API 服务端口不同，也不能与钉钉渠道冲突
	WebhookPort int
}

// DingTalkConfig 钉钉渠道配置
type DingTalkConfig struct {
	Enabled       bool   // 是否启用渠道
	ClientID      string // 应用 AppKey（新版控制台称为 ClientID）
	ClientSecret  string // 应用 AppSecret（新版控制台称为 ClientSecret）
	RobotCode     string // 机器人 RobotCode，通常与应用 AppKey 相同
	Mode          string // 接入模式：ws（默认，Stream 长连接）| webhook
	WebhookSecret string // 机器人加签密钥，用于校验 Webhook 请求签名
	// WebhookPort Webhook 模式下的监听端口，需与 API 服务端口不同
	WebhookPort int
}

// Ready 判断飞书渠道是否具备启动条件
func (c FeishuConfig) Ready() bool {
	return c.Enabled && c.AppID != "" && c.AppSecret != ""
}

// Ready 判断钉钉渠道是否具备启动条件
func (c DingTalkConfig) Ready() bool {
	return c.Enabled && c.ClientID != "" && c.ClientSecret != ""
}

// TransportMode 返回飞书渠道的接入模式，未配置时默认使用长连接
func (c FeishuConfig) TransportMode() string {
	return normalizeChannelMode(c.Mode)
}

// TransportMode 返回钉钉渠道的接入模式，未配置时默认使用 Stream 长连接
func (c DingTalkConfig) TransportMode() string {
	return normalizeChannelMode(c.Mode)
}

// normalizeChannelMode 归一化接入模式
//
// 桌面端默认走长连接：无需公网 IP，开箱即用；只有显式配置 webhook 才切到回调模式。
func normalizeChannelMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), ChannelModeWebhook) {
		return ChannelModeWebhook
	}
	return ChannelModeWS
}

// Load 加载配置
func Load() *Config {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// 数据目录：桌面端默认放在用户目录下，保证开箱即用
	dataDir := getEnv("SOREN_DATA_DIR", defaultDataDir())

	// 默认 SQLite 文件位于数据目录内
	sqlitePath := getEnv("SQLITE_PATH", filepath.Join(dataDir, "soren.db"))

	return &Config{
		ServerHost: getEnv("SERVER_HOST", "127.0.0.1"),
		ServerPort: getEnv("SERVER_PORT", "8080"),

		// 存储层：默认 SQLite，只能通过显式配置切换到 PostgreSQL
		StorageDriver: getEnv("STORAGE_DRIVER", StorageDriverSQLite),
		SQLitePath:    sqlitePath,
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/soren?sslmode=disable"),
		DataDir:       dataDir,

		// 缓存层：默认进程内存缓存，Redis 可选
		CacheDriver: getEnv("CACHE_DRIVER", CacheDriverMemory),
		RedisURL:    getEnv("REDIS_URL", "localhost:6379"),

		// 记忆系统默认开启，召回 10 条
		MemoryEnabled:     getEnvBool("MEMORY_ENABLED", true),
		MemoryRecallLimit: getEnvInt("MEMORY_RECALL_LIMIT", 10),

		OpenAIAPIKey:     getEnv("OPENAI_API_KEY", ""),
		Model:            strings.TrimSpace(getEnv("SOREN_MODEL", "")),
		OpenAIBaseURL:    getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		AnthropicAPIKey:  getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicBaseURL: getEnv("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),

		// 飞书渠道：默认关闭，未配置时不会影响服务启动
		Feishu: FeishuConfig{
			Enabled:           getEnvBool("FEISHU_ENABLED", false),
			AppID:             getEnv("FEISHU_APP_ID", ""),
			AppSecret:         getEnv("FEISHU_APP_SECRET", ""),
			VerificationToken: getEnv("FEISHU_VERIFICATION_TOKEN", ""),
			EncryptKey:        getEnv("FEISHU_ENCRYPT_KEY", ""),
			// 桌面端默认走长连接，无需公网 IP
			Mode:        getEnv("FEISHU_MODE", ChannelModeWS),
			BaseURL:     getEnv("FEISHU_BASE_URL", "https://open.feishu.cn"),
			WebhookPort: getEnvInt("FEISHU_WEBHOOK_PORT", defaultFeishuWebhookPort),
		},

		// 钉钉渠道：默认关闭，未配置时不会影响服务启动
		DingTalk: DingTalkConfig{
			Enabled:       getEnvBool("DINGTALK_ENABLED", false),
			ClientID:      getEnv("DINGTALK_CLIENT_ID", ""),
			ClientSecret:  getEnv("DINGTALK_CLIENT_SECRET", ""),
			RobotCode:     getEnv("DINGTALK_ROBOT_CODE", ""),
			Mode:          getEnv("DINGTALK_MODE", ChannelModeWS),
			WebhookSecret: getEnv("DINGTALK_WEBHOOK_SECRET", ""),
			WebhookPort:   getEnvInt("DINGTALK_WEBHOOK_PORT", defaultDingTalkWebhookPort),
		},
	}
}

// defaultDataDir 返回默认数据目录（~/.soren）
func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// 极端情况下退回到当前目录，保证仍然能启动
		return ".soren"
	}
	return filepath.Join(home, ".soren")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
