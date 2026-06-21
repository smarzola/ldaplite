package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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

	entries, err := s.queryEntriesWithAttributes(ctx, "get entry", query, dn)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	return entries[0], nil
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

	return s.queryEntriesWithAttributes(ctx, "get all entries", query)
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

	return s.queryEntriesWithAttributes(ctx, "get children", query, dn)
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
