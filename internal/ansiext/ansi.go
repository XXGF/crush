// Package ansiext 提供 ANSI 控制字符的扩展处理功能。
//
// 主要用于将不可见的控制字符转换为可视化的 Unicode 控制图片符号，
// 确保终端 UI 中能正确显示这些特殊字符。
package ansiext

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Escape 将字符串中的 ASCII 控制字符替换为对应的 Unicode 控制图片符号（U+2400 区段）。
//
// 转换规则：
//   - 0x00-0x1F 范围的控制字符映射到 U+2400-U+241F（如 NUL→␀, ESC→␛）
//   - DEL (0x7F) 映射到 U+2421（␡）
//   - 其他字符保持不变
//
// 该函数预分配了与输入等长的缓冲区以优化性能。
func Escape(content string) string {
	var sb strings.Builder
	sb.Grow(len(content))
	for _, r := range content {
		switch {
		case r >= 0 && r <= 0x1f: // 控制字符 0x00-0x1F
			sb.WriteRune('\u2400' + r)
		case r == ansi.DEL:
			sb.WriteRune('\u2421')
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
