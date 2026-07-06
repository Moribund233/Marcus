// Package config 提供 Marcus 运行配置与敏感信息加密能力。
package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// masterKeyEnvVar 是存放 AES-256-GCM 主密钥的环境变量名。
// 若未设置，本包会尝试从一个固定但公开的占位密钥派生（仅用于开发环境，生产环境必须设置）。
const masterKeyEnvVar = "MARCUS_MASTER_KEY"

// EncryptAPIKey 使用 AES-256-GCM 加密 API Key。
// 返回 base64 编码的密文，包含 nonce 和 ciphertext，可直接存入数据库。
//
// 加密密钥优先从环境变量 MARCUS_MASTER_KEY 获取；若未设置，则使用一个
// 从环境变量名派生的占位密钥（仅用于开发/测试，生产环境必须设置真实密钥）。
func EncryptAPIKey(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key, err := deriveMasterKey()
	if err != nil {
		return "", fmt.Errorf("derive master key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAPIKey 解密由 EncryptAPIKey 加密的 API Key。
func DecryptAPIKey(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	key, err := deriveMasterKey()
	if err != nil {
		return "", fmt.Errorf("derive master key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// deriveMasterKey 返回 32 字节的 AES 主密钥。
//
// 优先读取 MARCUS_MASTER_KEY 环境变量；若其长度正好是 32 字节则直接使用。
// 否则使用 SHA-256 对环境变量值（或占位字符串）进行派生。
func deriveMasterKey() ([]byte, error) {
	value := os.Getenv(masterKeyEnvVar)
	if value == "" {
		// 占位密钥：仅用于开发/测试，确保功能可运行。
		// 生产环境务必设置 MARCUS_MASTER_KEY。
		value = "marcus-default-dev-key-do-not-use-in-production"
	}

	if len(value) == 32 {
		return []byte(value), nil
	}

	hash := sha256.Sum256([]byte(value))
	return hash[:], nil
}
