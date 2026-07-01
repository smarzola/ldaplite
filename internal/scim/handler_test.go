package scim

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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

func TestHandlerReturnsDeliberateNotImplementedBeforeRoutesAreMounted(t *testing.T) {
	cfg, st := setupTestStore(t)
	defer st.Close()
	handler := NewHandler(st, cfg)

	req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/scim/v2/Users", nil)
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

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
