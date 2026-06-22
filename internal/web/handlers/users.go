package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

type UserHandler struct {
	store     store.Store
	cfg       *config.Config
	templates TemplateGetter
	hasher    *crypto.PasswordHasher
}

var userFormAttributes = []string{"uid", "cn", "sn", "givenName", "mail"}
var userFormExcludeAttributes = []string{"uid", "cn", "sn", "givenName", "mail", "objectClass", "userPassword", "createTimestamp", "modifyTimestamp", "memberOf"}

func NewUserHandler(st store.Store, cfg *config.Config, getter TemplateGetter) *UserHandler {
	return &UserHandler{
		store:     st,
		cfg:       cfg,
		templates: getter,
		hasher:    crypto.NewPasswordHasher(cfg.Security.Argon2Config),
	}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all users from all OUs (search recursively from base DN)
	entries, err := searchEntriesWithoutMemberOf(ctx, h.store, h.cfg.LDAP.BaseDN, "(objectClass=inetOrgPerson)")
	if err != nil {
		slog.Error("Failed to search users", "error", err)
		entries = []*models.Entry{}
	}

	data := struct {
		BaseData
		Users []*models.Entry
	}{
		BaseData: NewBaseData(h.cfg, r, "users"),
		Users:    entries,
	}

	RenderTemplate(w, h.templates, "users.html", data)
}

func (h *UserHandler) New(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.create(w, r)
		return
	}

	ctx := r.Context()
	ous := loadOrganizationalUnits(ctx, h.store, h.cfg.LDAP.BaseDN)

	data := struct {
		BaseData
		User            *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData: NewBaseData(h.cfg, r, "users"),
		OUs:      ous,
	}

	RenderTemplate(w, h.templates, "user_form.html", data)
}

func (h *UserHandler) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		auditWebWrite(r, "create", "user", "", http.StatusBadRequest, err)
		h.showError(w, r, "Invalid form data", nil)
		return
	}

	parentDN := strings.TrimSpace(r.FormValue("parentDN"))
	uid := strings.TrimSpace(r.FormValue("uid"))
	cn := strings.TrimSpace(r.FormValue("cn"))
	sn := strings.TrimSpace(r.FormValue("sn"))
	givenName := strings.TrimSpace(r.FormValue("givenName"))
	mail := strings.TrimSpace(r.FormValue("mail"))
	password := r.FormValue("userPassword")

	if parentDN == "" || uid == "" || cn == "" || sn == "" || password == "" {
		auditWebWrite(r, "create", "user", "", http.StatusBadRequest, fmt.Errorf("required user fields missing"))
		h.showError(w, r, "Parent OU, UID, CN, SN, and password are required", nil)
		return
	}

	// Create user
	user := models.NewUser(parentDN, uid, cn, sn, mail)
	if givenName != "" {
		user.SetAttribute("givenName", givenName)
	}

	// Hash password
	hashedPassword, err := h.hasher.Hash(password)
	if err != nil {
		auditWebWrite(r, "create", "user", user.DN, http.StatusInternalServerError, err)
		h.showError(w, r, "Failed to hash password", nil)
		return
	}
	user.SetPassword(hashedPassword)

	// Add extra attributes
	addExtraAttributes(user.Entry, ParseAttributes(r.FormValue("attributes")))

	if err := h.store.CreateEntry(ctx, user.Entry); err != nil {
		auditWebWrite(r, "create", "user", user.DN, http.StatusInternalServerError, err)
		h.showError(w, r, fmt.Sprintf("Failed to create user: %v", err), nil)
		return
	}

	auditWebWrite(r, "create", "user", user.DN, http.StatusFound, nil)
	redirectWithMessage(w, r, "/users", "success", "User created successfully")
}

func (h *UserHandler) Edit(w http.ResponseWriter, r *http.Request) {
	dn, ok := queryDN(w, r)
	if !ok {
		return
	}

	if r.Method == http.MethodPost {
		h.update(w, r, dn)
		return
	}

	ctx := r.Context()
	entry, err := getEntryWithoutMemberOf(ctx, h.store, dn)
	if err != nil {
		h.showError(w, r, fmt.Sprintf("User not found: %v", err), nil)
		return
	}

	ous := loadOrganizationalUnits(ctx, h.store, h.cfg.LDAP.BaseDN)

	data := struct {
		BaseData
		User            *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData:        NewBaseData(h.cfg, r, "users"),
		User:            entry,
		ExtraAttributes: formatExtraAttributesForForm(entry, userFormExcludeAttributes),
		OUs:             ous,
	}

	RenderTemplate(w, h.templates, "user_form.html", data)
}

func (h *UserHandler) update(w http.ResponseWriter, r *http.Request, dn string) {
	ctx := r.Context()

	entry, err := getEntryWithoutMemberOf(ctx, h.store, dn)
	if err != nil {
		auditWebWrite(r, "update", "user", dn, http.StatusNotFound, err)
		h.showError(w, r, fmt.Sprintf("User not found: %v", err), nil)
		return
	}

	if err := r.ParseForm(); err != nil {
		auditWebWrite(r, "update", "user", dn, http.StatusBadRequest, err)
		h.showError(w, r, "Invalid form data", entry)
		return
	}

	// Update basic attributes
	entry.SetAttribute("cn", strings.TrimSpace(r.FormValue("cn")))
	entry.SetAttribute("sn", strings.TrimSpace(r.FormValue("sn")))

	setOptionalAttribute(entry, "givenName", r.FormValue("givenName"))
	setOptionalAttribute(entry, "mail", r.FormValue("mail"))

	// Parse and add new custom attributes
	extraAttrs := ParseAttributes(r.FormValue("attributes"))
	ReplaceExtraAttributes(entry, userFormAttributes, extraAttrs)

	// Update password if provided
	password := r.FormValue("userPassword")
	if password != "" {
		hashedPassword, err := h.hasher.Hash(password)
		if err != nil {
			auditWebWrite(r, "update", "user", dn, http.StatusInternalServerError, err)
			h.showError(w, r, "Failed to hash password", entry)
			return
		}
		entry.SetAttribute("userPassword", hashedPassword)
	}

	if err := h.store.UpdateEntry(ctx, entry); err != nil {
		auditWebWrite(r, "update", "user", dn, http.StatusInternalServerError, err)
		h.showError(w, r, fmt.Sprintf("Failed to update user: %v", err), entry)
		return
	}

	auditWebWrite(r, "update", "user", dn, http.StatusFound, nil)
	redirectWithMessage(w, r, "/users", "success", "User updated successfully")
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	deleteEntry(w, r, h.store, "/users", "user", "User")
}

func (h *UserHandler) showError(w http.ResponseWriter, r *http.Request, errMsg string, user *models.Entry) {
	ctx := r.Context()

	ous := loadOrganizationalUnits(ctx, h.store, h.cfg.LDAP.BaseDN)

	data := struct {
		BaseData
		User            *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData:        NewBaseDataWithError(h.cfg, r, "users", errMsg),
		User:            user,
		ExtraAttributes: formatExtraAttributesForForm(user, userFormExcludeAttributes),
		OUs:             ous,
	}

	RenderTemplate(w, h.templates, "user_form.html", data)
}
