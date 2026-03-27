package event

import (
	"fmt"
	"log/slog"

	"github.com/posthog/posthog-go"
)

// logger 是 PostHog 日志接口的适配器，将 PostHog 的日志转发到标准库 slog。
var _ posthog.Logger = logger{}

type logger struct{}

// Debugf 将 PostHog 的 Debug 级别日志转发到 slog.Debug。
func (logger) Debugf(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...))
}

// Logf 将 PostHog 的 Info 级别日志转发到 slog.Info。
func (logger) Logf(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

// Warnf 将 PostHog 的 Warn 级别日志转发到 slog.Warn。
func (logger) Warnf(format string, args ...any) {
	slog.Warn(fmt.Sprintf(format, args...))
}

// Errorf 将 PostHog 的 Error 级别日志转发到 slog.Error。
func (logger) Errorf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
}
