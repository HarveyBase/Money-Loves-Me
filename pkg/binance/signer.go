package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HMACSigner 提供用于 Binance API 请求的 HMAC-SHA256 签名功能。
type HMACSigner struct {
	secretKey []byte
}

// NewHMACSigner 使用给定的密钥创建一个新的 HMACSigner。
func NewHMACSigner(secretKey []byte) *HMACSigner {
	return &HMACSigner{secretKey: secretKey}
}

// Sign 返回给定载荷的十六进制编码 HMAC-SHA256 签名。
func (s *HMACSigner) Sign(payload string) string {
	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify 检查给定的十六进制编码签名对于该载荷是否有效。
func (s *HMACSigner) Verify(payload, signature string) bool {
	expected := s.Sign(payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}
