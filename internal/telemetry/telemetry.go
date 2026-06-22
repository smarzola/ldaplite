package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/smarzola/ldaplite/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

type Runtime struct {
	cfg           config.TelemetryConfig
	meterProvider *sdkmetric.MeterProvider
	server        *http.Server
	listener      net.Listener
}

func Start(ctx context.Context, cfg *config.Config) (*Runtime, error) {
	runtime, err := NewRuntime(cfg.Telemetry)
	if err != nil {
		return nil, err
	}
	if err := runtime.Start(ctx); err != nil {
		return nil, err
	}
	return runtime, nil
}

func NewRuntime(cfg config.TelemetryConfig) (*Runtime, error) {
	runtime := &Runtime{cfg: cfg}
	if !cfg.MetricsEnabled {
		return runtime, nil
	}

	registry := promclient.NewRegistry()
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	serviceName := cfg.OTelServiceName
	if serviceName == "" {
		serviceName = "ldaplite"
	}

	runtime.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(resource.NewWithAttributes(
			"",
			attribute.String("service.name", serviceName),
		)),
	)
	otel.SetMeterProvider(runtime.meterProvider)

	mux := http.NewServeMux()
	metricsPath := cfg.MetricsPath
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	mux.Handle(metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	runtime.server = &http.Server{Handler: mux}

	return runtime, nil
}

func (r *Runtime) Start(ctx context.Context) error {
	_ = ctx
	if r == nil || r.server == nil {
		return nil
	}

	addr := net.JoinHostPort(r.cfg.MetricsBindAddress, strconv.Itoa(r.cfg.MetricsPort))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("start metrics listener on %s: %w", addr, err)
	}
	r.listener = listener
	r.server.Addr = listener.Addr().String()

	go func() {
		if err := r.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

	slog.Info("Metrics server is running", "address", r.server.Addr, "path", r.cfg.MetricsPath)
	return nil
}

func (r *Runtime) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}

	var stopErr error
	if r.server != nil {
		if err := r.server.Shutdown(ctx); err != nil {
			stopErr = fmt.Errorf("metrics server shutdown failed: %w", err)
		}
	}
	if r.meterProvider != nil {
		if err := r.meterProvider.Shutdown(ctx); err != nil && stopErr == nil {
			stopErr = fmt.Errorf("meter provider shutdown failed: %w", err)
		}
	}
	return stopErr
}

func (r *Runtime) Addr() string {
	if r == nil || r.server == nil {
		return ""
	}
	return r.server.Addr
}
