// Package fsext 提供文件系统的扩展工具函数。
package fsext

import (
	"os"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

// Expand 对字符串进行 Shell 展开处理。
// 支持展开 ~ 符号、环境变量和其他 Shell 特殊符号。
// 内部使用 mvdan.cc/sh 库进行解析和展开。
func Expand(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	p := syntax.NewParser()
	word, err := p.Document(strings.NewReader(s))
	if err != nil {
		return "", err
	}
	cfg := &expand.Config{
		Env:      expand.FuncEnviron(os.Getenv),
		ReadDir2: os.ReadDir,
		GlobStar: true,
	}
	return expand.Literal(cfg, word)
}
