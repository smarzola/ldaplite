package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server   ServerConfig
	LDAP     LDAPConfig
	Database DatabaseConfig
	Logging  LoggingConfig
	Security SecurityConfig
}

type ServerConfig struct {
	Port         int
	BindAddress  string
	ReadTimeout  int // seconds
	WriteTimeout int // seconds
}

type LDAPConfig struct {
	BaseDN string
}

type DatabaseConfig struct {
	Path           string
	MaxOpenConns   int
	MaxIdleConns   int
	ConnMaxLifetime int // seconds
}

type LoggingConfig struct {
	Level  string // debug, info, warn, error
	Format string // json or text
}

type SecurityConfig struct {
	PasswordAlgorithm  string // argon2id
	AllowAnonymousBind bool   // allow anonymous binds (default: false)
	Argon2Config       Argon2Config
}

type Argon2Config struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func Load() *Config {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnvInt("LDAP_PORT", 3389),
			BindAddress:  getEnvString("LDAP_BIND_ADDRESS", "0.0.0.0"),
			ReadTimeout:  getEnvInt("LDAP_READ_TIMEOUT", 30),
			WriteTimeout: getEnvInt("LDAP_WRITE_TIMEOUT", 30),
		},
		LDAP: LDAPConfig{
			BaseDN: getEnvString("LDAP_BASE_DN", "dc=example,dc=com"),
		},
		Database: DatabaseConfig{
			Path:            getEnvString("LDAP_DATABASE_PATH", "/data/ldaplite.db"),
			MaxOpenConns:    getEnvInt("LDAP_DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("LDAP_DATABASE_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvInt("LDAP_DATABASE_CONN_MAX_LIFETIME", 300),
		},
		Logging: LoggingConfig{
			Level:  getEnvString("LDAP_LOG_LEVEL", "info"),
			Format: getEnvString("LDAP_LOG_FORMAT", "json"),
		},
		Security: SecurityConfig{
			PasswordAlgorithm:  "argon2id",
			AllowAnonymousBind: getEnvBool("LDAP_ALLOW_ANONYMOUS_BIND", false),
			Argon2Config: Argon2Config{
				Memory:      uint32(getEnvInt("LDAP_ARGON2_MEMORY", 65536)),
				Iterations:  uint32(getEnvInt("LDAP_ARGON2_ITERATIONS", 3)),
				Parallelism: uint8(getEnvInt("LDAP_ARGON2_PARALLELISM", 2)),
				SaltLength:  uint32(getEnvInt("LDAP_ARGON2_SALT_LENGTH", 16)),
				KeyLength:   uint32(getEnvInt("LDAP_ARGON2_KEY_LENGTH", 32)),
			},
		},
	}

	// Validate required fields
	if cfg.LDAP.BaseDN == "" {
		slog.Error("LDAP_BASE_DN is required")
		os.Exit(1)
	}

	return cfg
}

func (c *Config) Print() {
	slog.Info("Configuration loaded",
		"port", c.Server.Port,
		"bind_address", c.Server.BindAddress,
		"base_dn", c.LDAP.BaseDN,
		"database_path", c.Database.Path,
		"log_level", c.Logging.Level,
		"log_format", c.Logging.Format,
		"allow_anonymous_bind", c.Security.AllowAnonymousBind,
	)
}

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func ParseBaseDNComponents(baseDN string) []string {
	components := []string{}
	parts := strings.Split(baseDN, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			components = append(components, part)
		}
	}
	return components
}
