package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/smarzola/ldaplite/internal/models"
)

// attrPair represents a single attribute name-value pair for JSON encoding
type attrPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// decodeAttributesJSON decodes a JSON array of {name, value} pairs into a map
// of attribute names to their values. Handles NULL values and empty arrays.
//
// Input format: [{"name":"cn","value":"John Doe"},{"name":"mail","value":"john@example.com"}]
// Output format: map[string][]string{"cn": {"John Doe"}, "mail": {"john@example.com"}}
func decodeAttributesJSON(jsonStr string) (map[string][]string, error) {
	attrs := make(map[string][]string)

	// Handle empty, null, or [null] cases
	if jsonStr == "" || jsonStr == "null" || jsonStr == "[null]" {
		return attrs, nil
	}

	var pairs []attrPair
	if err := json.Unmarshal([]byte(jsonStr), &pairs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attributes JSON: %w", err)
	}

	// Convert pairs to map, grouping multi-valued attributes
	for _, p := range pairs {
		// Skip null/empty entries (from LEFT JOIN with no attributes)
		if p.Name == "" {
			continue
		}

		name := strings.ToLower(p.Name)
		attrs[name] = append(attrs[name], p.Value)
	}

	return attrs, nil
}

func scanEntriesWithAttributes(rows *sql.Rows) ([]*models.Entry, error) {
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

		attrs, err := decodeAttributesJSON(attrsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
		}
		entry.Attributes = attrs

		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan entries: %w", err)
	}
	return entries, nil
}

func isSQLiteUniqueConstraint(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}

func (s *SQLiteStore) queryEntriesWithAttributes(ctx context.Context, operation string, query string, args ...interface{}) ([]*models.Entry, error) {
	return s.queryEntriesWithAttributesOptions(ctx, operation, true, query, args...)
}

func (s *SQLiteStore) queryEntriesWithAttributesOptions(ctx context.Context, operation string, includeMemberOf bool, query string, args ...interface{}) ([]*models.Entry, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to %s: %w", operation, err)
	}
	defer rows.Close()

	entries, err := scanEntriesWithAttributes(rows)
	if err != nil {
		return nil, err
	}

	if includeMemberOf {
		if err := s.populateMemberOf(ctx, entries); err != nil {
			return nil, fmt.Errorf("failed to populate memberOf: %w", err)
		}
	}

	return entries, nil
}
