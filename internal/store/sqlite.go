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
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/schema"
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

	// Run migrations from embedded filesystem
	srcDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	dbURL := fmt.Sprintf("sqlite://%s", s.cfg.Database.Path)
	m, err := migrate.NewWithSourceInstance("iofs", srcDriver, dbURL)
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
		if err := s.CreateEntry(ctx, ouEntry.Entry); err != nil {
			return fmt.Errorf("failed to create OU %s: %w", ou.name, err)
		}
		slog.Info("Created OU", "dn", ouEntry.DN)
	}

	// Create admin user
	adminUser := models.NewUser(baseDN, "admin", "Administrator", "Administrator", "admin@example.com")
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
	// Use JSON aggregation to fetch entry with attributes in a single query
	query := `
		SELECT
			e.id,
			e.dn,
			e.parent_dn,
			e.object_class,
			e.created_at,
			e.updated_at,
			json_group_array(
				CASE WHEN a.name IS NOT NULL
				THEN json_object('name', a.name, 'value', a.value)
				ELSE NULL END
			) as attributes_json
		FROM entries e
		LEFT JOIN attributes a ON e.id = a.entry_id
		WHERE e.dn = ?
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
	`

	var entry models.Entry
	var attrsJSON string

	err := s.db.QueryRowContext(ctx, query, dn).Scan(
		&entry.ID,
		&entry.DN,
		&entry.ParentDN,
		&entry.ObjectClass,
		&entry.CreatedAt,
		&entry.UpdatedAt,
		&attrsJSON,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}

	// Decode attributes from JSON
	entry.Attributes, err = decodeAttributesJSON(attrsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
	}

	// Add operational attributes (objectClass, timestamps)
	entry.AddOperationalAttributes()

	return &entry, nil
}

// CreateEntry creates a new entry using the dual-storage architecture:
//
// 1. Core entry metadata → entries table (DN, object class, timestamps)
// 2. Generic attributes → attributes table (EAV pattern for flexibility)
// 3. Type-specific data → specialized tables (users, groups, OUs for performance)
//
// SECURITY: userPassword is NEVER stored in attributes table, only in users.password_hash
// CONSISTENCY: Attributes like uid, cn, ou are duplicated in specialized tables for query performance
func (s *SQLiteStore) CreateEntry(ctx context.Context, entry *models.Entry) error {
	if err := entry.Validate(); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Insert core entry metadata into entries table
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

	// Step 2: Insert generic attributes into attributes table (EAV pattern)
	// This provides flexible, schema-free storage for all LDAP attributes
	// SECURITY EXCEPTION: userPassword is excluded and stored only in users.password_hash
	attrQuery := `INSERT INTO attributes (entry_id, name, value) VALUES (?, ?, ?)`
	for name, values := range entry.Attributes {
		// Skip userPassword - stored securely in users.password_hash only (never exposed in searches)
		if strings.EqualFold(name, "userPassword") {
			continue
		}
		for _, value := range values {
			if _, err := tx.ExecContext(ctx, attrQuery, entryID, name, value); err != nil {
				return fmt.Errorf("failed to insert attribute: %w", err)
			}
		}
	}

	// Step 3: Insert type-specific data into specialized tables
	// Phase 3 optimization: Specialized tables contain ONLY essential data:
	// - users: entry_id + password_hash (security-sensitive, never in attributes)
	// - groups: entry_id (needed for group_members foreign key relationships)
	// - organizational_units: entry_id (referential integrity marker)
	//
	// All other attributes (uid, cn, ou) are in attributes table with optimized indexes:
	// - idx_attributes_uid_lookup on (name, value) WHERE name = 'uid'
	// - idx_attributes_cn_lookup on (name, value) WHERE name = 'cn'
	// - idx_attributes_ou_lookup on (name, value) WHERE name = 'ou'
	//
	// Result: Zero storage redundancy while maintaining query performance
	if entry.IsUser() {
		// Validate user-specific requirements
		user := &models.User{Entry: entry, UID: entry.GetAttribute("uid")}
		if err := user.ValidateUser(); err != nil {
			return err
		}
		// Users table stores only password_hash (security-sensitive data)
		passwordHash := entry.GetAttribute("userPassword")
		userQuery := `INSERT INTO users (entry_id, password_hash) VALUES (?, ?)`
		if _, err := tx.ExecContext(ctx, userQuery, entryID, passwordHash); err != nil {
			return fmt.Errorf("failed to create user entry: %w", err)
		}
	} else if entry.IsGroup() {
		// Validate group-specific requirements
		group := &models.Group{Entry: entry, CN: entry.GetAttribute("cn")}
		if err := group.ValidateGroup(); err != nil {
			return err
		}
		// Groups table stores only entry_id (needed for group_members FK)
		groupQuery := `INSERT INTO groups (entry_id) VALUES (?)`
		if _, err := tx.ExecContext(ctx, groupQuery, entryID); err != nil {
			return fmt.Errorf("failed to create group entry: %w", err)
		}
	} else if entry.IsOrganizationalUnit() {
		// Validate OU-specific requirements
		ouModel := &models.OrganizationalUnit{Entry: entry, OU: entry.GetAttribute("ou")}
		if err := ouModel.ValidateOU(); err != nil {
			return err
		}
		// OUs table stores only entry_id (referential integrity marker)
		ouQuery := `INSERT INTO organizational_units (entry_id) VALUES (?)`
		if _, err := tx.ExecContext(ctx, ouQuery, entryID); err != nil {
			return fmt.Errorf("failed to create OU entry: %w", err)
		}
	}

	return tx.Commit()
}

// UpdateEntry updates an existing entry while maintaining dual-storage consistency:
//
// 1. Update timestamp in entries table
// 2. Replace all attributes in attributes table (delete + insert pattern)
// 3. Update password in users.password_hash if changed (security isolation)
//
// SECURITY: userPassword is NEVER written to attributes table, only to users.password_hash
// CONSISTENCY: Specialized table data (uid, cn, ou) remains in sync via initial CreateEntry
func (s *SQLiteStore) UpdateEntry(ctx context.Context, entry *models.Entry) error {
	if err := entry.Validate(); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Update entry metadata (timestamp)
	query := `UPDATE entries SET updated_at = ? WHERE dn = ?`
	if _, err := tx.ExecContext(ctx, query, entry.UpdatedAt, entry.DN); err != nil {
		return fmt.Errorf("failed to update entry: %w", err)
	}

	// Step 2: Replace attributes in attributes table (delete-then-insert pattern)
	// This is simpler than diffing changes and ensures consistency
	delAttrQuery := `DELETE FROM attributes WHERE entry_id = (SELECT id FROM entries WHERE dn = ?)`
	if _, err := tx.ExecContext(ctx, delAttrQuery, entry.DN); err != nil {
		return fmt.Errorf("failed to delete attributes: %w", err)
	}

	// Insert updated attributes (excluding security-sensitive attributes)
	insertAttrQuery := `INSERT INTO attributes (entry_id, name, value) VALUES ((SELECT id FROM entries WHERE dn = ?), ?, ?)`
	for name, values := range entry.Attributes {
		// Skip userPassword - stored securely in users.password_hash only (never in attributes)
		if strings.EqualFold(name, "userPassword") {
			continue
		}
		for _, value := range values {
			if _, err := tx.ExecContext(ctx, insertAttrQuery, entry.DN, name, value); err != nil {
				return fmt.Errorf("failed to insert attribute: %w", err)
			}
		}
	}

	// Step 3: Update password in specialized users table if changed
	// This maintains security isolation - password never touches attributes table
	if entry.IsUser() {
		passwordHash := entry.GetAttribute("userPassword")
		if passwordHash != "" {
			updatePasswordQuery := `UPDATE users SET password_hash = ? WHERE entry_id = (SELECT id FROM entries WHERE dn = ?)`
			if _, err := tx.ExecContext(ctx, updatePasswordQuery, passwordHash, entry.DN); err != nil {
				return fmt.Errorf("failed to update user password: %w", err)
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
func (s *SQLiteStore) SearchEntries(ctx context.Context, baseDN string, filterStr string) ([]*models.Entry, error) {
	// Parse the LDAP filter
	parsedFilter, err := schema.ParseFilter(filterStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse filter: %w", err)
	}

	// Try to compile filter to SQL (hybrid approach)
	compiler := schema.NewFilterCompiler()
	var filterClause string
	var filterArgs []interface{}
	var useInMemoryFilter bool

	if compiler.CanCompileToSQL(parsedFilter) {
		// Compile filter to SQL WHERE clause
		filterClause, filterArgs, err = compiler.CompileToSQL(parsedFilter)
		if err != nil {
			// If compilation fails, fall back to in-memory filtering
			filterClause = "1=1"
			filterArgs = nil
			useInMemoryFilter = true
		} else {
			useInMemoryFilter = false
		}
	} else {
		// Filter not supported in SQL, use in-memory filtering
		filterClause = "1=1"
		filterArgs = nil
		useInMemoryFilter = true
	}

	// Build query with recursive CTE for hierarchy traversal
	// This avoids the leading % LIKE pattern that prevents index usage
	// Maximum depth of 100 prevents infinite recursion from circular references
	query := `
		WITH RECURSIVE subtree AS (
			-- Base case: exact DN match
			SELECT id, dn, parent_dn, object_class, created_at, updated_at, 0 as depth
			FROM entries
			WHERE dn = ?

			UNION ALL

			-- Recursive case: children (uses index on parent_dn = ?)
			SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at, s.depth + 1
			FROM entries e
			INNER JOIN subtree s ON e.parent_dn = s.dn
			WHERE s.depth < 100
		)
		SELECT
			e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at,
			json_group_array(
				CASE WHEN a.name IS NOT NULL
				THEN json_object('name', a.name, 'value', a.value)
				ELSE NULL END
			) as attributes_json
		FROM subtree e
		LEFT JOIN attributes a ON e.id = a.entry_id
		WHERE (` + filterClause + `)
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
	`

	// Args: baseDN for CTE + filter args
	args := []interface{}{baseDN}
	args = append(args, filterArgs...)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search entries: %w", err)
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		entry := &models.Entry{}
		var attrsJSON string

		if err := rows.Scan(
			&entry.ID,
			&entry.DN,
			&entry.ParentDN,
			&entry.ObjectClass,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&attrsJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		// Decode attributes from JSON
		entry.Attributes, err = decodeAttributesJSON(attrsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
		}

		// Add operational attributes (objectClass, timestamps)
		entry.AddOperationalAttributes()

		// Apply in-memory filter if needed (fallback for unsupported filters)
		if useInMemoryFilter {
			if !parsedFilter.Matches(entry) {
				continue
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// GetAllEntries returns all entries
func (s *SQLiteStore) GetAllEntries(ctx context.Context) ([]*models.Entry, error) {
	// Use JSON aggregation to fetch entries with attributes in a single query
	query := `
		SELECT
			e.id,
			e.dn,
			e.parent_dn,
			e.object_class,
			e.created_at,
			e.updated_at,
			json_group_array(
				CASE WHEN a.name IS NOT NULL
				THEN json_object('name', a.name, 'value', a.value)
				ELSE NULL END
			) as attributes_json
		FROM entries e
		LEFT JOIN attributes a ON e.id = a.entry_id
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all entries: %w", err)
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		entry := &models.Entry{}
		var attrsJSON string

		if err := rows.Scan(
			&entry.ID,
			&entry.DN,
			&entry.ParentDN,
			&entry.ObjectClass,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&attrsJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		// Decode attributes from JSON
		entry.Attributes, err = decodeAttributesJSON(attrsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
		}

		// Add operational attributes (objectClass, timestamps)
		entry.AddOperationalAttributes()

		entries = append(entries, entry)
	}

	return entries, nil
}

// GetChildren returns all children of a given DN
func (s *SQLiteStore) GetChildren(ctx context.Context, dn string) ([]*models.Entry, error) {
	// Use JSON aggregation to fetch entries with attributes in a single query
	query := `
		SELECT
			e.id,
			e.dn,
			e.parent_dn,
			e.object_class,
			e.created_at,
			e.updated_at,
			json_group_array(
				CASE WHEN a.name IS NOT NULL
				THEN json_object('name', a.name, 'value', a.value)
				ELSE NULL END
			) as attributes_json
		FROM entries e
		LEFT JOIN attributes a ON e.id = a.entry_id
		WHERE e.parent_dn = ?
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
	`

	rows, err := s.db.QueryContext(ctx, query, dn)
	if err != nil {
		return nil, fmt.Errorf("failed to get children: %w", err)
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		entry := &models.Entry{}
		var attrsJSON string

		if err := rows.Scan(
			&entry.ID,
			&entry.DN,
			&entry.ParentDN,
			&entry.ObjectClass,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&attrsJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		// Decode attributes from JSON
		entry.Attributes, err = decodeAttributesJSON(attrsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
		}

		// Add operational attributes (objectClass, timestamps)
		entry.AddOperationalAttributes()

		entries = append(entries, entry)
	}

	return entries, nil
}

// GetUserPasswordHash retrieves the password hash for a user by UID.
//
// SECURITY: This method provides controlled access to password hashes for authentication only.
// Password hashes are stored exclusively in users.password_hash and are NEVER:
// - Stored in the attributes table
// - Returned in LDAP search operations
// - Accessible through generic GetEntry/SearchEntries methods
//
// This isolation ensures passwords cannot be accidentally exposed via LDAP queries.
// Only bind (authentication) operations should call this method.
//
// Phase 3: Uses optimized index (idx_attributes_uid_lookup) for fast uid lookup,
// then joins to users table for password retrieval.
func (s *SQLiteStore) GetUserPasswordHash(ctx context.Context, uid string) (string, error) {
	// Join attributes table (for uid lookup) with users table (for password_hash)
	// The WHERE clause uses idx_attributes_uid_lookup index for fast lookup
	query := `
		SELECT u.password_hash
		FROM users u
		INNER JOIN attributes a ON u.entry_id = a.entry_id
		WHERE a.name = 'uid' AND a.value = ?
		LIMIT 1
	`
	var passwordHash string
	err := s.db.QueryRowContext(ctx, query, uid).Scan(&passwordHash)
	if err == sql.ErrNoRows {
		return "", nil // User not found - return empty string, not an error
	}
	if err != nil {
		return "", fmt.Errorf("failed to get user password hash: %w", err)
	}
	return passwordHash, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
