package audit

import (
	"context"
	"log/slog"
	"strconv"
	"time"
)

const (
	ComponentLDAP = "ldap"

	EventLDAPOperation    = "ldap.operation"
	EventLDAPReadError    = "ldap.read_error"
	EventLDAPHandlerError = "ldap.handler_error"
)

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
