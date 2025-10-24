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
	assert.Equal(t, "User Organization Unit", ou.GetDescription())
}

func TestOUSetDescription(t *testing.T) {
	ou := NewOrganizationalUnit("dc=example,dc=com", "users", "Initial Description")

	ou.SetDescription("Updated Description")

	assert.Equal(t, "Updated Description", ou.GetDescription())
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
}

func TestExtractOUNameFromDN(t *testing.T) {
	tests := []struct {
		name    string
		dn      string
		ouName  string
		wantErr bool
	}{
		{
			name:    "valid ou dn",
			dn:      "ou=users,dc=example,dc=com",
			ouName:  "users",
			wantErr: false,
		},
		{
			name:    "groups ou",
			dn:      "ou=groups,dc=example,dc=com",
			ouName:  "groups",
			wantErr: false,
		},
		{
			name:    "invalid dn",
			dn:      "uid=john,dc=example,dc=com",
			ouName:  "",
			wantErr: true,
		},
		{
			name:    "empty dn",
			dn:      "",
			ouName:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ouName, err := ExtractOUNameFromDN(tt.dn)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.ouName, ouName)
			}
		})
	}
}

func TestOUHierarchy(t *testing.T) {
	baseDN := "dc=example,dc=com"

	ou1 := NewOrganizationalUnit(baseDN, "departments", "Departments")
	ou2 := NewOrganizationalUnit(baseDN, "teams", "Teams")

	assert.Equal(t, "ou=departments,dc=example,dc=com", ou1.DN)
	assert.Equal(t, "ou=teams,dc=example,dc=com", ou2.DN)
	assert.NotEqual(t, ou1.DN, ou2.DN)
}
