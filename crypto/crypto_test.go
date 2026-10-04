package crypto

import (
	"testing"
)

func TestCryptoHelpers(t *testing.T) {
	t.Run("Bcrypt hashing and verification", func(t *testing.T) {
		password := "secretPassword123"
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}

		if !CheckPasswordHash(password, hash) {
			t.Errorf("expected password to match hash")
		}

		if CheckPasswordHash("wrongPassword", hash) {
			t.Errorf("expected wrong password to fail")
		}
	})

	t.Run("AES-256-GCM encryption and decryption", func(t *testing.T) {
		key := []byte("01234567890123456789012345678901") // 32 bytes
		plainText := "sensitive_credit_card_data"

		encrypted, err := EncryptAESGCM(plainText, key)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}

		decrypted, err := DecryptAESGCM(encrypted, key)
		if err != nil {
			t.Fatalf("failed to decrypt: %v", err)
		}

		if decrypted != plainText {
			t.Errorf("expected %s, got %s", plainText, decrypted)
		}
	})

	t.Run("Argon2id hashing and verification", func(t *testing.T) {
		pwd := "SuperSecretPassword#2026"
		h, err := HashArgon2id(pwd)
		if err != nil {
			t.Fatalf("failed to hash with Argon2id: %v", err)
		}

		if err := VerifyArgon2id(pwd, h); err != nil {
			t.Errorf("expected valid password verification, got: %v", err)
		}

		if err := VerifyArgon2id("incorrectPassword", h); err == nil {
			t.Errorf("expected incorrect password to fail")
		}

		randPwd := GenerateRandomPassword(16)
		if len(randPwd) != 16 {
			t.Errorf("expected random password of length 16, got %d", len(randPwd))
		}
	})
}

