// Package filepathext 提供跨平台的文件路径处理扩展功能。
package filepathext

import (
	"path/filepath"
	"runtime"
	"strings"
)

// SmartJoin 智能拼接两个路径。
// 如果第二个路径是绝对路径，则直接返回第二个路径；
// 否则将两个路径拼接在一起。
func SmartJoin(one, two string) string {
	if SmartIsAbs(two) {
		return two
	}
	return filepath.Join(one, two)
}

// SmartIsAbs 智能判断路径是否为绝对路径。
// 在 Windows 上同时支持 OS 原生格式（C:\path）和 Unix 风格（/path）的绝对路径。
func SmartIsAbs(path string) bool {
	switch runtime.GOOS {
	case "windows":
		return filepath.IsAbs(path) || strings.HasPrefix(filepath.ToSlash(path), "/")
	default:
		return filepath.IsAbs(path)
	}
}
