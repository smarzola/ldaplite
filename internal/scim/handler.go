package scim

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/smarzola/ldaplite/internal/directory"
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
	default:
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
	if startIndex < 1 {
		startIndex = 1
	}
	return listResponse[T]{
		Schemas:      []string{listResponseSchema},
		TotalResults: len(resources),
		Resources:    resources,
		StartIndex:   startIndex,
		ItemsPerPage: len(resources),
	}
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
