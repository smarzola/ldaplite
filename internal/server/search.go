package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/smarzola/ldaplite/internal/audit"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/protocol"
	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/internal/telemetry"
)

// handleSearch handles search operations
func (s *Server) handleSearch(ctx context.Context, conn *protocol.Connection, msg *ldapmsg.Message) error {
	start := time.Now()
	searchReq := msg.Op.(ldapmsg.SearchRequest)
	baseDN := searchReq.BaseObject
	scope := ldapSearchScope(searchReq.Scope)
	selection := newSearchAttributeSelection(searchReq.Attributes)
	resultCode := ldapmsg.ResultCodeOperationsError
	var resultCount *int
	ctx, span := telemetry.StartLDAPSpan(ctx, "search")
	defer func() {
		telemetry.EndLDAPSpan(span, int(resultCode))
	}()
	defer func() {
		s.auditLDAPOperation(ctx, conn, msg, "search", audit.LDAPEvent{
			ActorDN:     conn.GetBoundDN(),
			BaseDN:      baseDN,
			Scope:       searchScopeString(scope),
			ResultCode:  int(resultCode),
			ResultCount: resultCount,
			Duration:    time.Since(start),
		})
	}()

	// Handle RootDSE queries (empty base DN)
	if baseDN == "" {
		slog.Debug("RootDSE query")
		err := s.handleRootDSE(conn, msg)
		if err == nil {
			resultCode = ldapmsg.ResultCodeSuccess
			count := 1
			resultCount = &count
		}
		return err
	}

	// Handle schema queries
	if baseDN == "cn=Subschema" || baseDN == "cn=subschema" {
		slog.Debug("Schema query")
		err := s.handleSchema(conn, msg)
		if err == nil {
			resultCode = ldapmsg.ResultCodeSuccess
			count := 1
			resultCount = &count
		}
		return err
	}

	if !s.canSearch(conn, baseDN) {
		slog.Info("Search rejected - bind required", "baseDN", baseDN)
		resultCode = ldapmsg.ResultCodeInsufficientAccessRights
		return conn.WriteResponse(msg.ID, protocol.NewSearchResultDone(ldapmsg.ResultCodeInsufficientAccessRights))
	}

	// Get filter from request
	filterStr := serializeFilter(searchReq.Filter)
	if filterStr == "" {
		filterStr = "(objectClass=*)"
	}

	slog.Debug("Search request", "baseDN", baseDN, "scope", scope, "filter", filterStr)

	entries, err := s.store.SearchEntriesWithOptions(ctx, store.SearchOptions{
		BaseDN:          baseDN,
		Filter:          filterStr,
		Scope:           scope,
		IncludeMemberOf: selection.includes("memberOf"),
	})
	if err != nil {
		slog.Error("Search error", "error", err)
		resultCode = ldapmsg.ResultCodeOperationsError
		return conn.WriteResponse(msg.ID, protocol.NewSearchResultDone(ldapmsg.ResultCodeOperationsError))
	}

	// Return matching entries
	for _, entry := range entries {
		// Build search result entry
		result := protocol.NewSearchResultEntry(entry.DN)

		for _, attr := range searchResponseAttributes(entry, selection) {
			addSearchAttribute(&result, attr.name, attr.values, searchReq.TypesOnly)
		}

		// Write entry
		if err := conn.WriteResponse(msg.ID, result); err != nil {
			return err
		}
	}

	slog.Debug("Search completed", "baseDN", baseDN, "results", len(entries))
	resultCode = ldapmsg.ResultCodeSuccess
	count := len(entries)
	resultCount = &count
	return conn.WriteResponse(msg.ID, protocol.NewSearchResultDone(ldapmsg.ResultCodeSuccess))
}

func ldapSearchScope(scope ldapmsg.SearchScope) store.SearchScope {
	switch scope {
	case ldapmsg.SearchScopeBaseObject:
		return store.SearchScopeBaseObject
	case ldapmsg.SearchScopeSingleLevel:
		return store.SearchScopeSingleLevel
	default:
		return store.SearchScopeWholeSubtree
	}
}

func searchScopeString(scope store.SearchScope) string {
	switch scope {
	case store.SearchScopeBaseObject:
		return "base"
	case store.SearchScopeSingleLevel:
		return "one"
	default:
		return "subtree"
	}
}

func (s *Server) canSearch(conn *protocol.Connection, baseDN string) bool {
	if isPublicSearchBase(baseDN) {
		return true
	}
	if !conn.IsBound() {
		return false
	}
	if conn.GetBoundDN() == "" {
		return s.cfg.Security.AllowAnonymousBind
	}
	return true
}

func isPublicSearchBase(baseDN string) bool {
	return baseDN == "" || strings.EqualFold(baseDN, "cn=Subschema")
}

type searchAttributeSelection struct {
	noAttributes       bool
	includeAll         bool
	includeOperational bool
	names              map[string]bool
}

type searchResponseAttribute struct {
	name   string
	values []string
}

func newSearchAttributeSelection(selection []string) searchAttributeSelection {
	if len(selection) == 0 {
		return searchAttributeSelection{includeAll: true, includeOperational: true}
	}

	result := searchAttributeSelection{names: make(map[string]bool)}
	for _, selector := range selection {
		name := strings.TrimSpace(selector)
		if name == "" {
			continue
		}

		switch strings.ToLower(name) {
		case "1.1":
			result.noAttributes = true
		case "*":
			result.includeAll = true
			result.noAttributes = false
		case "+":
			result.includeOperational = true
			result.noAttributes = false
		default:
			result.names[strings.ToLower(name)] = true
			result.noAttributes = false
		}
	}

	return result
}

func (s searchAttributeSelection) includes(attrName string) bool {
	if s.noAttributes {
		return false
	}
	attrLower := strings.ToLower(attrName)
	if s.names[attrLower] {
		return true
	}
	if s.includeOperational && isOperationalAttribute(attrLower) {
		return true
	}
	return s.includeAll && !isOperationalAttribute(attrLower)
}

func isOperationalAttribute(attrName string) bool {
	switch strings.ToLower(attrName) {
	case "createtimestamp", "modifytimestamp", "memberof":
		return true
	default:
		return false
	}
}

func searchResponseAttributes(entry *models.Entry, selection searchAttributeSelection) []searchResponseAttribute {
	attrs := make([]searchResponseAttribute, 0, len(entry.Attributes)+4)
	if entry.ObjectClass != "" && selection.includes("objectClass") {
		attrs = append(attrs, searchResponseAttribute{
			name:   "objectClass",
			values: []string{entry.ObjectClass},
		})
	}
	if selection.includes("createTimestamp") {
		attrs = append(attrs, searchResponseAttribute{
			name:   "createTimestamp",
			values: []string{models.FormatLDAPTimestamp(entry.CreatedAt)},
		})
	}
	if selection.includes("modifyTimestamp") {
		attrs = append(attrs, searchResponseAttribute{
			name:   "modifyTimestamp",
			values: []string{models.FormatLDAPTimestamp(entry.UpdatedAt)},
		})
	}
	if memberOf := entry.GetAttributes("memberOf"); len(memberOf) > 0 && selection.includes("memberOf") {
		attrs = append(attrs, searchResponseAttribute{
			name:   "memberOf",
			values: memberOf,
		})
	}

	for attrName, attrValues := range entry.Attributes {
		if isSearchProjectedAttribute(attrName) {
			continue
		}
		if selection.includes(attrName) {
			attrs = append(attrs, searchResponseAttribute{
				name:   attrName,
				values: attrValues,
			})
		}
	}

	return attrs
}

func isSearchProjectedAttribute(attrName string) bool {
	switch strings.ToLower(attrName) {
	case "objectclass", "createtimestamp", "modifytimestamp", "memberof":
		return true
	default:
		return false
	}
}

func addSearchAttribute(result *ldapmsg.SearchResultEntry, name string, values []string, typesOnly bool) {
	if typesOnly {
		protocol.AddAttribute(result, name)
		return
	}
	protocol.AddAttribute(result, name, values...)
}

func serializeFilter(f ldapmsg.Filter) string {
	if f == nil {
		return ""
	}

	switch filter := f.(type) {
	case ldapmsg.EqualityMatchFilter:
		return fmt.Sprintf("(%s=%s)", filter.Attribute, escapeLDAPFilterAssertionValue(filter.Value))

	case ldapmsg.PresentFilter:
		return fmt.Sprintf("(%s=*)", filter.Attribute)

	case ldapmsg.AndFilter:
		if len(filter.Filters) == 0 {
			return ""
		}
		var parts []string
		for _, subFilter := range filter.Filters {
			parts = append(parts, serializeFilter(subFilter))
		}
		return "(&" + strings.Join(parts, "") + ")"

	case ldapmsg.OrFilter:
		if len(filter.Filters) == 0 {
			return ""
		}
		var parts []string
		for _, subFilter := range filter.Filters {
			parts = append(parts, serializeFilter(subFilter))
		}
		return "(|" + strings.Join(parts, "") + ")"

	case ldapmsg.NotFilter:
		return "(!" + serializeFilter(filter.Filter) + ")"

	case ldapmsg.GreaterOrEqualFilter:
		return fmt.Sprintf("(%s>=%s)", filter.Attribute, escapeLDAPFilterAssertionValue(filter.Value))

	case ldapmsg.LessOrEqualFilter:
		return fmt.Sprintf("(%s<=%s)", filter.Attribute, escapeLDAPFilterAssertionValue(filter.Value))

	case ldapmsg.ApproxMatchFilter:
		return fmt.Sprintf("(%s~=%s)", filter.Attribute, escapeLDAPFilterAssertionValue(filter.Value))

	case ldapmsg.SubstringsFilter:
		attr := filter.Attribute
		var sb strings.Builder
		sb.WriteString("(")
		sb.WriteString(attr)
		sb.WriteString("=")

		for _, sub := range filter.Substrings {
			switch sub.Kind {
			case ldapmsg.SubstringInitial:
				sb.WriteString(escapeLDAPFilterAssertionValue(sub.Value))
				sb.WriteString("*")
			case ldapmsg.SubstringAny:
				sb.WriteString(escapeLDAPFilterAssertionValue(sub.Value))
				sb.WriteString("*")
			case ldapmsg.SubstringFinal:
				sb.WriteString(escapeLDAPFilterAssertionValue(sub.Value))
			}
		}
		sb.WriteString(")")
		return sb.String()

	default:
		return "(objectClass=*)"
	}
}

func escapeLDAPFilterAssertionValue(value string) string {
	var escaped strings.Builder
	for _, r := range value {
		switch r {
		case '*':
			escaped.WriteString(`\2a`)
		case '(':
			escaped.WriteString(`\28`)
		case ')':
			escaped.WriteString(`\29`)
		case '\\':
			escaped.WriteString(`\5c`)
		case 0:
			escaped.WriteString(`\00`)
		default:
			escaped.WriteRune(r)
		}
	}
	return escaped.String()
}
