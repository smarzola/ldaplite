package store

import (
	"context"
	"fmt"

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
	// Use JSON aggregation to fetch OUs with attributes in a single query
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
		WHERE (e.dn = ? OR e.parent_dn LIKE ?)
		AND e.object_class = ?
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
	`

	likePattern := "%" + baseDN
	rows, err := s.db.QueryContext(ctx, query, baseDN, likePattern, string(models.ObjectClassOrganizationalUnit))
	if err != nil {
		return nil, fmt.Errorf("failed to search OUs: %w", err)
	}
	defer rows.Close()

	var ous []*models.OrganizationalUnit
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

		ou := &models.OrganizationalUnit{
			Entry: entry,
			OU:    entry.GetAttribute("ou"),
		}
		ous = append(ous, ou)
	}

	return ous, nil
}
