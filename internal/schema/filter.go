package schema

import (
	"fmt"
	"strings"

	"github.com/smarzola/ldaplite/internal/models"
)

// FilterType represents the type of LDAP filter
type FilterType int

const (
	FilterTypeAnd FilterType = iota
	FilterTypeOr
	FilterTypeNot
	FilterTypeEquality
	FilterTypePresent
	FilterTypeApproxMatch
	FilterTypeGreaterOrEqual
	FilterTypeLessOrEqual
	FilterTypeSubstrings
)

// Filter represents an LDAP search filter
type Filter struct {
	Type      FilterType
	Attribute string
	Value     string
	Filters   []*Filter
}

// ParseFilter parses an LDAP filter string
// Supports basic filter syntax: (&(objectClass=*)), (uid=john), etc.
func ParseFilter(filterStr string) (*Filter, error) {
	if filterStr == "" {
		// Empty filter means match all
		return &Filter{
			Type:      FilterTypePresent,
			Attribute: "objectClass",
		}, nil
	}

	filterStr = strings.TrimSpace(filterStr)
	if !strings.HasPrefix(filterStr, "(") || !strings.HasSuffix(filterStr, ")") {
		return nil, fmt.Errorf("filter must be enclosed in parentheses")
	}

	filter, _, err := parseFilterRecursive(filterStr, 0)
	return filter, err
}

// parseFilterRecursive recursively parses filter components
func parseFilterRecursive(filterStr string, pos int) (*Filter, int, error) {
	if pos >= len(filterStr) {
		return nil, pos, fmt.Errorf("unexpected end of filter")
	}

	if filterStr[pos] != '(' {
		return nil, pos, fmt.Errorf("expected '(' at position %d", pos)
	}

	pos++ // skip '('

	if pos >= len(filterStr) {
		return nil, pos, fmt.Errorf("unexpected end of filter")
	}

	// Check for complex filters (&, |, !)
	if filterStr[pos] == '&' {
		pos++ // skip '&'
		filter := &Filter{Type: FilterTypeAnd}

		for pos < len(filterStr) && filterStr[pos] == '(' {
			subFilter, newPos, err := parseFilterRecursive(filterStr, pos)
			if err != nil {
				return nil, pos, err
			}
			filter.Filters = append(filter.Filters, subFilter)
			pos = newPos

			if pos >= len(filterStr) {
				return nil, pos, fmt.Errorf("unexpected end of filter")
			}
		}

		if pos >= len(filterStr) || filterStr[pos] != ')' {
			return nil, pos, fmt.Errorf("expected ')' at position %d", pos)
		}
		pos++ // skip ')'

		return filter, pos, nil
	}

	if filterStr[pos] == '|' {
		pos++ // skip '|'
		filter := &Filter{Type: FilterTypeOr}

		for pos < len(filterStr) && filterStr[pos] == '(' {
			subFilter, newPos, err := parseFilterRecursive(filterStr, pos)
			if err != nil {
				return nil, pos, err
			}
			filter.Filters = append(filter.Filters, subFilter)
			pos = newPos

			if pos >= len(filterStr) {
				return nil, pos, fmt.Errorf("unexpected end of filter")
			}
		}

		if pos >= len(filterStr) || filterStr[pos] != ')' {
			return nil, pos, fmt.Errorf("expected ')' at position %d", pos)
		}
		pos++ // skip ')'

		return filter, pos, nil
	}

	if filterStr[pos] == '!' {
		pos++ // skip '!'
		subFilter, newPos, err := parseFilterRecursive(filterStr, pos)
		if err != nil {
			return nil, pos, err
		}

		filter := &Filter{
			Type:    FilterTypeNot,
			Filters: []*Filter{subFilter},
		}

		if newPos >= len(filterStr) || filterStr[newPos] != ')' {
			return nil, newPos, fmt.Errorf("expected ')' at position %d", newPos)
		}

		return filter, newPos + 1, nil
	}

	// Simple filter: attribute=value
	endPos := strings.IndexByte(filterStr[pos:], ')')
	if endPos == -1 {
		return nil, pos, fmt.Errorf("expected ')'")
	}

	filterPart := filterStr[pos : pos+endPos]

	// Parse attribute=value, attribute=*, etc.
	parts := strings.SplitN(filterPart, "=", 2)
	if len(parts) != 2 {
		return nil, pos, fmt.Errorf("invalid filter format: %s", filterPart)
	}

	attribute := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	var filterType FilterType
	if value == "*" {
		// Presence filter: (attr=*)
		filterType = FilterTypePresent
	} else if strings.Contains(value, "*") {
		// Substring filter: (attr=*value*), (attr=value*), (attr=*value)
		filterType = FilterTypeSubstrings
	} else {
		// Equality filter: (attr=value)
		filterType = FilterTypeEquality
	}

	filter := &Filter{
		Type:      filterType,
		Attribute: attribute,
		Value:     value,
	}

	return filter, pos + endPos + 1, nil
}

// matchSubstring checks if a value matches an LDAP substring pattern
// Pattern can contain wildcards: a* (starts with), *a (ends with), *a* (contains), a*b (complex)
// Matching is case-insensitive per LDAP RFC 4517
func matchSubstring(value, pattern string) bool {
	// Case-insensitive matching
	value = strings.ToLower(value)
	pattern = strings.ToLower(pattern)

	// Split pattern by wildcards
	parts := strings.Split(pattern, "*")

	// No wildcards (shouldn't happen for substring filter, but handle it)
	if len(parts) == 1 {
		return value == pattern
	}

	// Check first part (if not empty, must be at start)
	if parts[0] != "" {
		if !strings.HasPrefix(value, parts[0]) {
			return false
		}
		value = value[len(parts[0]):]
	}

	// Check last part (if not empty, must be at end)
	if parts[len(parts)-1] != "" {
		if !strings.HasSuffix(value, parts[len(parts)-1]) {
			return false
		}
		value = value[:len(value)-len(parts[len(parts)-1])]
	}

	// Check middle parts (must appear in order)
	for i := 1; i < len(parts)-1; i++ {
		if parts[i] == "" {
			continue
		}
		idx := strings.Index(value, parts[i])
		if idx == -1 {
			return false
		}
		value = value[idx+len(parts[i]):]
	}

	return true
}

// Matches checks if an entry matches this filter
func (f *Filter) Matches(entry *models.Entry) bool {
	switch f.Type {
	case FilterTypeAnd:
		for _, subFilter := range f.Filters {
			if !subFilter.Matches(entry) {
				return false
			}
		}
		return true

	case FilterTypeOr:
		for _, subFilter := range f.Filters {
			if subFilter.Matches(entry) {
				return true
			}
		}
		return false

	case FilterTypeNot:
		if len(f.Filters) > 0 {
			return !f.Filters[0].Matches(entry)
		}
		return true

	case FilterTypePresent:
		return entry.HasAttribute(f.Attribute)

	case FilterTypeEquality:
		// Case-insensitive equality matching per LDAP RFC 4517
		values := entry.GetAttributes(f.Attribute)
		filterValue := strings.ToLower(f.Value)
		for _, v := range values {
			if strings.ToLower(v) == filterValue {
				return true
			}
		}
		return false

	case FilterTypeSubstrings:
		// Substring matching with wildcards
		// Convert LDAP wildcard pattern to Go regexp-like matching
		values := entry.GetAttributes(f.Attribute)
		for _, v := range values {
			if matchSubstring(v, f.Value) {
				return true
			}
		}
		return false

	case FilterTypeApproxMatch, FilterTypeGreaterOrEqual, FilterTypeLessOrEqual:
		// Not implemented yet, treat as equality
		values := entry.GetAttributes(f.Attribute)
		for _, v := range values {
			if v == f.Value {
				return true
			}
		}
		return false

	default:
		return false
	}
}

// String returns a string representation of the filter
func (f *Filter) String() string {
	switch f.Type {
	case FilterTypeAnd:
		parts := []string{"(&"}
		for _, subFilter := range f.Filters {
			parts = append(parts, subFilter.String())
		}
		parts = append(parts, ")")
		return strings.Join(parts, "")

	case FilterTypeOr:
		parts := []string{"(|"}
		for _, subFilter := range f.Filters {
			parts = append(parts, subFilter.String())
		}
		parts = append(parts, ")")
		return strings.Join(parts, "")

	case FilterTypeNot:
		if len(f.Filters) > 0 {
			return "(!" + f.Filters[0].String() + ")"
		}
		return "(!)"

	case FilterTypePresent:
		return fmt.Sprintf("(%s=*)", f.Attribute)

	case FilterTypeEquality:
		return fmt.Sprintf("(%s=%s)", f.Attribute, f.Value)

	default:
		return ""
	}
}
