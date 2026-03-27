package config

// Scope 确定配置文件的读写作用域。
type Scope int

const (
	// ScopeGlobal 指向全局数据配置文件（~/.local/share/crush/crush.json）。
	ScopeGlobal Scope = iota
	// ScopeWorkspace 指向工作区配置文件（.crush/crush.json）。
	ScopeWorkspace
)
