package middleware

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

type contextKey string

const UserDNKey contextKey = "user_dn"

// Auth is the authentication middleware that validates HTTP Basic Auth
// against LDAP credentials and checks admin group membership
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

// RequireAuth is middleware that requires Basic Auth with LDAP credentials
// and membership in the ldaplite.admin group
func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			a.requestAuth(w)
			return
		}

		// Parse Basic Auth
		const prefix = "Basic "
		if !strings.HasPrefix(authHeader, prefix) {
			a.requestAuth(w)
			return
		}

		// Decode base64
		decoded, err := base64.StdEncoding.DecodeString(authHeader[len(prefix):])
		if err != nil {
			a.requestAuth(w)
			return
		}

		// Split username:password
		credentials := string(decoded)
		colonIndex := strings.Index(credentials, ":")
		if colonIndex == -1 {
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
			a.requestAuth(w)
			return
		}

		// Search for the admin group entry to get its actual DN
		adminGroups, err := a.store.SearchEntries(ctx, a.cfg.LDAP.BaseDN, "(&(objectClass=groupOfNames)(cn=ldaplite.admin))")
		if err != nil {
			slog.Error("Failed to search for admin group", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if len(adminGroups) == 0 {
			slog.Error("Admin group not found", "expected_cn", "ldaplite.admin")
			http.Error(w, "Internal server error: admin group not configured", http.StatusInternalServerError)
			return
		}
		adminGroupDN := adminGroups[0].DN

		// Check admin group membership
		isMember, err := a.store.IsUserInGroup(ctx, userDN, adminGroupDN)
		if err != nil {
			slog.Error("Failed to check group membership", "user_dn", userDN, "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !isMember {
			slog.Warn("Access denied: user not in admin group", "user_dn", userDN)
			http.Error(w, "Access denied: admin privileges required", http.StatusForbidden)
			return
		}

		// Add user DN to context
		ctx = context.WithValue(ctx, UserDNKey, userDN)
		next.ServeHTTP(w, r.WithContext(ctx))
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
	w.Header().Set("WWW-Authenticate", `Basic realm="LDAPLite Admin"`)
	http.Error(w, "Authentication required", http.StatusUnauthorized)
}

// GetUserDN retrieves the authenticated user DN from the request context
func GetUserDN(r *http.Request) string {
	if dn, ok := r.Context().Value(UserDNKey).(string); ok {
		return dn
	}
	return ""
}
