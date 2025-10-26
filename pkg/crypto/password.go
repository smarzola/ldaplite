package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/smarzola/ldaplite/pkg/config"
	"golang.org/x/crypto/argon2"
)

const (
	// LDAP password scheme identifiers (RFC 3112)
	SchemeArgon2ID     = "{ARGON2ID}"
	schemeArgon2IDName = "ARGON2ID" // Scheme name without braces

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

// ProcessPassword is the main entry point for LDAP operations.
// It accepts plain text passwords (hashes them) or pre-hashed passwords with scheme prefix.
// Returns LDAP-compliant password string: {ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$salt$hash
func (ph *PasswordHasher) ProcessPassword(password string) (string, error) {
	// Check if already hashed with scheme prefix
	if strings.HasPrefix(password, "{") {
		scheme, err := extractScheme(password)
		if err != nil {
			return "", err
		}

		// Only accept schemes we support (compare without braces)
		if scheme != schemeArgon2IDName {
			return "", fmt.Errorf("unsupported password scheme: {%s} (supported: %s)", scheme, SchemeArgon2ID)
		}

		// Validate format of hashed password
		if err := ph.validateHashedPassword(password); err != nil {
			return "", fmt.Errorf("invalid hashed password format: %w", err)
		}

		return password, nil
	}

	// Plain text password - hash it
	return ph.Hash(password)
}

// Hash creates a new password hash with LDAP scheme prefix
// Format: {ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$salt$hash
func (ph *PasswordHasher) Hash(password string) (string, error) {
	// Generate random salt
	salt := make([]byte, ph.cfg.SaltLength)
	if _, err := rand.Read(salt); err != nil {
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

	// Format inner hash: $argon2id$v=19$m=65536,t=3,p=2$salt$hash
	inner := fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2VersionString,
		argon2Version,
		ph.cfg.Memory,
		ph.cfg.Iterations,
		ph.cfg.Parallelism,
		saltB64,
		hashB64,
	)

	// Add LDAP scheme prefix
	return SchemeArgon2ID + inner, nil
}

// Verify verifies a password against its hash
// Expects hash format: {ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$salt$hash
func (ph *PasswordHasher) Verify(password, hashedPassword string) (bool, error) {
	// Must have scheme prefix
	if !strings.HasPrefix(hashedPassword, SchemeArgon2ID) {
		return false, fmt.Errorf("password hash missing scheme prefix")
	}

	// Strip scheme prefix to get inner hash
	inner := strings.TrimPrefix(hashedPassword, SchemeArgon2ID)

	// Parse the inner hash
	parts := strings.Split(inner, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	// Verify algorithm
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

// extractScheme extracts the scheme identifier from a password hash
// Example: "{ARGON2ID}$argon2id$..." -> "ARGON2ID"
func extractScheme(hashedPassword string) (string, error) {
	if !strings.HasPrefix(hashedPassword, "{") {
		return "", fmt.Errorf("no scheme prefix found")
	}

	endIdx := strings.Index(hashedPassword, "}")
	if endIdx == -1 {
		return "", fmt.Errorf("malformed scheme prefix")
	}

	return hashedPassword[1:endIdx], nil
}

// validateHashedPassword validates the structure of a hashed password
func (ph *PasswordHasher) validateHashedPassword(hashedPassword string) error {
	if !strings.HasPrefix(hashedPassword, SchemeArgon2ID) {
		return fmt.Errorf("missing or invalid scheme prefix")
	}

	inner := strings.TrimPrefix(hashedPassword, SchemeArgon2ID)
	parts := strings.Split(inner, "$")

	if len(parts) != 6 {
		return fmt.Errorf("invalid hash structure (expected 6 parts, got %d)", len(parts))
	}

	if parts[1] != argon2VersionString {
		return fmt.Errorf("invalid algorithm: %s", parts[1])
	}

	return nil
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
