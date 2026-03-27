package agent

import (
	"context"
	_ "embed"
	"errors"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
)

// agentToolDescription 是 Agent 工具的描述文档，通过 go:embed 嵌入。
// 该描述会展示给 LLM，帮助其理解何时以及如何调用子代理工具。
//
//go:embed templates/agent_tool.md
var agentToolDescription []byte

// AgentParams 定义了 Agent 工具的输入参数结构。
// LLM 通过该结构向子代理传递任务描述。
type AgentParams struct {
	Prompt string `json:"prompt" description:"The task for the agent to perform"` // 子代理需要执行的任务描述
}

const (
	// AgentToolName 是 Agent 工具在工具注册表中的唯一标识名称。
	AgentToolName = "agent"
)

// agentTool 创建并返回一个可并行执行的子代理工具。
//
// 该工具允许主编码助手将复杂任务委派给独立的子代理执行。
// 子代理拥有独立的会话和上下文，但共享相同的工具集。
// 返回的 fantasy.AgentTool 支持并行调用，多个子任务可同时执行。
//
// 调用流程：
//  1. 从配置中获取任务代理（Task Agent）的配置
//  2. 构建任务代理的系统提示词
//  3. 创建子代理实例
//  4. 包装为 ParallelAgentTool 返回
func (c *coordinator) agentTool(ctx context.Context) (fantasy.AgentTool, error) {
	agentCfg, ok := c.cfg.Config().Agents[config.AgentTask]
	if !ok {
		return nil, errors.New("task agent not configured")
	}
	prompt, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}

	agent, err := c.buildAgent(ctx, prompt, agentCfg, true)
	if err != nil {
		return nil, err
	}
	return fantasy.NewParallelAgentTool(
		AgentToolName,
		string(agentToolDescription),
		func(ctx context.Context, params AgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			return c.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         params.Prompt,
				SessionTitle:   "New Agent Session",
			})
		}), nil
}
