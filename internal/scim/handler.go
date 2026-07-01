package scim

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/smarzola/ldaplite/internal/directory"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
)

type Handler struct {
	store    store.Store
	cfg      *config.Config
	service  *directory.Service
	contract Contract
}

func NewHandler(st store.Store, cfg *config.Config) *Handler {
	return &Handler{
		store:    st,
		cfg:      cfg,
		service:  directory.NewService(st, cfg),
		contract: DefaultContract(),
	}
}

func (h *Handler) Contract() Contract {
	return h.contract
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case BasePath + "/ServiceProviderConfig":
		h.ServiceProviderConfig(w, r)
	case BasePath + "/Schemas":
		h.Schemas(w, r)
	case BasePath + "/ResourceTypes":
		h.ResourceTypes(w, r)
	case BasePath + "/Users":
		h.Users(w, r)
	default:
		if strings.HasPrefix(r.URL.Path, BasePath+"/Users/") {
			h.Users(w, r)
			return
		}
		writeSCIMError(w, http.StatusNotImplemented, "SCIM endpoint is not implemented yet")
	}
}

func (h *Handler) ServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeSCIMJSON(w, http.StatusOK, serviceProviderConfigResponse{
		Schemas: []string{serviceProviderConfigSchema},
		Patch: supportedConfig{
			Supported: h.contract.SupportsPatch,
		},
		Bulk: bulkConfig{
			Supported:      h.contract.SupportsBulk,
			MaxOperations:  0,
			MaxPayloadSize: 0,
		},
		Filter: filterConfig{
			Supported:  true,
			MaxResults: 200,
		},
		ChangePassword: supportedConfig{Supported: false},
		Sort:           supportedConfig{Supported: false},
		ETag:           supportedConfig{Supported: false},
		AuthenticationSchemes: []authenticationScheme{
			{
				Type:        "httpbasic",
				Name:        "HTTP Basic",
				Description: "LDAPLite user credentials",
				Primary:     true,
			},
		},
	})
}

func (h *Handler) Schemas(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	resources := []schemaResource{
		{
			Schemas:     []string{schemaSchema},
			ID:          userSchema,
			Name:        "User",
			Description: "LDAPLite SCIM user mapped to inetOrgPerson",
			Attributes: []schemaAttribute{
				{Name: "userName", Type: "string", MultiValued: false, Required: true, Mutability: "readWrite"},
				{Name: "name", Type: "complex", MultiValued: false, Required: true, Mutability: "readWrite"},
				{Name: "displayName", Type: "string", MultiValued: false, Required: true, Mutability: "readWrite"},
				{Name: "emails", Type: "complex", MultiValued: true, Required: false, Mutability: "readWrite"},
				{Name: "password", Type: "string", MultiValued: false, Required: false, Mutability: "writeOnly"},
			},
		},
		{
			Schemas:     []string{schemaSchema},
			ID:          groupSchema,
			Name:        "Group",
			Description: "LDAPLite SCIM group mapped to groupOfNames",
			Attributes: []schemaAttribute{
				{Name: "displayName", Type: "string", MultiValued: false, Required: true, Mutability: "readWrite"},
				{Name: "members", Type: "complex", MultiValued: true, Required: true, Mutability: "readWrite"},
			},
		},
	}
	writeSCIMJSON(w, http.StatusOK, newListResponse(resources, 1))
}

func (h *Handler) ResourceTypes(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	resources := []resourceTypeResource{
		{
			Schemas:     []string{resourceTypeSchema},
			ID:          "User",
			Name:        "User",
			Endpoint:    BasePath + "/Users",
			Description: "LDAPLite users",
			Schema:      userSchema,
		},
		{
			Schemas:     []string{resourceTypeSchema},
			ID:          "Group",
			Name:        "Group",
			Endpoint:    BasePath + "/Groups",
			Description: "LDAPLite groups",
			Schema:      groupSchema,
		},
	}
	writeSCIMJSON(w, http.StatusOK, newListResponse(resources, 1))
}

func (h *Handler) Users(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeSCIMError(w, http.StatusNotImplemented, "SCIM user writes are not implemented yet")
		return
	}
	if r.URL.Path == BasePath+"/Users" {
		h.listUsers(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, BasePath+"/Users/") {
		h.getUser(w, r)
		return
	}
	writeSCIMError(w, http.StatusNotFound, "SCIM user endpoint not found")
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	filter, err := userLDAPFilter(r.URL.Query().Get("filter"))
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, err.Error())
		return
	}
	startIndex, count, err := parsePagination(r)
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, err.Error())
		return
	}

	entries, err := h.store.SearchEntriesWithOptions(r.Context(), store.SearchOptions{
		BaseDN:          h.cfg.LDAP.BaseDN,
		Filter:          filter,
		Scope:           store.SearchScopeWholeSubtree,
		IncludeMemberOf: false,
	})
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "Failed to search SCIM users")
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].DN) < strings.ToLower(entries[j].DN)
	})

	total := len(entries)
	pageEntries := pageEntries(entries, startIndex, count)
	resources := make([]userResource, 0, len(pageEntries))
	for _, entry := range pageEntries {
		resources = append(resources, h.userResource(r, entry))
	}
	writeSCIMJSON(w, http.StatusOK, newListResponsePage(resources, total, startIndex))
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathResourceID(r.URL.Path, BasePath+"/Users/")
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, err.Error())
		return
	}
	entry, ok, err := h.userByID(r, id)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "Failed to load SCIM user")
		return
	}
	if !ok {
		writeSCIMError(w, http.StatusNotFound, "SCIM user not found")
		return
	}
	writeSCIMJSON(w, http.StatusOK, h.userResource(r, entry))
}

func (h *Handler) userByID(r *http.Request, id string) (*models.Entry, bool, error) {
	entries, err := h.store.SearchEntriesWithOptions(r.Context(), store.SearchOptions{
		BaseDN:          h.cfg.LDAP.BaseDN,
		Filter:          "(&(objectClass=inetOrgPerson)(entryUUID=" + escapeLDAPFilterAssertionValue(id) + "))",
		Scope:           store.SearchScopeWholeSubtree,
		IncludeMemberOf: false,
	})
	if err != nil {
		return nil, false, err
	}
	if len(entries) == 0 {
		return nil, false, nil
	}
	return entries[0], true, nil
}

func (h *Handler) userResource(r *http.Request, entry *models.Entry) userResource {
	id := entry.GetAttribute("entryUUID")
	resource := userResource{
		Schemas:     []string{userSchema},
		ID:          id,
		UserName:    entry.GetAttribute("uid"),
		DisplayName: entry.GetAttribute("cn"),
		Name: nameResource{
			GivenName:  entry.GetAttribute("givenName"),
			FamilyName: entry.GetAttribute("sn"),
			Formatted:  entry.GetAttribute("cn"),
		},
		Meta: metaResource{
			ResourceType: "User",
			Created:      formatSCIMTime(entry.CreatedAt),
			LastModified: formatSCIMTime(entry.UpdatedAt),
			Location:     absoluteURL(r, BasePath+"/Users/"+url.PathEscape(id)),
		},
	}
	if mail := entry.GetAttribute("mail"); mail != "" {
		resource.Emails = []emailResource{{Value: mail, Primary: true}}
	}
	return resource
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeSCIMError(w, http.StatusMethodNotAllowed, "Method not allowed")
	return false
}

func writeSCIMJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeSCIMError(w http.ResponseWriter, status int, detail string) {
	writeSCIMJSON(w, status, errorResponse{
		Schemas: []string{errorSchema},
		Detail:  detail,
		Status:  strconv.Itoa(status),
	})
}

func newListResponse[T any](resources []T, startIndex int) listResponse[T] {
	return newListResponsePage(resources, len(resources), startIndex)
}

func newListResponsePage[T any](resources []T, totalResults, startIndex int) listResponse[T] {
	if startIndex < 1 {
		startIndex = 1
	}
	return listResponse[T]{
		Schemas:      []string{listResponseSchema},
		TotalResults: totalResults,
		Resources:    resources,
		StartIndex:   startIndex,
		ItemsPerPage: len(resources),
	}
}

func parsePagination(r *http.Request) (startIndex int, count int, err error) {
	query := r.URL.Query()
	startIndex = 1
	count = 100
	if raw := strings.TrimSpace(query.Get("startIndex")); raw != "" {
		startIndex, err = strconv.Atoi(raw)
		if err != nil || startIndex < 1 {
			return 0, 0, errInvalidPagination("startIndex must be a positive integer")
		}
	}
	if raw := strings.TrimSpace(query.Get("count")); raw != "" {
		count, err = strconv.Atoi(raw)
		if err != nil || count < 0 {
			return 0, 0, errInvalidPagination("count must be a non-negative integer")
		}
		if count > 200 {
			count = 200
		}
	}
	return startIndex, count, nil
}

func pageEntries(entries []*models.Entry, startIndex, count int) []*models.Entry {
	if count == 0 || startIndex > len(entries) {
		return []*models.Entry{}
	}
	start := startIndex - 1
	end := start + count
	if end > len(entries) {
		end = len(entries)
	}
	return entries[start:end]
}

type requestError string

func (e requestError) Error() string {
	return string(e)
}

func errInvalidPagination(message string) error {
	return requestError(message)
}

func userLDAPFilter(rawFilter string) (string, error) {
	attr, value, ok, err := parseSimpleEqualityFilter(rawFilter)
	if err != nil {
		return "", err
	}
	if !ok {
		return "(objectClass=inetOrgPerson)", nil
	}

	var ldapAttr string
	switch attr {
	case "id":
		ldapAttr = "entryUUID"
	case "userName":
		ldapAttr = "uid"
	case "displayName":
		ldapAttr = "cn"
	default:
		return "", requestError("Unsupported SCIM user filter")
	}
	return "(&(objectClass=inetOrgPerson)(" + ldapAttr + "=" + escapeLDAPFilterAssertionValue(value) + "))", nil
}

func parseSimpleEqualityFilter(rawFilter string) (attr string, value string, ok bool, err error) {
	rawFilter = strings.TrimSpace(rawFilter)
	if rawFilter == "" {
		return "", "", false, nil
	}
	parts := strings.SplitN(rawFilter, " eq ", 2)
	if len(parts) != 2 {
		return "", "", false, requestError("Only simple SCIM eq filters are supported")
	}
	attr = strings.TrimSpace(parts[0])
	quotedValue := strings.TrimSpace(parts[1])
	value, err = strconv.Unquote(quotedValue)
	if err != nil {
		return "", "", false, requestError("SCIM filter value must be a quoted string")
	}
	return attr, value, true, nil
}

func pathResourceID(path, prefix string) (string, error) {
	escapedID := strings.TrimPrefix(path, prefix)
	if escapedID == "" || escapedID == path || strings.Contains(escapedID, "/") {
		return "", requestError("SCIM resource id is required")
	}
	id, err := url.PathUnescape(escapedID)
	if err != nil || strings.TrimSpace(id) == "" {
		return "", requestError("Invalid SCIM resource id")
	}
	return id, nil
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

func absoluteURL(r *http.Request, path string) string {
	scheme := r.URL.Scheme
	if scheme == "" {
		if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
			scheme = forwarded
		} else if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	return scheme + "://" + host + path
}

func formatSCIMTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

type serviceProviderConfigResponse struct {
	Schemas               []string               `json:"schemas"`
	Patch                 supportedConfig        `json:"patch"`
	Bulk                  bulkConfig             `json:"bulk"`
	Filter                filterConfig           `json:"filter"`
	ChangePassword        supportedConfig        `json:"changePassword"`
	Sort                  supportedConfig        `json:"sort"`
	ETag                  supportedConfig        `json:"etag"`
	AuthenticationSchemes []authenticationScheme `json:"authenticationSchemes"`
}

type supportedConfig struct {
	Supported bool `json:"supported"`
}

type bulkConfig struct {
	Supported      bool `json:"supported"`
	MaxOperations  int  `json:"maxOperations"`
	MaxPayloadSize int  `json:"maxPayloadSize"`
}

type filterConfig struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

type authenticationScheme struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Primary     bool   `json:"primary"`
}

type listResponse[T any] struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	Resources    []T      `json:"Resources"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
}

type userResource struct {
	Schemas     []string        `json:"schemas"`
	ID          string          `json:"id"`
	UserName    string          `json:"userName"`
	Name        nameResource    `json:"name"`
	DisplayName string          `json:"displayName"`
	Emails      []emailResource `json:"emails,omitempty"`
	Meta        metaResource    `json:"meta"`
}

type nameResource struct {
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

type emailResource struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary"`
}

type metaResource struct {
	ResourceType string `json:"resourceType"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Location     string `json:"location"`
}

type schemaResource struct {
	Schemas     []string          `json:"schemas"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Attributes  []schemaAttribute `json:"attributes"`
}

type schemaAttribute struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	MultiValued bool   `json:"multiValued"`
	Required    bool   `json:"required"`
	Mutability  string `json:"mutability"`
}

type resourceTypeResource struct {
	Schemas     []string `json:"schemas"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Endpoint    string   `json:"endpoint"`
	Description string   `json:"description"`
	Schema      string   `json:"schema"`
}

type errorResponse struct {
	Schemas []string `json:"schemas"`
	Detail  string   `json:"detail"`
	Status  string   `json:"status"`
}
