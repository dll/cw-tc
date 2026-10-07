// Package resource 提供资源签名 URL 的生成与校验。
//
// 签名 URL 短时有效，防止资源被长期外链；换厂商 / 换存储只改配置，不改业务。
package resource

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

// Sign 为存储键生成 HMAC-SHA256 签名（storageKey|unix 秒）。
func Sign(secret []byte, storageKey string, expiresAt time.Time) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(storageKey + "|" + strconv.FormatInt(expiresAt.Unix(), 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify 校验签名与有效期；过期或签名不符均拒绝。
func Verify(secret []byte, storageKey string, expiresAt time.Time, signature string) bool {
	if signature == "" || len(secret) == 0 || time.Now().After(expiresAt) {
		return false
	}
	expected := Sign(secret, storageKey, expiresAt)
	return hmac.Equal([]byte(expected), []byte(signature))
}
