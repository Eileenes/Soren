package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/models"
	"github.com/soren/backend/internal/store"

	"net/http"
	"net/http/httptest"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func TestAppCORSAllowsOnlyTrustedOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(cors.New(appCORSConfig()))
	router.GET("/api/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, test := range []struct {
		origin  string
		allowed bool
	}{
		{"http://localhost:1420", true},
		{"http://127.0.0.1:1420", true},
		{"tauri://localhost", true},
		{"http://tauri.localhost", true},
		{"https://tauri.localhost", true},
		{"https://untrusted.example", false},
		{"http://localhost:1420.untrusted.example", false},
		{"http://127.0.0.1:1421", false},
		{"null", false},
	} {
		t.Run(test.origin, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Access-Control-Request-Method", "GET")
			request.Header.Set("Access-Control-Request-Headers", "X-User-ID")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if test.allowed {
				if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != test.origin {
					t.Fatalf("trusted app origin rejected: status=%d headers=%v", response.Code, response.Header())
				}
			} else if response.Code != http.StatusForbidden || response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatalf("untrusted origin allowed: status=%d headers=%v", response.Code, response.Header())
			}
		})
	}
}

func TestAppCORSAllowsBotNavigationPatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(cors.New(appCORSConfig()))
	router.PATCH("/api/bots/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	for _, origin := range []string{"http://localhost:1420", "tauri://localhost", "http://tauri.localhost"} {
		request := httptest.NewRequest(http.MethodOptions, "/api/bots/writer", nil)
		request.Header.Set("Origin", origin)
		request.Header.Set("Access-Control-Request-Method", "PATCH")
		request.Header.Set("Access-Control-Request-Headers", "Content-Type, X-Bot-ID")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent || !strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), "PATCH") {
			t.Fatalf("Bot navigation preflight rejected for %s: status=%d headers=%v", origin, response.Code, response.Header())
		}
	}
}

func TestDefaultModelUsesDesktopSelection(t *testing.T) {
	for _, provider := range []string{"openai", "anthropic"} {
		cfg := &config.Config{Model: "my-provider-model"}
		if provider == "anthropic" {
			cfg.AnthropicAPIKey = "test-key"
		}
		if got := defaultModel(cfg); got != cfg.Model {
			t.Fatalf("selected model ignored: %s", got)
		}
	}
}

// TestGroupMigrationBacksUpBeforeChangingSchema 验证群聊 schema 变更前确实留下备份
//
// 现有的 bot_id 守卫是一次性历史迁移，对任何已升级的安装恒为 false，
// 不能指望它兜住本次变更。
func TestGroupMigrationBacksUpBeforeChangingSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migration.db")
	cfg := &config.Config{StorageDriver: config.StorageDriverSQLite, SQLitePath: path}

	db, err := store.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	// 只迁移到旧版本：群表与 speaker_bot_id 都不存在
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}); err != nil {
		t.Fatalf("迁移测试数据库失败: %v", err)
	}
	if !groupSchemaMissing(db) {
		t.Fatal("旧版本数据库应被判定为缺少群聊 schema")
	}

	backup, err := backupBeforeGroupMigration(db, cfg)
	if err != nil {
		t.Fatalf("备份失败: %v", err)
	}
	if backup == "" {
		t.Fatal("群聊 schema 变更前应生成备份文件")
	}
	info, err := os.Stat(backup)
	if err != nil {
		t.Fatalf("备份文件不存在: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Errorf("备份文件权限应为 0600，实际 %o", mode)
	}

	// 迁移完成后不应重复备份
	if err := db.AutoMigrate(&models.GroupChat{}, &models.GroupMember{}, &models.GroupTurn{}); err != nil {
		t.Fatalf("迁移群表失败: %v", err)
	}
	if again, err := backupBeforeGroupMigration(db, cfg); err != nil || again != "" {
		t.Errorf("schema 已是最新时不应再备份，得到 %q / %v", again, err)
	}

	// 非 SQLite 部署不落本地备份文件
	postgres := &config.Config{StorageDriver: config.StorageDriverPostgres, DatabaseURL: "postgres://localhost/soren"}
	if got, err := backupBeforeGroupMigration(db, postgres); err != nil || got != "" {
		t.Errorf("postgres 部署不应走文件备份，得到 %q / %v", got, err)
	}
}

// 表齐全但新增列缺失时同样要备份：AutoMigrate 对已存在的表走 ALTER TABLE 加列，
// 只检查 HasTable 的守卫会漏判，导致这次 ALTER 没有任何回退。
func TestGroupSchemaMissingDetectsNewColumnOnExistingTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "newcol.db")
	cfg := &config.Config{StorageDriver: config.StorageDriverSQLite, SQLitePath: path}
	db, err := store.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}, &models.GroupChat{}, &models.GroupMember{}, &models.GroupTurn{}); err != nil {
		t.Fatalf("迁移测试数据库失败: %v", err)
	}
	if groupSchemaMissing(db) {
		t.Fatal("全部列齐全时不应判定为缺失")
	}

	if err := db.Migrator().DropColumn(&models.GroupChat{}, "pinned_at"); err != nil {
		t.Skipf("当前 SQLite 不支持 DROP COLUMN，跳过：%v", err)
	}
	if !groupSchemaMissing(db) {
		t.Fatal("已有表缺少新列时应判定为 schema 变更")
	}
	backup, err := backupBeforeGroupMigration(db, cfg)
	if err != nil {
		t.Fatalf("备份失败: %v", err)
	}
	if backup == "" {
		t.Fatal("给已存在的群表加列前也应生成备份")
	}
	if info, err := os.Stat(backup); err != nil || info.Mode().Perm() != 0600 {
		t.Errorf("备份文件应存在且权限 0600，得到 %v / %v", info, err)
	}
}

func TestGroupFingerprintMigrationRequiresBackup(t *testing.T) {
	cfg := &config.Config{StorageDriver: config.StorageDriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "fingerprint.db")}
	db, err := store.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}, &models.GroupChat{}, &models.GroupMember{}, &models.GroupTurn{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&models.GroupTurn{}, "request_fingerprint"); err != nil {
		t.Fatal(err)
	}
	backup, err := backupBeforeGroupMigration(db, cfg)
	if err != nil || backup == "" {
		t.Fatalf("missing backup before fingerprint migration: %q, %v", backup, err)
	}
	if err := db.AutoMigrate(&models.GroupTurn{}); err != nil {
		t.Fatal(err)
	}
	if groupSchemaMissing(db) {
		t.Fatal("schema still reported missing after migration")
	}
}
