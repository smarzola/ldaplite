package store

import (
	"context"
	"fmt"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/schema"
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
