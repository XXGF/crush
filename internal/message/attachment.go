// Package message 提供消息附件的数据结构和工具函数。
package message

import (
	"slices"
	"strings"
)

// Attachment 表示消息的附件，支持文本和图片类型。
type Attachment struct {
	FilePath string // 文件路径
	FileName string // 文件名
	MimeType string // MIME 类型
	Content  []byte // 文件内容
}

// IsText 判断附件是否为文本类型。
func (a Attachment) IsText() bool { return strings.HasPrefix(a.MimeType, "text/") }

// IsImage 判断附件是否为图片类型。
func (a Attachment) IsImage() bool { return strings.HasPrefix(a.MimeType, "image/") }

// ContainsTextAttachment 检查附件列表中是否包含文本类型的附件。
func ContainsTextAttachment(attachments []Attachment) bool {
	return slices.ContainsFunc(attachments, func(a Attachment) bool {
		return a.IsText()
	})
}
