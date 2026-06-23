package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/smarzola/ldaplite/internal/authz"
	"github.com/smarzola/ldaplite/internal/directory"
	"github.com/smarzola/ldaplite/internal/ldapdn"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/internal/web/middleware"
	"github.com/smarzola/ldaplite/pkg/config"
)

type APIHandler struct {
	store   store.Store
	cfg     *config.Config
	service *directory.Service
}

type sessionResponse struct {
	BaseDN       string   `json:"baseDN"`
	UserDN       string   `json:"userDN"`
	UserID       string   `json:"userID"`
	Capabilities []string `json:"capabilities"`
	Roles        roles    `json:"roles"`
}

type roles struct {
	Admin          bool `json:"admin"`
	DirectoryRead  bool `json:"directoryRead"`
	DirectoryWrite bool `json:"directoryWrite"`
	PasswordSelf   bool `json:"passwordSelf"`
	PasswordReset  bool `json:"passwordReset"`
}

type directoryResponse struct {
	BaseDN string         `json:"baseDN"`
	Users  []entrySummary `json:"users"`
	Groups []entrySummary `json:"groups"`
	OUs    []entrySummary `json:"ous"`
}

type entrySummary struct {
	DN          string   `json:"dn"`
	Type        string   `json:"type"`
	ObjectClass string   `json:"objectClass"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Mail        string   `json:"mail,omitempty"`
	Members     []string `json:"members,omitempty"`
	MemberOf    []string `json:"memberOf,omitempty"`
}

type directorySearchResponse struct {
	BaseDN     string         `json:"baseDN"`
	Query      string         `json:"query"`
	Type       string         `json:"type"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	Total      int            `json:"total"`
	TotalPages int            `json:"totalPages"`
	Entries    []entrySummary `json:"entries"`
}

type directoryDetailResponse struct {
	BaseDN string      `json:"baseDN"`
	Entry  entryDetail `json:"entry"`
}

type entryDetail struct {
	DN          string              `json:"dn"`
	Type        string              `json:"type"`
	ObjectClass string              `json:"objectClass"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Mail        string              `json:"mail,omitempty"`
	Members     []string            `json:"members,omitempty"`
	MemberOf    []string            `json:"memberOf,omitempty"`
	Attributes  map[string][]string `json:"attributes"`
	CreatedAt   string              `json:"createdAt,omitempty"`
	UpdatedAt   string              `json:"updatedAt,omitempty"`
}

func NewAPIHandler(st store.Store, cfg *config.Config) *APIHandler {
	return &APIHandler{
		store:   st,
		cfg:     cfg,
		service: directory.NewService(st, cfg),
	}
}

func (h *APIHandler) Session(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	capabilities := middleware.GetCapabilities(r)
	writeJSON(w, sessionResponse{
		BaseDN:       h.cfg.LDAP.BaseDN,
		UserDN:       middleware.GetUserDN(r),
		UserID:       uidFromDN(middleware.GetUserDN(r)),
		Capabilities: capabilityStrings(capabilities),
		Roles: roles{
			Admin:          capabilities.Has(authz.UIAdmin),
			DirectoryRead:  capabilities.Has(authz.DirectoryRead),
			DirectoryWrite: capabilities.Has(authz.DirectoryWrite),
			PasswordSelf:   capabilities.Has(authz.PasswordChangeSelf),
			PasswordReset:  capabilities.Has(authz.PasswordResetAny),
		},
	})
}

func (h *APIHandler) DirectorySearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	entryType := normalizeDirectoryType(r.URL.Query().Get("type"))
	if entryType == "" {
		http.Error(w, "Unsupported directory type", http.StatusBadRequest)
		return
	}
	page, pageSize, ok := parsePagination(w, r)
	if !ok {
		return
	}

	entries, err := h.store.SearchEntriesWithOptions(r.Context(), store.SearchOptions{
		BaseDN:          h.cfg.LDAP.BaseDN,
		Filter:          directoryTypeFilter(entryType),
		Scope:           store.SearchScopeWholeSubtree,
		IncludeMemberOf: true,
	})
	if err != nil {
		http.Error(w, "Failed to search directory", http.StatusInternalServerError)
		return
	}

	summaries := make([]entrySummary, 0, len(entries))
	for _, entry := range entries {
		if !matchesDirectoryQuery(entry, query) {
			continue
		}
		summaries = append(summaries, summarizeEntry(entry))
	}
	sort.Slice(summaries, func(i, j int) bool {
		return strings.ToLower(summaries[i].DN) < strings.ToLower(summaries[j].DN)
	})

	total := len(summaries)
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	writeJSON(w, directorySearchResponse{
		BaseDN:     h.cfg.LDAP.BaseDN,
		Query:      query,
		Type:       entryType,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		Entries:    summaries[start:end],
	})
}

func (h *APIHandler) DirectoryEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dn := strings.TrimSpace(r.URL.Query().Get("dn"))
	if dn == "" {
		http.Error(w, "DN parameter required", http.StatusBadRequest)
		return
	}
	if !ldapdn.WithinBase(dn, h.cfg.LDAP.BaseDN) {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	entry, err := h.store.GetEntryWithOptions(r.Context(), dn, store.EntryOptions{IncludeMemberOf: true})
	if err != nil {
		if errors.Is(err, store.ErrNoSuchObject) {
			http.Error(w, "Entry not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to load entry", http.StatusInternalServerError)
		return
	}
	if entry == nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	writeJSON(w, directoryDetailResponse{
		BaseDN: h.cfg.LDAP.BaseDN,
		Entry:  detailEntry(entry),
	})
}

func (h *APIHandler) Directory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	users, err := h.searchSummaries(ctx, "(objectClass=inetOrgPerson)")
	if err != nil {
		http.Error(w, "Failed to load users", http.StatusInternalServerError)
		return
	}
	groups, err := h.searchSummaries(ctx, "(objectClass=groupOfNames)")
	if err != nil {
		http.Error(w, "Failed to load groups", http.StatusInternalServerError)
		return
	}
	ous, err := h.searchSummaries(ctx, "(objectClass=organizationalUnit)")
	if err != nil {
		http.Error(w, "Failed to load organizational units", http.StatusInternalServerError)
		return
	}

	writeJSON(w, directoryResponse{
		BaseDN: h.cfg.LDAP.BaseDN,
		Users:  users,
		Groups: groups,
		OUs:    ous,
	})
}

func (h *APIHandler) searchSummaries(ctx context.Context, filter string) ([]entrySummary, error) {
	entries, err := h.store.SearchEntriesWithOptions(ctx, store.SearchOptions{
		BaseDN:          h.cfg.LDAP.BaseDN,
		Filter:          filter,
		Scope:           store.SearchScopeWholeSubtree,
		IncludeMemberOf: true,
	})
	if err != nil {
		return nil, err
	}

	summaries := make([]entrySummary, 0, len(entries))
	for _, entry := range entries {
		summaries = append(summaries, summarizeEntry(entry))
	}
	sort.Slice(summaries, func(i, j int) bool {
		return strings.ToLower(summaries[i].DN) < strings.ToLower(summaries[j].DN)
	})
	return summaries, nil
}

func summarizeEntry(entry *models.Entry) entrySummary {
	summary := entrySummary{
		DN:          entry.DN,
		Type:        directoryEntryType(entry),
		ObjectClass: entry.ObjectClass,
		Description: entry.GetAttribute("description"),
		Members:     entry.GetAttributes("member"),
		MemberOf:    entry.GetAttributes("memberOf"),
	}

	switch {
	case entry.IsUser():
		summary.Name = entry.GetAttribute("uid")
		summary.Mail = entry.GetAttribute("mail")
	case entry.IsGroup():
		summary.Name = entry.GetAttribute("cn")
	case entry.IsOrganizationalUnit():
		summary.Name = entry.GetAttribute("ou")
	default:
		summary.Name = entry.GetRDN()
	}
	return summary
}

func detailEntry(entry *models.Entry) entryDetail {
	summary := summarizeEntry(entry)
	detail := entryDetail{
		DN:          summary.DN,
		Type:        summary.Type,
		ObjectClass: summary.ObjectClass,
		Name:        summary.Name,
		Description: summary.Description,
		Mail:        summary.Mail,
		Members:     summary.Members,
		MemberOf:    summary.MemberOf,
		Attributes:  safeAttributes(entry),
	}
	if !entry.CreatedAt.IsZero() {
		detail.CreatedAt = entry.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if !entry.UpdatedAt.IsZero() {
		detail.UpdatedAt = entry.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return detail
}

func safeAttributes(entry *models.Entry) map[string][]string {
	attrs := make(map[string][]string)
	for name, values := range entry.Attributes {
		if strings.EqualFold(name, "userPassword") {
			continue
		}
		attrs[strings.ToLower(name)] = append([]string(nil), values...)
	}
	for name, values := range entry.ComputedAttributes {
		if strings.EqualFold(name, "userPassword") {
			continue
		}
		attrs[strings.ToLower(name)] = append([]string(nil), values...)
	}
	return attrs
}

func parsePagination(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	page := 1
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			http.Error(w, "Page must be a positive integer", http.StatusBadRequest)
			return 0, 0, false
		}
		page = value
	}

	pageSize := 25
	if raw := strings.TrimSpace(r.URL.Query().Get("pageSize")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			http.Error(w, "Page size must be between 1 and 100", http.StatusBadRequest)
			return 0, 0, false
		}
		pageSize = value
	}

	return page, pageSize, true
}

func normalizeDirectoryType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return "all"
	case "user", "users":
		return "users"
	case "group", "groups":
		return "groups"
	case "ou", "ous", "organizationalunit", "organizationalunits", "organizational-units":
		return "ous"
	default:
		return ""
	}
}

func directoryTypeFilter(entryType string) string {
	switch entryType {
	case "users":
		return "(objectClass=inetOrgPerson)"
	case "groups":
		return "(objectClass=groupOfNames)"
	case "ous":
		return "(objectClass=organizationalUnit)"
	default:
		return "(objectClass=*)"
	}
}

func directoryEntryType(entry *models.Entry) string {
	switch {
	case entry.IsUser():
		return "user"
	case entry.IsGroup():
		return "group"
	case entry.IsOrganizationalUnit():
		return "ou"
	default:
		return "entry"
	}
}

func matchesDirectoryQuery(entry *models.Entry, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}

	values := []string{
		entry.DN,
		entry.ObjectClass,
		entry.GetRDN(),
		entry.GetAttribute("uid"),
		entry.GetAttribute("cn"),
		entry.GetAttribute("mail"),
		entry.GetAttribute("ou"),
		entry.GetAttribute("description"),
	}
	values = append(values, entry.GetAttributes("member")...)
	values = append(values, entry.GetAttributes("memberOf")...)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func capabilityStrings(capabilities authz.Set) []string {
	values := make([]string, 0, len(capabilities))
	for capability := range capabilities {
		values = append(values, string(capability))
	}
	sort.Strings(values)
	return values
}

func uidFromDN(dn string) string {
	rdn, _, _ := strings.Cut(dn, ",")
	attr, value, ok := strings.Cut(rdn, "=")
	if ok && strings.EqualFold(attr, "uid") {
		return value
	}
	return dn
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
