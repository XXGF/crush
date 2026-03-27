// Package history 提供会话中文件版本历史的管理功能。
//
// 支持文件的多版本存储、查询和删除，基于 SQLite 数据库存储。
// 版本号自动递增，支持事务重试以处理并发冲突。
package history

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

const (
	// InitialVersion 是文件的初始版本号。
	InitialVersion = 0
)

// File 表示一个文件的某个版本快照。
type File struct {
	ID        string // 版本记录的唯一标识符
	SessionID string // 所属会话 ID
	Path      string // 文件路径
	Content   string // 文件内容
	Version   int64  // 版本号，从 0 开始递增
	CreatedAt int64  // 创建时间戳（Unix 秒）
	UpdatedAt int64  // 更新时间戳（Unix 秒）
}

// Service 管理会话中文件的版本历史。
type Service interface {
	pubsub.Subscriber[File]
	// Create 创建文件的初始版本（版本号为 0）。
	Create(ctx context.Context, sessionID, path, content string) (File, error)

	// CreateVersion 创建文件的新版本，版本号自动递增。
	CreateVersion(ctx context.Context, sessionID, path, content string) (File, error)

	// Get 根据 ID 获取文件版本。
	Get(ctx context.Context, id string) (File, error)
	// GetByPathAndSession 根据文件路径和会话 ID 获取文件版本。
	GetByPathAndSession(ctx context.Context, path, sessionID string) (File, error)
	// ListBySession 列出指定会话的所有文件版本。
	ListBySession(ctx context.Context, sessionID string) ([]File, error)
	// ListLatestSessionFiles 列出指定会话中每个文件的最新版本。
	ListLatestSessionFiles(ctx context.Context, sessionID string) ([]File, error)
	// Delete 删除指定的文件版本。
	Delete(ctx context.Context, id string) error
	// DeleteSessionFiles 删除指定会话的所有文件版本。
	DeleteSessionFiles(ctx context.Context, sessionID string) error
}

// service 是 Service 接口的内部实现。
type service struct {
	*pubsub.Broker[File]
	db *sql.DB
	q  *db.Queries
}

// NewService 创建一个新的文件历史服务实例。
func NewService(q *db.Queries, db *sql.DB) Service {
	return &service{
		Broker: pubsub.NewBroker[File](),
		q:      q,
		db:     db,
	}
}

// Create 创建文件的初始版本（版本号为 0）。
func (s *service) Create(ctx context.Context, sessionID, path, content string) (File, error) {
	return s.createWithVersion(ctx, sessionID, path, content, InitialVersion)
}

// CreateVersion 创建文件的新版本，版本号自动递增。
// 如果该路径不存在历史版本，则创建初始版本。
func (s *service) CreateVersion(ctx context.Context, sessionID, path, content string) (File, error) {
	// Get the latest version for this path
	files, err := s.q.ListFilesByPath(ctx, path)
	if err != nil {
		return File{}, err
	}

	if len(files) == 0 {
		// No previous versions, create initial
		return s.Create(ctx, sessionID, path, content)
	}

	// Get the latest version
	latestFile := files[0] // Files are ordered by version DESC, created_at DESC
	nextVersion := latestFile.Version + 1

	return s.createWithVersion(ctx, sessionID, path, content, nextVersion)
}

// createWithVersion 在事务中创建指定版本号的文件记录。
// 支持最多 3 次重试以处理唯一约束冲突（自动递增版本号）。
func (s *service) createWithVersion(ctx context.Context, sessionID, path, content string, version int64) (File, error) {
	// Maximum number of retries for transaction conflicts
	const maxRetries = 3
	var file File
	var err error

	// Retry loop for transaction conflicts
	for attempt := range maxRetries {
		// Start a transaction
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return File{}, fmt.Errorf("failed to begin transaction: %w", txErr)
		}

		// Create a new queries instance with the transaction
		qtx := s.q.WithTx(tx)

		// Try to create the file within the transaction
		dbFile, txErr := qtx.CreateFile(ctx, db.CreateFileParams{
			ID:        uuid.New().String(),
			SessionID: sessionID,
			Path:      path,
			Content:   content,
			Version:   version,
		})
		if txErr != nil {
			// Rollback the transaction
			tx.Rollback()

			// Check if this is a uniqueness constraint violation
			if strings.Contains(txErr.Error(), "UNIQUE constraint failed") {
				if attempt < maxRetries-1 {
					// If we have retries left, increment version and try again
					version++
					continue
				}
			}
			return File{}, txErr
		}

		// Commit the transaction
		if txErr = tx.Commit(); txErr != nil {
			return File{}, fmt.Errorf("failed to commit transaction: %w", txErr)
		}

		file = s.fromDBItem(dbFile)
		s.Publish(pubsub.CreatedEvent, file)
		return file, nil
	}

	return file, err
}

func (s *service) Get(ctx context.Context, id string) (File, error) {
	dbFile, err := s.q.GetFile(ctx, id)
	if err != nil {
		return File{}, err
	}
	return s.fromDBItem(dbFile), nil
}

func (s *service) GetByPathAndSession(ctx context.Context, path, sessionID string) (File, error) {
	dbFile, err := s.q.GetFileByPathAndSession(ctx, db.GetFileByPathAndSessionParams{
		Path:      path,
		SessionID: sessionID,
	})
	if err != nil {
		return File{}, err
	}
	return s.fromDBItem(dbFile), nil
}

func (s *service) ListBySession(ctx context.Context, sessionID string) ([]File, error) {
	dbFiles, err := s.q.ListFilesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	files := make([]File, len(dbFiles))
	for i, dbFile := range dbFiles {
		files[i] = s.fromDBItem(dbFile)
	}
	return files, nil
}

func (s *service) ListLatestSessionFiles(ctx context.Context, sessionID string) ([]File, error) {
	dbFiles, err := s.q.ListLatestSessionFiles(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	files := make([]File, len(dbFiles))
	for i, dbFile := range dbFiles {
		files[i] = s.fromDBItem(dbFile)
	}
	return files, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	file, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	err = s.q.DeleteFile(ctx, id)
	if err != nil {
		return err
	}
	s.Publish(pubsub.DeletedEvent, file)
	return nil
}

func (s *service) DeleteSessionFiles(ctx context.Context, sessionID string) error {
	files, err := s.ListBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, file := range files {
		err = s.Delete(ctx, file.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// fromDBItem 将数据库模型转换为业务模型。
func (s *service) fromDBItem(item db.File) File {
	return File{
		ID:        item.ID,
		SessionID: item.SessionID,
		Path:      item.Path,
		Content:   item.Content,
		Version:   item.Version,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}
