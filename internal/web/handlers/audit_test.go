package handlers

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/audit"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
)

func TestDeleteEntryAuditsSuccessfulWebWrite(t *testing.T) {
	logs := captureHandlerAuditLogs(t)
	req := httptest.NewRequest(http.MethodPost, "http://ldaplite.test/users/delete", strings.NewReader("dn=uid%3Djane%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	info := &audit.RequestInfo{
		RequestID: "http-test-1",
		ActorDN:   "uid=admin,ou=users,dc=example,dc=com",
		Method:    http.MethodPost,
		Route:     "/users/delete",
	}
	req = req.WithContext(audit.WithRequestInfo(req.Context(), info))
	rr := httptest.NewRecorder()

	deleteEntry(rr, req, &handlerAuditStore{}, "/users", "user", "user")

	got := logs.String()
	assertHandlerLogContains(t, got, `"event":"web.write"`)
	assertHandlerLogContains(t, got, `"component":"web"`)
	assertHandlerLogContains(t, got, `"operation":"delete"`)
	assertHandlerLogContains(t, got, `"actor_dn":"uid=admin,ou=users,dc=example,dc=com"`)
	assertHandlerLogContains(t, got, `"target_dn":"uid=jane,ou=users,dc=example,dc=com"`)
	assertHandlerLogContains(t, got, `"resource":"user"`)
	assertHandlerLogContains(t, got, `"status":302`)
}

func captureHandlerAuditLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})
	return &buf
}

func assertHandlerLogContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("log output missing %q:\n%s", want, got)
	}
}

type handlerAuditStore struct{}

func (s *handlerAuditStore) Initialize(ctx context.Context) error { return nil }

func (s *handlerAuditStore) Close() error { return nil }

func (s *handlerAuditStore) GetEntry(ctx context.Context, dn string) (*models.Entry, error) {
	return nil, nil
}

func (s *handlerAuditStore) GetEntryWithOptions(ctx context.Context, dn string, options store.EntryOptions) (*models.Entry, error) {
	return nil, nil
}

func (s *handlerAuditStore) CreateEntry(ctx context.Context, entry *models.Entry) error { return nil }

func (s *handlerAuditStore) UpdateEntry(ctx context.Context, entry *models.Entry) error { return nil }

func (s *handlerAuditStore) DeleteEntry(ctx context.Context, dn string) error { return nil }

func (s *handlerAuditStore) SearchEntries(ctx context.Context, baseDN string, filter string) ([]*models.Entry, error) {
	return nil, nil
}

func (s *handlerAuditStore) SearchEntriesWithOptions(ctx context.Context, options store.SearchOptions) ([]*models.Entry, error) {
	return nil, nil
}

func (s *handlerAuditStore) EntryExists(ctx context.Context, dn string) (bool, error) {
	return false, nil
}

func (s *handlerAuditStore) GetUserPasswordHash(ctx context.Context, uid string) (string, string, error) {
	return "", "", nil
}

func (s *handlerAuditStore) GetUserPasswordHashByDN(ctx context.Context, dn string) (string, string, error) {
	return "", "", nil
}

func (s *handlerAuditStore) IsUserInGroup(ctx context.Context, userDN, groupDN string) (bool, error) {
	return false, nil
}
