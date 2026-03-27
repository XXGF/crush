// Package csync 提供线程安全的并发数据结构，用于多线程环境下的安全访问。
//
// 提供以下数据结构：
//   - [Map]: 线程安全的泛型 map，支持懒加载
//   - [Slice]: 线程安全的泛型切片，支持懒加载
//   - [Value]: 线程安全的泛型值包装器
//   - [VersionedMap]: 带版本跟踪的线程安全 map
package csync
