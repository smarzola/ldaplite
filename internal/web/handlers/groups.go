package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
)

type GroupHandler struct {
	store     store.Store
	cfg       *config.Config
	templates TemplateGetter
}

func NewGroupHandler(st store.Store, cfg *config.Config, getter TemplateGetter) *GroupHandler {
	return &GroupHandler{
		store:     st,
		cfg:       cfg,
		templates: getter,
	}
}

func (h *GroupHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all groups from all OUs (search recursively from base DN)
	entries, err := h.store.SearchEntries(ctx, h.cfg.LDAP.BaseDN, "(objectClass=groupOfNames)")
	if err != nil {
		slog.Error("Failed to search groups", "error", err)
		entries = []*models.Entry{}
	}

	data := struct {
		BaseData
		Groups []*models.Entry
	}{
		BaseData: NewBaseData(h.cfg, r, "groups"),
		Groups:   entries,
	}

	RenderTemplate(w, h.templates, "groups.html", data)
}

func (h *GroupHandler) New(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.create(w, r)
		return
	}

	ctx := r.Context()

	// Fetch all OUs for parent selection
	ous, err := h.store.SearchEntries(ctx, h.cfg.LDAP.BaseDN, "(objectClass=organizationalUnit)")
	if err != nil {
		slog.Error("Failed to fetch OUs", "error", err)
		ous = []*models.Entry{}
	}

	data := struct {
		BaseData
		Group           *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData: NewBaseData(h.cfg, r, "groups"),
		OUs:      ous,
	}

	RenderTemplate(w, h.templates, "group_form.html", data)
}

func (h *GroupHandler) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.showError(w, r, "Invalid form data", nil)
		return
	}

	parentDN := strings.TrimSpace(r.FormValue("parentDN"))
	cn := strings.TrimSpace(r.FormValue("cn"))
	description := strings.TrimSpace(r.FormValue("description"))
	membersInput := r.FormValue("member")

	if parentDN == "" || cn == "" {
		h.showError(w, r, "Parent OU and CN are required", nil)
		return
	}

	// Parse members
	var members []string
	for _, line := range strings.Split(membersInput, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			members = append(members, line)
		}
	}

	if len(members) == 0 {
		h.showError(w, r, "At least one member is required", nil)
		return
	}

	group := models.NewGroup(parentDN, cn, description)
	for _, member := range members {
		group.AddMember(member)
	}

	// Add extra attributes
	extraAttrs := ParseAttributes(r.FormValue("attributes"))
	for name, values := range extraAttrs {
		for _, value := range values {
			group.AddAttribute(name, value)
		}
	}

	if err := h.store.CreateEntry(ctx, group.Entry); err != nil {
		h.showError(w, r, fmt.Sprintf("Failed to create group: %v", err), nil)
		return
	}

	http.Redirect(w, r, "/groups?success=Group created successfully", http.StatusFound)
}

func (h *GroupHandler) Edit(w http.ResponseWriter, r *http.Request) {
	dn := r.URL.Query().Get("dn")
	if dn == "" {
		http.Error(w, "DN parameter required", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		h.update(w, r, dn)
		return
	}

	ctx := r.Context()
	entry, err := h.store.GetEntry(ctx, dn)
	if err != nil {
		h.showError(w, r, fmt.Sprintf("Group not found: %v", err), nil)
		return
	}

	// Fetch all OUs for parent selection
	ous, err := h.store.SearchEntries(ctx, h.cfg.LDAP.BaseDN, "(objectClass=organizationalUnit)")
	if err != nil {
		slog.Error("Failed to fetch OUs", "error", err)
		ous = []*models.Entry{}
	}

	exclude := []string{"cn", "description", "member", "objectClass", "createTimestamp", "modifyTimestamp"}
	data := struct {
		BaseData
		Group           *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData:        NewBaseData(h.cfg, r, "groups"),
		Group:           entry,
		ExtraAttributes: FormatExtraAttributes(entry, exclude),
		OUs:             ous,
	}

	RenderTemplate(w, h.templates, "group_form.html", data)
}

func (h *GroupHandler) update(w http.ResponseWriter, r *http.Request, dn string) {
	ctx := r.Context()

	entry, err := h.store.GetEntry(ctx, dn)
	if err != nil {
		h.showError(w, r, fmt.Sprintf("Group not found: %v", err), nil)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.showError(w, r, "Invalid form data", entry)
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))
	if description != "" {
		entry.SetAttribute("description", description)
	}

	// Update members
	membersInput := r.FormValue("member")
	var members []string
	for _, line := range strings.Split(membersInput, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			members = append(members, line)
		}
	}

	if len(members) > 0 {
		entry.Attributes["member"] = members
	}

	// Update extra attributes
	extraAttrs := ParseAttributes(r.FormValue("attributes"))
	for name, values := range extraAttrs {
		entry.Attributes[name] = values
	}

	entry.UpdatedAt = time.Now()

	if err := h.store.UpdateEntry(ctx, entry); err != nil {
		h.showError(w, r, fmt.Sprintf("Failed to update group: %v", err), entry)
		return
	}

	http.Redirect(w, r, "/groups?success=Group updated successfully", http.StatusFound)
}

func (h *GroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	dn := r.URL.Query().Get("dn")
	if dn == "" {
		http.Error(w, "DN parameter required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := h.store.DeleteEntry(ctx, dn); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/groups?error=Failed to delete group: %v", err), http.StatusFound)
		return
	}

	http.Redirect(w, r, "/groups?success=Group deleted successfully", http.StatusFound)
}

func (h *GroupHandler) showError(w http.ResponseWriter, r *http.Request, errMsg string, group *models.Entry) {
	ctx := r.Context()

	// Fetch all OUs for parent selection
	ous, err := h.store.SearchEntries(ctx, h.cfg.LDAP.BaseDN, "(objectClass=organizationalUnit)")
	if err != nil {
		slog.Error("Failed to fetch OUs", "error", err)
		ous = []*models.Entry{}
	}

	exclude := []string{"cn", "description", "member", "objectClass", "createTimestamp", "modifyTimestamp"}
	extraAttrs := ""
	if group != nil {
		extraAttrs = FormatExtraAttributes(group, exclude)
	}

	data := struct {
		BaseData
		Group           *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData:        func() BaseData { bd := NewBaseData(h.cfg, r, "groups"); bd.Error = errMsg; return bd }(),
		Group:           group,
		ExtraAttributes: extraAttrs,
		OUs:             ous,
	}

	RenderTemplate(w, h.templates, "group_form.html", data)
}
