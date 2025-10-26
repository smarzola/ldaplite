package schema

import (
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/stretchr/testify/assert"
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

func TestParseGreaterOrEqual(t *testing.T) {
	filter, err := ParseFilter("(modifyTimestamp>=20130905020304Z)")
	assert.NoError(t, err)
	assert.NotNil(t, filter)
	assert.Equal(t, FilterTypeGreaterOrEqual, filter.Type)
	assert.Equal(t, "modifyTimestamp", filter.Attribute)
	assert.Equal(t, "20130905020304Z", filter.Value)
}

func TestParseLessOrEqual(t *testing.T) {
	filter, err := ParseFilter("(createTimestamp<=20251027000000Z)")
	assert.NoError(t, err)
	assert.NotNil(t, filter)
	assert.Equal(t, FilterTypeLessOrEqual, filter.Type)
	assert.Equal(t, "createTimestamp", filter.Attribute)
	assert.Equal(t, "20251027000000Z", filter.Value)
}

func TestParseApproxMatch(t *testing.T) {
	filter, err := ParseFilter("(cn~=John)")
	assert.NoError(t, err)
	assert.NotNil(t, filter)
	assert.Equal(t, FilterTypeApproxMatch, filter.Type)
	assert.Equal(t, "cn", filter.Attribute)
	assert.Equal(t, "John", filter.Value)
}

func TestMatchesGreaterOrEqual(t *testing.T) {
	tests := []struct {
		name      string
		filter    string
		timestamp string
		expected  bool
	}{
		{
			name:      "modifyTimestamp >= past date (should match)",
			filter:    "(modifyTimestamp>=20130905020304Z)",
			timestamp: "20251026090445Z",
			expected:  true,
		},
		{
			name:      "modifyTimestamp >= future date (should not match)",
			filter:    "(modifyTimestamp>=20301027000000Z)",
			timestamp: "20251026090445Z",
			expected:  false,
		},
		{
			name:      "modifyTimestamp >= exact date (should match)",
			filter:    "(modifyTimestamp>=20251026090445Z)",
			timestamp: "20251026090445Z",
			expected:  true,
		},
		{
			name:      "createTimestamp >= past date (should match)",
			filter:    "(createTimestamp>=20200101000000Z)",
			timestamp: "20251026090445Z",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.filter)
			assert.NoError(t, err)

			entry := models.NewEntry("uid=test,ou=users,dc=example,dc=com", "inetOrgPerson")
			entry.SetAttribute("modifyTimestamp", tt.timestamp)
			entry.SetAttribute("createTimestamp", tt.timestamp)

			assert.Equal(t, tt.expected, filter.Matches(entry))
		})
	}
}

func TestMatchesLessOrEqual(t *testing.T) {
	tests := []struct {
		name      string
		filter    string
		timestamp string
		expected  bool
	}{
		{
			name:      "modifyTimestamp <= future date (should match)",
			filter:    "(modifyTimestamp<=20301027000000Z)",
			timestamp: "20251026090445Z",
			expected:  true,
		},
		{
			name:      "modifyTimestamp <= past date (should not match)",
			filter:    "(modifyTimestamp<=20130905020304Z)",
			timestamp: "20251026090445Z",
			expected:  false,
		},
		{
			name:      "modifyTimestamp <= exact date (should match)",
			filter:    "(modifyTimestamp<=20251026090445Z)",
			timestamp: "20251026090445Z",
			expected:  true,
		},
		{
			name:      "createTimestamp <= future date (should match)",
			filter:    "(createTimestamp<=20301027000000Z)",
			timestamp: "20251026090445Z",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.filter)
			assert.NoError(t, err)

			entry := models.NewEntry("uid=test,ou=users,dc=example,dc=com", "inetOrgPerson")
			entry.SetAttribute("modifyTimestamp", tt.timestamp)
			entry.SetAttribute("createTimestamp", tt.timestamp)

			assert.Equal(t, tt.expected, filter.Matches(entry))
		})
	}
}

func TestFilterStringGreaterOrEqual(t *testing.T) {
	filter, _ := ParseFilter("(modifyTimestamp>=20130905020304Z)")
	assert.Equal(t, "(modifyTimestamp>=20130905020304Z)", filter.String())
}

func TestFilterStringLessOrEqual(t *testing.T) {
	filter, _ := ParseFilter("(createTimestamp<=20251027000000Z)")
	assert.Equal(t, "(createTimestamp<=20251027000000Z)", filter.String())
}

func TestCompareTimestampWithoutZSuffix(t *testing.T) {
	filter, _ := ParseFilter("(modifyTimestamp>=20130905020304)")
	entry := models.NewEntry("uid=test,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("modifyTimestamp", "20251026090445")

	// Should work without Z suffix too
	assert.True(t, filter.Matches(entry))
}

func TestTimestampComparisonCaseInsensitive(t *testing.T) {
	// Test that attribute name is case-insensitive
	filter, _ := ParseFilter("(MODIFYTIMESTAMP>=20130905020304Z)")
	entry := models.NewEntry("uid=test,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("modifytimestamp", "20251026090445Z")

	assert.True(t, filter.Matches(entry))
}
