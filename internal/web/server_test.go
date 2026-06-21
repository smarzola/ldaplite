package web

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
)

func TestMutatingRoutesRejectCrossOriginWithValidAuth(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	tests := []struct {
		name string
		path string
		form string
	}{
		{
			name: "users",
			path: "/users/new",
			form: "parentDN=ou%3Dusers%2Cdc%3Dtest%2Cdc%3Dcom&uid=jdoe&cn=John+Doe&sn=Doe&userPassword=Secret123%21",
		},
		{
			name: "groups",
			path: "/groups/new",
			form: "parentDN=ou%3Dgroups%2Cdc%3Dtest%2Cdc%3Dcom&cn=developers&member=uid%3Dadmin%2Cou%3Dusers%2Cdc%3Dtest%2Cdc%3Dcom",
		},
		{
			name: "ous",
			path: "/ous/new",
			form: "parentDN=dc%3Dtest%2Cdc%3Dcom&ou=engineering",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://ldaplite.test"+tt.path, strings.NewReader(tt.form))
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:TestPassword123!")))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", "https://evil.test")
			rr := httptest.NewRecorder()

			srv.mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
			}
		})
	}
}

func setupTestServer(t *testing.T) (*Server, store.Store) {
	t.Helper()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
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
		WebUI: config.WebUIConfig{
			Enabled:     true,
			Port:        8080,
			BindAddress: "127.0.0.1",
		},
	}

	st := store.NewSQLiteStore(cfg)
	if err := st.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	srv, err := NewServer(cfg, st)
	if err != nil {
		st.Close()
		t.Fatalf("NewServer() failed: %v", err)
	}
	return srv, st
}
