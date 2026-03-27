// Package oauth 提供 OAuth2 认证令牌的通用数据结构和工具方法。
package oauth

import (
	"time"
)

// Token 表示一个 OAuth2 认证令牌。
type Token struct {
	AccessToken  string `json:"access_token"`  // 访问令牌
	RefreshToken string `json:"refresh_token"` // 刷新令牌
	ExpiresIn    int    `json:"expires_in"`    // 有效期（秒）
	ExpiresAt    int64  `json:"expires_at"`    // 过期时间戳（Unix 秒）
}

// SetExpiresAt 根据当前时间和 ExpiresIn 计算并设置过期时间戳。
func (t *Token) SetExpiresAt() {
	t.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second).Unix()
}

// IsExpired 判断令牌是否已过期或即将过期。
// 使用 10% 的安全边距：当剩余有效期不足总时长的 10% 时即视为过期。
func (t *Token) IsExpired() bool {
	return time.Now().Unix() >= (t.ExpiresAt - int64(t.ExpiresIn)/10)
}

// SetExpiresIn 根据 ExpiresAt 反算并设置剩余有效期（秒）。
func (t *Token) SetExpiresIn() {
	t.ExpiresIn = int(time.Until(time.Unix(t.ExpiresAt, 0)).Seconds())
}
