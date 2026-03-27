package agent

import (
	"context"
	_ "embed"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
)

// coderPromptTmpl 是编码助手（Coder Agent）的系统提示词模板。
// 通过 go:embed 从 templates/coder.md.tpl 文件嵌入。
//
//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

// taskPromptTmpl 是任务代理（Task Agent）的系统提示词模板。
// 通过 go:embed 从 templates/task.md.tpl 文件嵌入。
//
//go:embed templates/task.md.tpl
var taskPromptTmpl []byte

// initializePromptTmpl 是项目初始化分析的系统提示词模板。
// 通过 go:embed 从 templates/initialize.md.tpl 文件嵌入。
//
//go:embed templates/initialize.md.tpl
var initializePromptTmpl []byte

// coderPrompt 创建编码助手的系统提示词实例。
// 支持通过 prompt.Option 自定义提示词行为（如设置工作目录）。
func coderPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("coder", string(coderPromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

// taskPrompt 创建任务代理的系统提示词实例。
// 任务代理用于处理子任务，由主编码助手通过 agent 工具调用。
func taskPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("task", string(taskPromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

// InitializePrompt 构建项目初始化分析的完整系统提示词字符串。
// 该提示词用于分析项目代码库并生成 AGENTS.md 等上下文文件。
// cfg 参数提供项目配置信息，用于模板渲染。
func InitializePrompt(cfg *config.ConfigStore) (string, error) {
	systemPrompt, err := prompt.NewPrompt("initialize", string(initializePromptTmpl))
	if err != nil {
		return "", err
	}
	return systemPrompt.Build(context.Background(), "", "", cfg)
}
