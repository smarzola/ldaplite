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
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "modernc.org/sqlite"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

// SQLiteStore implements the Store interface using SQLite
type SQLiteStore struct {
	db     *sql.DB
	cfg    *config.Config
	hasher *crypto.PasswordHasher
}

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
	slog.Info("Database connection established", "path", s.cfg.Database.Path)

	// Run migrations
	migrationsPath := "file://./migrations"
	dbURL := fmt.Sprintf("sqlite://%s", s.cfg.Database.Path)

	m, err := migrate.New(migrationsPath, dbURL)
	if err != nil {
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
		if err := s.CreateOU(ctx, ouEntry); err != nil {
			return fmt.Errorf("failed to create OU %s: %w", ou.name, err)
		}
		slog.Info("Created OU", "dn", ouEntry.DN)
	}

	// Create admin user
	adminUser := models.NewUser(baseDN, "admin", "Administrator", "Administrator", "Administrator", "admin@example.com")
	hashedPassword, err := s.hasher.Hash(adminPassword)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	adminUser.SetPassword(hashedPassword)
	if err := s.CreateUser(ctx, adminUser); err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	slog.Info("Created admin user", "dn", adminUser.DN)
	slog.Warn("Admin user initialized - password was set from LDAP_ADMIN_PASSWORD environment variable")

	return nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetEntry retrieves an entry by DN
func (s *SQLiteStore) GetEntry(ctx context.Context, dn string) (*models.Entry, error) {
	query := `
		SELECT id, dn, parent_dn, object_class, created_at, updated_at
		FROM entries
		WHERE dn = ?
	`

	var entry models.Entry
	err := s.db.QueryRowContext(ctx, query, dn).Scan(
		&entry.ID,
		&entry.DN,
		&entry.ParentDN,
		&entry.ObjectClass,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}

	// Load attributes
	entry.Attributes = make(map[string][]string)
	attrQuery := `SELECT name, value FROM attributes WHERE entry_id = ?`
	rows, err := s.db.QueryContext(ctx, attrQuery, entry.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attributes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("failed to scan attribute: %w", err)
		}
		entry.Attributes[strings.ToLower(name)] = append(entry.Attributes[strings.ToLower(name)], value)
	}

	return &entry, nil
}

// CreateEntry creates a new entry
func (s *SQLiteStore) CreateEntry(ctx context.Context, entry *models.Entry) error {
	if err := entry.Validate(); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert entry
	query := `
		INSERT INTO entries (dn, parent_dn, object_class, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := tx.ExecContext(
		ctx,
		query,
		entry.DN,
		entry.ParentDN,
		entry.ObjectClass,
		entry.CreatedAt,
		entry.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create entry: %w", err)
	}

	entryID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get entry ID: %w", err)
	}

	entry.ID = entryID

	// Insert attributes
	attrQuery := `INSERT INTO attributes (entry_id, name, value) VALUES (?, ?, ?)`
	for name, values := range entry.Attributes {
		for _, value := range values {
			if _, err := tx.ExecContext(ctx, attrQuery, entryID, name, value); err != nil {
				return fmt.Errorf("failed to insert attribute: %w", err)
			}
		}
	}

	// Handle type-specific entries
	if entry.IsUser() {
		uid := entry.GetAttribute("uid")
		if uid == "" {
			return fmt.Errorf("user entry missing uid attribute")
		}
		passwordHash := entry.GetAttribute("userPassword")
		userQuery := `INSERT INTO users (entry_id, uid, password_hash) VALUES (?, ?, ?)`
		if _, err := tx.ExecContext(ctx, userQuery, entryID, uid, passwordHash); err != nil {
			return fmt.Errorf("failed to create user entry: %w", err)
		}
	} else if entry.IsGroup() {
		cn := entry.GetAttribute("cn")
		if cn == "" {
			return fmt.Errorf("group entry missing cn attribute")
		}
		groupQuery := `INSERT INTO groups (entry_id, cn) VALUES (?, ?)`
		if _, err := tx.ExecContext(ctx, groupQuery, entryID, cn); err != nil {
			return fmt.Errorf("failed to create group entry: %w", err)
		}
	} else if entry.IsOrganizationalUnit() {
		ou := entry.GetAttribute("ou")
		if ou == "" {
			return fmt.Errorf("OU entry missing ou attribute")
		}
		ouQuery := `INSERT INTO organizational_units (entry_id, ou) VALUES (?, ?)`
		if _, err := tx.ExecContext(ctx, ouQuery, entryID, ou); err != nil {
			return fmt.Errorf("failed to create OU entry: %w", err)
		}
	}

	return tx.Commit()
}

// UpdateEntry updates an existing entry
func (s *SQLiteStore) UpdateEntry(ctx context.Context, entry *models.Entry) error {
	if err := entry.Validate(); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update entry timestamp
	query := `UPDATE entries SET updated_at = ? WHERE dn = ?`
	if _, err := tx.ExecContext(ctx, query, entry.UpdatedAt, entry.DN); err != nil {
		return fmt.Errorf("failed to update entry: %w", err)
	}

	// Delete existing attributes
	delAttrQuery := `DELETE FROM attributes WHERE entry_id = (SELECT id FROM entries WHERE dn = ?)`
	if _, err := tx.ExecContext(ctx, delAttrQuery, entry.DN); err != nil {
		return fmt.Errorf("failed to delete attributes: %w", err)
	}

	// Insert new attributes
	insertAttrQuery := `INSERT INTO attributes (entry_id, name, value) VALUES ((SELECT id FROM entries WHERE dn = ?), ?, ?)`
	for name, values := range entry.Attributes {
		for _, value := range values {
			if _, err := tx.ExecContext(ctx, insertAttrQuery, entry.DN, name, value); err != nil {
				return fmt.Errorf("failed to insert attribute: %w", err)
			}
		}
	}

	return tx.Commit()
}

// DeleteEntry deletes an entry
func (s *SQLiteStore) DeleteEntry(ctx context.Context, dn string) error {
	query := `DELETE FROM entries WHERE dn = ?`
	result, err := s.db.ExecContext(ctx, query, dn)
	if err != nil {
		return fmt.Errorf("failed to delete entry: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("entry not found: %s", dn)
	}

	return nil
}

// EntryExists checks if an entry exists
func (s *SQLiteStore) EntryExists(ctx context.Context, dn string) (bool, error) {
	query := `SELECT 1 FROM entries WHERE dn = ? LIMIT 1`
	var exists int
	err := s.db.QueryRowContext(ctx, query, dn).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check entry existence: %w", err)
	}
	return true, nil
}

// SearchEntries searches for entries matching a filter
func (s *SQLiteStore) SearchEntries(ctx context.Context, baseDN string, filter string) ([]*models.Entry, error) {
	// For now, implement basic search that finds entries under baseDN
	// More complex filter parsing will be implemented in a separate module
	query := `
		SELECT id, dn, parent_dn, object_class, created_at, updated_at
		FROM entries
		WHERE (dn = ? OR parent_dn LIKE ?)
	`

	likePattern := "%" + baseDN
	rows, err := s.db.QueryContext(ctx, query, baseDN, likePattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search entries: %w", err)
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		entry := &models.Entry{Attributes: make(map[string][]string)}
		if err := rows.Scan(
			&entry.ID,
			&entry.DN,
			&entry.ParentDN,
			&entry.ObjectClass,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		// Load attributes
		attrQuery := `SELECT name, value FROM attributes WHERE entry_id = ?`
		attrRows, err := s.db.QueryContext(ctx, attrQuery, entry.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get attributes: %w", err)
		}

		for attrRows.Next() {
			var name, value string
			if err := attrRows.Scan(&name, &value); err != nil {
				attrRows.Close()
				return nil, fmt.Errorf("failed to scan attribute: %w", err)
			}
			entry.Attributes[strings.ToLower(name)] = append(entry.Attributes[strings.ToLower(name)], value)
		}
		attrRows.Close()

		// Add objectClass to attributes map for filter matching
		if entry.ObjectClass != "" {
			entry.Attributes["objectclass"] = []string{entry.ObjectClass}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// GetAllEntries returns all entries
func (s *SQLiteStore) GetAllEntries(ctx context.Context) ([]*models.Entry, error) {
	query := `
		SELECT id, dn, parent_dn, object_class, created_at, updated_at
		FROM entries
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all entries: %w", err)
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		entry := &models.Entry{Attributes: make(map[string][]string)}
		if err := rows.Scan(
			&entry.ID,
			&entry.DN,
			&entry.ParentDN,
			&entry.ObjectClass,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		// Load attributes
		attrQuery := `SELECT name, value FROM attributes WHERE entry_id = ?`
		attrRows, err := s.db.QueryContext(ctx, attrQuery, entry.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get attributes: %w", err)
		}

		for attrRows.Next() {
			var name, value string
			if err := attrRows.Scan(&name, &value); err != nil {
				attrRows.Close()
				return nil, fmt.Errorf("failed to scan attribute: %w", err)
			}
			entry.Attributes[strings.ToLower(name)] = append(entry.Attributes[strings.ToLower(name)], value)
		}
		attrRows.Close()

		// Add objectClass to attributes map for filter matching
		if entry.ObjectClass != "" {
			entry.Attributes["objectclass"] = []string{entry.ObjectClass}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// GetChildren returns all children of a given DN
func (s *SQLiteStore) GetChildren(ctx context.Context, dn string) ([]*models.Entry, error) {
	query := `
		SELECT id, dn, parent_dn, object_class, created_at, updated_at
		FROM entries
		WHERE parent_dn = ?
	`

	rows, err := s.db.QueryContext(ctx, query, dn)
	if err != nil {
		return nil, fmt.Errorf("failed to get children: %w", err)
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		entry := &models.Entry{Attributes: make(map[string][]string)}
		if err := rows.Scan(
			&entry.ID,
			&entry.DN,
			&entry.ParentDN,
			&entry.ObjectClass,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		// Load attributes
		attrQuery := `SELECT name, value FROM attributes WHERE entry_id = ?`
		attrRows, err := s.db.QueryContext(ctx, attrQuery, entry.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get attributes: %w", err)
		}

		for attrRows.Next() {
			var name, value string
			if err := attrRows.Scan(&name, &value); err != nil {
				attrRows.Close()
				return nil, fmt.Errorf("failed to scan attribute: %w", err)
			}
			entry.Attributes[strings.ToLower(name)] = append(entry.Attributes[strings.ToLower(name)], value)
		}
		attrRows.Close()

		// Add objectClass to attributes map for filter matching
		if entry.ObjectClass != "" {
			entry.Attributes["objectclass"] = []string{entry.ObjectClass}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
