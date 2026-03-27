// Package db 提供 SQLite 数据库的连接和迁移管理。
//
// 使用 goose 进行数据库迁移，配置了 WAL 模式和其他性能优化参数。
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/pressly/goose/v3"
)

// pragmas 是 SQLite 连接的性能优化参数。
var pragmas = map[string]string{
	"foreign_keys":  "ON",     // 启用外键约束
	"journal_mode":  "WAL",    // 使用 WAL 日志模式，提升并发性能
	"page_size":     "4096",   // 页大小 4KB
	"cache_size":    "-8000",  // 缓存大小 8MB（负数表示 KB）
	"synchronous":   "NORMAL", // 同步模式，平衡性能和安全性
	"secure_delete": "ON",     // 安全删除，覆盖已删除数据
	"busy_timeout":  "30000",  // 忙等待超时 30 秒
}

// Connect 打开 SQLite 数据库连接并执行数据库迁移。
//
// 数据库文件位于 dataDir/crush.db。
// 连接成功后会自动应用所有待执行的迁移脚本。
func Connect(ctx context.Context, dataDir string) (*sql.DB, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data.dir is not set")
	}
	dbPath := filepath.Join(dataDir, "crush.db")

	db, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}

	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	goose.SetBaseFS(FS)

	if err := goose.SetDialect("sqlite3"); err != nil {
		slog.Error("Failed to set dialect", "error", err)
		return nil, fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		slog.Error("Failed to apply migrations", "error", err)
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return db, nil
}
