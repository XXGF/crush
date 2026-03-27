// Package diff 提供文件内容的统一差异（unified diff）生成功能。
package diff

import (
	"strings"

	"github.com/aymanbagabas/go-udiff"
)

// GenerateDiff 根据修改前后的文件内容生成统一差异格式的输出。
//
// 返回值：
//   - unified: 统一差异格式的字符串（带 a/ 和 b/ 前缀）
//   - additions: 新增行数（以 "+" 开头的行，排除 "+++" 头部）
//   - removals: 删除行数（以 "-" 开头的行，排除 "---" 头部）
func GenerateDiff(beforeContent, afterContent, fileName string) (string, int, int) {
	fileName = strings.TrimPrefix(fileName, "/")

	var (
		unified   = udiff.Unified("a/"+fileName, "b/"+fileName, beforeContent, afterContent)
		additions = 0
		removals  = 0
	)

	lines := strings.SplitSeq(unified, "\n")
	for line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removals++
		}
	}

	return unified, additions, removals
}
