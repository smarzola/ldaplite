package middleware

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
)

func setupTestAuth(t *testing.T) (*Auth, store.Store) {
	t.Helper()

	// Create temp directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Configure logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Suppress logs during tests
	})))

	// Set required environment variable for admin password
	os.Setenv("LDAP_ADMIN_PASSWORD", "TestPassword123!")
	t.Cleanup(func() {
		os.Unsetenv("LDAP_ADMIN_PASSWORD")
	})

	// Create config
	cfg := &config.Config{
		LDAP: config.LDAPConfig{
			BaseDN: "dc=test,dc=com",
		},
		Database: config.DatabaseConfig{
			Path:            dbPath,
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 300,
		},
		Security: config.SecurityConfig{
			AllowAnonymousBind: false,
			Argon2Config: config.Argon2Config{
				Memory:      64 * 1024,
				Iterations:  3,
				Parallelism: 2,
				SaltLength:  16,
				KeyLength:   32,
			},
		},
	}

	// Initialize store
	st := store.NewSQLiteStore(cfg)
	ctx := context.Background()
	if err := st.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Create auth middleware
	auth := NewAuth(st, cfg)

	return auth, st
}

func TestRequireAuthSuccess(t *testing.T) {
	auth, st := setupTestAuth(t)
	defer st.Close()

	// Create a test handler that should only be called if auth succeeds
	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// Verify user DN is in context
		userDN := GetUserDN(r)
		if userDN != "uid=admin,ou=users,dc=test,dc=com" {
			t.Errorf("Expected user DN 'uid=admin,ou=users,dc=test,dc=com', got '%s'", userDN)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Wrap test handler with auth middleware
	handler := auth.RequireAuth(testHandler)

	// Create request with valid admin credentials (admin user created during Initialize)
	req := httptest.NewRequest("GET", "/test", nil)
	credentials := base64.StdEncoding.EncodeToString([]byte("admin:TestPassword123!"))
	req.Header.Set("Authorization", "Basic "+credentials)

	// Execute request
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if !called {
		t.Error("Test handler was not called - auth should have succeeded")
	}
}

func TestRequireAuthMissingCredentials(t *testing.T) {
	auth, st := setupTestAuth(t)
	defer st.Close()

	// Create a test handler that should NOT be called
	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.RequireAuth(testHandler)

	// Create request without Authorization header
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if called {
		t.Error("Test handler was called but should have been blocked by auth")
	}
	if rr.Header().Get("WWW-Authenticate") == "" {
		t.Error("Expected WWW-Authenticate header in 401 response")
	}
}

func TestRequireAuthInvalidPassword(t *testing.T) {
	auth, st := setupTestAuth(t)
	defer st.Close()

	// Create a test handler that should NOT be called
	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.RequireAuth(testHandler)

	// Create request with wrong password
	req := httptest.NewRequest("GET", "/test", nil)
	credentials := base64.StdEncoding.EncodeToString([]byte("admin:WrongPassword"))
	req.Header.Set("Authorization", "Basic "+credentials)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if called {
		t.Error("Test handler was called but should have been blocked by auth")
	}
}

func TestRequireAuthNonExistentUser(t *testing.T) {
	auth, st := setupTestAuth(t)
	defer st.Close()

	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.RequireAuth(testHandler)

	// Create request with non-existent user
	req := httptest.NewRequest("GET", "/test", nil)
	credentials := base64.StdEncoding.EncodeToString([]byte("nonexistent:password"))
	req.Header.Set("Authorization", "Basic "+credentials)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if called {
		t.Error("Test handler was called but should have been blocked by auth")
	}
}

func TestRequireAuthNotInAdminGroup(t *testing.T) {
	auth, st := setupTestAuth(t)
	defer st.Close()
	ctx := context.Background()

	// Hash the password first
	hasher := auth.hasher
	hashedPassword, err := hasher.Hash("RegularPassword123!")
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Create a regular user NOT in admin group with properly hashed password
	modelEntry := models.NewEntry("uid=regularuser,ou=users,dc=test,dc=com", "inetOrgPerson")
	modelEntry.SetAttribute("uid", "regularuser")
	modelEntry.SetAttribute("cn", "Regular User")
	modelEntry.SetAttribute("sn", "User")
	modelEntry.SetAttribute("userPassword", hashedPassword)
	modelEntry.AddOperationalAttributes()

	if err := st.CreateEntry(ctx, modelEntry); err != nil {
		t.Fatalf("Failed to create regular user: %v", err)
	}

	// Verify the user was created and can be looked up
	passwordHashRetrieved, dnRetrieved, err := st.GetUserPasswordHash(ctx, "regularuser")
	if err != nil {
		t.Fatalf("Failed to retrieve user password hash: %v", err)
	}
	if passwordHashRetrieved == "" {
		t.Fatal("User password hash is empty - user not found in database")
	}
	if dnRetrieved != "uid=regularuser,ou=users,dc=test,dc=com" {
		t.Fatalf("Expected DN 'uid=regularuser,ou=users,dc=test,dc=com', got '%s'", dnRetrieved)
	}

	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.RequireAuth(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	credentials := base64.StdEncoding.EncodeToString([]byte("regularuser:RegularPassword123!"))
	req.Header.Set("Authorization", "Basic "+credentials)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify response - should be Forbidden (not in admin group)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
	if called {
		t.Error("Test handler was called but should have been blocked by admin group check")
	}
}

func TestRequireAuthMalformedBasicAuth(t *testing.T) {
	auth, st := setupTestAuth(t)
	defer st.Close()

	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.RequireAuth(testHandler)

	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "invalid base64",
			header: "Basic !!!invalid!!!",
		},
		{
			name:   "missing colon separator",
			header: "Basic " + base64.StdEncoding.EncodeToString([]byte("usernameonly")),
		},
		{
			name:   "wrong scheme",
			header: "Bearer sometoken",
		},
		{
			name:   "empty basic auth",
			header: "Basic ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tt.header)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
			}
			if called {
				t.Error("Test handler was called but should have been blocked")
			}
		})
	}
}

func TestGetUserDNFromContext(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() *http.Request
		want     string
	}{
		{
			name: "DN present in context",
			setupCtx: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				ctx := context.WithValue(req.Context(), UserDNKey, "uid=admin,ou=users,dc=test,dc=com")
				return req.WithContext(ctx)
			},
			want: "uid=admin,ou=users,dc=test,dc=com",
		},
		{
			name: "DN not in context",
			setupCtx: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			want: "",
		},
		{
			name: "wrong type in context",
			setupCtx: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				ctx := context.WithValue(req.Context(), UserDNKey, 123) // wrong type
				return req.WithContext(ctx)
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupCtx()
			got := GetUserDN(req)
			if got != tt.want {
				t.Errorf("GetUserDN() = %q, want %q", got, tt.want)
			}
		})
	}
}
