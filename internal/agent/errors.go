package agent

import "errors"

// Agent 包级别的哨兵错误定义。
// 这些错误用于在 Agent 运行过程中标识特定的异常状态。
var (
	// ErrRequestCancelled 表示用户主动取消了当前请求。
	ErrRequestCancelled = errors.New("request canceled by user")
	// ErrSessionBusy 表示目标会话正在处理另一个请求，无法接受新的请求。
	ErrSessionBusy = errors.New("session is currently processing another request")
	// ErrEmptyPrompt 表示用户提交了空的提示词。
	ErrEmptyPrompt = errors.New("prompt is empty")
	// ErrSessionMissing 表示请求中缺少必要的会话 ID。
	ErrSessionMissing = errors.New("session id is missing")
)
