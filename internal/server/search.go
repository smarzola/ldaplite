package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/lor00x/goldap/message"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/protocol"
	"github.com/smarzola/ldaplite/internal/store"
)

// handleSearch handles search operations
func (s *Server) handleSearch(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	searchReq := msg.ProtocolOp().(message.SearchRequest)
	baseDN := string(searchReq.BaseObject())
	scope := ldapSearchScope(searchReq.Scope())
	selection := newSearchAttributeSelection(searchReq.Attributes())

	// Handle RootDSE queries (empty base DN)
	if baseDN == "" {
		slog.Debug("RootDSE query")
		return s.handleRootDSE(conn, msg)
	}

	// Handle schema queries
	if baseDN == "cn=Subschema" || baseDN == "cn=subschema" {
		slog.Debug("Schema query")
		return s.handleSchema(conn, msg)
	}

	if !s.canSearch(conn, baseDN) {
		slog.Info("Search rejected - bind required", "baseDN", baseDN)
		return conn.WriteResponse(msg.MessageID(), protocol.NewSearchResultDone(message.ResultCodeInsufficientAccessRights))
	}

	// Get filter from request
	filterStr := serializeFilter(searchReq.Filter())
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
		return conn.WriteResponse(msg.MessageID(), protocol.NewSearchResultDone(message.ResultCodeOperationsError))
	}

	// Return matching entries
	for _, entry := range entries {
		// Build search result entry
		result := protocol.NewSearchResultEntry(entry.DN)

		for _, attr := range searchResponseAttributes(entry, selection) {
			addSearchAttribute(&result, attr.name, attr.values, bool(searchReq.TypesOnly()))
		}

		// Write entry
		if err := conn.WriteResponse(msg.MessageID(), result); err != nil {
			return err
		}
	}

	slog.Debug("Search completed", "baseDN", baseDN, "results", len(entries))
	return conn.WriteResponse(msg.MessageID(), protocol.NewSearchResultDone(message.ResultCodeSuccess))
}

func ldapSearchScope(scope message.ENUMERATED) store.SearchScope {
	switch int(scope) {
	case 0:
		return store.SearchScopeBaseObject
	case 1:
		return store.SearchScopeSingleLevel
	default:
		return store.SearchScopeWholeSubtree
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

func newSearchAttributeSelection(selection message.AttributeSelection) searchAttributeSelection {
	if len(selection) == 0 {
		return searchAttributeSelection{includeAll: true, includeOperational: true}
	}

	result := searchAttributeSelection{names: make(map[string]bool)}
	for _, selector := range selection {
		name := strings.TrimSpace(string(selector))
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
	attrs := make([]searchResponseAttribute, 0, len(entry.Attributes)+3)
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
	case "objectclass", "createtimestamp", "modifytimestamp":
		return true
	default:
		return false
	}
}

func addSearchAttribute(result *message.SearchResultEntry, name string, values []string, typesOnly bool) {
	if typesOnly {
		protocol.AddAttribute(result, name)
		return
	}
	protocol.AddAttribute(result, name, values...)
}

// serializeFilter converts a goldap Filter to LDAP filter string
func serializeFilter(f interface{}) string {
	if f == nil {
		return ""
	}

	switch filter := f.(type) {
	case message.FilterEqualityMatch:
		return fmt.Sprintf("(%s=%s)", filter.AttributeDesc(), filter.AssertionValue())

	case message.FilterPresent:
		return fmt.Sprintf("(%s=*)", string(filter))

	case message.FilterAnd:
		if len(filter) == 0 {
			return ""
		}
		var parts []string
		for _, subFilter := range filter {
			parts = append(parts, serializeFilter(subFilter))
		}
		return "(&" + strings.Join(parts, "") + ")"

	case message.FilterOr:
		if len(filter) == 0 {
			return ""
		}
		var parts []string
		for _, subFilter := range filter {
			parts = append(parts, serializeFilter(subFilter))
		}
		return "(|" + strings.Join(parts, "") + ")"

	case message.FilterNot:
		return "(!" + serializeFilter(filter.Filter) + ")"

	case message.FilterGreaterOrEqual:
		return fmt.Sprintf("(%s>=%s)", filter.AttributeDesc(), filter.AssertionValue())

	case message.FilterLessOrEqual:
		return fmt.Sprintf("(%s<=%s)", filter.AttributeDesc(), filter.AssertionValue())

	case message.FilterApproxMatch:
		return fmt.Sprintf("(%s~=%s)", filter.AttributeDesc(), filter.AssertionValue())

	case message.FilterSubstrings:
		attr := string(filter.Type_())
		var sb strings.Builder
		sb.WriteString("(")
		sb.WriteString(attr)
		sb.WriteString("=")

		for _, sub := range filter.Substrings() {
			switch s := sub.(type) {
			case message.SubstringInitial:
				sb.WriteString(string(s))
				sb.WriteString("*")
			case message.SubstringAny:
				sb.WriteString(string(s))
				sb.WriteString("*")
			case message.SubstringFinal:
				sb.WriteString(string(s))
			}
		}
		sb.WriteString(")")
		return sb.String()

	default:
		str := fmt.Sprintf("%v", f)
		if str != "" && str[0] == '(' {
			return str
		}
		return "(objectClass=*)"
	}
}
