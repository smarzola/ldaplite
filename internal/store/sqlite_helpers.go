package store

import (
	"encoding/json"
	"fmt"
	"strings"
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
