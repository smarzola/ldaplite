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

	rows, err := s.db.QueryContext(ctx, query, dn)
	if err != nil {
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}
	defer rows.Close()

	entries, err := scanEntriesWithAttributes(rows)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	entry := entries[0]

	// Add memberOf attribute for user entries
	if err := s.populateMemberOf(ctx, []*models.Entry{entry}); err != nil {
		return nil, fmt.Errorf("failed to populate memberOf: %w", err)
	}

	return entry, nil
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
		return classifyModelValidationError(err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.validateEntryPlacement(ctx, tx, entry); err != nil {
		return err
	}

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
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("%w: %s", ErrEntryAlreadyExists, entry.DN)
		}
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
	// All other attributes (uid, cn, ou) are in attributes table with indexes for:
	// - exact lookup by stored name/value
	// - per-entry case-insensitive lookup via expression indexes on LOWER(name/value)
	//
	// Result: Zero storage redundancy while maintaining query performance
	if entry.IsUser() {
		// Validate user-specific requirements
		user := &models.User{Entry: entry, UID: entry.GetAttribute("uid")}
		if err := user.ValidateUser(); err != nil {
			return classifyModelValidationError(err)
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
			return classifyModelValidationError(err)
		}
		// Groups table stores only entry_id (needed for group_members FK)
		groupQuery := `INSERT INTO groups (entry_id) VALUES (?)`
		if _, err := tx.ExecContext(ctx, groupQuery, entryID); err != nil {
			return fmt.Errorf("failed to create group entry: %w", err)
		}

		// Sync group_members table with member attributes for referential integrity
		// This allows efficient group membership queries and supports future nested group features
		if err := syncGroupMembers(ctx, tx, entryID, entry.DN, entry.GetAttributes("member"), false); err != nil {
			return err
		}
	} else if entry.IsOrganizationalUnit() {
		// Validate OU-specific requirements
		ouModel := &models.OrganizationalUnit{Entry: entry, OU: entry.GetAttribute("ou")}
		if err := ouModel.ValidateOU(); err != nil {
			return classifyModelValidationError(err)
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
		return classifyModelValidationError(err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Update entry metadata (timestamp)
	query := `UPDATE entries SET updated_at = ? WHERE dn = ?`
	result, err := tx.ExecContext(ctx, query, entry.UpdatedAt, entry.DN)
	if err != nil {
		return fmt.Errorf("failed to update entry: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to verify entry update: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: entry not found: %s", ErrNoSuchObject, entry.DN)
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

	// Step 4: Sync group_members table if this is a group
	// This keeps the junction table in sync with member attributes for efficient queries
	if entry.IsGroup() {
		// Get the entry ID
		var entryID int64
		getIDQuery := `SELECT id FROM entries WHERE dn = ?`
		if err := tx.QueryRowContext(ctx, getIDQuery, entry.DN).Scan(&entryID); err != nil {
			return fmt.Errorf("failed to get entry ID: %w", err)
		}

		if err := syncGroupMembers(ctx, tx, entryID, entry.DN, entry.GetAttributes("member"), true); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func syncGroupMembers(ctx context.Context, tx *sql.Tx, groupEntryID int64, groupDN string, memberDNs []string, replace bool) error {
	if len(memberDNs) == 0 {
		if replace {
			if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_entry_id = ?`, groupEntryID); err != nil {
				return fmt.Errorf("failed to delete group members: %w", err)
			}
		}
		return nil
	}

	memberEntryIDs := make([]int64, 0, len(memberDNs))
	for _, memberDN := range memberDNs {
		var memberEntryID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM entries WHERE dn = ?`, memberDN).Scan(&memberEntryID)
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: group member does not exist: %s", ErrConstraintViolation, memberDN)
		}
		if err != nil {
			return fmt.Errorf("failed to verify group member %s: %w", memberDN, err)
		}
		memberEntryIDs = append(memberEntryIDs, memberEntryID)
	}

	if replace {
		if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_entry_id = ?`, groupEntryID); err != nil {
			return fmt.Errorf("failed to delete group members: %w", err)
		}
	}

	for i, memberEntryID := range memberEntryIDs {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO group_members (group_entry_id, member_entry_id) VALUES (?, ?)`,
			groupEntryID,
			memberEntryID,
		); err != nil {
			return fmt.Errorf("failed to add member %s to group %s: %w", memberDNs[i], groupDN, err)
		}
	}

	return nil
}

func (s *SQLiteStore) validateEntryPlacement(ctx context.Context, tx *sql.Tx, entry *models.Entry) error {
	baseDN := strings.TrimSpace(s.cfg.LDAP.BaseDN)
	if baseDN == "" {
		return fmt.Errorf("base DN is not configured")
	}
	if !dnWithinBase(entry.DN, baseDN) {
		return fmt.Errorf("entry DN %s is outside base DN %s", entry.DN, baseDN)
	}

	if strings.EqualFold(entry.DN, baseDN) {
		if entry.ParentDN != "" {
			return fmt.Errorf("base DN entry must not have parent DN: %s", entry.ParentDN)
		}
		return nil
	}

	if entry.ParentDN == "" {
		return fmt.Errorf("%w: parent DN is required for entry: %s", ErrNoSuchObject, entry.DN)
	}

	exists, err := entryExistsTx(ctx, tx, entry.ParentDN)
	if err != nil {
		return fmt.Errorf("failed to verify parent DN: %w", err)
	}
	if !exists {
		return fmt.Errorf("%w: parent DN does not exist: %s", ErrNoSuchObject, entry.ParentDN)
	}

	return nil
}

func entryExistsTx(ctx context.Context, tx *sql.Tx, dn string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM entries WHERE dn = ?)`
	var exists bool
	if err := tx.QueryRowContext(ctx, query, dn).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func dnWithinBase(dn, baseDN string) bool {
	dn = strings.TrimSpace(dn)
	baseDN = strings.TrimSpace(baseDN)
	return strings.EqualFold(dn, baseDN) || strings.HasSuffix(strings.ToLower(dn), ","+strings.ToLower(baseDN))
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
		return fmt.Errorf("%w: entry not found: %s", ErrNoSuchObject, dn)
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
	return s.SearchEntriesWithOptions(ctx, SearchOptions{
		BaseDN:          baseDN,
		Filter:          filterStr,
		Scope:           SearchScopeWholeSubtree,
		IncludeMemberOf: true,
	})
}

// SearchEntriesWithOptions searches for entries matching a filter and LDAP scope.
func (s *SQLiteStore) SearchEntriesWithOptions(ctx context.Context, options SearchOptions) ([]*models.Entry, error) {
	filterStr := options.Filter
	if filterStr == "" {
		filterStr = "(objectClass=*)"
	}

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

	query, args := searchEntriesQuery(options.Scope, filterClause, options.BaseDN, filterArgs)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search entries: %w", err)
	}
	defer rows.Close()

	// First pass: collect all entries from SQL query
	allEntries, err := scanEntriesWithAttributes(rows)
	if err != nil {
		return nil, err
	}

	// Optimization: Order of operations depends on filter requirements
	// - If filter uses computed attributes (memberOf): populate first, then filter
	// - If filter doesn't use computed attributes: filter first, then populate
	// This reduces work when non-memberOf filters significantly reduce the result set
	var entries []*models.Entry

	filterUsesComputed := schema.FilterUsesComputedAttributes(parsedFilter)

	if useInMemoryFilter {
		if filterUsesComputed {
			// Filter needs memberOf → populate first, then filter
			if err := s.populateMemberOf(ctx, allEntries); err != nil {
				return nil, fmt.Errorf("failed to populate memberOf: %w", err)
			}
			for _, entry := range allEntries {
				if parsedFilter.Matches(entry) {
					entries = append(entries, entry)
				}
			}
		} else {
			// Filter doesn't need memberOf → filter first (optimization!)
			// This reduces the number of entries we need to populate memberOf for
			for _, entry := range allEntries {
				if parsedFilter.Matches(entry) {
					entries = append(entries, entry)
				}
			}
			// Now populate memberOf only for filtered entries
			if options.IncludeMemberOf {
				if err := s.populateMemberOf(ctx, entries); err != nil {
					return nil, fmt.Errorf("failed to populate memberOf: %w", err)
				}
			}
		}
	} else {
		// No in-memory filter needed - all entries pass.
		if options.IncludeMemberOf {
			if err := s.populateMemberOf(ctx, allEntries); err != nil {
				return nil, fmt.Errorf("failed to populate memberOf: %w", err)
			}
		}
		entries = allEntries
	}

	if filterUsesComputed && !options.IncludeMemberOf {
		for _, entry := range entries {
			delete(entry.Attributes, "memberof")
		}
	}

	return entries, nil
}

func searchEntriesQuery(scope SearchScope, filterClause string, baseDN string, filterArgs []interface{}) (string, []interface{}) {
	selectClause := `
		SELECT
			e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at,
			json_group_array(
				CASE WHEN a.name IS NOT NULL
				THEN json_object('name', a.name, 'value', a.value)
				ELSE NULL END
			) as attributes_json
	`
	joinWhere := `
		LEFT JOIN attributes a ON e.id = a.entry_id
		WHERE (` + filterClause + `)
	`
	groupBy := `
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
	`

	switch scope {
	case SearchScopeBaseObject:
		args := append([]interface{}{}, filterArgs...)
		args = append(args, baseDN)
		return selectClause + `
		FROM entries e
	` + joinWhere + `
		  AND e.dn = ?
	` + groupBy, args
	case SearchScopeSingleLevel:
		args := append([]interface{}{}, filterArgs...)
		args = append(args, baseDN)
		return selectClause + `
		FROM entries e
	` + joinWhere + `
		  AND e.parent_dn = ?
	` + groupBy, args
	default:
		args := []interface{}{baseDN}
		args = append(args, filterArgs...)
		// Recursive CTE for subtree traversal. This avoids leading % LIKE
		// patterns and uses the parent_dn index for each level.
		return `
		WITH RECURSIVE subtree AS (
			SELECT id, dn, parent_dn, object_class, created_at, updated_at, 0 as depth
			FROM entries
			WHERE dn = ?

			UNION ALL

			SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at, s.depth + 1
			FROM entries e
			INNER JOIN subtree s ON e.parent_dn = s.dn
			WHERE s.depth < 100
		)
	` + selectClause + `
		FROM subtree e
	` + joinWhere + groupBy, args
	}
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

	entries, err := scanEntriesWithAttributes(rows)
	if err != nil {
		return nil, err
	}

	// Add memberOf attribute for user entries
	if err := s.populateMemberOf(ctx, entries); err != nil {
		return nil, fmt.Errorf("failed to populate memberOf: %w", err)
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

	entries, err := scanEntriesWithAttributes(rows)
	if err != nil {
		return nil, err
	}

	// Add memberOf attribute for user entries
	if err := s.populateMemberOf(ctx, entries); err != nil {
		return nil, fmt.Errorf("failed to populate memberOf: %w", err)
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
func (s *SQLiteStore) GetUserPasswordHash(ctx context.Context, uid string) (string, string, error) {
	// Join entries, attributes, and users tables to get both password_hash and DN
	// The WHERE clause uses idx_attributes_uid_lookup index for fast uid lookup
	query := `
		SELECT u.password_hash, e.dn
		FROM users u
		INNER JOIN entries e ON u.entry_id = e.id
		INNER JOIN attributes a ON u.entry_id = a.entry_id
		WHERE a.name = 'uid' AND a.value = ?
		LIMIT 1
	`
	var passwordHash, dn string
	err := s.db.QueryRowContext(ctx, query, uid).Scan(&passwordHash, &dn)
	if err == sql.ErrNoRows {
		return "", "", nil // User not found - return empty strings, not an error
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to get user password hash: %w", err)
	}
	return passwordHash, dn, nil
}

// IsUserInGroup checks if a user is a member of a group, including membership
// through nested groups. A recursive CTE walks from the user's direct groups up
// through parent groups with cycle protection.
func (s *SQLiteStore) IsUserInGroup(ctx context.Context, userDN, groupDN string) (bool, error) {
	query := `
		WITH RECURSIVE user_groups(group_id, depth, path) AS (
			-- Direct groups containing the user
			SELECT gm.group_entry_id, 0, printf(',%d,', gm.group_entry_id)
			FROM group_members gm
			INNER JOIN entries user_entry ON gm.member_entry_id = user_entry.id
			WHERE user_entry.dn = ?

			UNION ALL

			-- Parent groups containing one of the user's groups
			SELECT gm.group_entry_id, ug.depth + 1, ug.path || gm.group_entry_id || ','
			FROM group_members gm
			INNER JOIN user_groups ug ON gm.member_entry_id = ug.group_id
			WHERE ug.depth < 100
			  AND instr(ug.path, printf(',%d,', gm.group_entry_id)) = 0
		)
		SELECT EXISTS(
			SELECT 1
			FROM user_groups ug
			INNER JOIN entries group_entry ON ug.group_id = group_entry.id
			WHERE group_entry.dn = ?
		)
	`
	var isMember bool
	err := s.db.QueryRowContext(ctx, query, userDN, groupDN).Scan(&isMember)
	if err != nil {
		return false, fmt.Errorf("failed to check group membership: %w", err)
	}
	return isMember, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// populateMemberOf adds the memberOf attribute to user entries (inetOrgPerson).
// This is a virtual attribute computed from the group_members table. It
// includes direct and nested group memberships with cycle protection.
//
// This function efficiently batches the lookup to minimize database queries:
// 1. Collect all user entry IDs
// 2. Single query to get all group memberships for those users
// 3. Populate memberOf attribute for each user entry
func (s *SQLiteStore) populateMemberOf(ctx context.Context, entries []*models.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	// Collect user entry IDs
	userEntryIDs := make([]int64, 0)
	userEntriesByID := make(map[int64]*models.Entry)
	for _, entry := range entries {
		if entry.IsUser() && entry.ID > 0 {
			userEntryIDs = append(userEntryIDs, entry.ID)
			userEntriesByID[entry.ID] = entry
		}
	}

	if len(userEntryIDs) == 0 {
		return nil
	}

	// Build query with placeholders for user entry IDs
	placeholders := make([]string, len(userEntryIDs))
	args := make([]interface{}, len(userEntryIDs))
	for i, id := range userEntryIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	// Query all direct and nested group memberships for these users.
	// Returns: user entry_id, group_dn.
	query := `
		WITH RECURSIVE memberships(user_id, group_id, group_dn, depth, path) AS (
			-- Direct groups containing each user
			SELECT gm.member_entry_id, gm.group_entry_id, g_entry.dn, 0, printf(',%d,', gm.group_entry_id)
			FROM group_members gm
			INNER JOIN entries g_entry ON gm.group_entry_id = g_entry.id
			WHERE gm.member_entry_id IN (` + strings.Join(placeholders, ",") + `)

			UNION ALL

			-- Parent groups containing one of the user's groups
			SELECT m.user_id, gm.group_entry_id, g_entry.dn, m.depth + 1, m.path || gm.group_entry_id || ','
			FROM memberships m
			INNER JOIN group_members gm ON gm.member_entry_id = m.group_id
			INNER JOIN entries g_entry ON gm.group_entry_id = g_entry.id
			WHERE m.depth < 100
			  AND instr(m.path, printf(',%d,', gm.group_entry_id)) = 0
		)
		SELECT DISTINCT user_id, group_dn
		FROM memberships
	`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to query group memberships: %w", err)
	}
	defer rows.Close()

	// Populate memberOf attribute for each user
	for rows.Next() {
		var memberEntryID int64
		var groupDN string
		if err := rows.Scan(&memberEntryID, &groupDN); err != nil {
			return fmt.Errorf("failed to scan group membership: %w", err)
		}

		if entry, ok := userEntriesByID[memberEntryID]; ok {
			entry.AddAttribute("memberOf", groupDN)
		}
	}

	return rows.Err()
}
