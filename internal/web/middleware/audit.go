package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/smarzola/ldaplite/internal/audit"
	"github.com/smarzola/ldaplite/internal/telemetry"
)

func AuditHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := &audit.RequestInfo{
			RequestID: audit.NewWebRequestID(),
			Method:    r.Method,
			Route:     NormalizeRoute(r.URL.Path),
		}
		ctx := audit.WithRequestInfo(r.Context(), info)
		r = r.WithContext(ctx)

		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}
		recorder.Header().Set("X-Request-ID", info.RequestID)

		start := time.Now()
		next.ServeHTTP(recorder, r)
		duration := time.Since(start)

		audit.LogWeb(r.Context(), audit.WebEvent{
			Event:      audit.EventHTTPRequest,
			RequestID:  info.RequestID,
			RemoteAddr: r.RemoteAddr,
			ActorDN:    info.ActorDN,
			Method:     r.Method,
			Route:      info.Route,
			Status:     recorder.status,
			Duration:   duration,
		})
		telemetry.RecordHTTPRequest(r.Context(), r.Method, info.Route, recorder.status, duration)
	})
}

func NormalizeRoute(path string) string {
	switch path {
	case "/", "/logout", "/users", "/users/new", "/users/edit", "/users/delete", "/groups", "/groups/new", "/groups/edit", "/groups/delete", "/ous", "/ous/new", "/ous/edit", "/ous/delete":
		return path
	default:
		if strings.HasPrefix(path, "/static/") {
			return "/static/"
		}
		return "unknown"
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
