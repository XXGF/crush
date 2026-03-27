// Package permission 提供工具调用的权限管理服务。
//
// 当 AI 代理需要执行文件操作、Shell 命令等敏感操作时，
// 通过此服务向用户请求授权。支持以下授权模式：
//   - 单次授权：仅对当前操作生效
//   - 持久授权：对同一会话中相同工具+动作+路径的操作自动授权
//   - 会话自动批准：对指定会话的所有操作自动授权
//   - 工具白名单：指定工具无需授权
//   - YOLO 模式：跳过所有权限请求
package permission

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// ErrorPermissionDenied 表示用户拒绝了权限请求。
var ErrorPermissionDenied = errors.New("user denied permission")

// CreatePermissionRequest 是创建权限请求的参数。
type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`   // 所属会话 ID
	ToolCallID  string `json:"tool_call_id"` // 工具调用 ID
	ToolName    string `json:"tool_name"`    // 工具名称
	Description string `json:"description"`  // 操作描述
	Action      string `json:"action"`       // 操作类型（如 read、write、execute）
	Params      any    `json:"params"`       // 操作参数
	Path        string `json:"path"`         // 操作目标路径
}

// PermissionNotification 是权限处理结果的通知。
type PermissionNotification struct {
	ToolCallID string `json:"tool_call_id"` // 工具调用 ID
	Granted    bool   `json:"granted"`      // 是否已授权
	Denied     bool   `json:"denied"`       // 是否已拒绝
}

// PermissionRequest 表示一个待处理的权限请求。
type PermissionRequest struct {
	ID          string `json:"id"`           // 请求的唯一标识符
	SessionID   string `json:"session_id"`   // 所属会话 ID
	ToolCallID  string `json:"tool_call_id"` // 工具调用 ID
	ToolName    string `json:"tool_name"`    // 工具名称
	Description string `json:"description"`  // 操作描述
	Action      string `json:"action"`       // 操作类型
	Params      any    `json:"params"`       // 操作参数
	Path        string `json:"path"`         // 操作目标路径
}

// Service 定义了权限管理服务的接口。
type Service interface {
	pubsub.Subscriber[PermissionRequest]
	// GrantPersistent 授予权限并记录，同一会话中相同操作将自动授权。
	GrantPersistent(permission PermissionRequest)
	// Grant 授予一次性权限，不记录。
	Grant(permission PermissionRequest)
	// Deny 拒绝权限请求。
	Deny(permission PermissionRequest)
	// Request 发起权限请求并等待用户响应。返回是否授权。
	Request(ctx context.Context, opts CreatePermissionRequest) (bool, error)
	// AutoApproveSession 将指定会话设置为自动批准模式。
	AutoApproveSession(sessionID string)
	// SetSkipRequests 设置是否跳过所有权限请求（YOLO 模式）。
	SetSkipRequests(skip bool)
	// SkipRequests 返回是否处于 YOLO 模式。
	SkipRequests() bool
	// SubscribeNotifications 订阅权限处理结果的通知。
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification]
}

// permissionService 是 Service 接口的内部实现。
type permissionService struct {
	*pubsub.Broker[PermissionRequest]

	notificationBroker    *pubsub.Broker[PermissionNotification] // 权限通知的发布代理
	workingDir            string                                 // 工作目录
	sessionPermissions    []PermissionRequest                    // 已授权的持久权限列表
	sessionPermissionsMu  sync.RWMutex                           // 保护 sessionPermissions 的读写锁
	pendingRequests       *csync.Map[string, chan bool]           // 待处理的权限请求映射
	autoApproveSessions   map[string]bool                        // 自动批准的会话 ID 集合
	autoApproveSessionsMu sync.RWMutex                           // 保护 autoApproveSessions 的读写锁
	skip                  bool                                   // YOLO 模式标志
	allowedTools          []string                               // 工具白名单

	// requestMu 确保同一时间只处理一个权限请求
	requestMu       sync.Mutex
	activeRequest   *PermissionRequest // 当前活跃的权限请求
	activeRequestMu sync.Mutex         // 保护 activeRequest 的互斥锁
}

// GrantPersistent 授予权限并记录到会话权限列表中。
// 同一会话中相同工具+动作+路径的后续请求将自动授权。
func (s *permissionService) GrantPersistent(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
		Granted:    true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	s.sessionPermissionsMu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	s.sessionPermissionsMu.Unlock()

	s.activeRequestMu.Lock()
	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
	s.activeRequestMu.Unlock()
}

// Grant 授予一次性权限，不记录到会话权限列表。
func (s *permissionService) Grant(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
		Granted:    true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	s.activeRequestMu.Lock()
	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
	s.activeRequestMu.Unlock()
}

// Deny 拒绝权限请求。
func (s *permissionService) Deny(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
		Granted:    false,
		Denied:     true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- false
	}

	s.activeRequestMu.Lock()
	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
	s.activeRequestMu.Unlock()
}

// Request 发起权限请求并阻塞等待用户响应。
//
// 处理流程：
//  1. YOLO 模式下直接返回 true
//  2. 检查工具白名单
//  3. 检查会话自动批准
//  4. 检查已有的持久授权记录
//  5. 向 UI 发布权限请求并等待响应
func (s *permissionService) Request(ctx context.Context, opts CreatePermissionRequest) (bool, error) {
	if s.skip {
		return true, nil
	}

	// Check if the tool/action combination is in the allowlist
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true, nil
	}

	// tell the UI that a permission was requested
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: opts.ToolCallID,
	})
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	s.autoApproveSessionsMu.RLock()
	autoApprove := s.autoApproveSessions[opts.SessionID]
	s.autoApproveSessionsMu.RUnlock()

	if autoApprove {
		s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
			ToolCallID: opts.ToolCallID,
			Granted:    true,
		})
		return true, nil
	}

	fileInfo, err := os.Stat(opts.Path)
	dir := opts.Path
	if err == nil {
		if fileInfo.IsDir() {
			dir = opts.Path
		} else {
			dir = filepath.Dir(opts.Path)
		}
	}

	if dir == "." {
		dir = s.workingDir
	}
	permission := PermissionRequest{
		ID:          uuid.New().String(),
		Path:        dir,
		SessionID:   opts.SessionID,
		ToolCallID:  opts.ToolCallID,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
	}

	s.sessionPermissionsMu.RLock()
	for _, p := range s.sessionPermissions {
		if p.ToolName == permission.ToolName && p.Action == permission.Action && p.SessionID == permission.SessionID && p.Path == permission.Path {
			s.sessionPermissionsMu.RUnlock()
			s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
				ToolCallID: opts.ToolCallID,
				Granted:    true,
			})
			return true, nil
		}
	}
	s.sessionPermissionsMu.RUnlock()

	s.activeRequestMu.Lock()
	s.activeRequest = &permission
	s.activeRequestMu.Unlock()

	respCh := make(chan bool, 1)
	s.pendingRequests.Set(permission.ID, respCh)
	defer s.pendingRequests.Del(permission.ID)

	// Publish the request
	s.Publish(pubsub.CreatedEvent, permission)

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case granted := <-respCh:
		return granted, nil
	}
}

// AutoApproveSession 将指定会话设置为自动批准模式。
func (s *permissionService) AutoApproveSession(sessionID string) {
	s.autoApproveSessionsMu.Lock()
	s.autoApproveSessions[sessionID] = true
	s.autoApproveSessionsMu.Unlock()
}

// SubscribeNotifications 订阅权限处理结果的通知。
func (s *permissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification] {
	return s.notificationBroker.Subscribe(ctx)
}

// SetSkipRequests 设置是否跳过所有权限请求（YOLO 模式）。
func (s *permissionService) SetSkipRequests(skip bool) {
	s.skip = skip
}

// SkipRequests 返回是否处于 YOLO 模式。
func (s *permissionService) SkipRequests() bool {
	return s.skip
}

// NewPermissionService 创建一个新的权限管理服务实例。
//
// 参数：
//   - workingDir: 工作目录路径
//   - skip: 是否启用 YOLO 模式（跳过所有权限请求）
//   - allowedTools: 工具白名单，格式为 "toolName" 或 "toolName:action"
func NewPermissionService(workingDir string, skip bool, allowedTools []string) Service {
	return &permissionService{
		Broker:              pubsub.NewBroker[PermissionRequest](),
		notificationBroker:  pubsub.NewBroker[PermissionNotification](),
		workingDir:          workingDir,
		sessionPermissions:  make([]PermissionRequest, 0),
		autoApproveSessions: make(map[string]bool),
		skip:                skip,
		allowedTools:        allowedTools,
		pendingRequests:     csync.NewMap[string, chan bool](),
	}
}
