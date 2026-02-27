package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HMACSigner provides HMAC-SHA256 signing for Binance API requests.
type HMACSigner struct {
	secretKey []byte
}

// NewHMACSigner creates a new HMACSigner with the given secret key.
func NewHMACSigner(secretKey []byte) *HMACSigner {
	return &HMACSigner{secretKey: secretKey}
}

// Sign returns the hex-encoded HMAC-SHA256 signature of the given payload.
func (s *HMACSigner) Sign(payload string) string {
	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks whether the given hex-encoded signature is valid for the payload.
func (s *HMACSigner) Verify(payload, signature string) bool {
	expected := s.Sign(payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}
