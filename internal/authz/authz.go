package authz

import (
	"context"
	"fmt"
)

// Capability is a coarse LDAPLite permission used across LDAP, Web UI, and
// future HTTP provisioning surfaces.
type Capability string

const (
	DirectoryRead         Capability = "directory.read"
	DirectoryWrite        Capability = "directory.write"
	DirectoryManageGroups Capability = "directory.manageGroups"
	PasswordChangeSelf    Capability = "password.changeSelf"
	PasswordResetAny      Capability = "password.resetAny"
	UIRead                Capability = "ui.read"
	UIAdmin               Capability = "ui.admin"
)

type Set map[Capability]struct{}

func NewSet(capabilities ...Capability) Set {
	set := make(Set, len(capabilities))
	for _, capability := range capabilities {
		set[capability] = struct{}{}
	}
	return set
}

func (s Set) Has(capability Capability) bool {
	_, ok := s[capability]
	return ok
}

func (s Set) Add(capabilities ...Capability) {
	for _, capability := range capabilities {
		s[capability] = struct{}{}
	}
}

type Actor struct {
	DN    string
	Bound bool
}

func BoundUser(dn string) Actor {
	return Actor{DN: dn, Bound: true}
}

type MembershipStore interface {
	IsUserInGroup(ctx context.Context, userDN, groupDN string) (bool, error)
}

type Authorizer struct {
	baseDN string
	store  MembershipStore
}

func New(baseDN string, store MembershipStore) *Authorizer {
	return &Authorizer{
		baseDN: baseDN,
		store:  store,
	}
}

func (a *Authorizer) Capabilities(ctx context.Context, actor Actor) (Set, error) {
	if !actor.Bound || actor.DN == "" {
		return NewSet(), nil
	}

	capabilities := NewSet(DirectoryRead, PasswordChangeSelf, UIRead)

	isAdmin, err := a.IsAdmin(ctx, actor.DN)
	if err != nil {
		return nil, err
	}
	if isAdmin {
		capabilities.Add(
			DirectoryWrite,
			DirectoryManageGroups,
			PasswordResetAny,
			UIAdmin,
		)
	}

	return capabilities, nil
}

func (a *Authorizer) IsAdmin(ctx context.Context, userDN string) (bool, error) {
	return a.isMember(ctx, userDN, a.AdminGroupDN())
}

func (a *Authorizer) IsExplicitReadOnly(ctx context.Context, userDN string) (bool, error) {
	return a.isMember(ctx, userDN, a.ReadOnlyGroupDN())
}

// CanWriteLegacy preserves LDAPLite's pre-redesign write policy until the
// breaking least-privilege inversion is implemented in the next milestone.
func (a *Authorizer) CanWriteLegacy(ctx context.Context, actor Actor) (bool, error) {
	if !actor.Bound || actor.DN == "" {
		return false, nil
	}

	isReadOnly, err := a.IsExplicitReadOnly(ctx, actor.DN)
	if err != nil {
		return false, err
	}
	return !isReadOnly, nil
}

func (a *Authorizer) AdminGroupDN() string {
	return a.groupDN("ldaplite.admin")
}

func (a *Authorizer) ReadOnlyGroupDN() string {
	return a.groupDN("ldaplite.readonly")
}

func (a *Authorizer) PasswordGroupDN() string {
	return a.groupDN("ldaplite.password")
}

func (a *Authorizer) groupDN(cn string) string {
	if a == nil || a.baseDN == "" {
		return ""
	}
	return fmt.Sprintf("cn=%s,ou=groups,%s", cn, a.baseDN)
}

func (a *Authorizer) isMember(ctx context.Context, userDN, groupDN string) (bool, error) {
	if a == nil || a.store == nil || userDN == "" || groupDN == "" {
		return false, nil
	}
	return a.store.IsUserInGroup(ctx, userDN, groupDN)
}
