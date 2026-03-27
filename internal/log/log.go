// Package log 提供应用程序的日志初始化和 panic 恢复功能。
//
// 使用 lumberjack 实现日志文件的自动轮转，支持 JSON 格式输出。
package log

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/event"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	initOnce    sync.Once
	initialized atomic.Bool
)

// Setup 初始化日志系统，配置日志文件轮转和日志级别。
//
// 日志文件配置：
//   - 最大单文件 10MB
//   - 保留 30 天
//   - 不压缩、不保留备份
//
// debug 为 true 时日志级别为 Debug，否则为 Info。
// 该函数只会执行一次，重复调用无效。
func Setup(logFile string, debug bool) {
	initOnce.Do(func() {
		logRotator := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    10,    // Max size in MB
			MaxBackups: 0,     // Number of backups
			MaxAge:     30,    // Days
			Compress:   false, // Enable compression
		}

		level := slog.LevelInfo
		if debug {
			level = slog.LevelDebug
		}

		logger := slog.NewJSONHandler(logRotator, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})

		slog.SetDefault(slog.New(logger))
		initialized.Store(true)
	})
}

// Initialized 返回日志系统是否已初始化。
func Initialized() bool {
	return initialized.Load()
}

// RecoverPanic 捕获并记录 panic 信息。
//
// 当发生 panic 时：
//  1. 向 PostHog 发送错误事件
//  2. 创建带时间戳的 panic 日志文件（crush-panic-{name}-{timestamp}.log）
//  3. 写入 panic 信息和堆栈跟踪
//  4. 执行可选的清理回调函数
//
// 应在 defer 语句中使用：defer log.RecoverPanic("myFunc", cleanup)
func RecoverPanic(name string, cleanup func()) {
	if r := recover(); r != nil {
		event.Error(r, "panic", true, "name", name)

		// Create a timestamped panic log file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("crush-panic-%s-%s.log", name, timestamp)

		file, err := os.Create(filename)
		if err == nil {
			defer file.Close()

			// Write panic information and stack trace
			fmt.Fprintf(file, "Panic in %s: %v\n\n", name, r)
			fmt.Fprintf(file, "Time: %s\n\n", time.Now().Format(time.RFC3339))
			fmt.Fprintf(file, "Stack Trace:\n%s\n", debug.Stack())

			// Execute cleanup function if provided
			if cleanup != nil {
				cleanup()
			}
		}
	}
}
