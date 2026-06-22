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
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Runtime struct {
	cfg            config.TelemetryConfig
	meterProvider  *sdkmetric.MeterProvider
	tracerProvider *sdktrace.TracerProvider
	server         *http.Server
	listener       net.Listener
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
	if cfg.Enabled {
		if err := runtime.initTracing(context.Background(), nil); err != nil {
			return nil, err
		}
	}
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
	if err := initMetrics(); err != nil {
		return nil, fmt.Errorf("initialize metrics: %w", err)
	}

	mux := http.NewServeMux()
	metricsPath := cfg.MetricsPath
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	mux.Handle(metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	runtime.server = &http.Server{Handler: mux}

	return runtime, nil
}

func newRuntimeWithTraceExporter(cfg config.TelemetryConfig, exporter sdktrace.SpanExporter) (*Runtime, error) {
	runtime := &Runtime{cfg: cfg}
	if err := runtime.initTracing(context.Background(), exporter); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (r *Runtime) initTracing(ctx context.Context, exporter sdktrace.SpanExporter) error {
	injectedExporter := exporter != nil
	serviceName := r.cfg.OTelServiceName
	if serviceName == "" {
		serviceName = "ldaplite"
	}

	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource.NewWithAttributes(
			"",
			attribute.String("service.name", serviceName),
		)),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}

	if exporter == nil && r.cfg.OTelExporterOTLPEndpoint != "" {
		var err error
		exporter, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(r.cfg.OTelExporterOTLPEndpoint))
		if err != nil {
			return fmt.Errorf("create OTLP trace exporter: %w", err)
		}
	}
	if injectedExporter {
		options = append(options, sdktrace.WithSyncer(exporter))
	} else if exporter != nil {
		options = append(options, sdktrace.WithBatcher(exporter))
	}

	r.tracerProvider = sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(r.tracerProvider)
	return nil
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
	if r.tracerProvider != nil {
		if err := r.tracerProvider.Shutdown(ctx); err != nil && stopErr == nil {
			stopErr = fmt.Errorf("tracer provider shutdown failed: %w", err)
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
