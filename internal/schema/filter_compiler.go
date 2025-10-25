package schema

import (
	"fmt"
	"strings"
)

// FilterCompiler compiles LDAP filters to SQL WHERE clauses
type FilterCompiler struct{}

// NewFilterCompiler creates a new filter compiler
func NewFilterCompiler() *FilterCompiler {
	return &FilterCompiler{}
}

// CompileToSQL converts an LDAP filter to a SQL WHERE clause
// Returns: (whereClause, args, error)
func (fc *FilterCompiler) CompileToSQL(filter *Filter) (string, []interface{}, error) {
	if filter == nil {
		return "", nil, fmt.Errorf("filter is nil")
	}

	switch filter.Type {
	case FilterTypeAnd:
		return fc.compileAnd(filter.Filters)
	case FilterTypeOr:
		return fc.compileOr(filter.Filters)
	case FilterTypeNot:
		return fc.compileNot(filter.Filters)
	case FilterTypeEquality:
		return fc.compileEquality(filter.Attribute, filter.Value)
	case FilterTypePresent:
		return fc.compilePresent(filter.Attribute)
	case FilterTypeSubstrings:
		return fc.compileSubstring(filter.Attribute, filter.Value)
	default:
		return "", nil, fmt.Errorf("unsupported filter type: %d", filter.Type)
	}
}

// CanCompileToSQL checks if a filter can be compiled to SQL
func (fc *FilterCompiler) CanCompileToSQL(filter *Filter) bool {
	if filter == nil {
		return false
	}

	switch filter.Type {
	case FilterTypeEquality, FilterTypePresent:
		return true
	case FilterTypeSubstrings:
		// Substring support depends on value containing wildcards
		return strings.Contains(filter.Value, "*")
	case FilterTypeAnd, FilterTypeOr:
		// All sub-filters must be compilable
		for _, sf := range filter.Filters {
			if !fc.CanCompileToSQL(sf) {
				return false
			}
		}
		return true
	case FilterTypeNot:
		// NOT filter is compilable if its sub-filter is
		return len(filter.Filters) == 1 && fc.CanCompileToSQL(filter.Filters[0])
	default:
		// GreaterOrEqual, LessOrEqual, ApproxMatch not supported yet
		return false
	}
}

// compileEquality compiles an equality filter: (attr=value)
func (fc *FilterCompiler) compileEquality(attr, value string) (string, []interface{}, error) {
	attrLower := strings.ToLower(attr)

	// Special case: objectClass is in entries table
	// Use case-insensitive comparison for LDAP compliance
	if attrLower == "objectclass" {
		return "LOWER(e.object_class) = LOWER(?)", []interface{}{value}, nil
	}

	// All other attributes in attributes table
	// Use EXISTS subquery for efficiency with case-insensitive comparison
	clause := `EXISTS (
		SELECT 1 FROM attributes a
		WHERE a.entry_id = e.id
		  AND LOWER(a.name) = LOWER(?)
		  AND LOWER(a.value) = LOWER(?)
	)`
	return clause, []interface{}{attr, value}, nil
}

// compilePresent compiles a presence filter: (attr=*)
func (fc *FilterCompiler) compilePresent(attr string) (string, []interface{}, error) {
	attrLower := strings.ToLower(attr)

	// Special case: objectClass
	if attrLower == "objectclass" {
		return "e.object_class IS NOT NULL AND e.object_class != ''", nil, nil
	}

	// Check attribute exists
	clause := `EXISTS (
		SELECT 1 FROM attributes a
		WHERE a.entry_id = e.id
		  AND LOWER(a.name) = LOWER(?)
	)`
	return clause, []interface{}{attr}, nil
}

// compileSubstring compiles a substring filter: (attr=value*)
// The value contains wildcards (*) that need to be converted to SQL LIKE patterns
func (fc *FilterCompiler) compileSubstring(attr, value string) (string, []interface{}, error) {
	attrLower := strings.ToLower(attr)

	// objectClass doesn't support substring matching
	if attrLower == "objectclass" {
		return "", nil, fmt.Errorf("substring filter not supported for objectClass")
	}

	// Convert LDAP wildcard (*) to SQL LIKE wildcard (%)
	// LDAP: (cn=John*) or (cn=*Doe) or (cn=*oh*oe*)
	likePattern := strings.ReplaceAll(value, "*", "%")

	// Escape SQL LIKE special characters (underscore)
	likePattern = strings.ReplaceAll(likePattern, "_", "\\_")

	// LDAP attributes are case-insensitive, so use LOWER() for both sides
	clause := `EXISTS (
		SELECT 1 FROM attributes a
		WHERE a.entry_id = e.id
		  AND LOWER(a.name) = LOWER(?)
		  AND LOWER(a.value) LIKE LOWER(?) ESCAPE '\'
	)`

	return clause, []interface{}{attr, likePattern}, nil
}

// compileAnd compiles an AND filter: (&(filter1)(filter2)...)
func (fc *FilterCompiler) compileAnd(subFilters []*Filter) (string, []interface{}, error) {
	if len(subFilters) == 0 {
		return "1=1", nil, nil // Always true (empty AND)
	}

	var clauses []string
	var allArgs []interface{}

	for _, sf := range subFilters {
		clause, args, err := fc.CompileToSQL(sf)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "("+clause+")")
		allArgs = append(allArgs, args...)
	}

	return strings.Join(clauses, " AND "), allArgs, nil
}

// compileOr compiles an OR filter: (|(filter1)(filter2)...)
func (fc *FilterCompiler) compileOr(subFilters []*Filter) (string, []interface{}, error) {
	if len(subFilters) == 0 {
		return "1=0", nil, nil // Always false (empty OR)
	}

	var clauses []string
	var allArgs []interface{}

	for _, sf := range subFilters {
		clause, args, err := fc.CompileToSQL(sf)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "("+clause+")")
		allArgs = append(allArgs, args...)
	}

	return strings.Join(clauses, " OR "), allArgs, nil
}

// compileNot compiles a NOT filter: (!(filter))
func (fc *FilterCompiler) compileNot(subFilters []*Filter) (string, []interface{}, error) {
	if len(subFilters) != 1 {
		return "", nil, fmt.Errorf("NOT filter must have exactly one sub-filter")
	}

	clause, args, err := fc.CompileToSQL(subFilters[0])
	if err != nil {
		return "", nil, err
	}

	return "NOT (" + clause + ")", args, nil
}
