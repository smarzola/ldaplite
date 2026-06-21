package store

import (
	"context"
	"database/sql"
	"fmt"
)

// GetUserPasswordHash retrieves the password hash for a user by UID.
//
// SECURITY: This method provides controlled access to password hashes for authentication only.
// Password hashes are stored exclusively in users.password_hash and are NEVER:
// - Stored in the attributes table
// - Returned in LDAP search operations
// - Accessible through generic GetEntry/SearchEntries methods
//
// This isolation ensures passwords cannot be accidentally exposed via LDAP queries.
// Only authentication operations should call this method.
//
// Uses optimized index (idx_attributes_uid_lookup) for fast uid lookup,
// then joins to users table for password retrieval.
func (s *SQLiteStore) GetUserPasswordHash(ctx context.Context, uid string) (string, string, error) {
	query := `
		SELECT u.password_hash, e.dn
		FROM users u
		INNER JOIN entries e ON u.entry_id = e.id
		INNER JOIN attributes a ON u.entry_id = a.entry_id
		WHERE a.name = 'uid' AND a.value = ?
		LIMIT 1
	`
	return s.queryPasswordHash(ctx, "get user password hash", query, uid)
}

// GetUserPasswordHashByDN retrieves the password hash for a user by bind DN.
//
// LDAP bind receives a DN, not a uid. Looking up by DN avoids ambiguity when
// identical uid values exist in different subtrees and returns the stored DN so
// callers can bind the canonical value onto the connection.
func (s *SQLiteStore) GetUserPasswordHashByDN(ctx context.Context, dn string) (string, string, error) {
	query := `
		SELECT u.password_hash, e.dn
		FROM users u
		INNER JOIN entries e ON u.entry_id = e.id
		WHERE LOWER(e.dn) = LOWER(?)
		LIMIT 1
	`
	return s.queryPasswordHash(ctx, "get user password hash by DN", query, dn)
}

func (s *SQLiteStore) queryPasswordHash(ctx context.Context, operation string, query string, arg string) (string, string, error) {
	var passwordHash, dn string
	err := s.db.QueryRowContext(ctx, query, arg).Scan(&passwordHash, &dn)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to %s: %w", operation, err)
	}
	return passwordHash, dn, nil
}
