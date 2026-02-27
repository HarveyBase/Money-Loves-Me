package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// Feature: binance-trading-system, Property 2: HMAC-SHA256 签名验证
// Validates: Requirements 1.3

func TestProperty2_HMACSHA256SignatureVerification(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成一个随机密钥（16-64 字节）
		keyLen := rapid.IntRange(16, 64).Draw(t, "keyLen")
		key := make([]byte, keyLen)
		for i := range key {
			key[i] = rapid.Byte().Draw(t, fmt.Sprintf("key[%d]", i))
		}

		// 生成一个任意载荷
		payload := rapid.String().Draw(t, "payload")

		signer := NewHMACSigner(key)

		// --- 子属性 1：签名后验证的往返测试 ---
		sig := signer.Sign(payload)
		if !signer.Verify(payload, sig) {
			t.Fatalf("Verify failed for payload %q with its own signature", payload)
		}

		// --- 子属性 2：确定性 - 相同输入 → 相同签名 ---
		sig2 := signer.Sign(payload)
		if sig != sig2 {
			t.Fatalf("Sign is not deterministic: got %q and %q for the same payload", sig, sig2)
		}

		// --- 子属性 3：标准 HMAC-SHA256 验证 ---
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(payload))
		expected := hex.EncodeToString(mac.Sum(nil))
		if sig != expected {
			t.Fatalf("signature %q does not match standard HMAC-SHA256 result %q", sig, expected)
		}

		// --- 子属性 4：不同载荷 → 不同签名（高概率） ---
		payload2 := rapid.String().Draw(t, "payload2")
		if payload != payload2 {
			sig3 := signer.Sign(payload2)
			if sig == sig3 {
				t.Fatalf("different payloads %q and %q produced the same signature", payload, payload2)
			}
		}

		// --- 子属性 5：错误密钥 → Verify 返回 false ---
		wrongKeyLen := rapid.IntRange(16, 64).Draw(t, "wrongKeyLen")
		wrongKey := make([]byte, wrongKeyLen)
		for i := range wrongKey {
			wrongKey[i] = rapid.Byte().Draw(t, fmt.Sprintf("wrongKey[%d]", i))
		}
		// 确保错误密钥确实不同
		wrongSigner := NewHMACSigner(wrongKey)
		wrongSig := wrongSigner.Sign(payload)
		if wrongSig != sig {
			// 不同密钥产生不同签名，因此交叉验证应失败
			if wrongSigner.Verify(payload, sig) {
				t.Fatalf("Verify should fail with a different key")
			}
		}
	})
}
