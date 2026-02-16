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
	"path/filepath"
	"strings"
)

const encPrefix = "enc:"

// SecretKey manages the master encryption key for API secrets.
// Uses AES-256-GCM for authenticated encryption.
type SecretKey struct {
	key []byte
}

// NewSecretKey initializes encryption from AULE_SECRET_KEY env or auto-generates
// a persistent key at ~/.aule/secret.key. Follows the pattern used by Gitea/Grafana
// for encrypting sensitive configuration values.
func NewSecretKey() (*SecretKey, error) {
	rawKey := os.Getenv("AULE_SECRET_KEY")
	if rawKey != "" {
		h := sha256.Sum256([]byte(rawKey))
		return &SecretKey{key: h[:]}, nil
	}

	// Auto-generate and persist on first run
	keyPath := filepath.Join(homeDir(), ".aule", "secret.key")
	if data, err := os.ReadFile(keyPath); err == nil && len(data) >= 32 {
		return &SecretKey{key: data[:32]}, nil
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate secret key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", err)
	}
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("failed to write secret key: %w", err)
	}

	return &SecretKey{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns base64-encoded
// ciphertext with "enc:" prefix for storage identification.
func (s *SecretKey) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts an "enc:" prefixed base64 string back to plaintext.
func (s *SecretKey) Decrypt(encrypted string) (string, error) {
	if encrypted == "" || !strings.HasPrefix(encrypted, encPrefix) {
		return encrypted, nil
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encrypted, encPrefix))
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// MaskSecret returns a masked version safe for API display: "****abcd"
func MaskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 4 {
		return "****"
	}
	return "****" + secret[len(secret)-4:]
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "/tmp"
}
