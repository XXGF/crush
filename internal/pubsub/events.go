package pubsub

import "context"

const (
	// CreatedEvent 表示资源创建事件。
	CreatedEvent EventType = "created"
	// UpdatedEvent 表示资源更新事件。
	UpdatedEvent EventType = "updated"
	// DeletedEvent 表示资源删除事件。
	DeletedEvent EventType = "deleted"
)

// Subscriber 定义了事件订阅者的接口。
type Subscriber[T any] interface {
	// Subscribe 创建订阅并返回事件接收通道，当 context 取消时自动清理。
	Subscribe(context.Context) <-chan Event[T]
}

type (
	// EventType 标识事件的类型（创建、更新、删除）。
	EventType string

	// Event 表示资源生命周期中的一个事件。
	Event[T any] struct {
		Type    EventType // 事件类型
		Payload T         // 事件负载数据
	}

	// Publisher 定义了事件发布者的接口。
	Publisher[T any] interface {
		// Publish 发布指定类型的事件。
		Publish(EventType, T)
	}
)
