package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/smarzola/ldaplite/internal/models"
)

// GetUser retrieves a user by DN
func (s *SQLiteStore) GetUser(ctx context.Context, dn string) (*models.User, error) {
	entry, err := s.GetEntry(ctx, dn)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	if !entry.IsUser() {
		return nil, fmt.Errorf("entry is not a user: %s", dn)
	}

	uid := entry.GetAttribute("uid")
	if uid == "" {
		return nil, fmt.Errorf("user missing uid attribute: %s", dn)
	}

	user := &models.User{
		Entry:    entry,
		UID:      uid,
		Password: entry.GetAttribute("userPassword"),
	}

	return user, nil
}

// GetUserByUID retrieves a user by UID
func (s *SQLiteStore) GetUserByUID(ctx context.Context, uid string) (*models.User, error) {
	// Use JSON aggregation to fetch user with attributes in a single query
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
		INNER JOIN users u ON e.id = u.entry_id
		LEFT JOIN attributes a ON e.id = a.entry_id
		WHERE u.uid = ?
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
	`

	var entry models.Entry
	var attrsJSON string

	err := s.db.QueryRowContext(ctx, query, uid).Scan(
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
		return nil, fmt.Errorf("failed to get user by UID: %w", err)
	}

	// Decode attributes from JSON
	entry.Attributes, err = decodeAttributesJSON(attrsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
	}

	// Add operational attributes (objectClass, timestamps)
	entry.AddOperationalAttributes()

	user := &models.User{
		Entry:    &entry,
		UID:      uid,
		Password: entry.GetAttribute("userPassword"),
	}

	return user, nil
}

// CreateUser creates a new user
func (s *SQLiteStore) CreateUser(ctx context.Context, user *models.User) error {
	if err := user.ValidateUser(); err != nil {
		return err
	}

	return s.CreateEntry(ctx, user.Entry)
}

// UpdateUser updates an existing user
func (s *SQLiteStore) UpdateUser(ctx context.Context, user *models.User) error {
	if err := user.ValidateUser(); err != nil {
		return err
	}

	return s.UpdateEntry(ctx, user.Entry)
}

// DeleteUser deletes a user
func (s *SQLiteStore) DeleteUser(ctx context.Context, dn string) error {
	return s.DeleteEntry(ctx, dn)
}

// SearchUsers searches for users matching a filter
func (s *SQLiteStore) SearchUsers(ctx context.Context, baseDN string, filter string) ([]*models.User, error) {
	entries, err := s.SearchEntries(ctx, baseDN, filter)
	if err != nil {
		return nil, err
	}

	var users []*models.User
	for _, entry := range entries {
		if entry.IsUser() {
			user := &models.User{
				Entry:    entry,
				UID:      entry.GetAttribute("uid"),
				Password: entry.GetAttribute("userPassword"),
			}
			users = append(users, user)
		}
	}

	return users, nil
}
