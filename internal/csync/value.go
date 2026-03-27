package csync

import (
	"reflect"
	"sync"
)

// Value 是线程安全的泛型值包装器。
//
// 对于切片请使用 [Slice]，对于 map 请使用 [Map]。不支持指针类型。
type Value[T any] struct {
	v  T
	mu sync.RWMutex
}

// NewValue 创建一个新的线程安全值包装器。
// 如果传入指针、切片或 map 类型会 panic。
func NewValue[T any](t T) *Value[T] {
	v := reflect.ValueOf(t)
	switch v.Kind() {
	case reflect.Pointer:
		panic("csync.Value does not support pointer types")
	case reflect.Slice:
		panic("csync.Value does not support slice types; use csync.Slice")
	case reflect.Map:
		panic("csync.Value does not support map types; use csync.Map")
	}
	return &Value[T]{v: t}
}

// Get 返回当前值。
func (v *Value[T]) Get() T {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.v
}

// Set 更新值。
func (v *Value[T]) Set(t T) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.v = t
}
