package csync

import (
	"iter"
	"sync/atomic"
)

// NewVersionedMap 创建一个新的带版本跟踪的线程安全 map。
func NewVersionedMap[K comparable, V any]() *VersionedMap[K, V] {
	return &VersionedMap[K, V]{
		m: NewMap[K, V](),
	}
}

// VersionedMap 是带版本跟踪的线程安全 map。
// 每次写操作（Set/Del）会自动递增版本号，可用于变更检测。
type VersionedMap[K comparable, V any] struct {
	m *Map[K, V]
	v atomic.Uint64
}

// Get 获取指定键的值。
func (m *VersionedMap[K, V]) Get(key K) (V, bool) {
	return m.m.Get(key)
}

// Set 设置指定键的值并递增版本号。
func (m *VersionedMap[K, V]) Set(key K, value V) {
	m.m.Set(key, value)
	m.v.Add(1)
}

// Del 删除指定的键并递增版本号。
func (m *VersionedMap[K, V]) Del(key K) {
	m.m.Del(key)
	m.v.Add(1)
}

// Seq2 返回产生键值对的迭代器。
func (m *VersionedMap[K, V]) Seq2() iter.Seq2[K, V] {
	return m.m.Seq2()
}

// Copy 返回内部 map 的副本。
func (m *VersionedMap[K, V]) Copy() map[K]V {
	return m.m.Copy()
}

// Len 返回 map 中的元素数量。
func (m *VersionedMap[K, V]) Len() int {
	return m.m.Len()
}

// Version 返回当前的版本号。
func (m *VersionedMap[K, V]) Version() uint64 {
	return m.v.Load()
}
