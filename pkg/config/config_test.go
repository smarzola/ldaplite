package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadDefaults(t *testing.T) {
	// Clear environment
	t.Cleanup(func() {
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("LDAP_PORT")
		os.Unsetenv("LDAP_BIND_ADDRESS")
	})

	// Set required variable
	os.Setenv("LDAP_BASE_DN", "dc=example,dc=com")

	cfg := Load()

	assert.NotNil(t, cfg)
	assert.Equal(t, "dc=example,dc=com", cfg.LDAP.BaseDN)
	assert.Equal(t, 3389, cfg.Server.Port)
	assert.Equal(t, "0.0.0.0", cfg.Server.BindAddress)
	assert.Equal(t, "/data/ldaplite.db", cfg.Database.Path)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
}

func TestLoadCustomPort(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("LDAP_PORT")
	})

	os.Setenv("LDAP_BASE_DN", "dc=test,dc=com")
	os.Setenv("LDAP_PORT", "10389")

	cfg := Load()

	assert.Equal(t, 10389, cfg.Server.Port)
}

func TestLoadCustomBindAddress(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("LDAP_BIND_ADDRESS")
	})

	os.Setenv("LDAP_BASE_DN", "dc=test,dc=com")
	os.Setenv("LDAP_BIND_ADDRESS", "127.0.0.1")

	cfg := Load()

	assert.Equal(t, "127.0.0.1", cfg.Server.BindAddress)
}

func TestLoadCustomDatabase(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("LDAP_DATABASE_PATH")
	})

	os.Setenv("LDAP_BASE_DN", "dc=test,dc=com")
	os.Setenv("LDAP_DATABASE_PATH", "/custom/path/ldaplite.db")

	cfg := Load()

	assert.Equal(t, "/custom/path/ldaplite.db", cfg.Database.Path)
}

func TestLoadLoggingConfig(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("LDAP_LOG_LEVEL")
		os.Unsetenv("LDAP_LOG_FORMAT")
	})

	os.Setenv("LDAP_BASE_DN", "dc=test,dc=com")
	os.Setenv("LDAP_LOG_LEVEL", "debug")
	os.Setenv("LDAP_LOG_FORMAT", "text")

	cfg := Load()

	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
}

func TestLoadArgon2Config(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("LDAP_ARGON2_MEMORY")
		os.Unsetenv("LDAP_ARGON2_ITERATIONS")
	})

	os.Setenv("LDAP_BASE_DN", "dc=test,dc=com")
	os.Setenv("LDAP_ARGON2_MEMORY", "32768")
	os.Setenv("LDAP_ARGON2_ITERATIONS", "4")

	cfg := Load()

	assert.Equal(t, uint32(32768), cfg.Security.Argon2Config.Memory)
	assert.Equal(t, uint32(4), cfg.Security.Argon2Config.Iterations)
}

func TestParseBaseDNComponents(t *testing.T) {
	tests := []struct {
		name      string
		baseDN    string
		expected  []string
	}{
		{
			name:     "single component",
			baseDN:   "dc=com",
			expected: []string{"dc=com"},
		},
		{
			name:     "two components",
			baseDN:   "dc=example,dc=com",
			expected: []string{"dc=example", "dc=com"},
		},
		{
			name:     "three components",
			baseDN:   "ou=users,dc=example,dc=com",
			expected: []string{"ou=users", "dc=example", "dc=com"},
		},
		{
			name:     "with spaces",
			baseDN:   "ou=users , dc=example , dc=com",
			expected: []string{"ou=users", "dc=example", "dc=com"},
		},
		{
			name:     "empty string",
			baseDN:   "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseBaseDNComponents(tt.baseDN)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigPrint(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LDAP_BASE_DN")
	})

	os.Setenv("LDAP_BASE_DN", "dc=test,dc=com")

	cfg := Load()

	// Should not panic
	assert.NotPanics(t, func() {
		cfg.Print()
	})
}

func TestConfigTimeouts(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("LDAP_READ_TIMEOUT")
		os.Unsetenv("LDAP_WRITE_TIMEOUT")
	})

	os.Setenv("LDAP_BASE_DN", "dc=test,dc=com")
	os.Setenv("LDAP_READ_TIMEOUT", "60")
	os.Setenv("LDAP_WRITE_TIMEOUT", "45")

	cfg := Load()

	assert.Equal(t, 60, cfg.Server.ReadTimeout)
	assert.Equal(t, 45, cfg.Server.WriteTimeout)
}

func TestConfigDatabase(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("LDAP_DATABASE_MAX_OPEN_CONNS")
		os.Unsetenv("LDAP_DATABASE_MAX_IDLE_CONNS")
	})

	os.Setenv("LDAP_BASE_DN", "dc=test,dc=com")
	os.Setenv("LDAP_DATABASE_MAX_OPEN_CONNS", "50")
	os.Setenv("LDAP_DATABASE_MAX_IDLE_CONNS", "10")

	cfg := Load()

	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 10, cfg.Database.MaxIdleConns)
}
