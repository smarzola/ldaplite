package handlers

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/smarzola/ldaplite/internal/ldapdn"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
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

func ReplaceExtraAttributes(entry *models.Entry, preserve []string, extras map[string][]string) {
	preserveMap := make(map[string]bool)
	for _, name := range preserve {
		preserveMap[strings.ToLower(name)] = true
	}

	for name := range entry.Attributes {
		if !preserveMap[strings.ToLower(name)] {
			delete(entry.Attributes, name)
		}
	}

	for name, values := range extras {
		entry.Attributes[strings.ToLower(name)] = values
	}
}

func setOptionalAttribute(entry *models.Entry, name, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		entry.RemoveAttribute(name)
		return
	}
	entry.SetAttribute(name, value)
}

func parseNonEmptyLines(input string) []string {
	var lines []string
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func addExtraAttributes(entry *models.Entry, attrs map[string][]string) {
	for name, values := range attrs {
		for _, value := range values {
			entry.AddAttribute(name, value)
		}
	}
}

func loadOrganizationalUnits(ctx context.Context, st store.Store, baseDN string) []*models.Entry {
	ous, err := searchEntriesWithoutMemberOf(ctx, st, baseDN, "(objectClass=organizationalUnit)")
	if err != nil {
		slog.Error("Failed to fetch OUs", "error", err)
		return []*models.Entry{}
	}
	return ous
}

func searchEntriesWithoutMemberOf(ctx context.Context, st store.Store, baseDN, filter string) ([]*models.Entry, error) {
	return st.SearchEntriesWithOptions(ctx, store.SearchOptions{
		BaseDN:          baseDN,
		Filter:          filter,
		Scope:           store.SearchScopeWholeSubtree,
		IncludeMemberOf: false,
	})
}

func getEntryWithoutMemberOf(ctx context.Context, st store.Store, dn string) (*models.Entry, error) {
	return st.GetEntryWithOptions(ctx, dn, store.EntryOptions{IncludeMemberOf: false})
}

// GetBaseDN returns the base DN string for templates
func GetBaseDN(cfg *config.Config) string {
	return cfg.LDAP.BaseDN
}

// ExtractUIDFromDN extracts the uid from a DN like "uid=admin,ou=users,dc=example,dc=com"
func ExtractUIDFromDN(dn string) string {
	return ldapdn.FirstRDNValue(dn, "uid")
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
