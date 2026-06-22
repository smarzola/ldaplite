package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/smarzola/ldaplite/internal/ldapdn"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/schema"
	"github.com/smarzola/ldaplite/internal/telemetry"
)

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
func (s *SQLiteStore) SearchEntriesWithOptions(ctx context.Context, options SearchOptions) (entries []*models.Entry, err error) {
	ctx, span := telemetry.StartStoreSpan(ctx, "SearchEntriesWithOptions")
	defer func() {
		telemetry.EndStoreSpan(span, err)
	}()

	filterStr := options.Filter
	if filterStr == "" {
		filterStr = "(objectClass=*)"
	}

	// Parse the LDAP filter
	parsedFilter, err := schema.ParseFilter(filterStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse filter: %w", err)
	}

	if fastEntries, handled, fastErr := s.searchEntriesFastPath(ctx, options, parsedFilter); handled {
		return fastEntries, fastErr
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
	filterUsesComputed := schema.FilterUsesComputedAttributes(parsedFilter)

	if useInMemoryFilter {
		if filterUsesComputed {
			// Filter needs memberOf -> populate first, then filter
			if err := s.populateMemberOf(ctx, allEntries); err != nil {
				return nil, fmt.Errorf("failed to populate memberOf: %w", err)
			}
			for _, entry := range allEntries {
				if parsedFilter.Matches(entry) {
					entries = append(entries, entry)
				}
			}
		} else {
			// Filter doesn't need memberOf -> filter first, reducing projection work.
			for _, entry := range allEntries {
				if parsedFilter.Matches(entry) {
					entries = append(entries, entry)
				}
			}
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
			entry.ClearComputedAttribute("memberOf")
		}
	}

	return entries, nil
}

func (s *SQLiteStore) searchEntriesFastPath(ctx context.Context, options SearchOptions, parsedFilter *schema.Filter) ([]*models.Entry, bool, error) {
	if groupDN, ok := schema.MemberOfEqualityValue(parsedFilter); ok {
		entries, err := s.searchEntriesByMemberOfEquality(ctx, groupDN, options)
		return entries, true, err
	}

	attr, value, ok := schema.SimpleAttributeEquality(parsedFilter)
	if !ok {
		return nil, false, nil
	}

	entries, err := s.searchEntriesByAttributeEquality(ctx, attr, value, options)
	return entries, true, err
}

func (s *SQLiteStore) searchEntriesByAttributeEquality(ctx context.Context, attr, value string, options SearchOptions) ([]*models.Entry, error) {
	query := `
		SELECT
			e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at,
			a.name, a.value
		FROM attributes match
		INNER JOIN entries e ON match.entry_id = e.id
		LEFT JOIN attributes a ON e.id = a.entry_id
		WHERE LOWER(match.name) = LOWER(?)
		  AND LOWER(match.value) = LOWER(?)
		ORDER BY e.id
	`

	rows, err := s.db.QueryContext(ctx, query, attr, value)
	if err != nil {
		return nil, fmt.Errorf("failed to search entries by equality: %w", err)
	}
	defer rows.Close()

	entries, err := scanEntriesWithAttributeRows(rows)
	if err != nil {
		return nil, err
	}

	entries = filterEntriesByScope(entries, options.BaseDN, options.Scope)
	if options.IncludeMemberOf {
		if err := s.populateMemberOf(ctx, entries); err != nil {
			return nil, fmt.Errorf("failed to populate memberOf: %w", err)
		}
	}

	return entries, nil
}

func (s *SQLiteStore) searchEntriesByMemberOfEquality(ctx context.Context, groupDN string, options SearchOptions) ([]*models.Entry, error) {
	query := `
		WITH RECURSIVE members(entry_id, depth, path) AS (
			SELECT gm.member_entry_id, 0, printf(',%d,', gm.member_entry_id)
			FROM group_members gm
			INNER JOIN entries target_group ON gm.group_entry_id = target_group.id
			WHERE LOWER(target_group.dn) = LOWER(?)

			UNION ALL

			SELECT gm.member_entry_id, m.depth + 1, m.path || gm.member_entry_id || ','
			FROM members m
			INNER JOIN entries member_group ON m.entry_id = member_group.id
			INNER JOIN group_members gm ON gm.group_entry_id = member_group.id
			WHERE member_group.object_class = 'groupOfNames'
			  AND m.depth < 100
			  AND instr(m.path, printf(',%d,', gm.member_entry_id)) = 0
		),
		matched_entries AS (
			SELECT DISTINCT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
			FROM members m
			INNER JOIN entries e ON m.entry_id = e.id
			WHERE e.object_class = 'inetOrgPerson'
		)
		SELECT
			e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at,
			a.name, a.value
		FROM matched_entries e
		LEFT JOIN attributes a ON e.id = a.entry_id
		ORDER BY e.id
	`

	rows, err := s.db.QueryContext(ctx, query, groupDN)
	if err != nil {
		return nil, fmt.Errorf("failed to search entries by memberOf: %w", err)
	}
	defer rows.Close()

	entries, err := scanEntriesWithAttributeRows(rows)
	if err != nil {
		return nil, err
	}

	entries = filterEntriesByScope(entries, options.BaseDN, options.Scope)
	if options.IncludeMemberOf {
		if err := s.populateMemberOf(ctx, entries); err != nil {
			return nil, fmt.Errorf("failed to populate memberOf: %w", err)
		}
	}

	return entries, nil
}

func scanEntriesWithAttributeRows(rows *sql.Rows) ([]*models.Entry, error) {
	var entries []*models.Entry
	var current *models.Entry

	for rows.Next() {
		var id int64
		var dn string
		var parentDN string
		var objectClass string
		var createdAt time.Time
		var updatedAt time.Time
		var attrName sql.NullString
		var attrValue sql.NullString
		if err := rows.Scan(
			&id,
			&dn,
			&parentDN,
			&objectClass,
			&createdAt,
			&updatedAt,
			&attrName,
			&attrValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry attribute row: %w", err)
		}

		if current == nil || current.ID != id {
			entry := &models.Entry{
				ID:          id,
				DN:          dn,
				ParentDN:    parentDN,
				ObjectClass: objectClass,
				Attributes:  make(map[string][]string),
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			}
			entries = append(entries, entry)
			current = entry
		}
		if attrName.Valid && attrValue.Valid {
			current.Attributes[attrName.String] = append(current.Attributes[attrName.String], attrValue.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan entry attribute rows: %w", err)
	}
	return entries, nil
}

func filterEntriesByScope(entries []*models.Entry, baseDN string, scope SearchScope) []*models.Entry {
	filtered := entries[:0]
	for _, entry := range entries {
		if entryInSearchScope(entry, baseDN, scope) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func entryInSearchScope(entry *models.Entry, baseDN string, scope SearchScope) bool {
	switch scope {
	case SearchScopeBaseObject:
		return ldapdn.Equal(entry.DN, baseDN)
	case SearchScopeSingleLevel:
		return ldapdn.Equal(entry.ParentDN, baseDN)
	default:
		return ldapdn.WithinBase(entry.DN, baseDN)
	}
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
		  AND LOWER(e.dn) = LOWER(?)
	` + groupBy, args
	case SearchScopeSingleLevel:
		args := append([]interface{}{}, filterArgs...)
		args = append(args, baseDN)
		return selectClause + `
		FROM entries e
	` + joinWhere + `
		  AND LOWER(e.parent_dn) = LOWER(?)
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
			WHERE LOWER(dn) = LOWER(?)

			UNION ALL

			SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at, s.depth + 1
			FROM entries e
			INNER JOIN subtree s ON LOWER(e.parent_dn) = LOWER(s.dn)
			WHERE s.depth < 100
		)
	` + selectClause + `
		FROM subtree e
	` + joinWhere + groupBy, args
	}
}
