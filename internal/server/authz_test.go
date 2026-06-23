package server

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/protocol"
	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
)

func testAuthzServer(allowAnonymous bool) *Server {
	return &Server{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				AllowAnonymousBind: allowAnonymous,
			},
			LDAP: config.LDAPConfig{
				BaseDN: "dc=example,dc=com",
			},
		},
		store: &authzStore{},
	}
}

func TestCanSearchAccessPolicy(t *testing.T) {
	tests := []struct {
		name           string
		allowAnonymous bool
		bindDN         *string
		baseDN         string
		want           bool
	}{
		{
			name:   "unbound RootDSE search allowed",
			baseDN: "",
			want:   true,
		},
		{
			name:   "unbound schema search allowed",
			baseDN: "cn=Subschema",
			want:   true,
		},
		{
			name:   "unbound normal search rejected",
			baseDN: "dc=example,dc=com",
			want:   false,
		},
		{
			name:           "anonymous bound normal search rejected when anonymous disabled",
			allowAnonymous: false,
			bindDN:         strPtr(""),
			baseDN:         "dc=example,dc=com",
			want:           false,
		},
		{
			name:           "anonymous bound normal search allowed when anonymous enabled",
			allowAnonymous: true,
			bindDN:         strPtr(""),
			baseDN:         "dc=example,dc=com",
			want:           true,
		},
		{
			name:   "authenticated normal search allowed",
			bindDN: strPtr("uid=admin,ou=users,dc=example,dc=com"),
			baseDN: "dc=example,dc=com",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testAuthzServer(tt.allowAnonymous)
			conn := protocol.NewConnection(nil, protocol.OperationHandlers{})
			if tt.bindDN != nil {
				conn.SetBoundDN(*tt.bindDN)
			}

			if got := srv.canSearch(conn, tt.baseDN); got != tt.want {
				t.Fatalf("canSearch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanWriteAccessPolicy(t *testing.T) {
	tests := []struct {
		name       string
		bindDN     *string
		admin      bool
		storeErr   error
		want       bool
		wantErr    bool
		wantChecks int
	}{
		{
			name: "unbound write rejected",
			want: false,
		},
		{
			name:   "anonymous bound write rejected",
			bindDN: strPtr(""),
			want:   false,
		},
		{
			name:       "admin write allowed",
			bindDN:     strPtr("uid=admin,ou=users,dc=example,dc=com"),
			admin:      true,
			want:       true,
			wantChecks: 1,
		},
		{
			name:       "authenticated non-admin write rejected",
			bindDN:     strPtr("uid=jane,ou=users,dc=example,dc=com"),
			want:       false,
			wantChecks: 1,
		},
		{
			name:       "read-only group member write rejected by default",
			bindDN:     strPtr("uid=app,ou=users,dc=example,dc=com"),
			want:       false,
			wantChecks: 1,
		},
		{
			name:       "membership check error returned",
			bindDN:     strPtr("uid=app,ou=users,dc=example,dc=com"),
			storeErr:   errors.New("membership failed"),
			want:       false,
			wantErr:    true,
			wantChecks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testAuthzServer(true)
			authzStore := &authzStore{admin: tt.admin, err: tt.storeErr}
			srv.store = authzStore
			conn := protocol.NewConnection(nil, protocol.OperationHandlers{})
			if tt.bindDN != nil {
				conn.SetBoundDN(*tt.bindDN)
			}

			got, err := srv.canWrite(context.Background(), conn)
			if (err != nil) != tt.wantErr {
				t.Fatalf("canWrite() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("canWrite() = %v, want %v", got, tt.want)
			}
			if authzStore.checks != tt.wantChecks {
				t.Fatalf("membership checks = %d, want %d", authzStore.checks, tt.wantChecks)
			}
		})
	}
}

func TestCanModifyAccessPolicy(t *testing.T) {
	tests := []struct {
		name       string
		bindDN     *string
		targetDN   string
		changes    []ldapmsg.ModifyChange
		admin      bool
		storeErr   error
		want       bool
		wantErr    bool
		wantChecks int
	}{
		{
			name:       "admin can modify ordinary attribute",
			bindDN:     strPtr("uid=admin,ou=users,dc=example,dc=com"),
			targetDN:   "uid=jane,ou=users,dc=example,dc=com",
			changes:    replaceChange("mail", "jane@example.com"),
			admin:      true,
			want:       true,
			wantChecks: 1,
		},
		{
			name:       "non-admin cannot modify ordinary attribute",
			bindDN:     strPtr("uid=jane,ou=users,dc=example,dc=com"),
			targetDN:   "uid=jane,ou=users,dc=example,dc=com",
			changes:    replaceChange("mail", "jane@example.com"),
			want:       false,
			wantChecks: 1,
		},
		{
			name:       "non-admin can replace own password",
			bindDN:     strPtr("uid=jane,ou=users,dc=example,dc=com"),
			targetDN:   "UID=JANE,OU=USERS,DC=EXAMPLE,DC=COM",
			changes:    replaceChange("userPassword", "NewPassword123!"),
			want:       true,
			wantChecks: 1,
		},
		{
			name:       "non-admin cannot replace another user password",
			bindDN:     strPtr("uid=jane,ou=users,dc=example,dc=com"),
			targetDN:   "uid=bob,ou=users,dc=example,dc=com",
			changes:    replaceChange("userPassword", "NewPassword123!"),
			want:       false,
			wantChecks: 1,
		},
		{
			name:     "non-admin cannot mix own password with ordinary attribute",
			bindDN:   strPtr("uid=jane,ou=users,dc=example,dc=com"),
			targetDN: "uid=jane,ou=users,dc=example,dc=com",
			changes: append(
				replaceChange("userPassword", "NewPassword123!"),
				replaceChange("mail", "jane@example.com")...,
			),
			want:       false,
			wantChecks: 1,
		},
		{
			name:       "non-admin cannot add own password value",
			bindDN:     strPtr("uid=jane,ou=users,dc=example,dc=com"),
			targetDN:   "uid=jane,ou=users,dc=example,dc=com",
			changes:    addChange("userPassword", "NewPassword123!"),
			want:       false,
			wantChecks: 1,
		},
		{
			name:       "membership check error returned",
			bindDN:     strPtr("uid=jane,ou=users,dc=example,dc=com"),
			targetDN:   "uid=jane,ou=users,dc=example,dc=com",
			changes:    replaceChange("userPassword", "NewPassword123!"),
			storeErr:   errors.New("membership failed"),
			want:       false,
			wantErr:    true,
			wantChecks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testAuthzServer(true)
			authzStore := &authzStore{admin: tt.admin, err: tt.storeErr}
			srv.store = authzStore
			conn := protocol.NewConnection(nil, protocol.OperationHandlers{})
			if tt.bindDN != nil {
				conn.SetBoundDN(*tt.bindDN)
			}

			got, err := srv.canModify(context.Background(), conn, tt.targetDN, tt.changes)
			if (err != nil) != tt.wantErr {
				t.Fatalf("canModify() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("canModify() = %v, want %v", got, tt.want)
			}
			if authzStore.checks != tt.wantChecks {
				t.Fatalf("membership checks = %d, want %d", authzStore.checks, tt.wantChecks)
			}
		})
	}
}

func TestPublicSearchBase(t *testing.T) {
	tests := []struct {
		baseDN string
		want   bool
	}{
		{baseDN: "", want: true},
		{baseDN: "cn=Subschema", want: true},
		{baseDN: "cn=subschema", want: true},
		{baseDN: "dc=example,dc=com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.baseDN, func(t *testing.T) {
			if got := isPublicSearchBase(tt.baseDN); got != tt.want {
				t.Fatalf("isPublicSearchBase(%q) = %v, want %v", tt.baseDN, got, tt.want)
			}
		})
	}
}

func TestProtectedAttributePolicy(t *testing.T) {
	if isAddProtectedAttribute("objectClass") {
		t.Fatal("objectClass must be allowed during Add so clients can declare the structural class")
	}
	if !isModifyProtectedAttribute("objectClass") {
		t.Fatal("objectClass must be protected from Modify after entry creation")
	}

	for _, attr := range []string{"createTimestamp", "modifyTimestamp", "memberOf"} {
		t.Run(attr, func(t *testing.T) {
			if !isAddProtectedAttribute(attr) {
				t.Fatalf("%s must be protected during Add", attr)
			}
			if !isModifyProtectedAttribute(attr) {
				t.Fatalf("%s must be protected during Modify", attr)
			}
		})
	}
}

func TestEntryWriteResultCodeUsesTypedStoreErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ldapmsg.ResultCode
	}{
		{
			name: "entry already exists",
			err:  fmt.Errorf("wrapped: %w", store.ErrEntryAlreadyExists),
			want: ldapmsg.ResultCodeEntryAlreadyExists,
		},
		{
			name: "no such object",
			err:  fmt.Errorf("wrapped: %w", store.ErrNoSuchObject),
			want: ldapmsg.ResultCodeNoSuchObject,
		},
		{
			name: "object class violation",
			err:  fmt.Errorf("wrapped: %w", store.ErrObjectClassViolation),
			want: ldapmsg.ResultCodeObjectClassViolation,
		},
		{
			name: "constraint violation",
			err:  fmt.Errorf("wrapped: %w", store.ErrConstraintViolation),
			want: ldapmsg.ResultCodeConstraintViolation,
		},
		{
			name: "unknown error",
			err:  fmt.Errorf("unknown"),
			want: ldapmsg.ResultCodeOperationsError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := entryWriteResultCode(tt.err); got != tt.want {
				t.Fatalf("entryWriteResultCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLDAPSearchScopeMapping(t *testing.T) {
	tests := []struct {
		name  string
		scope ldapmsg.SearchScope
		want  store.SearchScope
	}{
		{name: "base object", scope: ldapmsg.SearchScopeBaseObject, want: store.SearchScopeBaseObject},
		{name: "single level", scope: ldapmsg.SearchScopeSingleLevel, want: store.SearchScopeSingleLevel},
		{name: "whole subtree", scope: ldapmsg.SearchScopeWholeSubtree, want: store.SearchScopeWholeSubtree},
		{name: "unknown defaults to subtree", scope: 99, want: store.SearchScopeWholeSubtree},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ldapSearchScope(tt.scope); got != tt.want {
				t.Fatalf("ldapSearchScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

type authzStore struct {
	admin  bool
	err    error
	checks int
}

func (s *authzStore) Initialize(ctx context.Context) error { return nil }

func (s *authzStore) Close() error { return nil }

func (s *authzStore) GetEntry(ctx context.Context, dn string) (*models.Entry, error) { return nil, nil }

func (s *authzStore) GetEntryWithOptions(ctx context.Context, dn string, options store.EntryOptions) (*models.Entry, error) {
	return nil, nil
}

func (s *authzStore) CreateEntry(ctx context.Context, entry *models.Entry) error { return nil }

func (s *authzStore) UpdateEntry(ctx context.Context, entry *models.Entry) error { return nil }

func (s *authzStore) DeleteEntry(ctx context.Context, dn string) error { return nil }

func (s *authzStore) SearchEntries(ctx context.Context, baseDN string, filter string) ([]*models.Entry, error) {
	return nil, nil
}

func (s *authzStore) SearchEntriesWithOptions(ctx context.Context, options store.SearchOptions) ([]*models.Entry, error) {
	return nil, nil
}

func (s *authzStore) EntryExists(ctx context.Context, dn string) (bool, error) { return false, nil }

func (s *authzStore) GetUserPasswordHash(ctx context.Context, uid string) (string, string, error) {
	return "", "", nil
}

func (s *authzStore) GetUserPasswordHashByDN(ctx context.Context, dn string) (string, string, error) {
	return "", "", nil
}

func (s *authzStore) IsUserInGroup(ctx context.Context, userDN, groupDN string) (bool, error) {
	s.checks++
	if want := "cn=ldaplite.admin,ou=groups,dc=example,dc=com"; groupDN != want {
		return false, fmt.Errorf("groupDN = %q, want %q", groupDN, want)
	}
	return s.admin, s.err
}

func replaceChange(attr string, values ...string) []ldapmsg.ModifyChange {
	return []ldapmsg.ModifyChange{{
		Operation: ldapmsg.ModifyOperationReplace,
		Modification: ldapmsg.Attribute{
			Name:   attr,
			Values: values,
		},
	}}
}

func addChange(attr string, values ...string) []ldapmsg.ModifyChange {
	return []ldapmsg.ModifyChange{{
		Operation: ldapmsg.ModifyOperationAdd,
		Modification: ldapmsg.Attribute{
			Name:   attr,
			Values: values,
		},
	}}
}
