// Package pubsub 提供轻量级的泛型发布-订阅消息代理。
//
// Broker 支持多个订阅者并发接收事件，采用非阻塞发布策略，
// 慢速的订阅者会被跳过以避免阻塞发布者。
package pubsub

import (
	"context"
	"sync"
)

// bufferSize 是订阅者通道的默认缓冲区大小。
const bufferSize = 64

// Broker 是泛型的发布-订阅消息代理。
// 支持多个订阅者并发接收事件，线程安全。
type Broker[T any] struct {
	subs      map[chan Event[T]]struct{} // 活跃订阅者集合
	mu        sync.RWMutex               // 保护 subs 的读写锁
	done      chan struct{}               // 关闭信号
	subCount  int                        // 当前订阅者数量
	maxEvents int                        // 最大事件数限制
}

// NewBroker 创建一个使用默认配置的消息代理。
// 默认通道缓冲区大小为 64，最大事件数为 1000。
func NewBroker[T any]() *Broker[T] {
	return NewBrokerWithOptions[T](bufferSize, 1000)
}

// NewBrokerWithOptions 创建一个使用自定义配置的消息代理。
func NewBrokerWithOptions[T any](channelBufferSize, maxEvents int) *Broker[T] {
	return &Broker[T]{
		subs:      make(map[chan Event[T]]struct{}),
		done:      make(chan struct{}),
		maxEvents: maxEvents,
	}
}

// Shutdown 关闭消息代理并清理所有订阅者。
// 重复调用是安全的。
func (b *Broker[T]) Shutdown() {
	select {
	case <-b.done: // Already closed
		return
	default:
		close(b.done)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.subs {
		delete(b.subs, ch)
		close(ch)
	}

	b.subCount = 0
}

// Subscribe 创建一个新的订阅并返回事件接收通道。
// 当传入的 context 被取消时，订阅会自动清理。
// 如果 Broker 已关闭，返回一个已关闭的通道。
func (b *Broker[T]) Subscribe(ctx context.Context) <-chan Event[T] {
	b.mu.Lock()
	defer b.mu.Unlock()

	select {
	case <-b.done:
		ch := make(chan Event[T])
		close(ch)
		return ch
	default:
	}

	sub := make(chan Event[T], bufferSize)
	b.subs[sub] = struct{}{}
	b.subCount++

	go func() {
		<-ctx.Done()

		b.mu.Lock()
		defer b.mu.Unlock()

		select {
		case <-b.done:
			return
		default:
		}

		delete(b.subs, sub)
		close(sub)
		b.subCount--
	}()

	return sub
}

// GetSubscriberCount 返回当前活跃的订阅者数量。
func (b *Broker[T]) GetSubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.subCount
}

// Publish 向所有订阅者发布事件。
// 采用非阻塞发布策略：如果订阅者的通道已满，则跳过该订阅者，
// 避免慢速订阅者阻塞发布者。
func (b *Broker[T]) Publish(t EventType, payload T) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	select {
	case <-b.done:
		return
	default:
	}

	event := Event[T]{Type: t, Payload: payload}

	for sub := range b.subs {
		select {
		case sub <- event:
		default:
			// 通道已满，订阅者处理过慢——跳过此事件
			// 这可以防止发布者被阻塞
		}
	}
}
