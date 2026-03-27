// Crush 是一个终端 AI 编码助手的主入口程序。
//
// 它支持多种大语言模型（LLM），提供基于会话的 AI 编程辅助功能，
// 包括代码编辑、工具调用、MCP 集成等能力。
// 当设置了 CRUSH_PROFILE 环境变量时，会启动 pprof 性能分析服务。
package main

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/charmbracelet/crush/internal/cmd"
	_ "github.com/joho/godotenv/autoload"
)

// main 是 Crush 应用程序的入口函数。
//
// 如果设置了 CRUSH_PROFILE 环境变量，会在 localhost:6060 启动 pprof
// 性能分析 HTTP 服务，用于运行时性能诊断。
// 随后调用 cmd.Execute() 启动 CLI 命令解析和执行。
func main() {
	if os.Getenv("CRUSH_PROFILE") != "" {
		go func() {
			slog.Info("Serving pprof at localhost:6060")
			if httpErr := http.ListenAndServe("localhost:6060", nil); httpErr != nil {
				slog.Error("Failed to pprof listen", "error", httpErr)
			}
		}()
	}

	cmd.Execute()
}