package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// generateTestKey 创建一个用于测试的随机 32 字节密钥。
func generateTestKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := generateTestKey(t)
	plaintext := "my-secret-api-key-12345"

	encrypted, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncrypt_OutputIsBase64(t *testing.T) {
	key := generateTestKey(t)
	encrypted, err := Encrypt(key, "test-data")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		t.Errorf("encrypted output is not valid base64: %v", err)
	}
}

func TestEncrypt_CiphertextDoesNotContainPlaintext(t *testing.T) {
	key := generateTestKey(t)
	plaintext := "super-secret-value"

	encrypted, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if strings.Contains(encrypted, plaintext) {
		t.Error("ciphertext should not contain the plaintext")
	}
}

func TestEncrypt_DifferentCiphertextEachTime(t *testing.T) {
	key := generateTestKey(t)
	plaintext := "same-input"

	enc1, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	enc2, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if enc1 == enc2 {
		t.Error("encrypting the same plaintext twice should produce different ciphertexts (random nonce)")
	}
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	shortKey := make([]byte, 16)
	_, err := Encrypt(shortKey, "test")
	if err == nil {
		t.Fatal("expected error for 16-byte key")
	}
}

func TestDecrypt_InvalidKeyLength(t *testing.T) {
	shortKey := make([]byte, 16)
	_, err := Decrypt(shortKey, "dGVzdA==")
	if err == nil {
		t.Fatal("expected error for 16-byte key")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	key := generateTestKey(t)
	_, err := Decrypt(key, "not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := generateTestKey(t)
	key2 := generateTestKey(t)

	encrypted, err := Encrypt(key1, "secret")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(key2, encrypted)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := generateTestKey(t)

	encrypted, err := Encrypt(key, "secret")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// 篡改密文
	data, _ := base64.StdEncoding.DecodeString(encrypted)
	if len(data) > 0 {
		data[len(data)-1] ^= 0xFF
	}
	tampered := base64.StdEncoding.EncodeToString(data)

	_, err = Decrypt(key, tampered)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestEncryptDecrypt_EmptyString(t *testing.T) {
	key := generateTestKey(t)

	encrypted, err := Encrypt(key, "")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

// Feature: binance-trading-system, Property 1: AES-256 加密解密往返
// Validates: Requirements 1.4, 12.2
func TestProperty1_AESEncryptionRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成一个随机的 32 字节 AES-256 密钥
		key := make([]byte, 32)
		for i := range key {
			key[i] = rapid.Byte().Draw(t, fmt.Sprintf("key[%d]", i))
		}

		// 生成一个任意的明文字符串
		plaintext := rapid.String().Draw(t, "plaintext")

		// 加密明文
		encrypted, err := Encrypt(key, plaintext)
		if err != nil {
			t.Fatalf("Encrypt failed: %v", err)
		}

		// base64 编码的密文不应包含原始明文。
		// 短明文（≤2 个字符）可能偶然出现在 base64 输出中，
		// 因此仅对足够长的字符串进行此断言。
		if len(plaintext) > 2 {
			if strings.Contains(encrypted, plaintext) {
				t.Fatalf("ciphertext %q contains plaintext %q", encrypted, plaintext)
			}
		}

		// 解密并验证往返一致性
		decrypted, err := Decrypt(key, encrypted)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}

		if decrypted != plaintext {
			t.Fatalf("round-trip failed: expected %q, got %q", plaintext, decrypted)
		}
	})
}
