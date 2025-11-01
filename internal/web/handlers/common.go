package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/web/middleware"
	"github.com/smarzola/ldaplite/pkg/config"
)

// TemplateGetter is a function that returns a parsed template for a given name
type TemplateGetter func(name string) (*template.Template, error)

// BaseData holds common data for all templates
type BaseData struct {
	BaseDN      string
	CurrentPage string
	Success     string
	Error       string
	UserDN      string
	UserID      string
}

// RenderTemplate renders a template with base data
func RenderTemplate(w http.ResponseWriter, getter TemplateGetter, name string, data interface{}) {
	tmpl, err := getter(name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse template %s: %v", name, err), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("Failed to render template %s: %v", name, err), http.StatusInternalServerError)
	}
}

// ParseAttributes parses additional attributes from form input
// Format: "name: value" one per line
func ParseAttributes(input string) map[string][]string {
	attrs := make(map[string][]string)
	if input == "" {
		return attrs
	}

	lines := strings.Split(input, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if name != "" && value != "" {
			attrs[name] = append(attrs[name], value)
		}
	}

	return attrs
}

// FormatExtraAttributes formats attributes back to text format
func FormatExtraAttributes(entry *models.Entry, exclude []string) string {
	var lines []string
	excludeMap := make(map[string]bool)
	for _, e := range exclude {
		excludeMap[strings.ToLower(e)] = true
	}

	for name, values := range entry.Attributes {
		if excludeMap[strings.ToLower(name)] {
			continue
		}
		for _, value := range values {
			lines = append(lines, fmt.Sprintf("%s: %s", name, value))
		}
	}

	return strings.Join(lines, "\n")
}

// GetBaseDN returns the base DN string for templates
func GetBaseDN(cfg *config.Config) string {
	return cfg.LDAP.BaseDN
}

// ExtractUIDFromDN extracts the uid from a DN like "uid=admin,ou=users,dc=example,dc=com"
func ExtractUIDFromDN(dn string) string {
	if dn == "" {
		return ""
	}
	// Split by comma to get RDN components
	parts := strings.Split(dn, ",")
	if len(parts) == 0 {
		return ""
	}
	// Get the first component (uid=...)
	firstPart := strings.TrimSpace(parts[0])
	// Split by = to get the value
	kvPair := strings.SplitN(firstPart, "=", 2)
	if len(kvPair) != 2 {
		return ""
	}
	// Check if it's a uid attribute
	if strings.EqualFold(kvPair[0], "uid") {
		return kvPair[1]
	}
	return ""
}

// NewBaseData creates a BaseData struct with common fields populated
func NewBaseData(cfg *config.Config, r *http.Request, currentPage string) BaseData {
	userDN := middleware.GetUserDN(r)
	return BaseData{
		BaseDN:      GetBaseDN(cfg),
		CurrentPage: currentPage,
		UserDN:      userDN,
		UserID:      ExtractUIDFromDN(userDN),
		Success:     r.URL.Query().Get("success"),
		Error:       r.URL.Query().Get("error"),
	}
}
