package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHashFormat = errors.New("invalid hash format")
	ErrHashGeneration    = errors.New("failed to generate hash")
	ErrSaltGeneration    = errors.New("failed to generate salt")
	ErrDecodeHash        = errors.New("failed to decode hash")
)

const (
	argon2Memory      = 64 * 1024
	argon2Iterations  = 3
	argon2Parallelism = 2
	argon2SaltLength  = 16
	argon2KeyLength   = 32
)

func generateSalt() ([]byte, error) {
	salt := make([]byte, argon2SaltLength)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, err
	}
	return salt, nil
}

// HashArgon2id securely hashes a plain-text password using the Argon2id key derivation function.
func HashArgon2id(data string) (string, error) {
	salt, err := generateSalt()
	if err != nil {
		return "", ErrSaltGeneration
	}

	hash := argon2.IDKey(
		[]byte(data),
		salt,
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		argon2KeyLength,
	)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		b64Salt,
		b64Hash,
	)

	if encoded == "" {
		return "", ErrHashGeneration
	}

	return encoded, nil
}

// VerifyArgon2id verifies that a plain-text string matches an Argon2id encoded hash in constant time.
func VerifyArgon2id(data string, encodedHash string) error {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return ErrInvalidHashFormat
	}

	if parts[1] != "argon2id" {
		return errors.New("unsupported algorithm")
	}

	var parsedMemory, parsedIterations uint32
	var parsedParallelism uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &parsedMemory, &parsedIterations, &parsedParallelism)
	if err != nil {
		return ErrInvalidHashFormat
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrDecodeHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return ErrDecodeHash
	}

	comparisonHash := argon2.IDKey(
		[]byte(data),
		salt,
		parsedIterations,
		parsedMemory,
		parsedParallelism,
		uint32(len(hash)),
	)

	if subtle.ConstantTimeCompare(hash, comparisonHash) != 1 {
		return errors.New("invalid credentials")
	}

	return nil
}

// GenerateRandomPassword generates a cryptographically random secure password with letters, digits, and symbols.
func GenerateRandomPassword(length int) string {
	if length <= 0 {
		length = 16
	}
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$"
	b := make([]byte, length)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}
