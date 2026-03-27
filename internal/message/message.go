package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// CreateMessageParams 是创建消息的参数。
type CreateMessageParams struct {
	Role             MessageRole // 消息角色（user/assistant）
	Parts            []ContentPart // 消息内容部分列表
	Model            string        // 使用的模型名称
	Provider         string        // 模型提供商
	IsSummaryMessage bool          // 是否为摘要消息
}

// Service 定义了消息管理的服务接口。
type Service interface {
	pubsub.Subscriber[Message]
	// Create 创建一条新消息。非 Assistant 角色的消息会自动添加 Finish 部分。
	Create(ctx context.Context, sessionID string, params CreateMessageParams) (Message, error)
	// Update 更新消息内容。
	Update(ctx context.Context, message Message) error
	// Get 根据 ID 获取消息。
	Get(ctx context.Context, id string) (Message, error)
	// List 列出指定会话的所有消息。
	List(ctx context.Context, sessionID string) ([]Message, error)
	// ListUserMessages 列出指定会话的用户消息。
	ListUserMessages(ctx context.Context, sessionID string) ([]Message, error)
	// ListAllUserMessages 列出所有会话的用户消息。
	ListAllUserMessages(ctx context.Context) ([]Message, error)
	// Delete 删除指定消息。
	Delete(ctx context.Context, id string) error
	// DeleteSessionMessages 删除指定会话的所有消息。
	DeleteSessionMessages(ctx context.Context, sessionID string) error
}

// service 是 Service 接口的内部实现。
type service struct {
	*pubsub.Broker[Message]
	q db.Querier
}

// NewService 创建一个新的消息管理服务实例。
func NewService(q db.Querier) Service {
	return &service{
		Broker: pubsub.NewBroker[Message](),
		q:      q,
	}
}

// Delete 删除指定消息并发布删除事件。
func (s *service) Delete(ctx context.Context, id string) error {
	message, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	err = s.q.DeleteMessage(ctx, message.ID)
	if err != nil {
		return err
	}
	// 发布前克隆消息，避免并发修改 Parts 切片导致竞态。
	s.Publish(pubsub.DeletedEvent, message.Clone())
	return nil
}

// Create 创建一条新消息并发布创建事件。
// 非 Assistant 角色的消息会自动添加 Finish 部分。
func (s *service) Create(ctx context.Context, sessionID string, params CreateMessageParams) (Message, error) {
	if params.Role != Assistant {
		params.Parts = append(params.Parts, Finish{
			Reason: "stop",
		})
	}
	partsJSON, err := marshalParts(params.Parts)
	if err != nil {
		return Message{}, err
	}
	isSummary := int64(0)
	if params.IsSummaryMessage {
		isSummary = 1
	}
	dbMessage, err := s.q.CreateMessage(ctx, db.CreateMessageParams{
		ID:               uuid.New().String(),
		SessionID:        sessionID,
		Role:             string(params.Role),
		Parts:            string(partsJSON),
		Model:            sql.NullString{String: string(params.Model), Valid: true},
		Provider:         sql.NullString{String: params.Provider, Valid: params.Provider != ""},
		IsSummaryMessage: isSummary,
	})
	if err != nil {
		return Message{}, err
	}
	message, err := s.fromDBItem(dbMessage)
	if err != nil {
		return Message{}, err
	}
	// 发布前克隆消息，避免并发修改 Parts 切片导致竞态。
	s.Publish(pubsub.CreatedEvent, message.Clone())
	return message, nil
}

// DeleteSessionMessages 删除指定会话的所有消息。
func (s *service) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	messages, err := s.List(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if message.SessionID == sessionID {
			err = s.Delete(ctx, message.ID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Update 更新消息内容并发布更新事件。
func (s *service) Update(ctx context.Context, message Message) error {
	parts, err := marshalParts(message.Parts)
	if err != nil {
		return err
	}
	finishedAt := sql.NullInt64{}
	if f := message.FinishPart(); f != nil {
		finishedAt.Int64 = f.Time
		finishedAt.Valid = true
	}
	err = s.q.UpdateMessage(ctx, db.UpdateMessageParams{
		ID:         message.ID,
		Parts:      string(parts),
		FinishedAt: finishedAt,
	})
	if err != nil {
		return err
	}
	message.UpdatedAt = time.Now().Unix()
	// 发布前克隆消息，避免并发修改 Parts 切片导致竞态。
	s.Publish(pubsub.UpdatedEvent, message.Clone())
	return nil
}

// Get 根据 ID 获取消息。
func (s *service) Get(ctx context.Context, id string) (Message, error) {
	dbMessage, err := s.q.GetMessage(ctx, id)
	if err != nil {
		return Message{}, err
	}
	return s.fromDBItem(dbMessage)
}

// List 列出指定会话的所有消息。
func (s *service) List(ctx context.Context, sessionID string) ([]Message, error) {
	dbMessages, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, len(dbMessages))
	for i, dbMessage := range dbMessages {
		messages[i], err = s.fromDBItem(dbMessage)
		if err != nil {
			return nil, err
		}
	}
	return messages, nil
}

// ListUserMessages 列出指定会话的用户消息。
func (s *service) ListUserMessages(ctx context.Context, sessionID string) ([]Message, error) {
	dbMessages, err := s.q.ListUserMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, len(dbMessages))
	for i, dbMessage := range dbMessages {
		messages[i], err = s.fromDBItem(dbMessage)
		if err != nil {
			return nil, err
		}
	}
	return messages, nil
}

// ListAllUserMessages 列出所有会话的用户消息。
func (s *service) ListAllUserMessages(ctx context.Context) ([]Message, error) {
	dbMessages, err := s.q.ListAllUserMessages(ctx)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, len(dbMessages))
	for i, dbMessage := range dbMessages {
		messages[i], err = s.fromDBItem(dbMessage)
		if err != nil {
			return nil, err
		}
	}
	return messages, nil
}

// fromDBItem 将数据库模型转换为业务模型。
func (s *service) fromDBItem(item db.Message) (Message, error) {
	parts, err := unmarshalParts([]byte(item.Parts))
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID:               item.ID,
		SessionID:        item.SessionID,
		Role:             MessageRole(item.Role),
		Parts:            parts,
		Model:            item.Model.String,
		Provider:         item.Provider.String,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		IsSummaryMessage: item.IsSummaryMessage != 0,
	}, nil
}

// partType 定义消息内容部分的类型标识，用于 JSON 序列化/反序列化。
type partType string

const (
	reasoningType  partType = "reasoning"   // 推理内容
	textType       partType = "text"        // 文本内容
	imageURLType   partType = "image_url"   // 图片 URL
	binaryType     partType = "binary"      // 二进制数据
	toolCallType   partType = "tool_call"   // 工具调用
	toolResultType partType = "tool_result" // 工具结果
	finishType     partType = "finish"      // 完成标记
)

// partWrapper 是带类型标识的内容部分包装器，用于多态 JSON 序列化。
type partWrapper struct {
	Type partType    `json:"type"` // 内容类型标识
	Data ContentPart `json:"data"` // 实际内容数据
}

// marshalParts 将消息内容部分列表序列化为 JSON。
// 每个部分会被包装为带类型标识的 partWrapper，以支持多态反序列化。
func marshalParts(parts []ContentPart) ([]byte, error) {
	wrappedParts := make([]partWrapper, len(parts))

	for i, part := range parts {
		var typ partType

		switch part.(type) {
		case ReasoningContent:
			typ = reasoningType
		case TextContent:
			typ = textType
		case ImageURLContent:
			typ = imageURLType
		case BinaryContent:
			typ = binaryType
		case ToolCall:
			typ = toolCallType
		case ToolResult:
			typ = toolResultType
		case Finish:
			typ = finishType
		default:
			return nil, fmt.Errorf("unknown part type: %T", part)
		}

		wrappedParts[i] = partWrapper{
			Type: typ,
			Data: part,
		}
	}
	return json.Marshal(wrappedParts)
}

// unmarshalParts 从 JSON 反序列化消息内容部分列表。
// 根据 type 字段确定具体类型并反序列化为对应的结构体。
func unmarshalParts(data []byte) ([]ContentPart, error) {
	temp := []json.RawMessage{}

	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, err
	}

	parts := make([]ContentPart, 0)

	for _, rawPart := range temp {
		var wrapper struct {
			Type partType        `json:"type"`
			Data json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(rawPart, &wrapper); err != nil {
			return nil, err
		}

		switch wrapper.Type {
		case reasoningType:
			part := ReasoningContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case textType:
			part := TextContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case imageURLType:
			part := ImageURLContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case binaryType:
			part := BinaryContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case toolCallType:
			part := ToolCall{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case toolResultType:
			part := ToolResult{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case finishType:
			part := Finish{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		default:
			return nil, fmt.Errorf("unknown part type: %s", wrapper.Type)
		}
	}

	return parts, nil
}
