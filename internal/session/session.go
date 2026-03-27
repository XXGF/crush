// Package session 提供对话会话的管理功能。
//
// 会话是用户与 AI 代理之间的一次完整对话，包含消息历史、
// Token 使用量统计、任务列表等信息。支持父子会话关系。
package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/zeebo/xxh3"
)

// TodoStatus 表示任务项的状态。
type TodoStatus string

const (
	// TodoStatusPending 表示任务待处理。
	TodoStatusPending TodoStatus = "pending"
	// TodoStatusInProgress 表示任务进行中。
	TodoStatusInProgress TodoStatus = "in_progress"
	// TodoStatusCompleted 表示任务已完成。
	TodoStatusCompleted TodoStatus = "completed"
)

// HashID 使用 XXH3 算法将会话 ID（UUID）哈希为十六进制字符串。
// 用于生成短标识符。
func HashID(id string) string {
	h := xxh3.New()
	h.WriteString(id)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Todo 表示会话中的一个任务项。
type Todo struct {
	Content    string     `json:"content"`     // 任务内容
	Status     TodoStatus `json:"status"`      // 任务状态
	ActiveForm string     `json:"active_form"` // 当前活跃的表单
}

// HasIncompleteTodos 检查任务列表中是否存在未完成的任务。
func HasIncompleteTodos(todos []Todo) bool {
	for _, todo := range todos {
		if todo.Status != TodoStatusCompleted {
			return true
		}
	}
	return false
}

// Session 表示一个对话会话。
type Session struct {
	ID               string // 会话的唯一标识符
	ParentSessionID  string // 父会话 ID（用于子代理会话）
	Title            string // 会话标题
	MessageCount     int64  // 消息数量
	PromptTokens     int64  // 提示词 Token 使用量
	CompletionTokens int64  // 完成 Token 使用量
	SummaryMessageID string // 摘要消息 ID（用于自动摘要）
	Cost             float64 // 总费用
	Todos            []Todo // 任务列表
	CreatedAt        int64  // 创建时间戳
	UpdatedAt        int64  // 更新时间戳
}

// Service 定义了会话管理的服务接口。
type Service interface {
	pubsub.Subscriber[Session]
	// Create 创建一个新的会话。
	Create(ctx context.Context, title string) (Session, error)
	// CreateTitleSession 创建用于生成标题的子会话。
	CreateTitleSession(ctx context.Context, parentSessionID string) (Session, error)
	// CreateTaskSession 创建用于子代理任务的子会话。
	CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (Session, error)
	// Get 根据 ID 获取会话。
	Get(ctx context.Context, id string) (Session, error)
	// GetLast 获取最近的会话。
	GetLast(ctx context.Context) (Session, error)
	// List 列出所有会话。
	List(ctx context.Context) ([]Session, error)
	// Save 保存会话的完整状态。
	Save(ctx context.Context, session Session) (Session, error)
	// UpdateTitleAndUsage 原子更新会话的标题和使用量统计。
	UpdateTitleAndUsage(ctx context.Context, sessionID, title string, promptTokens, completionTokens int64, cost float64) error
	// Rename 仅更新会话标题。
	Rename(ctx context.Context, id string, title string) error
	// Delete 删除会话及其所有消息和文件。
	Delete(ctx context.Context, id string) error

	// CreateAgentToolSessionID 创建子代理会话的复合 ID，格式为 "messageID$$toolCallID"。
	CreateAgentToolSessionID(messageID, toolCallID string) string
	// ParseAgentToolSessionID 解析子代理会话 ID 为 messageID 和 toolCallID。
	ParseAgentToolSessionID(sessionID string) (messageID string, toolCallID string, ok bool)
	// IsAgentToolSession 判断会话 ID 是否为子代理会话格式。
	IsAgentToolSession(sessionID string) bool
}

// service 是 Service 接口的内部实现。
type service struct {
	*pubsub.Broker[Session]
	db *sql.DB
	q  *db.Queries
}

func (s *service) Create(ctx context.Context, title string) (Session, error) {
	dbSession, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:    uuid.New().String(),
		Title: title,
	})
	if err != nil {
		return Session{}, err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.CreatedEvent, session)
	event.SessionCreated()
	return session, nil
}

func (s *service) CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (Session, error) {
	dbSession, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:              toolCallID,
		ParentSessionID: sql.NullString{String: parentSessionID, Valid: true},
		Title:           title,
	})
	if err != nil {
		return Session{}, err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.CreatedEvent, session)
	return session, nil
}

func (s *service) CreateTitleSession(ctx context.Context, parentSessionID string) (Session, error) {
	dbSession, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:              "title-" + parentSessionID,
		ParentSessionID: sql.NullString{String: parentSessionID, Valid: true},
		Title:           "Generate a title",
	})
	if err != nil {
		return Session{}, err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.CreatedEvent, session)
	return session, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.q.WithTx(tx)

	dbSession, err := qtx.GetSessionByID(ctx, id)
	if err != nil {
		return err
	}
	if err = qtx.DeleteSessionMessages(ctx, dbSession.ID); err != nil {
		return fmt.Errorf("deleting session messages: %w", err)
	}
	if err = qtx.DeleteSessionFiles(ctx, dbSession.ID); err != nil {
		return fmt.Errorf("deleting session files: %w", err)
	}
	if err = qtx.DeleteSession(ctx, dbSession.ID); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.DeletedEvent, session)
	event.SessionDeleted()
	return nil
}

func (s *service) Get(ctx context.Context, id string) (Session, error) {
	dbSession, err := s.q.GetSessionByID(ctx, id)
	if err != nil {
		return Session{}, err
	}
	return s.fromDBItem(dbSession), nil
}

func (s *service) GetLast(ctx context.Context) (Session, error) {
	dbSession, err := s.q.GetLastSession(ctx)
	if err != nil {
		return Session{}, err
	}
	return s.fromDBItem(dbSession), nil
}

func (s *service) Save(ctx context.Context, session Session) (Session, error) {
	todosJSON, err := marshalTodos(session.Todos)
	if err != nil {
		return Session{}, err
	}

	dbSession, err := s.q.UpdateSession(ctx, db.UpdateSessionParams{
		ID:               session.ID,
		Title:            session.Title,
		PromptTokens:     session.PromptTokens,
		CompletionTokens: session.CompletionTokens,
		SummaryMessageID: sql.NullString{
			String: session.SummaryMessageID,
			Valid:  session.SummaryMessageID != "",
		},
		Cost: session.Cost,
		Todos: sql.NullString{
			String: todosJSON,
			Valid:  todosJSON != "",
		},
	})
	if err != nil {
		return Session{}, err
	}
	session = s.fromDBItem(dbSession)
	s.Publish(pubsub.UpdatedEvent, session)
	return session, nil
}

// UpdateTitleAndUsage 原子更新会话的标题和使用量字段。
// 相比 Save 更安全，避免了先读取再修改再保存的竞态问题。
func (s *service) UpdateTitleAndUsage(ctx context.Context, sessionID, title string, promptTokens, completionTokens int64, cost float64) error {
	return s.q.UpdateSessionTitleAndUsage(ctx, db.UpdateSessionTitleAndUsageParams{
		ID:               sessionID,
		Title:            title,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Cost:             cost,
	})
}

// Rename 仅更新会话标题，不修改 updated_at 和使用量字段。
func (s *service) Rename(ctx context.Context, id string, title string) error {
	return s.q.RenameSession(ctx, db.RenameSessionParams{
		ID:    id,
		Title: title,
	})
}

func (s *service) List(ctx context.Context) ([]Session, error) {
	dbSessions, err := s.q.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, len(dbSessions))
	for i, dbSession := range dbSessions {
		sessions[i] = s.fromDBItem(dbSession)
	}
	return sessions, nil
}

func (s service) fromDBItem(item db.Session) Session {
	todos, err := unmarshalTodos(item.Todos.String)
	if err != nil {
		slog.Error("Failed to unmarshal todos", "session_id", item.ID, "error", err)
	}
	return Session{
		ID:               item.ID,
		ParentSessionID:  item.ParentSessionID.String,
		Title:            item.Title,
		MessageCount:     item.MessageCount,
		PromptTokens:     item.PromptTokens,
		CompletionTokens: item.CompletionTokens,
		SummaryMessageID: item.SummaryMessageID.String,
		Cost:             item.Cost,
		Todos:            todos,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func marshalTodos(todos []Todo) (string, error) {
	if len(todos) == 0 {
		return "", nil
	}
	data, err := json.Marshal(todos)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalTodos(data string) ([]Todo, error) {
	if data == "" {
		return []Todo{}, nil
	}
	var todos []Todo
	if err := json.Unmarshal([]byte(data), &todos); err != nil {
		return []Todo{}, err
	}
	return todos, nil
}

// NewService 创建一个新的会话管理服务实例。
func NewService(q *db.Queries, conn *sql.DB) Service {
	broker := pubsub.NewBroker[Session]()
	return &service{
		Broker: broker,
		db:     conn,
		q:      q,
	}
}

// CreateAgentToolSessionID 创建子代理会话的复合 ID，格式为 "messageID$$toolCallID"。
func (s *service) CreateAgentToolSessionID(messageID, toolCallID string) string {
	return fmt.Sprintf("%s$$%s", messageID, toolCallID)
}

// ParseAgentToolSessionID 解析子代理会话 ID 为 messageID 和 toolCallID。
func (s *service) ParseAgentToolSessionID(sessionID string) (messageID string, toolCallID string, ok bool) {
	parts := strings.Split(sessionID, "$$")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// IsAgentToolSession 判断会话 ID 是否符合子代理会话的格式（包含 "$$" 分隔符）。
func (s *service) IsAgentToolSession(sessionID string) bool {
	_, _, ok := s.ParseAgentToolSessionID(sessionID)
	return ok
}
