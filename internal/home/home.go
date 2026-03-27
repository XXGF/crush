// Package home 提供用户主目录的工具函数。
//
// 支持将绝对路径与主目录之间进行缩写转换（~ 符号）。
package home

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// homedir 缓存用户主目录路径，在包初始化时加载。
var homedir, homedirErr = os.UserHomeDir()

func init() {
	if homedirErr != nil {
		slog.Error("获取用户主目录失败", "error", homedirErr)
	}
}

// Dir 返回用户主目录的绝对路径。
// 如果获取失败则返回空字符串。
func Dir() string {
	return homedir
}

// Short 将路径中的主目录前缀替换为 "~" 符号。
// 例如：/Users/foo/bar → ~/bar。
// 如果路径不以主目录开头，则原样返回。
func Short(p string) string {
	if homedir == "" || !strings.HasPrefix(p, homedir) {
		return p
	}
	return filepath.Join("~", strings.TrimPrefix(p, homedir))
}

// Long 将路径中的 "~" 符号展开为实际的主目录路径。
// 例如：~/bar → /Users/foo/bar。
// 如果路径不以 "~" 开头，则原样返回。
func Long(p string) string {
	if homedir == "" || !strings.HasPrefix(p, "~") {
		return p
	}
	return strings.Replace(p, "~", homedir, 1)
}
