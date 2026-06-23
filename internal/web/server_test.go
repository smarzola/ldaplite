package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
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

func TestAPISessionReturnsRoleAwareCapabilities(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	createTestUser(t, st, "regularuser", "RegularPassword123!")

	tests := []struct {
		name               string
		credentials        string
		wantAdmin          bool
		wantDirectoryWrite bool
	}{
		{
			name:               "admin",
			credentials:        "admin:TestPassword123!",
			wantAdmin:          true,
			wantDirectoryWrite: true,
		},
		{
			name:               "authenticated non-admin",
			credentials:        "regularuser:RegularPassword123!",
			wantAdmin:          false,
			wantDirectoryWrite: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/session", nil)
			req.Header.Set("Authorization", basicAuth(tt.credentials))
			rr := httptest.NewRecorder()

			srv.mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
			}

			var got struct {
				UserDN       string   `json:"userDN"`
				Capabilities []string `json:"capabilities"`
				Roles        struct {
					Admin          bool `json:"admin"`
					DirectoryRead  bool `json:"directoryRead"`
					DirectoryWrite bool `json:"directoryWrite"`
					PasswordSelf   bool `json:"passwordSelf"`
				} `json:"roles"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("failed to decode session response: %v", err)
			}

			if got.UserDN == "" {
				t.Fatal("userDN was empty")
			}
			if got.Roles.Admin != tt.wantAdmin {
				t.Fatalf("roles.admin = %v, want %v", got.Roles.Admin, tt.wantAdmin)
			}
			if got.Roles.DirectoryRead != true {
				t.Fatal("roles.directoryRead = false, want true")
			}
			if got.Roles.DirectoryWrite != tt.wantDirectoryWrite {
				t.Fatalf("roles.directoryWrite = %v, want %v", got.Roles.DirectoryWrite, tt.wantDirectoryWrite)
			}
			if got.Roles.PasswordSelf != true {
				t.Fatal("roles.passwordSelf = false, want true")
			}
			if !containsString(got.Capabilities, "ui.read") {
				t.Fatalf("capabilities missing ui.read: %v", got.Capabilities)
			}
			if containsString(got.Capabilities, "ui.admin") != tt.wantAdmin {
				t.Fatalf("ui.admin presence = %v, want %v in %v", containsString(got.Capabilities, "ui.admin"), tt.wantAdmin, got.Capabilities)
			}
		})
	}
}

func TestNonAdminCanReadDirectoryAPIButCannotReachWriteRoute(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	createTestUser(t, st, "regularuser", "RegularPassword123!")

	readReq := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/directory", nil)
	readReq.Header.Set("Authorization", basicAuth("regularuser:RegularPassword123!"))
	readRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(readRR, readReq)

	if readRR.Code != http.StatusOK {
		t.Fatalf("directory status = %d, want %d; body=%s", readRR.Code, http.StatusOK, readRR.Body.String())
	}

	var directory struct {
		BaseDN string `json:"baseDN"`
		Users  []struct {
			DN   string `json:"dn"`
			Name string `json:"name"`
		} `json:"users"`
	}
	if err := json.Unmarshal(readRR.Body.Bytes(), &directory); err != nil {
		t.Fatalf("failed to decode directory response: %v", err)
	}
	if directory.BaseDN != "dc=test,dc=com" {
		t.Fatalf("baseDN = %q, want dc=test,dc=com", directory.BaseDN)
	}
	if !directoryHasUser(directory.Users, "regularuser") {
		t.Fatalf("directory response missing regularuser: %+v", directory.Users)
	}

	writeReq := httptest.NewRequest(http.MethodPost, "http://ldaplite.test/users/new", strings.NewReader("parentDN=ou%3Dusers%2Cdc%3Dtest%2Cdc%3Dcom&uid=hacker&cn=Hack&sn=Er&userPassword=Secret123%21"))
	writeReq.Header.Set("Authorization", basicAuth("regularuser:RegularPassword123!"))
	writeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	writeReq.Header.Set("Origin", "http://ldaplite.test")
	writeRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(writeRR, writeReq)

	if writeRR.Code != http.StatusForbidden {
		t.Fatalf("write status = %d, want %d; body=%s", writeRR.Code, http.StatusForbidden, writeRR.Body.String())
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

func basicAuth(credentials string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))
}

func createTestUser(t *testing.T, st store.Store, uid, password string) {
	t.Helper()

	hasher := crypto.NewPasswordHasher(config.Argon2Config{
		Memory:      64,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})
	hashedPassword, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() failed: %v", err)
	}

	user := models.NewEntry("uid="+uid+",ou=users,dc=test,dc=com", "inetOrgPerson")
	user.SetAttribute("uid", uid)
	user.SetAttribute("cn", uid)
	user.SetAttribute("sn", uid)
	user.SetAttribute("userPassword", hashedPassword)

	if err := st.CreateEntry(context.Background(), user); err != nil {
		t.Fatalf("CreateEntry(%s) failed: %v", uid, err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func directoryHasUser(users []struct {
	DN   string `json:"dn"`
	Name string `json:"name"`
}, uid string) bool {
	for _, user := range users {
		if user.Name == uid {
			return true
		}
	}
	return false
}
