package config

import (
	"os"
	"testing"
)

func TestSecretKey_EncryptDecrypt(t *testing.T) {
	os.Setenv("AULE_SECRET_KEY", "test-secret-key-for-unit-tests")
	defer os.Unsetenv("AULE_SECRET_KEY")

	sk, err := NewSecretKey()
	if err != nil {
		t.Fatalf("NewSecretKey: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"api_key", "sk-abc123def456xyz"},
		{"empty", ""},
		{"long_key", "sk-proj-very-long-api-key-that-might-be-used-by-some-providers-1234567890"},
		{"special_chars", "sk-+/=!@#$%^&*()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := sk.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}

			if tt.plaintext == "" {
				if encrypted != "" {
					t.Fatal("expected empty encrypted for empty plaintext")
				}
				return
			}

			// Should have enc: prefix
			if encrypted[:4] != "enc:" {
				t.Fatalf("expected enc: prefix, got %s", encrypted[:4])
			}

			// Should not equal plaintext
			if encrypted == tt.plaintext {
				t.Fatal("encrypted should differ from plaintext")
			}

			// Decrypt
			decrypted, err := sk.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Fatalf("expected %q, got %q", tt.plaintext, decrypted)
			}
		})
	}
}

func TestSecretKey_DecryptPlaintext(t *testing.T) {
	os.Setenv("AULE_SECRET_KEY", "test-key")
	defer os.Unsetenv("AULE_SECRET_KEY")

	sk, err := NewSecretKey()
	if err != nil {
		t.Fatalf("NewSecretKey: %v", err)
	}

	// Non-encrypted string should pass through
	result, err := sk.Decrypt("plain-text-value")
	if err != nil {
		t.Fatalf("Decrypt plain: %v", err)
	}
	if result != "plain-text-value" {
		t.Fatalf("expected plain-text-value, got %s", result)
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"ab", "****"},
		{"abcd", "****"},
		{"sk-abc123def", "****3def"},
		{"sk-proj-very-long-key-12345", "****2345"},
	}

	for _, tt := range tests {
		result := MaskSecret(tt.input)
		if result != tt.expected {
			t.Errorf("MaskSecret(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
