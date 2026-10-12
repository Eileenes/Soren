package store

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/soren/backend/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenDB 根据配置打开数据库连接
//
// 桌面端默认使用嵌入式 SQLite（纯 Go 实现，无需 cgo，无需外部服务）；
// 设置 STORAGE_DRIVER=postgres 可切换到 PostgreSQL 作为服务端部署的升级路径。
func OpenDB(cfg *config.Config) (*gorm.DB, error) {
	switch cfg.StorageDriver {
	case config.StorageDriverPostgres:
		log.Print("Storage driver: postgres")
		return openPostgres(cfg.DatabaseURL)
	case config.StorageDriverSQLite, "":
		log.Printf("Storage driver: sqlite (%s)", cfg.SQLitePath)
		return openSQLite(cfg.SQLitePath)
	default:
		return nil, fmt.Errorf("unknown storage driver: %q (supported: sqlite, postgres)", cfg.StorageDriver)
	}
}

// openSQLite 打开嵌入式 SQLite 数据库
func openSQLite(path string) (*gorm.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite path is empty")
	}

	// 确保数据库文件所在目录存在（默认 ~/.soren）
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create data directory %s: %w", dir, err)
	}

	// Create the database privately before SQLite creates its WAL/SHM siblings.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("cannot create private database: %w", err)
	}
	if err = file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Chmod(path+suffix, 0o600); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	// _pragma 参数用于启用 WAL 与外键约束，兼顾并发读性能与数据完整性
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// SQLite 写入需要串行化，限制连接数避免 database is locked
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	return db, nil
}

// openPostgres 打开 PostgreSQL 数据库
func openPostgres(databaseURL string) (*gorm.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is empty")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	return db, nil
}
