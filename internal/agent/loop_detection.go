package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"io"

	"charm.land/fantasy"
)

const (
	// loopDetectionWindowSize 是循环检测的滑动窗口大小（检查最近 N 步）。
	loopDetectionWindowSize = 10
	// loopDetectionMaxRepeats 是允许的最大重复次数，超过则判定为循环。
	loopDetectionMaxRepeats = 5
)

// hasRepeatedToolCalls 检测 Agent 是否陷入工具调用循环。
// 检查最近 windowSize 步中是否有相同的工具调用签名出现超过 maxRepeats 次。
func hasRepeatedToolCalls(steps []fantasy.StepResult, windowSize, maxRepeats int) bool {
	if len(steps) < windowSize {
		return false
	}

	window := steps[len(steps)-windowSize:]
	counts := make(map[string]int)

	for _, step := range window {
		sig := getToolInteractionSignature(step.Content)
		if sig == "" {
			continue
		}
		counts[sig]++
		if counts[sig] > maxRepeats {
			return true
		}
	}

	return false
}

// getToolInteractionSignature 计算单步中工具交互的哈希签名。
// 将工具调用与其结果配对（通过 ToolCallID），生成 SHA-256 哈希。
// 如果该步不包含工具调用，返回空字符串。
func getToolInteractionSignature(content fantasy.ResponseContent) string {
	toolCalls := content.ToolCalls()
	if len(toolCalls) == 0 {
		return ""
	}

	// 按 ToolCallID 索引工具结果，便于快速查找。
	resultsByID := make(map[string]fantasy.ToolResultContent)
	for _, tr := range content.ToolResults() {
		resultsByID[tr.ToolCallID] = tr
	}

	h := sha256.New()
	for _, tc := range toolCalls {
		output := ""
		if tr, ok := resultsByID[tc.ToolCallID]; ok {
			output = toolResultOutputString(tr.Result)
		}
		io.WriteString(h, tc.ToolName)
		io.WriteString(h, "\x00")
		io.WriteString(h, tc.Input)
		io.WriteString(h, "\x00")
		io.WriteString(h, output)
		io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// toolResultOutputString 将工具结果转换为稳定的字符串表示，用于签名比较。
func toolResultOutputString(result fantasy.ToolResultOutputContent) string {
	if result == nil {
		return ""
	}
	if text, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result); ok {
		return text.Text
	}
	if errResult, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result); ok {
		if errResult.Error != nil {
			return errResult.Error.Error()
		}
		return ""
	}
	if media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](result); ok {
		return media.Data
	}
	return ""
}
