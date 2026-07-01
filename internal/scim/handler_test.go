package scim

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
)

func TestDefaultContractCapturesBaselineScope(t *testing.T) {
	contract := DefaultContract()

	if contract.BasePath != BasePath {
		t.Fatalf("BasePath = %q, want %q", contract.BasePath, BasePath)
	}
	if !contains(contract.DiscoveryEndpoints, BasePath+"/ServiceProviderConfig") {
		t.Fatalf("DiscoveryEndpoints missing ServiceProviderConfig: %v", contract.DiscoveryEndpoints)
	}
	if !contains(contract.ResourceTypes, "User") || !contains(contract.ResourceTypes, "Group") {
		t.Fatalf("ResourceTypes = %v, want User and Group", contract.ResourceTypes)
	}
	if contract.SupportsPatch {
		t.Fatal("SupportsPatch = true, want false for baseline contract")
	}
	if contract.SupportsBulk {
		t.Fatal("SupportsBulk = true, want false for baseline contract")
	}
	if contract.ReadCapability != "directory.read" {
		t.Fatalf("ReadCapability = %q, want directory.read", contract.ReadCapability)
	}
	if contract.WriteCapability != "directory.write" {
		t.Fatalf("WriteCapability = %q, want directory.write", contract.WriteCapability)
	}
}

func TestHandlerCanBeConstructedWithSQLiteStore(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()

	handler := NewHandler(st, cfg)

	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}
	if handler.store == nil {
		t.Fatal("handler.store is nil")
	}
	if handler.cfg != cfg {
		t.Fatal("handler.cfg does not reference the provided config")
	}
	if handler.service == nil {
		t.Fatal("handler.service is nil")
	}
	if handler.Contract().BasePath != BasePath {
		t.Fatalf("handler contract base path = %q, want %q", handler.Contract().BasePath, BasePath)
	}
}

func TestHandlerReturnsDeliberateNotImplementedForUnknownRoutes(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()
	handler := NewHandler(st, cfg)

	req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/scim/v2/Groups", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotImplemented, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != ContentType {
		t.Fatalf("Content-Type = %q, want %q", got, ContentType)
	}

	var body errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode SCIM error response: %v", err)
	}
	if !contains(body.Schemas, errorSchema) {
		t.Fatalf("schemas = %v, want %s", body.Schemas, errorSchema)
	}
	if body.Status != "501" {
		t.Fatalf("status = %q, want 501", body.Status)
	}
}

func TestServiceProviderConfigDiscovery(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()
	handler := NewHandler(st, cfg)

	req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/scim/v2/ServiceProviderConfig", nil)
	rr := httptest.NewRecorder()

	handler.ServiceProviderConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != ContentType {
		t.Fatalf("Content-Type = %q, want %q", got, ContentType)
	}

	var body serviceProviderConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode ServiceProviderConfig: %v", err)
	}
	if !contains(body.Schemas, serviceProviderConfigSchema) {
		t.Fatalf("schemas = %v, want %s", body.Schemas, serviceProviderConfigSchema)
	}
	if body.Patch.Supported {
		t.Fatal("patch.supported = true, want false")
	}
	if body.Bulk.Supported {
		t.Fatal("bulk.supported = true, want false")
	}
	if !body.Filter.Supported {
		t.Fatal("filter.supported = false, want true")
	}
	if len(body.AuthenticationSchemes) != 1 || body.AuthenticationSchemes[0].Type != "httpbasic" || !body.AuthenticationSchemes[0].Primary {
		t.Fatalf("authenticationSchemes = %+v, want primary httpbasic", body.AuthenticationSchemes)
	}
}

func TestSchemasDiscovery(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()
	handler := NewHandler(st, cfg)

	req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/scim/v2/Schemas", nil)
	rr := httptest.NewRecorder()

	handler.Schemas(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var body listResponse[schemaResource]
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode Schemas response: %v", err)
	}
	if !contains(body.Schemas, listResponseSchema) {
		t.Fatalf("schemas = %v, want %s", body.Schemas, listResponseSchema)
	}
	if body.TotalResults != 2 || body.ItemsPerPage != 2 || body.StartIndex != 1 {
		t.Fatalf("list metadata = %+v, want two resources at start index 1", body)
	}
	if body.Resources[0].ID != userSchema || body.Resources[1].ID != groupSchema {
		t.Fatalf("schema resource ids = %+v, want user and group schemas", body.Resources)
	}
}

func TestResourceTypesDiscovery(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()
	handler := NewHandler(st, cfg)

	req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/scim/v2/ResourceTypes", nil)
	rr := httptest.NewRecorder()

	handler.ResourceTypes(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var body listResponse[resourceTypeResource]
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode ResourceTypes response: %v", err)
	}
	if body.TotalResults != 2 || len(body.Resources) != 2 {
		t.Fatalf("resource types = %+v, want two resources", body)
	}
	if body.Resources[0].Endpoint != BasePath+"/Users" || body.Resources[0].Schema != userSchema {
		t.Fatalf("user resource type = %+v, want Users endpoint and user schema", body.Resources[0])
	}
	if body.Resources[1].Endpoint != BasePath+"/Groups" || body.Resources[1].Schema != groupSchema {
		t.Fatalf("group resource type = %+v, want Groups endpoint and group schema", body.Resources[1])
	}
}

func TestDiscoveryRejectsUnsupportedMethodsWithSCIMError(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()
	handler := NewHandler(st, cfg)

	req := httptest.NewRequest(http.MethodPost, "http://ldaplite.test/scim/v2/ServiceProviderConfig", nil)
	rr := httptest.NewRecorder()

	handler.ServiceProviderConfig(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusMethodNotAllowed, rr.Body.String())
	}
	if got := rr.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", got)
	}
	var body errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode SCIM error: %v", err)
	}
	if body.Status != "405" || !contains(body.Schemas, errorSchema) {
		t.Fatalf("error response = %+v, want SCIM 405", body)
	}
}

func TestUsersListSupportsPaginationAndSafeMapping(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()
	handler := NewHandler(st, cfg)
	createSCIMTestUser(t, st, "anna", "Anna Operator", "Operator", "Anna", "anna@example.com")
	createSCIMTestUser(t, st, "brian", "Brian Builder", "Builder", "Brian", "brian@example.com")

	req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/scim/v2/Users?startIndex=1&count=1", nil)
	rr := httptest.NewRecorder()

	handler.Users(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if strings.Contains(strings.ToLower(rr.Body.String()), "userpassword") || strings.Contains(rr.Body.String(), "ARGON2") {
		t.Fatalf("SCIM user list leaked password material: %s", rr.Body.String())
	}

	var body listResponse[userResource]
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode Users list: %v", err)
	}
	if !contains(body.Schemas, listResponseSchema) {
		t.Fatalf("schemas = %v, want %s", body.Schemas, listResponseSchema)
	}
	if body.TotalResults < 2 || body.StartIndex != 1 || body.ItemsPerPage != 1 || len(body.Resources) != 1 {
		t.Fatalf("list response = %+v, want one paged resource from at least two users", body)
	}
	if body.Resources[0].ID == "" || body.Resources[0].UserName == "" || body.Resources[0].Meta.Location == "" {
		t.Fatalf("SCIM user resource missing identity/meta: %+v", body.Resources[0])
	}
}

func TestUsersListSupportsSimpleFilters(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()
	handler := NewHandler(st, cfg)
	createSCIMTestUser(t, st, "filteruser", "Filter User", "User", "Filter", "filter@example.com")

	req := httptest.NewRequest(http.MethodGet, `http://ldaplite.test/scim/v2/Users?filter=userName+eq+%22filteruser%22`, nil)
	rr := httptest.NewRecorder()

	handler.Users(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var body listResponse[userResource]
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode filtered Users list: %v", err)
	}
	if body.TotalResults != 1 || len(body.Resources) != 1 {
		t.Fatalf("filtered list = %+v, want exactly one resource", body)
	}
	if got := body.Resources[0]; got.UserName != "filteruser" || got.DisplayName != "Filter User" || len(got.Emails) != 1 || got.Emails[0].Value != "filter@example.com" {
		t.Fatalf("filtered user = %+v, want filteruser mapping", got)
	}
}

func TestUserGetByStableID(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()
	handler := NewHandler(st, cfg)
	user := createSCIMTestUser(t, st, "iduser", "ID User", "User", "ID", "id@example.com")
	id := user.GetAttribute("entryUUID")

	req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/scim/v2/Users/"+id, nil)
	rr := httptest.NewRecorder()

	handler.Users(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var body userResource
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode SCIM user: %v", err)
	}
	if body.ID != id || body.UserName != "iduser" || body.Name.FamilyName != "User" || body.Name.GivenName != "ID" {
		t.Fatalf("SCIM user = %+v, want iduser with stable id %s", body, id)
	}
	if body.Meta.ResourceType != "User" || !strings.Contains(body.Meta.Location, "/scim/v2/Users/"+id) {
		t.Fatalf("meta = %+v, want User location with id", body.Meta)
	}
}

func TestUsersReturnSCIMErrorsForUnsupportedReadInputs(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()
	handler := NewHandler(st, cfg)

	tests := []struct {
		name       string
		target     string
		wantStatus int
	}{
		{
			name:       "unsupported filter",
			target:     `http://ldaplite.test/scim/v2/Users?filter=emails.value+eq+%22a@example.com%22`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing user",
			target:     "http://ldaplite.test/scim/v2/Users/not-a-real-id",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "bad pagination",
			target:     "http://ldaplite.test/scim/v2/Users?startIndex=0",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			rr := httptest.NewRecorder()

			handler.Users(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			var body errorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode SCIM error: %v", err)
			}
			if !contains(body.Schemas, errorSchema) {
				t.Fatalf("schemas = %v, want SCIM error schema", body.Schemas)
			}
		})
	}
}

func setupTestStore(t *testing.T) (*config.Config, store.Store) {
	t.Helper()
	t.Setenv("LDAP_ADMIN_PASSWORD", "TestPassword123!")

	cfg := &config.Config{
		LDAP: config.LDAPConfig{
			BaseDN: "dc=test,dc=com",
		},
		Database: config.DatabaseConfig{
			Path:            filepath.Join(t.TempDir(), "test.db"),
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 300,
		},
		Security: config.SecurityConfig{
			Argon2Config: config.Argon2Config{
				Memory:      64,
				Iterations:  1,
				Parallelism: 1,
				SaltLength:  16,
				KeyLength:   32,
			},
		},
	}

	st := store.NewSQLiteStore(cfg)
	if err := st.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}
	return cfg, st
}

func createSCIMTestUser(t *testing.T, st store.Store, uid, cn, sn, givenName, mail string) *models.Entry {
	t.Helper()

	user := models.NewUser("ou=users,dc=test,dc=com", uid, cn, sn, mail)
	user.SetPassword("{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dummyhash$dummyhash")
	if givenName != "" {
		user.Entry.SetAttribute("givenName", givenName)
	}
	if err := st.CreateEntry(context.Background(), user.Entry); err != nil {
		t.Fatalf("CreateEntry(%s) failed: %v", uid, err)
	}
	entry, err := st.GetEntry(context.Background(), user.DN)
	if err != nil {
		t.Fatalf("GetEntry(%s) failed: %v", user.DN, err)
	}
	return entry
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
