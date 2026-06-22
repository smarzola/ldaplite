package telemetry

import (
	"context"
	"database/sql"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	instrumentsMu         sync.RWMutex
	instruments           metricInstruments
	activeLDAPConnections atomic.Int64
	dbStatsProvider       atomic.Value
)

type metricInstruments struct {
	ldapOperations        metric.Int64Counter
	ldapOperationDuration metric.Float64Histogram
	ldapConnections       metric.Int64Counter
	ldapReadErrors        metric.Int64Counter
	ldapHandlerErrors     metric.Int64Counter
	httpRequests          metric.Int64Counter
	httpRequestDuration   metric.Float64Histogram
	webWrites             metric.Int64Counter
}

func initMetrics() error {
	meter := otel.Meter("github.com/smarzola/ldaplite")

	ldapOperations, err := meter.Int64Counter(
		"ldaplite.ldap.operations",
		metric.WithDescription("LDAP operations completed."),
		metric.WithUnit("{operation}"),
	)
	if err != nil {
		return err
	}
	ldapOperationDuration, err := meter.Float64Histogram(
		"ldaplite.ldap.operation.duration",
		metric.WithDescription("LDAP operation duration."),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}
	ldapConnections, err := meter.Int64Counter(
		"ldaplite.ldap.connections.accepted",
		metric.WithDescription("LDAP connections accepted."),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return err
	}
	ldapReadErrors, err := meter.Int64Counter(
		"ldaplite.ldap.read_errors",
		metric.WithDescription("LDAP transport read errors."),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}
	ldapHandlerErrors, err := meter.Int64Counter(
		"ldaplite.ldap.handler_errors",
		metric.WithDescription("LDAP handler errors."),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}
	httpRequests, err := meter.Int64Counter(
		"ldaplite.http.requests",
		metric.WithDescription("Web UI HTTP requests completed."),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}
	httpRequestDuration, err := meter.Float64Histogram(
		"ldaplite.http.request.duration",
		metric.WithDescription("Web UI HTTP request duration."),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}
	webWrites, err := meter.Int64Counter(
		"ldaplite.web.writes",
		metric.WithDescription("Web UI write actions completed."),
		metric.WithUnit("{write}"),
	)
	if err != nil {
		return err
	}

	activeConnections, err := meter.Int64ObservableGauge(
		"ldaplite.ldap.connections.active",
		metric.WithDescription("Active LDAP connections."),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return err
	}
	dbOpenConnections, err := meter.Int64ObservableGauge(
		"ldaplite.db.connections.open",
		metric.WithDescription("Open SQLite database connections."),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return err
	}
	dbInUseConnections, err := meter.Int64ObservableGauge(
		"ldaplite.db.connections.in_use",
		metric.WithDescription("In-use SQLite database connections."),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return err
	}
	dbIdleConnections, err := meter.Int64ObservableGauge(
		"ldaplite.db.connections.idle",
		metric.WithDescription("Idle SQLite database connections."),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return err
	}
	if _, err := meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		observer.ObserveInt64(activeConnections, activeLDAPConnections.Load())
		if provider, ok := dbStatsProvider.Load().(func() sql.DBStats); ok && provider != nil {
			stats := provider()
			observer.ObserveInt64(dbOpenConnections, int64(stats.OpenConnections))
			observer.ObserveInt64(dbInUseConnections, int64(stats.InUse))
			observer.ObserveInt64(dbIdleConnections, int64(stats.Idle))
		}
		return nil
	}, activeConnections, dbOpenConnections, dbInUseConnections, dbIdleConnections); err != nil {
		return err
	}

	instrumentsMu.Lock()
	instruments = metricInstruments{
		ldapOperations:        ldapOperations,
		ldapOperationDuration: ldapOperationDuration,
		ldapConnections:       ldapConnections,
		ldapReadErrors:        ldapReadErrors,
		ldapHandlerErrors:     ldapHandlerErrors,
		httpRequests:          httpRequests,
		httpRequestDuration:   httpRequestDuration,
		webWrites:             webWrites,
	}
	instrumentsMu.Unlock()

	return nil
}

func RecordLDAPOperation(ctx context.Context, operation string, resultCode int, duration time.Duration) {
	current := currentInstruments()
	if current.ldapOperations == nil || current.ldapOperationDuration == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("operation", operation),
		attribute.String("result_code", strconv.Itoa(resultCode)),
	)
	current.ldapOperations.Add(ctx, 1, attrs)
	current.ldapOperationDuration.Record(ctx, float64(duration.Milliseconds()), attrs)
}

func RecordLDAPConnectionAccepted(ctx context.Context) {
	current := currentInstruments()
	if current.ldapConnections != nil {
		current.ldapConnections.Add(ctx, 1)
	}
}

func AddActiveLDAPConnection(delta int64) {
	activeLDAPConnections.Add(delta)
}

func RecordLDAPReadError(ctx context.Context) {
	current := currentInstruments()
	if current.ldapReadErrors != nil {
		current.ldapReadErrors.Add(ctx, 1)
	}
}

func RecordLDAPHandlerError(ctx context.Context, operation string) {
	current := currentInstruments()
	if current.ldapHandlerErrors != nil {
		current.ldapHandlerErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", operation)))
	}
}

func RecordHTTPRequest(ctx context.Context, method, route string, status int, duration time.Duration) {
	current := currentInstruments()
	if current.httpRequests == nil || current.httpRequestDuration == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("route", route),
		attribute.String("status", strconv.Itoa(status)),
	)
	current.httpRequests.Add(ctx, 1, attrs)
	current.httpRequestDuration.Record(ctx, float64(duration.Milliseconds()), attrs)
}

func RecordWebWrite(ctx context.Context, operation, resource string, status int) {
	current := currentInstruments()
	if current.webWrites != nil {
		current.webWrites.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.String("resource", resource),
			attribute.String("status", strconv.Itoa(status)),
		))
	}
}

func RegisterDatabaseStatsProvider(provider func() sql.DBStats) {
	dbStatsProvider.Store(provider)
}

func resetMetricsForTest() {
	instrumentsMu.Lock()
	instruments = metricInstruments{}
	instrumentsMu.Unlock()
	activeLDAPConnections.Store(0)
	dbStatsProvider.Store((func() sql.DBStats)(nil))
}

func currentInstruments() metricInstruments {
	instrumentsMu.RLock()
	defer instrumentsMu.RUnlock()
	return instruments
}
