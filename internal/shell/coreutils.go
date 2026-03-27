package shell

import (
	"os"
	"runtime"
	"strconv"
)

// useGoCoreUtils 控制是否使用 Go 实现的核心工具替代系统命令。
// 默认在 Windows 上启用，可通过 CRUSH_CORE_UTILS 环境变量覆盖。
var useGoCoreUtils bool

func init() {
	// If CRUSH_CORE_UTILS is set to either true or false, respect that.
	// By default, enable on Windows only.
	if v, err := strconv.ParseBool(os.Getenv("CRUSH_CORE_UTILS")); err == nil {
		useGoCoreUtils = v
	} else {
		useGoCoreUtils = runtime.GOOS == "windows"
	}
}
