package telemetry

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/smarzola/ldaplite/pkg/config"
)

func TestRuntimeDisabledDoesNotStartMetricsListener(t *testing.T) {
	resetMetricsForTest()
	runtime, err := NewRuntime(config.TelemetryConfig{})
	if err != nil {
		t.Fatalf("NewRuntime() failed: %v", err)
	}
	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if got := runtime.Addr(); got != "" {
		t.Fatalf("Addr() = %q, want empty for disabled metrics", got)
	}
}

func TestRuntimeStartsPrometheusCompatibleMetricsEndpoint(t *testing.T) {
	resetMetricsForTest()
	runtime, err := NewRuntime(config.TelemetryConfig{
		OTelServiceName:    "ldaplite-test",
		MetricsEnabled:     true,
		MetricsBindAddress: "127.0.0.1",
		MetricsPort:        0,
		MetricsPath:        "/metrics",
	})
	if err != nil {
		t.Fatalf("NewRuntime() failed: %v", err)
	}

	ctx := context.Background()
	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runtime.Stop(stopCtx); err != nil {
			t.Fatalf("Stop() failed: %v", err)
		}
	})

	resp, err := http.Get("http://" + runtime.Addr() + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /metrics status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
	}
}

func TestRuntimeScrapesRecordedMetricsWithBoundedLabels(t *testing.T) {
	resetMetricsForTest()
	runtime, err := NewRuntime(config.TelemetryConfig{
		OTelServiceName:    "ldaplite-test",
		MetricsEnabled:     true,
		MetricsBindAddress: "127.0.0.1",
		MetricsPort:        0,
		MetricsPath:        "/metrics",
	})
	if err != nil {
		t.Fatalf("NewRuntime() failed: %v", err)
	}

	ctx := context.Background()
	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runtime.Stop(stopCtx); err != nil {
			t.Fatalf("Stop() failed: %v", err)
		}
		resetMetricsForTest()
	})

	RegisterDatabaseStatsProvider(func() sql.DBStats {
		return sql.DBStats{OpenConnections: 2, InUse: 1, Idle: 1}
	})
	RecordLDAPConnectionAccepted(ctx)
	AddActiveLDAPConnection(1)
	RecordLDAPOperation(ctx, "bind", 0, 12*time.Millisecond)
	RecordLDAPReadError(ctx)
	RecordLDAPHandlerError(ctx, "search")
	RecordHTTPRequest(ctx, http.MethodPost, "/users/delete", http.StatusFound, 5*time.Millisecond)
	RecordWebWrite(ctx, "delete", "user", http.StatusFound)

	resp, err := http.Get("http://" + runtime.Addr() + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics body: %v", err)
	}
	body := string(bodyBytes)

	assertMetricsContain(t, body, `ldaplite_ldap_operations`)
	assertMetricsContain(t, body, `operation="bind"`)
	assertMetricsContain(t, body, `result_code="0"`)
	assertMetricsContain(t, body, `ldaplite_ldap_connections_accepted`)
	assertMetricsContain(t, body, `ldaplite_ldap_connections_active`)
	assertMetricsContain(t, body, `ldaplite_ldap_read_errors`)
	assertMetricsContain(t, body, `ldaplite_ldap_handler_errors`)
	assertMetricsContain(t, body, `operation="search"`)
	assertMetricsContain(t, body, `ldaplite_http_requests`)
	assertMetricsContain(t, body, `method="POST"`)
	assertMetricsContain(t, body, `route="/users/delete"`)
	assertMetricsContain(t, body, `status="302"`)
	assertMetricsContain(t, body, `ldaplite_web_writes`)
	assertMetricsContain(t, body, `resource="user"`)
	assertMetricsContain(t, body, `ldaplite_db_connections_open`)
	assertMetricsContain(t, body, `ldaplite_db_connections_in_use`)
	assertMetricsContain(t, body, `ldaplite_db_connections_idle`)
}

func assertMetricsContain(t *testing.T, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Fatalf("metrics output missing %q:\n%s", want, body)
	}
}
