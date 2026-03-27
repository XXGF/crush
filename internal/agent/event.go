package agent

import (
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/event"
)

// eventPromptSent 记录提示词已发送的事件指标。
// 该方法在每次向 LLM 发送请求时调用，用于使用量统计。
func (a *sessionAgent) eventPromptSent(sessionID string) {
	event.PromptSent(
		a.eventCommon(sessionID, a.largeModel.Get())...,
	)
}

// eventPromptResponded 记录提示词已响应的事件指标。
// duration 参数表示从发送请求到收到完整响应的耗时。
func (a *sessionAgent) eventPromptResponded(sessionID string, duration time.Duration) {
	event.PromptResponded(
		append(
			a.eventCommon(sessionID, a.largeModel.Get()),
			"prompt duration pretty", duration.String(),
			"prompt duration in seconds", int64(duration.Seconds()),
		)...,
	)
}

// eventTokensUsed 记录 Token 使用量的事件指标。
// 包括输入 Token、输出 Token、缓存读取/创建 Token 以及对应的费用。
func (a *sessionAgent) eventTokensUsed(sessionID string, model Model, usage fantasy.Usage, cost float64) {
	event.TokensUsed(
		append(
			a.eventCommon(sessionID, model),
			"input tokens", usage.InputTokens,
			"output tokens", usage.OutputTokens,
			"cache read tokens", usage.CacheReadTokens,
			"cache creation tokens", usage.CacheCreationTokens,
			"total tokens", usage.InputTokens+usage.OutputTokens+usage.CacheReadTokens+usage.CacheCreationTokens,
			"cost", cost,
		)...,
	)
}

// eventCommon 构建事件指标的通用字段列表。
// 返回包含会话 ID、提供商、模型名称、推理强度、思考模式和 YOLO 模式等信息的键值对切片。
func (a *sessionAgent) eventCommon(sessionID string, model Model) []any {
	m := model.ModelCfg

	return []any{
		"session id", sessionID,
		"provider", m.Provider,
		"model", m.Model,
		"reasoning effort", m.ReasoningEffort,
		"thinking mode", m.Think,
		"yolo mode", a.isYolo,
	}
}
