package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlitemigrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/telemetry"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(cfg *config.Config) *SQLiteStore {
	return &SQLiteStore{
		cfg:    cfg,
		hasher: crypto.NewPasswordHasher(cfg.Security.Argon2Config),
	}
}

// Initialize sets up the database and runs migrations
func (s *SQLiteStore) Initialize(ctx context.Context) error {
	// Create data directory if it doesn't exist
	dataDir := filepath.Dir(s.cfg.Database.Path)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Check if database is new (doesn't exist)
	isNew := !fileExists(s.cfg.Database.Path)

	// Open database connection
	db, err := sql.Open("sqlite", s.cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(s.cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(s.cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(s.cfg.Database.ConnMaxLifetime) * time.Second)

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	s.db = db
	telemetry.RegisterDatabaseStatsProvider(s.db.Stats)
	slog.Info("Database connection established", "path", s.cfg.Database.Path)

	// Run migrations from embedded filesystem
	srcDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	migrationDB, err := sql.Open("sqlite", s.cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open migration database: %w", err)
	}

	dbDriver, err := sqlitemigrate.WithInstance(migrationDB, &sqlitemigrate.Config{})
	if err != nil {
		_ = migrationDB.Close()
		return fmt.Errorf("failed to create migration database driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", srcDriver, "sqlite", dbDriver)
	if err != nil {
		_ = migrationDB.Close()
		return fmt.Errorf("failed to initialize migrations: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("Database migrations completed")

	// Initialize database on first run
	if isNew {
		if err := s.initializeDatabase(ctx); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
	}

	return nil
}

// initializeDatabase creates the base DN structure and admin user
func (s *SQLiteStore) initializeDatabase(ctx context.Context) error {
	adminPassword := os.Getenv("LDAP_ADMIN_PASSWORD")
	if adminPassword == "" {
		return fmt.Errorf("LDAP_ADMIN_PASSWORD environment variable is required for first run")
	}

	baseDN := s.cfg.LDAP.BaseDN
	components := config.ParseBaseDNComponents(baseDN)

	// Create base DN entry (root)
	baseEntry := models.NewEntry(baseDN, string(models.ObjectClassTop))
	baseEntry.ParentDN = ""
	for _, component := range components {
		if strings.HasPrefix(component, "dc=") {
			dc := strings.TrimPrefix(component, "dc=")
			baseEntry.SetAttribute("dc", dc)
		}
	}

	if err := s.CreateEntry(ctx, baseEntry); err != nil {
		return fmt.Errorf("failed to create base DN: %w", err)
	}

	slog.Info("Created base DN", "dn", baseDN)

	// Create default OUs
	ousToCreate := []struct {
		name string
		desc string
	}{
		{"users", "Users organizational unit"},
		{"groups", "Groups organizational unit"},
	}

	for _, ou := range ousToCreate {
		ouEntry := models.NewOrganizationalUnit(baseDN, ou.name, ou.desc)
		if err := s.CreateEntry(ctx, ouEntry.Entry); err != nil {
			return fmt.Errorf("failed to create OU %s: %w", ou.name, err)
		}
		slog.Info("Created OU", "dn", ouEntry.DN)
	}

	// Create admin user (under ou=users)
	usersOU := fmt.Sprintf("ou=users,%s", baseDN)
	adminUser := models.NewUser(usersOU, "admin", "Administrator", "Administrator", "admin@example.com")
	hashedPassword, err := s.hasher.Hash(adminPassword)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	adminUser.SetPassword(hashedPassword)
	if err := s.CreateEntry(ctx, adminUser.Entry); err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	slog.Info("Created admin user", "dn", adminUser.DN)
	slog.Warn("Admin user initialized - password was set from LDAP_ADMIN_PASSWORD environment variable")

	// Create ldaplite.admin group (under ou=groups)
	groupsOU := fmt.Sprintf("ou=groups,%s", baseDN)
	adminGroup := models.NewGroup(groupsOU, "ldaplite.admin", "LDAPLite administrators group - members have access to web UI")
	adminGroup.AddMember(adminUser.DN)
	if err := s.CreateEntry(ctx, adminGroup.Entry); err != nil {
		return fmt.Errorf("failed to create ldaplite.admin group: %w", err)
	}

	slog.Info("Created ldaplite.admin group and added admin user", "group_dn", adminGroup.DN)

	return nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
