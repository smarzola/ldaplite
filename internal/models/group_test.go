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
	assert.Contains(t, group.Members, memberDN)
}

func TestGroupAddMemberDuplicate(t *testing.T) {
	group := NewGroup("dc=example,dc=com", "developers", "")
	memberDN := "uid=john,ou=users,dc=example,dc=com"

	group.AddMember(memberDN)
	group.AddMember(memberDN)

	// Should not add duplicate
	assert.Equal(t, 1, len(group.Members))
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
	assert.Contains(t, err.Error(), "cn")
}
