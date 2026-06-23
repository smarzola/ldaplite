package middleware

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/smarzola/ldaplite/internal/audit"
	"github.com/smarzola/ldaplite/internal/authz"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

type contextKey string

const UserDNKey contextKey = "user_dn"
const capabilitiesKey contextKey = "capabilities"

// Auth is the authentication middleware that validates HTTP Basic Auth against
// LDAP credentials and attaches resolved capabilities to the request context.
type Auth struct {
	store  store.Store
	cfg    *config.Config
	hasher *crypto.PasswordHasher
}

// NewAuth creates a new authentication middleware
func NewAuth(st store.Store, cfg *config.Config) *Auth {
	return &Auth{
		store:  st,
		cfg:    cfg,
		hasher: crypto.NewPasswordHasher(cfg.Security.Argon2Config),
	}
}

// RequireAuth is middleware that requires Basic Auth with LDAP credentials and
// ui.read capability.
func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return a.RequireCapability(authz.UIRead, next)
}

// RequireCapability requires Basic Auth and a specific Web UI capability.
func (a *Auth) RequireCapability(capability authz.Capability, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			a.logAuthRequired(r, "")
			a.requestAuth(w)
			return
		}

		// Parse Basic Auth
		const prefix = "Basic "
		if !strings.HasPrefix(authHeader, prefix) {
			a.logAuthRequired(r, "")
			a.requestAuth(w)
			return
		}

		// Decode base64
		decoded, err := base64.StdEncoding.DecodeString(authHeader[len(prefix):])
		if err != nil {
			a.logAuthRequired(r, "")
			a.requestAuth(w)
			return
		}

		// Split username:password
		credentials := string(decoded)
		colonIndex := strings.Index(credentials, ":")
		if colonIndex == -1 {
			a.logAuthRequired(r, "")
			a.requestAuth(w)
			return
		}

		uid := credentials[:colonIndex]
		password := credentials[colonIndex+1:]

		// Authenticate against LDAP
		ctx := r.Context()
		userDN, err := a.authenticate(ctx, uid, password)
		if err != nil {
			slog.Warn("Authentication failed", "uid", uid, "error", err)
			audit.LogWeb(ctx, audit.WebEvent{
				Event:      audit.EventWebAuthFailed,
				RemoteAddr: r.RemoteAddr,
				ActorUID:   uid,
				Method:     r.Method,
				Route:      NormalizeRoute(r.URL.Path),
				Status:     http.StatusUnauthorized,
				Error:      err,
			})
			a.requestAuth(w)
			return
		}
		audit.SetActorDN(ctx, userDN)

		capabilities, err := authz.New(a.cfg.LDAP.BaseDN, a.store).Capabilities(ctx, authz.BoundUser(userDN))
		if err != nil {
			slog.Error("Failed to resolve capabilities", "user_dn", userDN, "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !capabilities.Has(capability) {
			slog.Warn("Access denied: missing capability", "user_dn", userDN, "capability", capability)
			audit.LogWeb(ctx, audit.WebEvent{
				Event:      audit.EventWebAuthorizationDeny,
				RemoteAddr: r.RemoteAddr,
				ActorDN:    userDN,
				Method:     r.Method,
				Route:      NormalizeRoute(r.URL.Path),
				Status:     http.StatusForbidden,
			})
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		// Add user DN and capabilities to context.
		ctx = context.WithValue(ctx, UserDNKey, userDN)
		ctx = context.WithValue(ctx, capabilitiesKey, capabilities)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) logAuthRequired(r *http.Request, uid string) {
	audit.LogWeb(r.Context(), audit.WebEvent{
		Event:      audit.EventWebAuthRequired,
		RemoteAddr: r.RemoteAddr,
		ActorUID:   uid,
		Method:     r.Method,
		Route:      NormalizeRoute(r.URL.Path),
		Status:     http.StatusUnauthorized,
	})
}

// authenticate validates credentials against LDAP and returns the user DN
func (a *Auth) authenticate(ctx context.Context, uid, password string) (string, error) {
	// Get password hash and DN from store
	passwordHash, userDN, err := a.store.GetUserPasswordHash(ctx, uid)
	if err != nil {
		return "", fmt.Errorf("failed to get password hash: %w", err)
	}

	if passwordHash == "" || userDN == "" {
		return "", fmt.Errorf("user not found: %s", uid)
	}

	// Verify password
	valid, err := a.hasher.Verify(password, passwordHash)
	if err != nil {
		return "", fmt.Errorf("password verification failed: %w", err)
	}
	if !valid {
		return "", fmt.Errorf("invalid credentials")
	}

	// Return user DN
	return userDN, nil
}

// requestAuth sends a 401 response requesting Basic Auth
func (a *Auth) requestAuth(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="LDAPLite Web UI"`)
	http.Error(w, "Authentication required", http.StatusUnauthorized)
}

// GetUserDN retrieves the authenticated user DN from the request context
func GetUserDN(r *http.Request) string {
	if dn, ok := r.Context().Value(UserDNKey).(string); ok {
		return dn
	}
	return ""
}

func GetCapabilities(r *http.Request) authz.Set {
	if capabilities, ok := r.Context().Value(capabilitiesKey).(authz.Set); ok {
		return capabilities
	}
	return authz.NewSet()
}
