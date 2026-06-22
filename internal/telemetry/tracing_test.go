package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracingHelpersEmitBoundedSpanAttributes(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(recorder),
	)
	previousProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		_ = provider.Shutdown(context.Background())
	})

	ctx := context.Background()
	ctx, ldapSpan := StartLDAPSpan(ctx, "bind")
	EndLDAPSpan(ldapSpan, 0)
	ctx, httpSpan := StartHTTPSpan(ctx, "POST", "/users/delete")
	EndHTTPSpan(httpSpan, 302)
	_, storeSpan := StartStoreSpan(ctx, "SearchEntriesWithOptions")
	EndStoreSpan(storeSpan, nil)

	spans := tracetest.SpanStubsFromReadOnlySpans(recorder.Ended())
	assertSpan(t, spans, "ldap.bind", codes.Ok, attribute.String("ldap.operation", "bind"), attribute.String("ldap.result_code", "0"))
	assertSpan(t, spans, "http.request", codes.Ok, attribute.String("http.request.method", "POST"), attribute.String("http.route", "/users/delete"), attribute.Int("http.response.status_code", 302))
	assertSpan(t, spans, "store.SearchEntriesWithOptions", codes.Ok, attribute.String("store.method", "SearchEntriesWithOptions"))
}

func assertSpan(t *testing.T, spans tracetest.SpanStubs, name string, status codes.Code, attrs ...attribute.KeyValue) {
	t.Helper()

	for _, span := range spans {
		if span.Name != name {
			continue
		}
		if span.Status.Code != status {
			t.Fatalf("span %s status = %v, want %v", name, span.Status.Code, status)
		}
		for _, want := range attrs {
			found := false
			for _, got := range span.Attributes {
				if got.Key == want.Key && got.Value == want.Value {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("span %s missing attribute %v in %v", name, want, span.Attributes)
			}
		}
		return
	}

	t.Fatalf("span %s not found in %v", name, spans)
}
