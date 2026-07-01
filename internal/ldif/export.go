package ldif

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
)

// EntrySearcher is the store capability needed for LDIF export.
type EntrySearcher interface {
	SearchEntriesWithOptions(ctx context.Context, options store.SearchOptions) ([]*models.Entry, error)
}

// ExportOptions configures LDIF export.
type ExportOptions struct {
	BaseDN                      string
	IncludeOperational          bool
	IncludePasswordPlaceholders bool
}

// BuildExportRecords reads directory entries and converts them to safe LDIF records.
func BuildExportRecords(ctx context.Context, searcher EntrySearcher, options ExportOptions) ([]Record, error) {
	if searcher == nil {
		return nil, fmt.Errorf("entry searcher is required")
	}
	if strings.TrimSpace(options.BaseDN) == "" {
		return nil, fmt.Errorf("base DN is required")
	}

	entries, err := searcher.SearchEntriesWithOptions(ctx, store.SearchOptions{
		BaseDN:          options.BaseDN,
		Filter:          "(objectClass=*)",
		Scope:           store.SearchScopeWholeSubtree,
		IncludeMemberOf: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search entries for export: %w", err)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		leftDepth := dnDepth(entries[i].DN)
		rightDepth := dnDepth(entries[j].DN)
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return strings.ToLower(entries[i].DN) < strings.ToLower(entries[j].DN)
	})

	records := make([]Record, 0, len(entries))
	for _, entry := range entries {
		records = append(records, recordFromEntry(entry, options))
	}
	return records, nil
}

func recordFromEntry(entry *models.Entry, options ExportOptions) Record {
	record := Record{
		DN: entry.DN,
		Attributes: []Attribute{
			{Name: "objectClass", Value: entry.ObjectClass},
		},
	}

	names := make([]string, 0, len(entry.Attributes))
	for name := range entry.Attributes {
		if exportableAttribute(name, options.IncludeOperational) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		for _, value := range entry.Attributes[name] {
			record.Attributes = append(record.Attributes, Attribute{Name: name, Value: value})
		}
	}

	if options.IncludeOperational {
		if !hasAttribute(record.Attributes, "createTimestamp") && !entry.CreatedAt.IsZero() {
			record.Attributes = append(record.Attributes, Attribute{Name: "createTimestamp", Value: models.FormatLDAPTimestamp(entry.CreatedAt)})
		}
		if !hasAttribute(record.Attributes, "modifyTimestamp") && !entry.UpdatedAt.IsZero() {
			record.Attributes = append(record.Attributes, Attribute{Name: "modifyTimestamp", Value: models.FormatLDAPTimestamp(entry.UpdatedAt)})
		}
	}
	if options.IncludePasswordPlaceholders && entry.IsUser() {
		record.Attributes = append(record.Attributes, Attribute{Name: "userPassword", Value: "{REDACTED}"})
	}

	return record
}

func exportableAttribute(name string, includeOperational bool) bool {
	switch strings.ToLower(name) {
	case "objectclass", "userpassword", "memberof", "uuid":
		return false
	case "entryuuid", "createtimestamp", "modifytimestamp":
		return includeOperational
	default:
		return true
	}
}

func hasAttribute(attrs []Attribute, name string) bool {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name, name) {
			return true
		}
	}
	return false
}
