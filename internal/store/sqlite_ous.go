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
	// Use recursive CTE for hierarchy traversal with indexed lookups
	// Maximum depth of 100 prevents infinite recursion from circular references
	query := `
		WITH RECURSIVE subtree AS (
			-- Base case: exact DN match if it's an OU
			SELECT id, dn, parent_dn, object_class, created_at, updated_at, 0 as depth
			FROM entries
			WHERE dn = ? AND object_class = ?

			UNION ALL

			-- Recursive case: find child OUs (uses index on parent_dn = ?)
			SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at, s.depth + 1
			FROM entries e
			INNER JOIN subtree s ON e.parent_dn = s.dn
			WHERE e.object_class = ? AND s.depth < 100
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
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
	`

	objectClass := string(models.ObjectClassOrganizationalUnit)
	rows, err := s.db.QueryContext(ctx, query, baseDN, objectClass, objectClass)
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
