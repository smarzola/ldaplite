package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/smarzola/ldaplite/internal/authz"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/internal/web/middleware"
	"github.com/smarzola/ldaplite/pkg/config"
)

type APIHandler struct {
	store store.Store
	cfg   *config.Config
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
	ObjectClass string   `json:"objectClass"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Mail        string   `json:"mail,omitempty"`
	Members     []string `json:"members,omitempty"`
	MemberOf    []string `json:"memberOf,omitempty"`
}

func NewAPIHandler(st store.Store, cfg *config.Config) *APIHandler {
	return &APIHandler{store: st, cfg: cfg}
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
