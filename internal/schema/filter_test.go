package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/smarzola/ldaplite/internal/models"
)

func TestParseSimpleEquality(t *testing.T) {
	filter, err := ParseFilter("(uid=john)")
	assert.NoError(t, err)
	assert.NotNil(t, filter)
	assert.Equal(t, FilterTypeEquality, filter.Type)
	assert.Equal(t, "uid", filter.Attribute)
	assert.Equal(t, "john", filter.Value)
}

func TestParsePresent(t *testing.T) {
	filter, err := ParseFilter("(objectClass=*)")
	assert.NoError(t, err)
	assert.NotNil(t, filter)
	assert.Equal(t, FilterTypePresent, filter.Type)
	assert.Equal(t, "objectClass", filter.Attribute)
}

func TestParseAnd(t *testing.T) {
	filter, err := ParseFilter("(&(uid=john)(cn=John Doe))")
	assert.NoError(t, err)
	assert.NotNil(t, filter)
	assert.Equal(t, FilterTypeAnd, filter.Type)
	assert.Equal(t, 2, len(filter.Filters))
	assert.Equal(t, "uid", filter.Filters[0].Attribute)
	assert.Equal(t, "cn", filter.Filters[1].Attribute)
}

func TestParseOr(t *testing.T) {
	filter, err := ParseFilter("(|(uid=john)(uid=jane))")
	assert.NoError(t, err)
	assert.NotNil(t, filter)
	assert.Equal(t, FilterTypeOr, filter.Type)
	assert.Equal(t, 2, len(filter.Filters))
}

func TestParseNot(t *testing.T) {
	filter, err := ParseFilter("(!(uid=john))")
	assert.NoError(t, err)
	assert.NotNil(t, filter)
	assert.Equal(t, FilterTypeNot, filter.Type)
	assert.Equal(t, 1, len(filter.Filters))
	assert.Equal(t, "uid", filter.Filters[0].Attribute)
}

func TestParseEmptyFilter(t *testing.T) {
	filter, err := ParseFilter("")
	assert.NoError(t, err)
	assert.NotNil(t, filter)
	assert.Equal(t, FilterTypePresent, filter.Type)
	assert.Equal(t, "objectClass", filter.Attribute)
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		name   string
		filter string
	}{
		{"missing closing paren", "(uid=john"},
		{"missing opening paren", "uid=john)"},
		{"invalid format", "(invalid)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFilter(tt.filter)
			assert.Error(t, err)
		})
	}
}

func TestMatchesEquality(t *testing.T) {
	filter, _ := ParseFilter("(uid=john)")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "john")

	assert.True(t, filter.Matches(entry))
}

func TestMatchesEqualityNoMatch(t *testing.T) {
	filter, _ := ParseFilter("(uid=jane)")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "john")

	assert.False(t, filter.Matches(entry))
}

func TestMatchesPresent(t *testing.T) {
	filter, _ := ParseFilter("(mail=*)")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("mail", "john@example.com")

	assert.True(t, filter.Matches(entry))
}

func TestMatchesPresentNoAttribute(t *testing.T) {
	filter, _ := ParseFilter("(mail=*)")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")

	assert.False(t, filter.Matches(entry))
}

func TestMatchesAnd(t *testing.T) {
	filter, _ := ParseFilter("(&(uid=john)(cn=John Doe))")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "john")
	entry.SetAttribute("cn", "John Doe")

	assert.True(t, filter.Matches(entry))
}

func TestMatchesAndFail(t *testing.T) {
	filter, _ := ParseFilter("(&(uid=john)(cn=Jane Doe))")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "john")
	entry.SetAttribute("cn", "John Doe")

	assert.False(t, filter.Matches(entry))
}

func TestMatchesOr(t *testing.T) {
	filter, _ := ParseFilter("(|(uid=john)(uid=jane))")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "john")

	assert.True(t, filter.Matches(entry))
}

func TestMatchesOrFail(t *testing.T) {
	filter, _ := ParseFilter("(|(uid=jane)(uid=bob))")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "john")

	assert.False(t, filter.Matches(entry))
}

func TestMatchesNot(t *testing.T) {
	filter, _ := ParseFilter("(!(uid=jane))")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "john")

	assert.True(t, filter.Matches(entry))
}

func TestMatchesNotFail(t *testing.T) {
	filter, _ := ParseFilter("(!(uid=john))")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "john")

	assert.False(t, filter.Matches(entry))
}

func TestComplexFilter(t *testing.T) {
	// (&(objectClass=inetOrgPerson)(|(uid=john)(uid=jane)))
	filter, err := ParseFilter("(&(objectClass=inetOrgPerson)(|(uid=john)(uid=jane)))")
	assert.NoError(t, err)
	assert.Equal(t, FilterTypeAnd, filter.Type)

	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("objectClass", "inetOrgPerson")
	entry.SetAttribute("uid", "john")

	assert.True(t, filter.Matches(entry))
}

func TestMultiValuedAttribute(t *testing.T) {
	filter, _ := ParseFilter("(mail=alternate@example.com)")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.AddAttribute("mail", "john@example.com")
	entry.AddAttribute("mail", "alternate@example.com")

	assert.True(t, filter.Matches(entry))
}

func TestFilterString(t *testing.T) {
	tests := []struct {
		name     string
		filter   string
		expected string
	}{
		{
			name:     "equality",
			filter:   "(uid=john)",
			expected: "(uid=john)",
		},
		{
			name:     "present",
			filter:   "(objectClass=*)",
			expected: "(objectClass=*)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := ParseFilter(tt.filter)
			result := f.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseMultipleOr(t *testing.T) {
	filter, err := ParseFilter("(|(uid=john)(uid=jane)(uid=bob))")
	assert.NoError(t, err)
	assert.Equal(t, FilterTypeOr, filter.Type)
	assert.Equal(t, 3, len(filter.Filters))
}

func TestParseMultipleAnd(t *testing.T) {
	filter, err := ParseFilter("(&(uid=john)(cn=John)(mail=john@example.com))")
	assert.NoError(t, err)
	assert.Equal(t, FilterTypeAnd, filter.Type)
	assert.Equal(t, 3, len(filter.Filters))
}

func TestCaseSensitiveAttribute(t *testing.T) {
	filter, _ := ParseFilter("(UID=john)")
	entry := models.NewEntry("uid=john,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "john")

	// Attributes are case-insensitive in LDAP
	assert.True(t, filter.Matches(entry))
}
