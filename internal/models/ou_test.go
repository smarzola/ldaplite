package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOrganizationalUnit(t *testing.T) {
	baseDN := "dc=example,dc=com"
	ou := NewOrganizationalUnit(baseDN, "users", "User Organization Unit")

	assert.NotNil(t, ou)
	assert.Equal(t, "ou=users,dc=example,dc=com", ou.DN)
	assert.Equal(t, "users", ou.OU)
	assert.True(t, ou.IsOrganizationalUnit())
	assert.Equal(t, "User Organization Unit", ou.Entry.GetAttribute("description"))
}

func TestValidateOU(t *testing.T) {
	ou := NewOrganizationalUnit("dc=example,dc=com", "users", "")

	err := ou.ValidateOU()
	assert.NoError(t, err)
}

func TestValidateOUMissingAttribute(t *testing.T) {
	ou := NewOrganizationalUnit("dc=example,dc=com", "users", "")
	ou.Entry.RemoveAttribute("ou")

	err := ou.ValidateOU()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ou")
}

func TestOUHierarchy(t *testing.T) {
	baseDN := "dc=example,dc=com"

	ou1 := NewOrganizationalUnit(baseDN, "departments", "Departments")
	ou2 := NewOrganizationalUnit(baseDN, "teams", "Teams")

	assert.Equal(t, "ou=departments,dc=example,dc=com", ou1.DN)
	assert.Equal(t, "ou=teams,dc=example,dc=com", ou2.DN)
	assert.NotEqual(t, ou1.DN, ou2.DN)
}
