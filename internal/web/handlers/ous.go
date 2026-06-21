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

type OUHandler struct {
	store     store.Store
	cfg       *config.Config
	templates TemplateGetter
}

var ouFormAttributes = []string{"ou", "description"}
var ouFormExcludeAttributes = []string{"ou", "description", "objectClass", "createTimestamp", "modifyTimestamp", "memberOf"}

func NewOUHandler(st store.Store, cfg *config.Config, getter TemplateGetter) *OUHandler {
	return &OUHandler{
		store:     st,
		cfg:       cfg,
		templates: getter,
	}
}

func (h *OUHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	entries, err := searchEntriesWithoutMemberOf(ctx, h.store, h.cfg.LDAP.BaseDN, "(objectClass=organizationalUnit)")
	if err != nil {
		slog.Error("Failed to search OUs", "error", err)
		entries = []*models.Entry{}
	}

	data := struct {
		BaseData
		OUs []*models.Entry
	}{
		BaseData: NewBaseData(h.cfg, r, "ous"),
		OUs:      entries,
	}

	RenderTemplate(w, h.templates, "ous.html", data)
}

func (h *OUHandler) New(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.create(w, r)
		return
	}

	ctx := r.Context()
	ous := loadOrganizationalUnits(ctx, h.store, h.cfg.LDAP.BaseDN)

	data := struct {
		BaseData
		OU              *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData: NewBaseData(h.cfg, r, "ous"),
		OUs:      ous,
	}

	RenderTemplate(w, h.templates, "ou_form.html", data)
}

func (h *OUHandler) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.showError(w, r, "Invalid form data", nil)
		return
	}

	parentDN := strings.TrimSpace(r.FormValue("parentDN"))
	ou := strings.TrimSpace(r.FormValue("ou"))
	description := strings.TrimSpace(r.FormValue("description"))

	if parentDN == "" || ou == "" {
		h.showError(w, r, "Parent DN and OU name are required", nil)
		return
	}

	ouEntry := models.NewOrganizationalUnit(parentDN, ou, description)

	// Add extra attributes
	addExtraAttributes(ouEntry.Entry, ParseAttributes(r.FormValue("attributes")))

	if err := h.store.CreateEntry(ctx, ouEntry.Entry); err != nil {
		h.showError(w, r, fmt.Sprintf("Failed to create OU: %v", err), nil)
		return
	}

	redirectWithMessage(w, r, "/ous", "success", "OU created successfully")
}

func (h *OUHandler) Edit(w http.ResponseWriter, r *http.Request) {
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
	entry, err := getEntryWithoutMemberOf(ctx, h.store, dn)
	if err != nil {
		h.showError(w, r, fmt.Sprintf("OU not found: %v", err), nil)
		return
	}

	ous := loadOrganizationalUnits(ctx, h.store, h.cfg.LDAP.BaseDN)

	data := struct {
		BaseData
		OU              *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData:        NewBaseData(h.cfg, r, "ous"),
		OU:              entry,
		ExtraAttributes: FormatExtraAttributes(entry, ouFormExcludeAttributes),
		OUs:             ous,
	}

	RenderTemplate(w, h.templates, "ou_form.html", data)
}

func (h *OUHandler) update(w http.ResponseWriter, r *http.Request, dn string) {
	ctx := r.Context()

	entry, err := getEntryWithoutMemberOf(ctx, h.store, dn)
	if err != nil {
		h.showError(w, r, fmt.Sprintf("OU not found: %v", err), nil)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.showError(w, r, "Invalid form data", entry)
		return
	}

	setOptionalAttribute(entry, "description", r.FormValue("description"))

	// Update extra attributes
	extraAttrs := ParseAttributes(r.FormValue("attributes"))
	ReplaceExtraAttributes(entry, ouFormAttributes, extraAttrs)

	entry.UpdatedAt = time.Now()

	if err := h.store.UpdateEntry(ctx, entry); err != nil {
		h.showError(w, r, fmt.Sprintf("Failed to update OU: %v", err), entry)
		return
	}

	redirectWithMessage(w, r, "/ous", "success", "OU updated successfully")
}

func (h *OUHandler) Delete(w http.ResponseWriter, r *http.Request) {
	deleteEntry(w, r, h.store, "/ous", "OU", "OU")
}

func (h *OUHandler) showError(w http.ResponseWriter, r *http.Request, errMsg string, ou *models.Entry) {
	ctx := r.Context()

	ous := loadOrganizationalUnits(ctx, h.store, h.cfg.LDAP.BaseDN)

	extraAttrs := ""
	if ou != nil {
		extraAttrs = FormatExtraAttributes(ou, ouFormExcludeAttributes)
	}

	data := struct {
		BaseData
		OU              *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData:        func() BaseData { bd := NewBaseData(h.cfg, r, "ous"); bd.Error = errMsg; return bd }(),
		OU:              ou,
		ExtraAttributes: extraAttrs,
		OUs:             ous,
	}

	RenderTemplate(w, h.templates, "ou_form.html", data)
}
