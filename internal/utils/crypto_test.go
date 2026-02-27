package utils

import (
	"os"
	"testing"
)

func TestGenerateRandomString(t *testing.T) {
	t.Run("deve gerar string com comprimento correto (hex = 2x bytes)", func(t *testing.T) {
		result := GenerateRandomString(8)
		if len(result) != 16 { // 8 bytes = 16 hex chars
			t.Errorf("esperava 16 chars, obteve %d: %q", len(result), result)
		}
	})

	t.Run("deve gerar strings únicas", func(t *testing.T) {
		a := GenerateRandomString(16)
		b := GenerateRandomString(16)
		if a == b {
			t.Errorf("duas chamadas retornaram o mesmo valor: %q", a)
		}
	})

	t.Run("deve funcionar com tamanho zero", func(t *testing.T) {
		result := GenerateRandomString(0)
		if result != "" {
			t.Errorf("esperava string vazia, obteve %q", result)
		}
	})

	t.Run("deve gerar strings somente hex", func(t *testing.T) {
		result := GenerateRandomString(32)
		for _, c := range result {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("char não-hex encontrado: %c em %q", c, result)
			}
		}
	})
}

func TestComposeFingerprintHash(t *testing.T) {
	t.Run("deve gerar hash SHA-256 (64 hex chars)", func(t *testing.T) {
		hash := ComposeFingerprintHash("192.168.1.1", "Mozilla/5.0")
		if len(hash) != 64 {
			t.Errorf("esperava 64 chars, obteve %d: %q", len(hash), hash)
		}
	})

	t.Run("mesmo input deve gerar mesmo hash", func(t *testing.T) {
		a := ComposeFingerprintHash("10.0.0.1", "Chrome")
		b := ComposeFingerprintHash("10.0.0.1", "Chrome")
		if a != b {
			t.Errorf("mesmo input gerou hashes diferentes: %q vs %q", a, b)
		}
	})

	t.Run("IP diferente deve gerar hash diferente", func(t *testing.T) {
		a := ComposeFingerprintHash("10.0.0.1", "Chrome")
		b := ComposeFingerprintHash("10.0.0.2", "Chrome")
		if a == b {
			t.Errorf("IPs diferentes geraram o mesmo hash: %q", a)
		}
	})

	t.Run("User-Agent diferente deve gerar hash diferente", func(t *testing.T) {
		a := ComposeFingerprintHash("10.0.0.1", "Chrome")
		b := ComposeFingerprintHash("10.0.0.1", "Firefox")
		if a == b {
			t.Errorf("UAs diferentes geraram o mesmo hash: %q", a)
		}
	})

	t.Run("deve respeitar APP_SALT do ambiente", func(t *testing.T) {
		os.Setenv("APP_SALT", "salt_A")
		a := ComposeFingerprintHash("10.0.0.1", "Chrome")
		os.Setenv("APP_SALT", "salt_B")
		b := ComposeFingerprintHash("10.0.0.1", "Chrome")
		os.Unsetenv("APP_SALT")

		if a == b {
			t.Errorf("salts diferentes geraram o mesmo hash: %q", a)
		}
	})

	t.Run("deve usar salt padrão quando APP_SALT vazio", func(t *testing.T) {
		os.Unsetenv("APP_SALT")
		hash := ComposeFingerprintHash("10.0.0.1", "Chrome")
		if hash == "" {
			t.Error("hash não deve ser vazio mesmo sem APP_SALT")
		}
	})
}
