package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/protocol"
	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

func TestBindAuditLogIncludesStableFieldsAndRedactsPassword(t *testing.T) {
	logs := captureAuditLogs(t)
	serverConn, clientConn, cleanup := auditTestConnection(t)
	defer cleanup()

	cfg := auditTestConfig()
	hasher := crypto.NewPasswordHasher(cfg.Security.Argon2Config)
	hash, err := hasher.Hash("CorrectHorseBatteryStaple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	srv := NewServer(cfg, &auditStore{
		passwordHash: hash,
		passwordDN:   "uid=jane,ou=users,dc=example,dc=com",
	}, "test")
	conn := protocol.NewConnection(serverConn, protocol.OperationHandlers{})
	msg := &ldapmsg.Message{
		ID: 7,
		Op: ldapmsg.BindRequest{
			Name:     "uid=jane,ou=users,dc=example,dc=com",
			Password: "CorrectHorseBatteryStaple",
		},
	}

	if err := srv.handleBind(context.Background(), conn, msg); err != nil {
		t.Fatalf("handleBind() failed: %v", err)
	}

	got := logs.String()
	assertLogContains(t, got, `"event":"ldap.operation"`)
	assertLogContains(t, got, `"component":"ldap"`)
	assertLogContains(t, got, `"operation":"bind"`)
	assertLogContains(t, got, `"message_id":7`)
	assertLogContains(t, got, `"actor_dn":"uid=jane,ou=users,dc=example,dc=com"`)
	assertLogContains(t, got, `"target_dn":"uid=jane,ou=users,dc=example,dc=com"`)
	assertLogContains(t, got, `"result_code":0`)
	assertLogContains(t, got, `"connection_id":"ldap-`)
	assertLogContains(t, got, `"request_id":"ldap-`)
	assertLogNotContains(t, got, "CorrectHorseBatteryStaple")
	assertLogNotContains(t, got, hash)
	assertLogNotContains(t, got, "userPassword")

	_ = clientConn.Close()
}

func TestSearchAuditLogIncludesBaseScopeResultCountAndActor(t *testing.T) {
	logs := captureAuditLogs(t)
	serverConn, clientConn, cleanup := auditTestConnection(t)
	defer cleanup()

	srv := NewServer(auditTestConfig(), &auditStore{
		searchEntries: []*models.Entry{
			models.NewEntry("uid=jane,ou=users,dc=example,dc=com", "inetOrgPerson"),
			models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson"),
		},
	}, "test")
	conn := protocol.NewConnection(serverConn, protocol.OperationHandlers{})
	conn.SetBoundDN("uid=admin,ou=users,dc=example,dc=com")
	msg := &ldapmsg.Message{
		ID: 9,
		Op: ldapmsg.SearchRequest{
			BaseObject: "dc=example,dc=com",
			Scope:      ldapmsg.SearchScopeWholeSubtree,
			Filter:     ldapmsg.PresentFilter{Attribute: "objectClass"},
		},
	}

	if err := srv.handleSearch(context.Background(), conn, msg); err != nil {
		t.Fatalf("handleSearch() failed: %v", err)
	}

	got := logs.String()
	assertLogContains(t, got, `"operation":"search"`)
	assertLogContains(t, got, `"actor_dn":"uid=admin,ou=users,dc=example,dc=com"`)
	assertLogContains(t, got, `"base_dn":"dc=example,dc=com"`)
	assertLogContains(t, got, `"scope":"subtree"`)
	assertLogContains(t, got, `"result_code":0`)
	assertLogContains(t, got, `"result_count":2`)

	_ = clientConn.Close()
}

func TestAddRejectedAuditLogIncludesActorTargetAndResultCode(t *testing.T) {
	logs := captureAuditLogs(t)
	serverConn, clientConn, cleanup := auditTestConnection(t)
	defer cleanup()

	srv := NewServer(auditTestConfig(), &auditStore{}, "test")
	conn := protocol.NewConnection(serverConn, protocol.OperationHandlers{})
	msg := &ldapmsg.Message{
		ID: 11,
		Op: ldapmsg.AddRequest{
			Entry: "uid=jane,ou=users,dc=example,dc=com",
		},
	}

	if err := srv.handleAdd(context.Background(), conn, msg); err != nil {
		t.Fatalf("handleAdd() failed: %v", err)
	}

	got := logs.String()
	assertLogContains(t, got, `"operation":"add"`)
	assertLogContains(t, got, `"target_dn":"uid=jane,ou=users,dc=example,dc=com"`)
	assertLogContains(t, got, `"result_code":50`)
	assertLogNotContains(t, got, `"actor_dn"`)

	_ = clientConn.Close()
}

func captureAuditLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})
	return &buf
}

func auditTestConnection(t *testing.T) (net.Conn, net.Conn, func()) {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, clientConn)
		close(done)
	}()

	cleanup := func() {
		_ = serverConn.Close()
		_ = clientConn.Close()
		<-done
	}
	return serverConn, clientConn, cleanup
}

func auditTestConfig() *config.Config {
	return &config.Config{
		LDAP: config.LDAPConfig{
			BaseDN: "dc=example,dc=com",
		},
		Security: config.SecurityConfig{
			Argon2Config: config.Argon2Config{
				Memory:      1,
				Iterations:  1,
				Parallelism: 1,
				SaltLength:  8,
				KeyLength:   16,
			},
		},
	}
}

func assertLogContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("log output missing %q:\n%s", want, got)
	}
}

func assertLogNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Fatalf("log output unexpectedly contains %q:\n%s", want, got)
	}
}

type auditStore struct {
	passwordHash  string
	passwordDN    string
	searchEntries []*models.Entry
}

func (s *auditStore) Initialize(ctx context.Context) error { return nil }

func (s *auditStore) Close() error { return nil }

func (s *auditStore) GetEntry(ctx context.Context, dn string) (*models.Entry, error) { return nil, nil }

func (s *auditStore) GetEntryWithOptions(ctx context.Context, dn string, options store.EntryOptions) (*models.Entry, error) {
	return nil, nil
}

func (s *auditStore) CreateEntry(ctx context.Context, entry *models.Entry) error { return nil }

func (s *auditStore) UpdateEntry(ctx context.Context, entry *models.Entry) error { return nil }

func (s *auditStore) DeleteEntry(ctx context.Context, dn string) error { return nil }

func (s *auditStore) SearchEntries(ctx context.Context, baseDN string, filter string) ([]*models.Entry, error) {
	return s.searchEntries, nil
}

func (s *auditStore) SearchEntriesWithOptions(ctx context.Context, options store.SearchOptions) ([]*models.Entry, error) {
	return s.searchEntries, nil
}

func (s *auditStore) EntryExists(ctx context.Context, dn string) (bool, error) { return false, nil }

func (s *auditStore) GetUserPasswordHash(ctx context.Context, uid string) (string, string, error) {
	return s.passwordHash, s.passwordDN, nil
}

func (s *auditStore) GetUserPasswordHashByDN(ctx context.Context, dn string) (string, string, error) {
	return s.passwordHash, s.passwordDN, nil
}

func (s *auditStore) IsUserInGroup(ctx context.Context, userDN, groupDN string) (bool, error) {
	return false, nil
}
