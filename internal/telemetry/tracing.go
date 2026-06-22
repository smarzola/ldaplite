package telemetry

import (
	"context"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func StartLDAPSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return otel.Tracer("github.com/smarzola/ldaplite").Start(
		ctx,
		"ldap."+operation,
		trace.WithAttributes(attribute.String("ldap.operation", operation)),
	)
}

func EndLDAPSpan(span trace.Span, resultCode int) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.String("ldap.result_code", strconv.Itoa(resultCode)))
	if resultCode == 0 {
		span.SetStatus(codes.Ok, "")
	} else {
		span.SetStatus(codes.Error, "LDAP result code "+strconv.Itoa(resultCode))
	}
	span.End()
}

func StartHTTPSpan(ctx context.Context, method, route string) (context.Context, trace.Span) {
	return otel.Tracer("github.com/smarzola/ldaplite").Start(
		ctx,
		"http.request",
		trace.WithAttributes(
			attribute.String("http.request.method", method),
			attribute.String("http.route", route),
		),
	)
}

func EndHTTPSpan(span trace.Span, status int) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.Int("http.response.status_code", status))
	if status >= 500 {
		span.SetStatus(codes.Error, "HTTP status "+strconv.Itoa(status))
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

func StartStoreSpan(ctx context.Context, method string) (context.Context, trace.Span) {
	return otel.Tracer("github.com/smarzola/ldaplite").Start(
		ctx,
		"store."+method,
		trace.WithAttributes(attribute.String("store.method", method)),
	)
}

func EndStoreSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}
