// Package env 提供环境变量的抽象接口和实现。
//
// 支持两种实现：
//   - osEnv: 基于操作系统环境变量的真实实现
//   - mapEnv: 基于内存 map 的模拟实现，主要用于测试
package env

import (
	"os"
)

// Env 定义了环境变量访问的抽象接口。
type Env interface {
	// Get 返回指定键名的环境变量值，不存在时返回空字符串。
	Get(key string) string
	// Env 返回所有环境变量的 "KEY=VALUE" 格式切片。
	Env() []string
}

// osEnv 是基于操作系统环境变量的 Env 实现。
type osEnv struct{}

// Get 返回操作系统中指定键名的环境变量值。
func (o *osEnv) Get(key string) string {
	return os.Getenv(key)
}

// Env 返回操作系统的所有环境变量。
func (o *osEnv) Env() []string {
	return os.Environ()
}

// New 创建一个基于操作系统环境变量的 Env 实例。
func New() Env {
	return &osEnv{}
}

// mapEnv 是基于内存 map 的 Env 实现，主要用于测试场景。
type mapEnv struct {
	m map[string]string
}

// Get 从内存 map 中获取指定键名的环境变量值。
func (m *mapEnv) Get(key string) string {
	if value, ok := m.m[key]; ok {
		return value
	}
	return ""
}

// Env 将内存 map 转换为 "KEY=VALUE" 格式的环境变量切片。
func (m *mapEnv) Env() []string {
	env := make([]string, 0, len(m.m))
	for k, v := range m.m {
		env = append(env, k+"="+v)
	}
	return env
}

// NewFromMap 从给定的 map 创建一个 Env 实例。
// 如果传入 nil，则自动初始化为空 map。
func NewFromMap(m map[string]string) Env {
	if m == nil {
		m = make(map[string]string)
	}
	return &mapEnv{m: m}
}
