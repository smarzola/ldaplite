package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGroup(t *testing.T) {
	baseDN := "dc=example,dc=com"
	group := NewGroup(baseDN, "developers", "Developer Group")

	assert.NotNil(t, group)
	assert.Equal(t, "cn=developers,ou=groups,dc=example,dc=com", group.DN)
	assert.Equal(t, "developers", group.CN)
	assert.True(t, group.IsGroup())
	assert.Empty(t, group.Members)
}

func TestGroupAddMember(t *testing.T) {
	group := NewGroup("dc=example,dc=com", "developers", "")
	memberDN := "uid=john,ou=users,dc=example,dc=com"

	group.AddMember(memberDN)

	assert.Equal(t, 1, len(group.Members))
	assert.True(t, group.HasMember(memberDN))
}

func TestGroupAddMemberDuplicate(t *testing.T) {
	group := NewGroup("dc=example,dc=com", "developers", "")
	memberDN := "uid=john,ou=users,dc=example,dc=com"

	group.AddMember(memberDN)
	group.AddMember(memberDN)

	// Should not add duplicate
	assert.Equal(t, 1, len(group.Members))
}

func TestGroupRemoveMember(t *testing.T) {
	group := NewGroup("dc=example,dc=com", "developers", "")
	memberDN := "uid=john,ou=users,dc=example,dc=com"

	group.AddMember(memberDN)
	assert.True(t, group.HasMember(memberDN))

	err := group.RemoveMember(memberDN)
	assert.NoError(t, err)
	assert.False(t, group.HasMember(memberDN))
	assert.Empty(t, group.Members)
}

func TestGroupRemoveMemberNotFound(t *testing.T) {
	group := NewGroup("dc=example,dc=com", "developers", "")
	memberDN := "uid=john,ou=users,dc=example,dc=com"

	err := group.RemoveMember(memberDN)
	assert.Error(t, err)
}

func TestGroupMultipleMembers(t *testing.T) {
	group := NewGroup("dc=example,dc=com", "developers", "")
	members := []string{
		"uid=john,ou=users,dc=example,dc=com",
		"uid=jane,ou=users,dc=example,dc=com",
		"uid=bob,ou=users,dc=example,dc=com",
	}

	for _, member := range members {
		group.AddMember(member)
	}

	assert.Equal(t, 3, len(group.Members))
	assert.Equal(t, members, group.GetMembers())
}

func TestGroupSetDescription(t *testing.T) {
	group := NewGroup("dc=example,dc=com", "developers", "Initial Description")

	group.SetDescription("Updated Description")

	assert.Equal(t, "Updated Description", group.GetDescription())
}

func TestValidateGroup(t *testing.T) {
	group := NewGroup("dc=example,dc=com", "developers", "")

	err := group.ValidateGroup()
	assert.NoError(t, err)
}

func TestValidateGroupMissingCN(t *testing.T) {
	group := NewGroup("dc=example,dc=com", "developers", "")
	group.Entry.RemoveAttribute("cn")

	err := group.ValidateGroup()
	assert.Error(t, err)
}

func TestLoadMembersFromAttributes(t *testing.T) {
	group := NewGroup("dc=example,dc=com", "developers", "")
	group.Entry.AddAttribute("member", "uid=john,ou=users,dc=example,dc=com")
	group.Entry.AddAttribute("member", "uid=jane,ou=users,dc=example,dc=com")

	group.LoadMembersFromAttributes()

	assert.Equal(t, 2, len(group.Members))
	assert.Contains(t, group.Members, "uid=john,ou=users,dc=example,dc=com")
	assert.Contains(t, group.Members, "uid=jane,ou=users,dc=example,dc=com")
}

func TestExtractCNFromDN(t *testing.T) {
	tests := []struct {
		name    string
		dn      string
		cn      string
		wantErr bool
	}{
		{
			name:    "valid group dn",
			dn:      "cn=developers,ou=groups,dc=example,dc=com",
			cn:      "developers",
			wantErr: false,
		},
		{
			name:    "simple cn",
			dn:      "cn=admins,dc=example,dc=com",
			cn:      "admins",
			wantErr: false,
		},
		{
			name:    "invalid dn",
			dn:      "uid=john,dc=example,dc=com",
			cn:      "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cn, err := ExtractCNFromDN(tt.dn)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.cn, cn)
			}
		})
	}
}

func TestExtractOUFromDN(t *testing.T) {
	tests := []struct {
		name    string
		dn      string
		ou      string
		wantErr bool
	}{
		{
			name:    "valid dn",
			dn:      "cn=developers,ou=groups,dc=example,dc=com",
			ou:      "groups",
			wantErr: false,
		},
		{
			name:    "user ou",
			dn:      "uid=john,ou=users,dc=example,dc=com",
			ou:      "users",
			wantErr: false,
		},
		{
			name:    "invalid dn",
			dn:      "dc=example,dc=com",
			ou:      "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ou, err := ExtractOUFromDN(tt.dn)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.ou, ou)
			}
		})
	}
}
