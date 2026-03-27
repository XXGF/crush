package event

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/denisbrodbeck/machineid"
)

// distinctId 是当前设备的唯一标识符，用于遥测事件的身份关联。
var distinctId string

const (
	// hashKey 是生成设备 ID 时使用的哈希密钥。
	hashKey = "charm"
	// fallbackId 是无法获取设备 ID 时的回退值。
	fallbackId = "unknown"
)

// getDistinctId 获取设备的唯一标识符。
// 优先级：机器 ID > MAC 地址哈希 > 回退值 "unknown"。
func getDistinctId() string {
	if id, err := machineid.ProtectedID(hashKey); err == nil {
		return id
	}
	if macAddr, err := getMacAddr(); err == nil {
		return hashString(macAddr)
	}
	return fallbackId
}

// getMacAddr 获取第一个活跃的非回环网络接口的 MAC 地址。
func getMacAddr() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 && len(iface.HardwareAddr) > 0 {
			if addrs, err := iface.Addrs(); err == nil && len(addrs) > 0 {
				return iface.HardwareAddr.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no active interface with mac address found")
}

// hashString 使用 HMAC-SHA256 对字符串进行哈希处理。
func hashString(str string) string {
	hash := hmac.New(sha256.New, []byte(str))
	hash.Write([]byte(hashKey))
	return hex.EncodeToString(hash.Sum(nil))
}
