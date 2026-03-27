// Package filetracker 提供会话中文件读取记录的跟踪功能。
//
// 用于记录和查询每个会话中文件的读取时间和历史，
// 支持判断文件是否已被读取以及获取最后读取时间。
package filetracker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/crush/internal/db"
)

// Service 定义了文件读取跟踪的服务接口。
type Service interface {
	// RecordRead 记录文件被读取的事件。
	RecordRead(ctx context.Context, sessionID, path string)

	// LastReadTime 返回文件最后一次被读取的时间。
	// 如果文件从未被读取，返回零值时间。
	LastReadTime(ctx context.Context, sessionID, path string) time.Time

	// ListReadFiles 返回指定会话中所有已读取文件的绝对路径列表。
	ListReadFiles(ctx context.Context, sessionID string) ([]string, error)
}

// service 是 Service 接口的内部实现，基于 SQLite 数据库存储。
type service struct {
	q *db.Queries
}

// NewService 创建一个新的文件跟踪服务实例。
func NewService(q *db.Queries) Service {
	return &service{q: q}
}

// RecordRead 记录文件被读取的事件。
// 文件路径会被转换为相对于工作目录的相对路径后存储。
func (s *service) RecordRead(ctx context.Context, sessionID, path string) {
	if err := s.q.RecordFileRead(ctx, db.RecordFileReadParams{
		SessionID: sessionID,
		Path:      relpath(path),
	}); err != nil {
		slog.Error("记录文件读取失败", "error", err, "file", path)
	}
}

// LastReadTime 返回文件最后一次被读取的时间。
// 如果文件从未被读取或查询失败，返回零值时间。
func (s *service) LastReadTime(ctx context.Context, sessionID, path string) time.Time {
	readFile, err := s.q.GetFileRead(ctx, db.GetFileReadParams{
		SessionID: sessionID,
		Path:      relpath(path),
	})
	if err != nil {
		return time.Time{}
	}

	return time.Unix(readFile.ReadAt, 0)
}

// relpath 将绝对路径转换为相对于当前工作目录的相对路径。
// 转换失败时返回原始路径。
func relpath(path string) string {
	path = filepath.Clean(path)
	basepath, err := os.Getwd()
	if err != nil {
		slog.Warn("获取工作目录失败", "error", err)
		return path
	}
	relpath, err := filepath.Rel(basepath, path)
	if err != nil {
		slog.Warn("计算相对路径失败", "error", err)
		return path
	}
	return relpath
}

// ListReadFiles 返回指定会话中所有已读取文件的绝对路径列表。
// 存储的相对路径会被转换回基于当前工作目录的绝对路径。
func (s *service) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	readFiles, err := s.q.ListSessionReadFiles(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("列出已读取文件失败: %w", err)
	}

	basepath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("获取工作目录失败: %w", err)
	}

	paths := make([]string, 0, len(readFiles))
	for _, rf := range readFiles {
		paths = append(paths, filepath.Join(basepath, rf.Path))
	}
	return paths, nil
}
