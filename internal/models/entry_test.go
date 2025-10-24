package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEntry(t *testing.T) {
	dn := "cn=admin,dc=example,dc=com"
	objectClass := "inetOrgPerson"

	entry := NewEntry(dn, objectClass)

	assert.NotNil(t, entry)
	assert.Equal(t, dn, entry.DN)
	assert.Equal(t, objectClass, entry.ObjectClass)
	assert.Equal(t, "dc=example,dc=com", entry.ParentDN)
	assert.NotNil(t, entry.Attributes)
	assert.NotNil(t, entry.CreatedAt)
	assert.NotNil(t, entry.UpdatedAt)
}

func TestSetAttribute(t *testing.T) {
	entry := NewEntry("cn=test,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("cn", "Test User")

	value := entry.GetAttribute("cn")
	assert.Equal(t, "Test User", value)
}

func TestAddAttribute(t *testing.T) {
	entry := NewEntry("cn=test,dc=example,dc=com", "inetOrgPerson")
	entry.AddAttribute("mail", "test@example.com")
	entry.AddAttribute("mail", "alt@example.com")

	values := entry.GetAttributes("mail")
	assert.Equal(t, 2, len(values))
	assert.Contains(t, values, "test@example.com")
	assert.Contains(t, values, "alt@example.com")
}

func TestGetAttribute(t *testing.T) {
	entry := NewEntry("cn=test,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("cn", "Test User")

	value := entry.GetAttribute("cn")
	assert.Equal(t, "Test User", value)

	// Non-existent attribute
	value = entry.GetAttribute("nonexistent")
	assert.Equal(t, "", value)
}

func TestGetAttributes(t *testing.T) {
	entry := NewEntry("cn=test,dc=example,dc=com", "inetOrgPerson")
	entry.AddAttribute("cn", "Test User")
	entry.AddAttribute("cn", "Test")

	values := entry.GetAttributes("cn")
	assert.Equal(t, 2, len(values))

	// Non-existent attribute
	values = entry.GetAttributes("nonexistent")
	assert.Empty(t, values)
}

func TestHasAttribute(t *testing.T) {
	entry := NewEntry("cn=test,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("cn", "Test User")

	assert.True(t, entry.HasAttribute("cn"))
	assert.False(t, entry.HasAttribute("nonexistent"))
}

func TestRemoveAttribute(t *testing.T) {
	entry := NewEntry("cn=test,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("cn", "Test User")

	assert.True(t, entry.HasAttribute("cn"))
	entry.RemoveAttribute("cn")
	assert.False(t, entry.HasAttribute("cn"))
}

func TestRemoveAttributeValue(t *testing.T) {
	entry := NewEntry("cn=test,dc=example,dc=com", "inetOrgPerson")
	entry.AddAttribute("mail", "test@example.com")
	entry.AddAttribute("mail", "alt@example.com")

	err := entry.RemoveAttributeValue("mail", "test@example.com")
	assert.NoError(t, err)

	values := entry.GetAttributes("mail")
	assert.Equal(t, 1, len(values))
	assert.Equal(t, "alt@example.com", values[0])
}

func TestRemoveAttributeValueNotFound(t *testing.T) {
	entry := NewEntry("cn=test,dc=example,dc=com", "inetOrgPerson")
	entry.AddAttribute("mail", "test@example.com")

	err := entry.RemoveAttributeValue("mail", "nonexistent@example.com")
	assert.Error(t, err)
}

func TestIsUser(t *testing.T) {
	userEntry := NewEntry("uid=john,ou=users,dc=example,dc=com", string(ObjectClassInetOrgPerson))
	assert.True(t, userEntry.IsUser())

	ouEntry := NewEntry("ou=users,dc=example,dc=com", string(ObjectClassOrganizationalUnit))
	assert.False(t, ouEntry.IsUser())
}

func TestIsGroup(t *testing.T) {
	groupEntry := NewEntry("cn=developers,ou=groups,dc=example,dc=com", string(ObjectClassGroupOfNames))
	assert.True(t, groupEntry.IsGroup())

	userEntry := NewEntry("uid=john,ou=users,dc=example,dc=com", string(ObjectClassInetOrgPerson))
	assert.False(t, userEntry.IsGroup())
}

func TestIsOrganizationalUnit(t *testing.T) {
	ouEntry := NewEntry("ou=users,dc=example,dc=com", string(ObjectClassOrganizationalUnit))
	assert.True(t, ouEntry.IsOrganizationalUnit())

	userEntry := NewEntry("uid=john,ou=users,dc=example,dc=com", string(ObjectClassInetOrgPerson))
	assert.False(t, userEntry.IsOrganizationalUnit())
}

func TestGetRDN(t *testing.T) {
	tests := []struct {
		name     string
		dn       string
		expected string
	}{
		{
			name:     "user dn",
			dn:       "uid=john,ou=users,dc=example,dc=com",
			expected: "uid=john",
		},
		{
			name:     "group dn",
			dn:       "cn=developers,ou=groups,dc=example,dc=com",
			expected: "cn=developers",
		},
		{
			name:     "ou dn",
			dn:       "ou=users,dc=example,dc=com",
			expected: "ou=users",
		},
		{
			name:     "single component",
			dn:       "dc=com",
			expected: "dc=com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := NewEntry(tt.dn, "top")
			assert.Equal(t, tt.expected, entry.GetRDN())
		})
	}
}

func TestValidate(t *testing.T) {
	// Valid entry
	entry := NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	err := entry.Validate()
	assert.NoError(t, err)

	// Invalid entry - missing DN
	invalidEntry := &Entry{
		DN:          "",
		ObjectClass: "inetOrgPerson",
		Attributes:  make(map[string][]string),
	}
	err = invalidEntry.Validate()
	assert.Error(t, err)

	// Invalid entry - missing object class
	invalidEntry2 := &Entry{
		DN:          "uid=john,ou=users,dc=example,dc=com",
		ObjectClass: "",
		Attributes:  make(map[string][]string),
	}
	err = invalidEntry2.Validate()
	assert.Error(t, err)
}

func TestAttributeCaseSensitivity(t *testing.T) {
	entry := NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("CN", "John Doe")

	// Attributes should be case-insensitive
	value1 := entry.GetAttribute("cn")
	value2 := entry.GetAttribute("CN")
	value3 := entry.GetAttribute("Cn")

	assert.Equal(t, value1, value2)
	assert.Equal(t, value2, value3)
	assert.Equal(t, "John Doe", value1)
}

func TestToLDIF(t *testing.T) {
	entry := NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "john")
	entry.SetAttribute("cn", "John Doe")

	ldif := entry.ToLDIF()

	assert.Contains(t, ldif, "dn: uid=john,ou=users,dc=example,dc=com")
	assert.Contains(t, ldif, "objectClass: inetOrgPerson")
	assert.Contains(t, ldif, "uid: john")
	assert.Contains(t, ldif, "cn: John Doe")
}
