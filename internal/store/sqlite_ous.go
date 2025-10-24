package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/smarzola/ldaplite/internal/models"
)

// GetOU retrieves an OU by DN
func (s *SQLiteStore) GetOU(ctx context.Context, dn string) (*models.OrganizationalUnit, error) {
	entry, err := s.GetEntry(ctx, dn)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	if !entry.IsOrganizationalUnit() {
		return nil, fmt.Errorf("entry is not an OU: %s", dn)
	}

	ou := entry.GetAttribute("ou")
	if ou == "" {
		return nil, fmt.Errorf("OU missing ou attribute: %s", dn)
	}

	ouEntry := &models.OrganizationalUnit{
		Entry: entry,
		OU:    ou,
	}

	return ouEntry, nil
}

// CreateOU creates a new OU
func (s *SQLiteStore) CreateOU(ctx context.Context, ou *models.OrganizationalUnit) error {
	if err := ou.ValidateOU(); err != nil {
		return err
	}

	return s.CreateEntry(ctx, ou.Entry)
}

// UpdateOU updates an existing OU
func (s *SQLiteStore) UpdateOU(ctx context.Context, ou *models.OrganizationalUnit) error {
	if err := ou.ValidateOU(); err != nil {
		return err
	}

	return s.UpdateEntry(ctx, ou.Entry)
}

// DeleteOU deletes an OU
func (s *SQLiteStore) DeleteOU(ctx context.Context, dn string) error {
	return s.DeleteEntry(ctx, dn)
}

// SearchOUs searches for OUs
func (s *SQLiteStore) SearchOUs(ctx context.Context, baseDN string) ([]*models.OrganizationalUnit, error) {
	query := `
		SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
		FROM entries e
		WHERE (e.dn = ? OR e.parent_dn LIKE ?)
		AND e.object_class = ?
	`

	likePattern := "%" + baseDN
	rows, err := s.db.QueryContext(ctx, query, baseDN, likePattern, string(models.ObjectClassOrganizationalUnit))
	if err != nil {
		return nil, fmt.Errorf("failed to search OUs: %w", err)
	}
	defer rows.Close()

	var ous []*models.OrganizationalUnit
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

		ou := &models.OrganizationalUnit{
			Entry: entry,
			OU:    entry.GetAttribute("ou"),
		}
		ous = append(ous, ou)
	}

	return ous, nil
}
