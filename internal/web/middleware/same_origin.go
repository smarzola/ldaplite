package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// RequireSameOrigin rejects mutating requests that do not come from the same
// host. This protects the Basic Auth Web UI from cross-site form submissions.
func RequireSameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isMutatingMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if !sameOrigin(r) {
			http.Error(w, "Forbidden: invalid request origin", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func sameOrigin(r *http.Request) bool {
	expectedHost := forwardedHost(r)

	if origin := r.Header.Get("Origin"); origin != "" {
		return originMatchesHost(origin, expectedHost)
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		return originMatchesHost(referer, expectedHost)
	}

	return false
}

func forwardedHost(r *http.Request) string {
	if host := r.Header.Get("X-Forwarded-Host"); host != "" {
		return strings.TrimSpace(strings.Split(host, ",")[0])
	}
	return r.Host
}

func originMatchesHost(rawURL, expectedHost string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, expectedHost)
}
