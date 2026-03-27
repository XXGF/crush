// Package version 提供应用程序的版本信息。
//
// 版本号通过以下优先级确定：
//  1. 构建时通过 -ldflags 注入的版本号
//  2. go install 时嵌入的模块版本信息
//  3. 默认值 "devel"
package version

import "runtime/debug"

// Version 是应用程序的当前版本号。
// 通常在构建时通过 -ldflags 设置，默认值为 "devel"。
var Version = "devel"

// init 尝试从 Go 模块的构建信息中获取版本号。
// 这主要用于通过 `go install` 安装但未使用 -ldflags 的场景。
func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	mainVersion := info.Main.Version
	if mainVersion != "" && mainVersion != "(devel)" {
		Version = mainVersion
	}
}
