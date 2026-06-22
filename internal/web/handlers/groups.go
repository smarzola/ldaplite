package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
)

type GroupHandler struct {
	store     store.Store
	cfg       *config.Config
	templates TemplateGetter
}

var groupFormAttributes = []string{"cn", "description", "member"}
var groupFormExcludeAttributes = []string{"cn", "description", "member", "objectClass", "createTimestamp", "modifyTimestamp", "memberOf"}

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
	entries, err := searchEntriesWithoutMemberOf(ctx, h.store, h.cfg.LDAP.BaseDN, "(objectClass=groupOfNames)")
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
	ous := loadOrganizationalUnits(ctx, h.store, h.cfg.LDAP.BaseDN)

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

	if parentDN == "" || cn == "" {
		h.showError(w, r, "Parent OU and CN are required", nil)
		return
	}

	members := parseNonEmptyLines(r.FormValue("member"))
	if len(members) == 0 {
		h.showError(w, r, "At least one member is required", nil)
		return
	}

	group := models.NewGroup(parentDN, cn, description)
	for _, member := range members {
		group.AddMember(member)
	}

	// Add extra attributes
	addExtraAttributes(group.Entry, ParseAttributes(r.FormValue("attributes")))

	if err := h.store.CreateEntry(ctx, group.Entry); err != nil {
		h.showError(w, r, fmt.Sprintf("Failed to create group: %v", err), nil)
		return
	}

	redirectWithMessage(w, r, "/groups", "success", "Group created successfully")
}

func (h *GroupHandler) Edit(w http.ResponseWriter, r *http.Request) {
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
		h.showError(w, r, fmt.Sprintf("Group not found: %v", err), nil)
		return
	}

	ous := loadOrganizationalUnits(ctx, h.store, h.cfg.LDAP.BaseDN)

	data := struct {
		BaseData
		Group           *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData:        NewBaseData(h.cfg, r, "groups"),
		Group:           entry,
		ExtraAttributes: formatExtraAttributesForForm(entry, groupFormExcludeAttributes),
		OUs:             ous,
	}

	RenderTemplate(w, h.templates, "group_form.html", data)
}

func (h *GroupHandler) update(w http.ResponseWriter, r *http.Request, dn string) {
	ctx := r.Context()

	entry, err := getEntryWithoutMemberOf(ctx, h.store, dn)
	if err != nil {
		h.showError(w, r, fmt.Sprintf("Group not found: %v", err), nil)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.showError(w, r, "Invalid form data", entry)
		return
	}

	setOptionalAttribute(entry, "description", r.FormValue("description"))

	// Update members
	entry.SetAttributes("member", parseNonEmptyLines(r.FormValue("member")))

	// Update extra attributes
	extraAttrs := ParseAttributes(r.FormValue("attributes"))
	ReplaceExtraAttributes(entry, groupFormAttributes, extraAttrs)

	if err := h.store.UpdateEntry(ctx, entry); err != nil {
		h.showError(w, r, fmt.Sprintf("Failed to update group: %v", err), entry)
		return
	}

	redirectWithMessage(w, r, "/groups", "success", "Group updated successfully")
}

func (h *GroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	deleteEntry(w, r, h.store, "/groups", "group", "Group")
}

func (h *GroupHandler) showError(w http.ResponseWriter, r *http.Request, errMsg string, group *models.Entry) {
	ctx := r.Context()

	ous := loadOrganizationalUnits(ctx, h.store, h.cfg.LDAP.BaseDN)

	data := struct {
		BaseData
		Group           *models.Entry
		ExtraAttributes string
		OUs             []*models.Entry
	}{
		BaseData:        NewBaseDataWithError(h.cfg, r, "groups", errMsg),
		Group:           group,
		ExtraAttributes: formatExtraAttributesForForm(group, groupFormExcludeAttributes),
		OUs:             ous,
	}

	RenderTemplate(w, h.templates, "group_form.html", data)
}
