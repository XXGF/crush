package event

import (
	"time"
)

var appStartTime time.Time

// AppInitialized 记录应用程序初始化完成事件，并记录启动时间。
func AppInitialized() {
	appStartTime = time.Now()
	send("app initialized")
}

// AppExited 记录应用程序退出事件，包含运行时长信息，并刷新所有待发送事件。
func AppExited() {
	duration := time.Since(appStartTime).Truncate(time.Second)
	send(
		"app exited",
		"app duration pretty", duration.String(),
		"app duration in seconds", int64(duration.Seconds()),
	)
	Flush()
}

// SessionCreated 记录会话创建事件。
func SessionCreated() {
	send("session created")
}

// SessionDeleted 记录会话删除事件。
func SessionDeleted() {
	send("session deleted")
}

// SessionSwitched 记录会话切换事件。
func SessionSwitched() {
	send("session switched")
}

// FilePickerOpened 记录文件选择器打开事件。
func FilePickerOpened() {
	send("filepicker opened")
}

// PromptSent 记录提示词发送事件，支持附加自定义属性。
func PromptSent(props ...any) {
	send(
		"prompt sent",
		props...,
	)
}

// PromptResponded 记录提示词响应事件，支持附加自定义属性。
func PromptResponded(props ...any) {
	send(
		"prompt responded",
		props...,
	)
}

// TokensUsed 记录 Token 使用量事件，支持附加自定义属性。
func TokensUsed(props ...any) {
	send(
		"tokens used",
		props...,
	)
}

// StatsViewed 记录统计信息查看事件。
func StatsViewed() {
	send("stats viewed")
}

// SessionListed 记录会话列表查看事件。
func SessionListed(json bool) {
	send("session listed", "json", json)
}

// SessionShown 记录会话详情查看事件。
func SessionShown(json bool) {
	send("session shown", "json", json)
}

// SessionLastShown 记录最近会话查看事件。
func SessionLastShown(json bool) {
	send("session last shown", "json", json)
}

// SessionDeletedCommand 记录通过命令删除会话的事件。
func SessionDeletedCommand(json bool) {
	send("session deleted", "json", json)
}

// SessionRenamed 记录会话重命名事件。
func SessionRenamed(json bool) {
	send("session renamed", "json", json)
}
