package crypto

import (
	"testing"

	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPasswordHasher(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	assert.NotNil(t, hasher)
}

func TestHash(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "test-password-123"

	hash, err := hasher.Hash(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Check hash format: {ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
	assert.Contains(t, hash, "{ARGON2ID}")
	assert.Contains(t, hash, "$argon2id$v=19$")
	assert.Contains(t, hash, "m=65536")
	assert.Contains(t, hash, "t=3")
	assert.Contains(t, hash, "p=2")
}

func TestHashDifferent(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "test-password"

	hash1, err := hasher.Hash(password)
	require.NoError(t, err)

	hash2, err := hasher.Hash(password)
	require.NoError(t, err)

	// Same password should produce different hashes (different salt)
	assert.NotEqual(t, hash1, hash2)
}

func TestVerify(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "correct-password"

	hash, err := hasher.Hash(password)
	require.NoError(t, err)

	// Correct password
	verified, err := hasher.Verify(password, hash)
	assert.NoError(t, err)
	assert.True(t, verified)
}

func TestVerifyWrongPassword(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "correct-password"

	hash, err := hasher.Hash(password)
	require.NoError(t, err)

	// Wrong password
	verified, err := hasher.Verify("wrong-password", hash)
	assert.NoError(t, err)
	assert.False(t, verified)
}

func TestVerifyInvalidHash(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)

	// Invalid hash format (missing scheme prefix)
	verified, err := hasher.Verify("password", "invalid-hash")
	assert.Error(t, err)
	assert.False(t, verified)
}

func TestVerifyEmptyPassword(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "test-password"

	hash, err := hasher.Hash(password)
	require.NoError(t, err)

	// Empty password should not verify
	verified, err := hasher.Verify("", hash)
	assert.NoError(t, err)
	assert.False(t, verified)
}

func TestHashEmpty(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)

	// Should be able to hash empty password
	hash, err := hasher.Hash("")
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Should be able to verify empty password
	verified, err := hasher.Verify("", hash)
	assert.NoError(t, err)
	assert.True(t, verified)
}

func TestVerifyMultipleTimes(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "test-password"

	hash, err := hasher.Hash(password)
	require.NoError(t, err)

	// Verify multiple times
	for i := 0; i < 5; i++ {
		verified, err := hasher.Verify(password, hash)
		assert.NoError(t, err)
		assert.True(t, verified)
	}
}

// New tests for ProcessPassword

func TestProcessPassword_PlainText(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "plain-text-password"

	processed, err := hasher.ProcessPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, processed)

	// Should have scheme prefix
	assert.Contains(t, processed, "{ARGON2ID}")

	// Should be able to verify with original password
	verified, err := hasher.Verify(password, processed)
	assert.NoError(t, err)
	assert.True(t, verified)
}

func TestProcessPassword_PreHashed(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "test-password"

	// Hash it first
	hashed, err := hasher.Hash(password)
	require.NoError(t, err)

	// Process the already-hashed password
	processed, err := hasher.ProcessPassword(hashed)
	assert.NoError(t, err)

	// Should return the same hash (not re-hash it)
	assert.Equal(t, hashed, processed)

	// Should still verify
	verified, err := hasher.Verify(password, processed)
	assert.NoError(t, err)
	assert.True(t, verified)
}

func TestProcessPassword_UnsupportedScheme(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)

	// Password with unsupported scheme
	password := "{SSHA}unsupported-hash-format"

	processed, err := hasher.ProcessPassword(password)
	assert.Error(t, err)
	assert.Empty(t, processed)
	assert.Contains(t, err.Error(), "unsupported password scheme")
}

func TestProcessPassword_MalformedScheme(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)

	// Password with malformed scheme (no closing brace)
	password := "{ARGON2IDinvalid"

	processed, err := hasher.ProcessPassword(password)
	assert.Error(t, err)
	assert.Empty(t, processed)
}

func TestProcessPassword_InvalidHashStructure(t *testing.T) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)

	// Password with valid scheme but invalid hash structure
	password := "{ARGON2ID}invalid-hash-structure"

	processed, err := hasher.ProcessPassword(password)
	assert.Error(t, err)
	assert.Empty(t, processed)
	assert.Contains(t, err.Error(), "invalid hashed password format")
}

func TestExtractScheme(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "Valid ARGON2ID scheme",
			input:    "{ARGON2ID}$argon2id$...",
			expected: "ARGON2ID",
			wantErr:  false,
		},
		{
			name:     "Valid SSHA scheme",
			input:    "{SSHA}base64data",
			expected: "SSHA",
			wantErr:  false,
		},
		{
			name:     "No opening brace",
			input:    "ARGON2ID}data",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "No closing brace",
			input:    "{ARGON2IDdata",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Empty scheme",
			input:    "{}data",
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, err := extractScheme(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, scheme)
			}
		})
	}
}

func BenchmarkHash(b *testing.B) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "benchmark-password"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hasher.Hash(password)
	}
}

func BenchmarkVerify(b *testing.B) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "benchmark-password"
	hash, _ := hasher.Hash(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hasher.Verify(password, hash)
	}
}

func BenchmarkProcessPassword(b *testing.B) {
	cfg := config.Argon2Config{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hasher := NewPasswordHasher(cfg)
	password := "benchmark-password"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hasher.ProcessPassword(password)
	}
}
