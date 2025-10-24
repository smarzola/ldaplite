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
	query := `
		SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
		FROM entries e
		INNER JOIN users u ON e.id = u.entry_id
		WHERE u.uid = ?
	`

	var entry models.Entry
	err := s.db.QueryRowContext(ctx, query, uid).Scan(
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
		return nil, fmt.Errorf("failed to get user by UID: %w", err)
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
		entry.Attributes[name] = append(entry.Attributes[name], value)
	}

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
