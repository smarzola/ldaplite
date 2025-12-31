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
	case FilterTypeGreaterOrEqual:
		return fc.compileGreaterOrEqual(filter.Attribute, filter.Value)
	case FilterTypeLessOrEqual:
		return fc.compileLessOrEqual(filter.Attribute, filter.Value)
	default:
		return "", nil, fmt.Errorf("unsupported filter type: %d", filter.Type)
	}
}

// computedAttributes are attributes that are not stored in the attributes table
// but computed dynamically (e.g., memberOf from group_members table).
// These require in-memory filtering and cannot be compiled to SQL.
var computedAttributes = map[string]bool{
	"memberof": true, // RFC2307bis: computed from group_members table
}

// isComputedAttribute checks if an attribute is computed (not stored in SQL)
func isComputedAttribute(attr string) bool {
	return computedAttributes[strings.ToLower(attr)]
}

// FilterUsesComputedAttributes checks if a filter references any computed attributes
// (like memberOf). This is used to optimize query execution order.
func FilterUsesComputedAttributes(filter *Filter) bool {
	if filter == nil {
		return false
	}

	switch filter.Type {
	case FilterTypeEquality, FilterTypePresent, FilterTypeSubstrings,
		FilterTypeGreaterOrEqual, FilterTypeLessOrEqual, FilterTypeApproxMatch:
		return isComputedAttribute(filter.Attribute)
	case FilterTypeAnd, FilterTypeOr:
		for _, sf := range filter.Filters {
			if FilterUsesComputedAttributes(sf) {
				return true
			}
		}
		return false
	case FilterTypeNot:
		if len(filter.Filters) > 0 {
			return FilterUsesComputedAttributes(filter.Filters[0])
		}
		return false
	default:
		return false
	}
}

// CanCompileToSQL checks if a filter can be compiled to SQL
func (fc *FilterCompiler) CanCompileToSQL(filter *Filter) bool {
	if filter == nil {
		return false
	}

	switch filter.Type {
	case FilterTypeEquality, FilterTypePresent:
		// Computed attributes (like memberOf) require in-memory filtering
		return !isComputedAttribute(filter.Attribute)
	case FilterTypeSubstrings:
		// Substring support depends on value containing wildcards
		// Computed attributes require in-memory filtering
		return !isComputedAttribute(filter.Attribute) && strings.Contains(filter.Value, "*")
	case FilterTypeGreaterOrEqual, FilterTypeLessOrEqual:
		// Comparison operators supported for operational timestamp attributes
		attrLower := strings.ToLower(filter.Attribute)
		return attrLower == "createtimestamp" || attrLower == "modifytimestamp"
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
		// ApproxMatch not supported yet
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

// compileGreaterOrEqual compiles a >= filter for operational timestamp attributes
// Converts LDAP Generalized Time format (YYYYMMDDHHMMSSz) to SQLite datetime comparison
func (fc *FilterCompiler) compileGreaterOrEqual(attr, value string) (string, []interface{}, error) {
	attrLower := strings.ToLower(attr)

	// Map operational attributes to database columns
	var column string
	switch attrLower {
	case "createtimestamp":
		column = "e.created_at"
	case "modifytimestamp":
		column = "e.updated_at"
	default:
		return "", nil, fmt.Errorf("comparison operators only supported for createTimestamp and modifyTimestamp")
	}

	// Convert LDAP timestamp (YYYYMMDDHHMMSSz) to SQLite datetime format
	sqliteTimestamp, err := convertLDAPTimestampToSQLite(value)
	if err != nil {
		return "", nil, fmt.Errorf("invalid timestamp format: %w", err)
	}

	// SQLite datetime comparison
	clause := fmt.Sprintf("%s >= ?", column)
	return clause, []interface{}{sqliteTimestamp}, nil
}

// compileLessOrEqual compiles a <= filter for operational timestamp attributes
// Converts LDAP Generalized Time format (YYYYMMDDHHMMSSz) to SQLite datetime comparison
func (fc *FilterCompiler) compileLessOrEqual(attr, value string) (string, []interface{}, error) {
	attrLower := strings.ToLower(attr)

	// Map operational attributes to database columns
	var column string
	switch attrLower {
	case "createtimestamp":
		column = "e.created_at"
	case "modifytimestamp":
		column = "e.updated_at"
	default:
		return "", nil, fmt.Errorf("comparison operators only supported for createTimestamp and modifyTimestamp")
	}

	// Convert LDAP timestamp (YYYYMMDDHHMMSSz) to SQLite datetime format
	sqliteTimestamp, err := convertLDAPTimestampToSQLite(value)
	if err != nil {
		return "", nil, fmt.Errorf("invalid timestamp format: %w", err)
	}

	// SQLite datetime comparison
	clause := fmt.Sprintf("%s <= ?", column)
	return clause, []interface{}{sqliteTimestamp}, nil
}

// convertLDAPTimestampToSQLite converts LDAP Generalized Time to SQLite datetime
// LDAP format: YYYYMMDDHHMMSSz (e.g., 20130905020304Z)
// SQLite format: YYYY-MM-DD HH:MM:SS (e.g., 2013-09-05 02:03:04)
func convertLDAPTimestampToSQLite(ldapTime string) (string, error) {
	// Remove trailing 'Z' if present
	ldapTime = strings.TrimSuffix(ldapTime, "Z")
	ldapTime = strings.TrimSuffix(ldapTime, "z")

	// Validate length (should be 14 characters: YYYYMMDDHHMMss)
	if len(ldapTime) != 14 {
		return "", fmt.Errorf("invalid LDAP timestamp length: expected 14, got %d", len(ldapTime))
	}

	// Parse components
	year := ldapTime[0:4]
	month := ldapTime[4:6]
	day := ldapTime[6:8]
	hour := ldapTime[8:10]
	minute := ldapTime[10:12]
	second := ldapTime[12:14]

	// Format as SQLite datetime
	return fmt.Sprintf("%s-%s-%s %s:%s:%s", year, month, day, hour, minute, second), nil
}
