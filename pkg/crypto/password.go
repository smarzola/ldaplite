package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"github.com/smarzola/ldaplite/pkg/config"
)

const (
	argon2VersionString = "argon2id"
	argon2Version       = 19 // Argon2id version
)

// PasswordHasher handles password hashing and verification
type PasswordHasher struct {
	cfg config.Argon2Config
}

// NewPasswordHasher creates a new password hasher
func NewPasswordHasher(cfg config.Argon2Config) *PasswordHasher {
	return &PasswordHasher{cfg: cfg}
}

// Hash hashes a password using Argon2id
// Returns hash in format: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
func (ph *PasswordHasher) Hash(password string) (string, error) {
	// Generate random salt
	salt := make([]byte, ph.cfg.SaltLength)
	_, err := rand.Read(salt)
	if err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash password using Argon2id
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		ph.cfg.Iterations,
		ph.cfg.Memory,
		ph.cfg.Parallelism,
		ph.cfg.KeyLength,
	)

	// Encode salt and hash to base64
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=65536,t=3,p=2$salt$hash
	return fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2VersionString,
		argon2Version,
		ph.cfg.Memory,
		ph.cfg.Iterations,
		ph.cfg.Parallelism,
		saltB64,
		hashB64,
	), nil
}

// Verify verifies a password against its hash
func (ph *PasswordHasher) Verify(password, hashedPassword string) (bool, error) {
	// Parse the hash
	parts := strings.Split(hashedPassword, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	// Extract parameters
	if parts[1] != argon2VersionString {
		return false, fmt.Errorf("unsupported hash algorithm: %s", parts[1])
	}

	// Decode salt and hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	// Compute hash of provided password
	computedHash := argon2.IDKey(
		[]byte(password),
		salt,
		ph.cfg.Iterations,
		ph.cfg.Memory,
		ph.cfg.Parallelism,
		ph.cfg.KeyLength,
	)

	// Compare hashes in constant time
	return constantTimeCompare(computedHash, expectedHash), nil
}

// constantTimeCompare compares two byte slices in constant time
func constantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	var result byte
	for i := range a {
		result |= a[i] ^ b[i]
	}

	return result == 0
}
