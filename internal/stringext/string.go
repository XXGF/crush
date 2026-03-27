// Package stringext 提供字符串处理的扩展工具函数。
package stringext

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Capitalize 将字符串的每个单词首字母转换为大写（英文标题格式）。
func Capitalize(text string) string {
	return cases.Title(language.English, cases.Compact).String(text)
}

// NormalizeSpace 对字符串中的空白字符进行标准化处理。
//
// 处理规则：
//  1. 将 Windows 风格的换行符 (\r\n) 替换为 Unix 风格 (\n)
//  2. 将制表符 (\t) 替换为四个空格
//  3. 去除首尾空白字符
func NormalizeSpace(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")
	content = strings.TrimSpace(content)
	return content
}
