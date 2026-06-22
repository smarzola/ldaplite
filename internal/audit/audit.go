package audit

import (
	"context"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"
)

const (
	ComponentLDAP = "ldap"
	ComponentWeb  = "web"

	EventLDAPOperation    = "ldap.operation"
	EventLDAPReadError    = "ldap.read_error"
	EventLDAPHandlerError = "ldap.handler_error"

	EventHTTPRequest          = "http.request"
	EventWebAuthRequired      = "web.auth_required"
	EventWebAuthFailed        = "web.auth_failed"
	EventWebAuthorizationDeny = "web.authorization_denied"
	EventWebSameOriginDeny    = "web.same_origin_denied"
	EventWebWrite             = "web.write"
)

var nextRequestID atomic.Uint64

type contextKey string

const requestInfoKey contextKey = "audit_request_info"

type RequestInfo struct {
	RequestID string
	ActorDN   string
	Route     string
	Method    string
}

type LDAPEvent struct {
	Event        string
	Operation    string
	RequestID    string
	ConnectionID string
	MessageID    int
	RemoteAddr   string
	ActorDN      string
	TargetDN     string
	BaseDN       string
	OID          string
	Scope        string
	ResultCode   int
	ResultCount  *int
	Duration     time.Duration
	Error        error
}

type WebEvent struct {
	Event      string
	RequestID  string
	RemoteAddr string
	ActorDN    string
	ActorUID   string
	Method     string
	Route      string
	Operation  string
	TargetDN   string
	Resource   string
	Status     int
	Duration   time.Duration
	Error      error
}

func NewWebRequestID() string {
	return "http-" + strconv.FormatUint(nextRequestID.Add(1), 10)
}

func WithRequestInfo(ctx context.Context, info *RequestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey, info)
}

func RequestInfoFromContext(ctx context.Context) *RequestInfo {
	info, _ := ctx.Value(requestInfoKey).(*RequestInfo)
	return info
}

func SetActorDN(ctx context.Context, actorDN string) {
	if info := RequestInfoFromContext(ctx); info != nil {
		info.ActorDN = actorDN
	}
}

func RequestIDFromContext(ctx context.Context) string {
	if info := RequestInfoFromContext(ctx); info != nil {
		return info.RequestID
	}
	return ""
}

func LogLDAP(ctx context.Context, event LDAPEvent) {
	if event.Event == "" {
		event.Event = EventLDAPOperation
	}

	attrs := []slog.Attr{
		slog.String("event", event.Event),
		slog.String("component", ComponentLDAP),
	}
	addStringAttr(&attrs, "operation", event.Operation)
	addStringAttr(&attrs, "request_id", event.RequestID)
	addStringAttr(&attrs, "connection_id", event.ConnectionID)
	if event.MessageID != 0 {
		attrs = append(attrs, slog.Int("message_id", event.MessageID))
	}
	addStringAttr(&attrs, "remote_addr", event.RemoteAddr)
	addStringAttr(&attrs, "actor_dn", event.ActorDN)
	addStringAttr(&attrs, "target_dn", event.TargetDN)
	addStringAttr(&attrs, "base_dn", event.BaseDN)
	addStringAttr(&attrs, "oid", event.OID)
	addStringAttr(&attrs, "scope", event.Scope)
	if event.ResultCode >= 0 {
		attrs = append(attrs, slog.Int("result_code", event.ResultCode))
	}
	if event.ResultCount != nil {
		attrs = append(attrs, slog.Int("result_count", *event.ResultCount))
	}
	if event.Duration > 0 {
		attrs = append(attrs, slog.Int64("duration_ms", event.Duration.Milliseconds()))
	}
	if event.Error != nil {
		attrs = append(attrs, slog.String("error", event.Error.Error()))
	}

	slog.LogAttrs(ctx, ldapLogLevel(event), "LDAP audit event", attrs...)
}

func LogWeb(ctx context.Context, event WebEvent) {
	if event.Event == "" {
		event.Event = EventHTTPRequest
	}
	if event.RequestID == "" {
		event.RequestID = RequestIDFromContext(ctx)
	}
	if event.ActorDN == "" {
		if info := RequestInfoFromContext(ctx); info != nil {
			event.ActorDN = info.ActorDN
		}
	}

	attrs := []slog.Attr{
		slog.String("event", event.Event),
		slog.String("component", ComponentWeb),
	}
	addStringAttr(&attrs, "operation", event.Operation)
	addStringAttr(&attrs, "request_id", event.RequestID)
	addStringAttr(&attrs, "remote_addr", event.RemoteAddr)
	addStringAttr(&attrs, "actor_dn", event.ActorDN)
	addStringAttr(&attrs, "actor_uid", event.ActorUID)
	addStringAttr(&attrs, "method", event.Method)
	addStringAttr(&attrs, "route", event.Route)
	addStringAttr(&attrs, "target_dn", event.TargetDN)
	addStringAttr(&attrs, "resource", event.Resource)
	if event.Status != 0 {
		attrs = append(attrs, slog.Int("status", event.Status))
	}
	if event.Duration > 0 {
		attrs = append(attrs, slog.Int64("duration_ms", event.Duration.Milliseconds()))
	}
	if event.Error != nil {
		attrs = append(attrs, slog.String("error", event.Error.Error()))
	}

	slog.LogAttrs(ctx, webLogLevel(event), "Web audit event", attrs...)
}

func RequestID(connectionID string, messageID int) string {
	if connectionID == "" || messageID == 0 {
		return connectionID
	}
	return connectionID + ":" + strconv.Itoa(messageID)
}

func addStringAttr(attrs *[]slog.Attr, name, value string) {
	if value != "" {
		*attrs = append(*attrs, slog.String(name, value))
	}
}

func ldapLogLevel(event LDAPEvent) slog.Level {
	switch event.Event {
	case EventLDAPReadError, EventLDAPHandlerError:
		return slog.LevelError
	default:
		if event.ResultCode == 0 {
			return slog.LevelInfo
		}
		return slog.LevelWarn
	}
}

func webLogLevel(event WebEvent) slog.Level {
	if event.Status >= 500 {
		return slog.LevelError
	}
	if event.Status >= 400 || event.Event == EventWebAuthFailed || event.Event == EventWebAuthorizationDeny || event.Event == EventWebSameOriginDeny {
		return slog.LevelWarn
	}
	return slog.LevelInfo
}
