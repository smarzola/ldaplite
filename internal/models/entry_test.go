package models

import (
	"strings"
	"testing"
	"time"

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

func TestFormatLDAPTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "specific time",
			time:     time.Date(2025, 1, 25, 14, 30, 45, 0, time.UTC),
			expected: "20250125143045Z",
		},
		{
			name:     "different timezone converts to UTC",
			time:     time.Date(2025, 1, 25, 14, 30, 45, 0, time.FixedZone("EST", -5*3600)),
			expected: "20250125193045Z", // 14:30 EST = 19:30 UTC
		},
		{
			name:     "midnight",
			time:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: "20250101000000Z",
		},
		{
			name:     "end of year",
			time:     time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: "20241231235959Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatLDAPTimestamp(tt.time)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddOperationalAttributes(t *testing.T) {
	entry := NewEntry("uid=test,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "test")
	entry.SetAttribute("cn", "Test User")

	// Add operational attributes
	entry.AddOperationalAttributes()

	// Check objectClass was added
	assert.True(t, entry.HasAttribute("objectclass"), "objectclass attribute should be present")
	assert.Equal(t, "inetOrgPerson", entry.GetAttribute("objectclass"))

	// Check createTimestamp was added
	assert.True(t, entry.HasAttribute("createtimestamp"), "createtimestamp attribute should be present")

	// Check modifyTimestamp was added
	assert.True(t, entry.HasAttribute("modifytimestamp"), "modifytimestamp attribute should be present")

	// Verify format (should be 14 chars + Z = 15 total)
	createTS := entry.GetAttribute("createtimestamp")
	assert.Len(t, createTS, 15, "createtimestamp should be 15 characters")
	assert.True(t, strings.HasSuffix(createTS, "Z"), "createtimestamp should end with 'Z'")

	modifyTS := entry.GetAttribute("modifytimestamp")
	assert.Len(t, modifyTS, 15, "modifytimestamp should be 15 characters")
	assert.True(t, strings.HasSuffix(modifyTS, "Z"), "modifytimestamp should end with 'Z'")

	// Verify format with regex (YYYYMMDDHHMMSSz)
	timestampPattern := `^\d{14}Z$`
	assert.Regexp(t, timestampPattern, createTS, "createtimestamp should match format YYYYMMDDHHMMSSz")
	assert.Regexp(t, timestampPattern, modifyTS, "modifytimestamp should match format YYYYMMDDHHMMSSz")
}

func TestAddOperationalAttributesUpdatesModifyTimestamp(t *testing.T) {
	entry := NewEntry("uid=test,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "test")

	// Add operational attributes first time
	entry.AddOperationalAttributes()
	createTS1 := entry.GetAttribute("createtimestamp")
	modifyTS1 := entry.GetAttribute("modifytimestamp")

	// Initially, createTimestamp and modifyTimestamp should be the same
	assert.Equal(t, createTS1, modifyTS1, "createTimestamp and modifyTimestamp should match initially")

	// Wait a bit and modify the entry
	time.Sleep(10 * time.Millisecond)
	entry.SetAttribute("cn", "Modified User")

	// Add operational attributes again
	entry.AddOperationalAttributes()
	createTS2 := entry.GetAttribute("createtimestamp")

	// createTimestamp should remain the same
	assert.Equal(t, createTS1, createTS2, "createTimestamp should not change")

	// modifyTimestamp should be updated - verify via UpdatedAt field
	assert.False(t, entry.CreatedAt.Equal(entry.UpdatedAt), "UpdatedAt should be different from CreatedAt after modification")
}

func TestOperationalAttributesCaseInsensitive(t *testing.T) {
	entry := NewEntry("uid=test,dc=example,dc=com", "inetOrgPerson")
	entry.AddOperationalAttributes()

	// Verify attributes are stored in lowercase
	assert.True(t, entry.HasAttribute("objectclass"))
	assert.True(t, entry.HasAttribute("createtimestamp"))
	assert.True(t, entry.HasAttribute("modifytimestamp"))

	// Verify case-insensitive access works
	assert.True(t, entry.HasAttribute("objectClass"))
	assert.True(t, entry.HasAttribute("createTimestamp"))
	assert.True(t, entry.HasAttribute("modifyTimestamp"))
	assert.True(t, entry.HasAttribute("CREATETIMESTAMP"))
}
