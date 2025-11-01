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
	"github.com/smarzola/ldaplite/pkg/crypto"
)

type UserHandler struct {
	store     store.Store
	cfg       *config.Config
	templates TemplateGetter
	hasher    *crypto.PasswordHasher
}

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
	entries, err := h.store.SearchEntries(ctx, h.cfg.LDAP.BaseDN, "(objectClass=inetOrgPerson)")
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

	// Fetch all OUs for parent selection, including base DN
	ous, err := h.store.SearchEntries(ctx, h.cfg.LDAP.BaseDN, "(objectClass=organizationalUnit)")
	if err != nil {
		slog.Error("Failed to fetch OUs", "error", err)
		ous = []*models.Entry{}
	}

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
		h.showError(w, r, "Failed to hash password", nil)
		return
	}
	user.SetPassword(hashedPassword)

	// Add extra attributes
	extraAttrs := ParseAttributes(r.FormValue("attributes"))
	for name, values := range extraAttrs {
		for _, value := range values {
			user.AddAttribute(name, value)
		}
	}

	if err := h.store.CreateEntry(ctx, user.Entry); err != nil {
		h.showError(w, r, fmt.Sprintf("Failed to create user: %v", err), nil)
		return
	}

	http.Redirect(w, r, "/users?success=User created successfully", http.StatusFound)
}

func (h *UserHandler) Edit(w http.ResponseWriter, r *http.Request) {
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
		h.showError(w, r, fmt.Sprintf("User not found: %v", err), nil)
		return
	}

	// Fetch all OUs for parent selection
	ous, err := h.store.SearchEntries(ctx, h.cfg.LDAP.BaseDN, "(objectClass=organizationalUnit)")
	if err != nil {
		slog.Error("Failed to fetch OUs", "error", err)
		ous = []*models.Entry{}
	}

	exclude := []string{"uid", "cn", "sn", "givenName", "mail", "objectClass", "userPassword", "createTimestamp", "modifyTimestamp"}
	data := struct {
		BaseData
		User            *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData:        NewBaseData(h.cfg, r, "users"),
		User:            entry,
		ExtraAttributes: FormatExtraAttributes(entry, exclude),
		OUs:             ous,
	}

	RenderTemplate(w, h.templates, "user_form.html", data)
}

func (h *UserHandler) update(w http.ResponseWriter, r *http.Request, dn string) {
	ctx := r.Context()

	entry, err := h.store.GetEntry(ctx, dn)
	if err != nil {
		h.showError(w, r, fmt.Sprintf("User not found: %v", err), nil)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.showError(w, r, "Invalid form data", entry)
		return
	}

	// Update basic attributes
	entry.SetAttribute("cn", strings.TrimSpace(r.FormValue("cn")))
	entry.SetAttribute("sn", strings.TrimSpace(r.FormValue("sn")))

	givenName := strings.TrimSpace(r.FormValue("givenName"))
	if givenName != "" {
		entry.SetAttribute("givenName", givenName)
	}

	mail := strings.TrimSpace(r.FormValue("mail"))
	if mail != "" {
		entry.SetAttribute("mail", mail)
	}

	// Update password if provided
	password := r.FormValue("userPassword")
	if password != "" {
		hashedPassword, err := h.hasher.Hash(password)
		if err != nil {
			h.showError(w, r, "Failed to hash password", entry)
			return
		}
		entry.SetAttribute("userPassword", hashedPassword)
	}

	// Update extra attributes (excluded fields are kept as-is in the entry)
	_ = []string{"uid", "cn", "sn", "givenName", "mail", "objectClass", "userPassword", "createTimestamp", "modifyTimestamp"}

	// Parse and add new custom attributes
	extraAttrs := ParseAttributes(r.FormValue("attributes"))
	for name, values := range extraAttrs {
		entry.Attributes[name] = values
	}

	entry.UpdatedAt = time.Now()

	if err := h.store.UpdateEntry(ctx, entry); err != nil {
		h.showError(w, r, fmt.Sprintf("Failed to update user: %v", err), entry)
		return
	}

	http.Redirect(w, r, "/users?success=User updated successfully", http.StatusFound)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	dn := r.URL.Query().Get("dn")
	if dn == "" {
		http.Error(w, "DN parameter required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := h.store.DeleteEntry(ctx, dn); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/users?error=Failed to delete user: %v", err), http.StatusFound)
		return
	}

	http.Redirect(w, r, "/users?success=User deleted successfully", http.StatusFound)
}

func (h *UserHandler) showError(w http.ResponseWriter, r *http.Request, errMsg string, user *models.Entry) {
	ctx := r.Context()

	// Fetch all OUs for parent selection
	ous, err := h.store.SearchEntries(ctx, h.cfg.LDAP.BaseDN, "(objectClass=organizationalUnit)")
	if err != nil {
		slog.Error("Failed to fetch OUs", "error", err)
		ous = []*models.Entry{}
	}

	exclude := []string{"uid", "cn", "sn", "givenName", "mail", "objectClass", "userPassword", "createTimestamp", "modifyTimestamp"}
	extraAttrs := ""
	if user != nil {
		extraAttrs = FormatExtraAttributes(user, exclude)
	}

	data := struct {
		BaseData
		User            *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData:        func() BaseData { bd := NewBaseData(h.cfg, r, "users"); bd.Error = errMsg; return bd }(),
		User:            user,
		ExtraAttributes: extraAttrs,
		OUs:             ous,
	}

	RenderTemplate(w, h.templates, "user_form.html", data)
}
