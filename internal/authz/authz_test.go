package authz

import (
	"context"
	"errors"
	"testing"
)

const (
	testBaseDN        = "dc=example,dc=com"
	testAdminDN       = "uid=admin,ou=users,dc=example,dc=com"
	testUserDN        = "uid=jane,ou=users,dc=example,dc=com"
	testReadOnlyDN    = "uid=app,ou=users,dc=example,dc=com"
	testPasswordDN    = "uid=help,ou=users,dc=example,dc=com"
	testNestedAdminDN = "uid=nested,ou=users,dc=example,dc=com"
)

func TestCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		actor    Actor
		groups   map[string]map[string]bool
		want     []Capability
		wantNot  []Capability
		wantErr  bool
		storeErr error
	}{
		{
			name:    "unbound actor has no capabilities",
			actor:   Actor{},
			wantNot: allCapabilities(),
		},
		{
			name:    "anonymous bind has no capabilities",
			actor:   Actor{Bound: true},
			wantNot: allCapabilities(),
		},
		{
			name:  "authenticated user gets read ui and self password",
			actor: BoundUser(testUserDN),
			want:  []Capability{DirectoryRead, UIRead, PasswordChangeSelf},
			wantNot: []Capability{
				DirectoryWrite,
				DirectoryManageGroups,
				PasswordResetAny,
				UIAdmin,
			},
		},
		{
			name:  "explicit read only user gets default read capabilities",
			actor: BoundUser(testReadOnlyDN),
			groups: membershipMap(testReadOnlyDN, map[string]bool{
				"cn=ldaplite.readonly,ou=groups,dc=example,dc=com": true,
			}),
			want: []Capability{DirectoryRead, UIRead, PasswordChangeSelf},
			wantNot: []Capability{
				DirectoryWrite,
				DirectoryManageGroups,
				PasswordResetAny,
				UIAdmin,
			},
		},
		{
			name:  "password group user gets account-only ui access",
			actor: BoundUser(testPasswordDN),
			groups: membershipMap(testPasswordDN, map[string]bool{
				"cn=ldaplite.password,ou=groups,dc=example,dc=com": true,
			}),
			want: []Capability{UIRead, PasswordChangeSelf},
			wantNot: []Capability{
				DirectoryRead,
				DirectoryWrite,
				DirectoryManageGroups,
				PasswordResetAny,
				UIAdmin,
			},
		},
		{
			name:  "admin gets full capabilities",
			actor: BoundUser(testAdminDN),
			groups: membershipMap(testAdminDN, map[string]bool{
				"cn=ldaplite.admin,ou=groups,dc=example,dc=com": true,
			}),
			want: allCapabilities(),
		},
		{
			name:  "nested admin membership is honored by store result",
			actor: BoundUser(testNestedAdminDN),
			groups: membershipMap(testNestedAdminDN, map[string]bool{
				"cn=ldaplite.admin,ou=groups,dc=example,dc=com": true,
			}),
			want: allCapabilities(),
		},
		{
			name:     "membership error is returned",
			actor:    BoundUser(testUserDN),
			storeErr: errors.New("membership failed"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorizer := New(testBaseDN, &membershipStore{
				groups: tt.groups,
				err:    tt.storeErr,
			})

			got, err := authorizer.Capabilities(context.Background(), tt.actor)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Capabilities() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			for _, capability := range tt.want {
				if !got.Has(capability) {
					t.Fatalf("Capabilities() missing %s in %v", capability, got)
				}
			}
			for _, capability := range tt.wantNot {
				if got.Has(capability) {
					t.Fatalf("Capabilities() unexpectedly includes %s in %v", capability, got)
				}
			}
		})
	}
}

func TestGroupDNs(t *testing.T) {
	authorizer := New(testBaseDN, nil)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "admin", got: authorizer.AdminGroupDN(), want: "cn=ldaplite.admin,ou=groups,dc=example,dc=com"},
		{name: "read only", got: authorizer.ReadOnlyGroupDN(), want: "cn=ldaplite.readonly,ou=groups,dc=example,dc=com"},
		{name: "password", got: authorizer.PasswordGroupDN(), want: "cn=ldaplite.password,ou=groups,dc=example,dc=com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("group DN = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestCanWrite(t *testing.T) {
	tests := []struct {
		name       string
		actor      Actor
		admin      bool
		storeErr   error
		want       bool
		wantErr    bool
		wantChecks int
	}{
		{name: "unbound denied", want: false},
		{name: "anonymous denied", actor: Actor{Bound: true}, want: false},
		{name: "authenticated non admin denied", actor: BoundUser(testUserDN), want: false, wantChecks: 2},
		{name: "admin allowed", actor: BoundUser(testAdminDN), admin: true, want: true, wantChecks: 1},
		{name: "read only denied", actor: BoundUser(testReadOnlyDN), want: false, wantChecks: 2},
		{
			name:       "membership error returned",
			actor:      BoundUser(testUserDN),
			storeErr:   errors.New("membership failed"),
			want:       false,
			wantErr:    true,
			wantChecks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &membershipStore{}
			if tt.actor.DN != "" {
				store.groups = membershipMap(tt.actor.DN, map[string]bool{
					"cn=ldaplite.admin,ou=groups,dc=example,dc=com": tt.admin,
				})
			}
			store.err = tt.storeErr

			got, err := New(testBaseDN, store).CanWrite(context.Background(), tt.actor)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CanWrite() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("CanWrite() = %v, want %v", got, tt.want)
			}
			if store.checks != tt.wantChecks {
				t.Fatalf("membership checks = %d, want %d", store.checks, tt.wantChecks)
			}
		})
	}
}

func allCapabilities() []Capability {
	return []Capability{
		DirectoryRead,
		DirectoryWrite,
		DirectoryManageGroups,
		PasswordChangeSelf,
		PasswordResetAny,
		UIRead,
		UIAdmin,
	}
}

func membershipMap(userDN string, groups map[string]bool) map[string]map[string]bool {
	return map[string]map[string]bool{userDN: groups}
}

type membershipStore struct {
	groups map[string]map[string]bool
	err    error
	checks int
}

func (s *membershipStore) IsUserInGroup(ctx context.Context, userDN, groupDN string) (bool, error) {
	s.checks++
	if s.err != nil {
		return false, s.err
	}
	return s.groups[userDN][groupDN], nil
}
