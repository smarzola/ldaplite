package telemetry

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/smarzola/ldaplite/pkg/config"
)

func TestRuntimeDisabledDoesNotStartMetricsListener(t *testing.T) {
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
