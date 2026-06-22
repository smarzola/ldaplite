package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/audit"
)

func TestAuditHTTPAddsRequestIDAndLogsNormalizedRouteStatusAndActor(t *testing.T) {
	logs := captureMiddlewareAuditLogs(t)

	handler := AuditHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		audit.SetActorDN(r.Context(), "uid=admin,ou=users,dc=example,dc=com")
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://ldaplite.test/users/delete?dn=uid%3Dadmin", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header was not set")
	}

	got := logs.String()
	assertAuditLogContains(t, got, `"event":"http.request"`)
	assertAuditLogContains(t, got, `"component":"web"`)
	assertAuditLogContains(t, got, `"request_id":"http-`)
	assertAuditLogContains(t, got, `"actor_dn":"uid=admin,ou=users,dc=example,dc=com"`)
	assertAuditLogContains(t, got, `"method":"POST"`)
	assertAuditLogContains(t, got, `"route":"/users/delete"`)
	assertAuditLogContains(t, got, `"status":204`)
	assertAuditLogNotContains(t, got, `dn=uid`)
}

func captureMiddlewareAuditLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})
	return &buf
}

func assertAuditLogContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("log output missing %q:\n%s", want, got)
	}
}

func assertAuditLogNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Fatalf("log output unexpectedly contains %q:\n%s", want, got)
	}
}
