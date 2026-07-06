package config

import (
	"strings"
	"testing"
)

// TestEncryptDecryptAPIKey 验证 API Key 加密后再解密能还原原文。
func TestEncryptDecryptAPIKey(t *testing.T) {
	original := "sk-test-1234567890abcdef"

	encrypted, err := EncryptAPIKey(original)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if encrypted == "" {
		t.Fatal("encrypted string should not be empty")
	}
	if encrypted == original {
		t.Fatal("encrypted string should differ from plaintext")
	}

	decrypted, err := DecryptAPIKey(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decrypted != original {
		t.Fatalf("decrypted mismatch: got %q, want %q", decrypted, original)
	}
}

// TestEncryptDecryptEmptyAPIKey 验证空字符串的加解密行为。
func TestEncryptDecryptEmptyAPIKey(t *testing.T) {
	encrypted, err := EncryptAPIKey("")
	if err != nil {
		t.Fatalf("encrypt empty failed: %v", err)
	}
	if encrypted != "" {
		t.Fatalf("encrypted empty should be empty, got %q", encrypted)
	}

	decrypted, err := DecryptAPIKey("")
	if err != nil {
		t.Fatalf("decrypt empty failed: %v", err)
	}
	if decrypted != "" {
		t.Fatalf("decrypted empty should be empty, got %q", decrypted)
	}
}

// TestDecryptTamperedCiphertext 验证篡改密文后解密会失败。
func TestDecryptTamperedCiphertext(t *testing.T) {
	original := "sk-secret"
	encrypted, err := EncryptAPIKey(original)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	tampered := encrypted[:len(encrypted)-1] + string(encrypted[0]^1)
	_, err = DecryptAPIKey(tampered)
	if err == nil {
		t.Fatal("decrypt tampered ciphertext should fail")
	}
}

// TestEncryptProducesDifferentCiphertext 验证相同明文两次加密结果不同（nonce 随机）。
func TestEncryptProducesDifferentCiphertext(t *testing.T) {
	plain := "sk-repeatable"

	enc1, err := EncryptAPIKey(plain)
	if err != nil {
		t.Fatalf("first encrypt failed: %v", err)
	}
	enc2, err := EncryptAPIKey(plain)
	if err != nil {
		t.Fatalf("second encrypt failed: %v", err)
	}

	if enc1 == enc2 {
		t.Fatal("same plaintext should produce different ciphertexts due to random nonce")
	}
}

// TestEncryptLargeAPIKey 验证较长 API Key 的加解密。
func TestEncryptLargeAPIKey(t *testing.T) {
	original := strings.Repeat("a", 4096)

	encrypted, err := EncryptAPIKey(original)
	if err != nil {
		t.Fatalf("encrypt large key failed: %v", err)
	}

	decrypted, err := DecryptAPIKey(encrypted)
	if err != nil {
		t.Fatalf("decrypt large key failed: %v", err)
	}
	if decrypted != original {
		t.Fatal("large key decrypted mismatch")
	}
}
