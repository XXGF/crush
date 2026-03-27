// Package event 提供应用程序的遥测事件跟踪功能。
//
// 通过 PostHog 分析平台收集匿名使用数据，包括应用启动/退出、
// 会话操作、提示词发送/响应、Token 使用量等事件。
package event

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"time"

	"github.com/charmbracelet/crush/internal/version"
	"github.com/posthog/posthog-go"
)

const (
	// endpoint 是 PostHog 数据收集端点。
	endpoint = "https://data.charm.land"
	// key 是 PostHog 项目的 API 密钥。
	key = "phc_4zt4VgDWLqbYnJYEwLRxFoaTL2noNrQij0C6E8k3I0V"

	// nonInteractiveAttrName 是非交互模式属性名。
	nonInteractiveAttrName = "NonInteractive"
	// continueSessionByIDAttrName 是通过 ID 继续会话的属性名。
	continueSessionByIDAttrName = "ContinueSessionByID"
	// continueLastSessionAttrName 是继续上次会话的属性名。
	continueLastSessionAttrName = "ContinueLastSession"
)

var (
	// client 是 PostHog 客户端实例，在 Init 中初始化。
	client posthog.Client

	// baseProps 是所有事件共享的基础属性，包含系统信息和版本号。
	baseProps = posthog.NewProperties().
			Set("GOOS", runtime.GOOS).
			Set("GOARCH", runtime.GOARCH).
			Set("TERM", os.Getenv("TERM")).
			Set("SHELL", filepath.Base(os.Getenv("SHELL"))).
			Set("Version", version.Version).
			Set("GoVersion", runtime.Version()).
			Set(nonInteractiveAttrName, false)
)

// SetNonInteractive 设置是否为非交互模式的遥测属性。
func SetNonInteractive(nonInteractive bool) {
	baseProps = baseProps.Set(nonInteractiveAttrName, nonInteractive)
}

// SetContinueBySessionID 设置是否通过会话 ID 继续的遥测属性。
func SetContinueBySessionID(continueBySessionID bool) {
	baseProps = baseProps.Set(continueSessionByIDAttrName, continueBySessionID)
}

// SetContinueLastSession 设置是否继续上次会话的遥测属性。
func SetContinueLastSession(continueLastSession bool) {
	baseProps = baseProps.Set(continueLastSessionAttrName, continueLastSession)
}

// Init 初始化 PostHog 客户端和设备标识符。
// 关闭超时时间为 500ms，确保应用退出时不会长时间等待。
func Init() {
	c, err := posthog.NewWithConfig(key, posthog.Config{
		Endpoint:        endpoint,
		Logger:          logger{},
		ShutdownTimeout: 500 * time.Millisecond,
	})
	if err != nil {
		slog.Error("Failed to initialize PostHog client", "error", err)
	}
	client = c
	distinctId = getDistinctId()
}

// GetID 返回当前设备的唯一标识符。
func GetID() string { return distinctId }

// Alias 将设备 ID 与用户 ID 关联，用于 PostHog 的用户身份合并。
func Alias(userID string) {
	if client == nil || distinctId == fallbackId || distinctId == "" || userID == "" {
		return
	}
	if err := client.Enqueue(posthog.Alias{
		DistinctId: distinctId,
		Alias:      userID,
	}); err != nil {
		slog.Error("Failed to enqueue PostHog alias event", "error", err)
		return
	}
	slog.Info("Aliased in PostHog", "machine_id", distinctId, "user_id", userID)
}

// send 向 PostHog 发送一个遥测事件。
// props 参数为键值对形式，会与 baseProps 合并。
func send(event string, props ...any) {
	if client == nil {
		return
	}
	err := client.Enqueue(posthog.Capture{
		DistinctId: distinctId,
		Event:      event,
		Properties: pairsToProps(props...).Merge(baseProps),
	})
	if err != nil {
		slog.Error("Failed to enqueue PostHog event", "event", event, "props", props, "error", err)
		return
	}
}

// Error 向 PostHog 发送一个异常事件，包含错误类型和消息。
func Error(errToLog any, props ...any) {
	if client == nil || distinctId == "" || errToLog == nil {
		return
	}
	posthogErr := client.Enqueue(posthog.NewDefaultException(
		time.Now(),
		distinctId,
		reflect.TypeOf(errToLog).String(),
		fmt.Sprintf("%v", errToLog),
	))
	if posthogErr != nil {
		slog.Error("Failed to enqueue PostHog error", "err", errToLog, "props", props, "posthogErr", posthogErr)
		return
	}
}

// Flush 刷新并关闭 PostHog 客户端，确保所有待发送的事件被提交。
func Flush() {
	if client == nil {
		return
	}
	if err := client.Close(); err != nil {
		slog.Error("Failed to flush PostHog events", "error", err)
	}
}

// pairsToProps 将键值对参数列表转换为 PostHog 属性对象。
// 参数必须为偶数个，奇数位为字符串类型的键，偶数位为对应的值。
func pairsToProps(props ...any) posthog.Properties {
	p := posthog.NewProperties()

	if !isEven(len(props)) {
		slog.Error("Event properties must be provided as key-value pairs", "props", props)
		return p
	}

	for i := 0; i < len(props); i += 2 {
		key := props[i].(string)
		value := props[i+1]
		p = p.Set(key, value)
	}
	return p
}

// isEven 判断整数是否为偶数。
func isEven(n int) bool {
	return n%2 == 0
}
