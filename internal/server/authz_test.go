package server

import (
	"fmt"
	"testing"

	"github.com/lor00x/goldap/message"

	"github.com/smarzola/ldaplite/internal/protocol"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
)

func testAuthzServer(allowAnonymous bool) *Server {
	return &Server{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				AllowAnonymousBind: allowAnonymous,
			},
		},
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
		name   string
		bindDN *string
		want   bool
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
			name:   "authenticated write allowed",
			bindDN: strPtr("uid=admin,ou=users,dc=example,dc=com"),
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testAuthzServer(true)
			conn := protocol.NewConnection(nil, protocol.OperationHandlers{})
			if tt.bindDN != nil {
				conn.SetBoundDN(*tt.bindDN)
			}

			if got := srv.canWrite(conn); got != tt.want {
				t.Fatalf("canWrite() = %v, want %v", got, tt.want)
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

	for _, attr := range []string{"createTimestamp", "modifyTimestamp"} {
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
		want int
	}{
		{
			name: "entry already exists",
			err:  fmt.Errorf("wrapped: %w", store.ErrEntryAlreadyExists),
			want: message.ResultCodeEntryAlreadyExists,
		},
		{
			name: "no such object",
			err:  fmt.Errorf("wrapped: %w", store.ErrNoSuchObject),
			want: message.ResultCodeNoSuchObject,
		},
		{
			name: "object class violation",
			err:  fmt.Errorf("wrapped: %w", store.ErrObjectClassViolation),
			want: message.ResultCodeObjectClassViolation,
		},
		{
			name: "constraint violation",
			err:  fmt.Errorf("wrapped: %w", store.ErrConstraintViolation),
			want: message.ResultCodeConstraintViolation,
		},
		{
			name: "unknown error",
			err:  fmt.Errorf("unknown"),
			want: message.ResultCodeOperationsError,
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

func strPtr(s string) *string {
	return &s
}
