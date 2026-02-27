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
		// Generate a random secret key (16–64 bytes)
		keyLen := rapid.IntRange(16, 64).Draw(t, "keyLen")
		key := make([]byte, keyLen)
		for i := range key {
			key[i] = rapid.Byte().Draw(t, fmt.Sprintf("key[%d]", i))
		}

		// Generate an arbitrary payload
		payload := rapid.String().Draw(t, "payload")

		signer := NewHMACSigner(key)

		// --- Sub-property 1: Sign then Verify round-trip ---
		sig := signer.Sign(payload)
		if !signer.Verify(payload, sig) {
			t.Fatalf("Verify failed for payload %q with its own signature", payload)
		}

		// --- Sub-property 2: Deterministic – same input → same signature ---
		sig2 := signer.Sign(payload)
		if sig != sig2 {
			t.Fatalf("Sign is not deterministic: got %q and %q for the same payload", sig, sig2)
		}

		// --- Sub-property 3: Standard HMAC-SHA256 verification ---
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(payload))
		expected := hex.EncodeToString(mac.Sum(nil))
		if sig != expected {
			t.Fatalf("signature %q does not match standard HMAC-SHA256 result %q", sig, expected)
		}

		// --- Sub-property 4: Different payload → different signature (high probability) ---
		payload2 := rapid.String().Draw(t, "payload2")
		if payload != payload2 {
			sig3 := signer.Sign(payload2)
			if sig == sig3 {
				t.Fatalf("different payloads %q and %q produced the same signature", payload, payload2)
			}
		}

		// --- Sub-property 5: Wrong key → Verify returns false ---
		wrongKeyLen := rapid.IntRange(16, 64).Draw(t, "wrongKeyLen")
		wrongKey := make([]byte, wrongKeyLen)
		for i := range wrongKey {
			wrongKey[i] = rapid.Byte().Draw(t, fmt.Sprintf("wrongKey[%d]", i))
		}
		// Ensure the wrong key is actually different
		wrongSigner := NewHMACSigner(wrongKey)
		wrongSig := wrongSigner.Sign(payload)
		if wrongSig != sig {
			// Keys produce different signatures, so cross-verify should fail
			if wrongSigner.Verify(payload, sig) {
				t.Fatalf("Verify should fail with a different key")
			}
		}
	})
}
