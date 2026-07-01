package ldif

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseValidBootstrapFixture(t *testing.T) {
	data := readFixture(t, "valid-bootstrap.ldif")

	records, err := Parse(data)

	require.NoError(t, err)
	require.Len(t, records, 7)
	assert.Equal(t, "dc=example,dc=com", records[0].DN)
	assert.Equal(t, "top", records[0].FirstValue("objectClass"))

	jane := records[3]
	assert.Equal(t, "uid=jane,ou=users,dc=example,dc=com", jane.DN)
	assert.Equal(t, "inetOrgPerson", jane.FirstValue("objectClass"))
	assert.Equal(t, []string{"jane@example.com", "jane.alt@example.com"}, jane.Values("mail"))

	appBind := records[4]
	assert.Equal(t, "Imported application bind user with a folded line continuation for parser coverage", appBind.FirstValue("description"))

	engineering := records[6]
	assert.Equal(t, []string{
		"uid=jane,ou=users,dc=example,dc=com",
		"uid=appbind,ou=users,dc=example,dc=com",
	}, engineering.Values("member"))
}

func TestParseBase64Value(t *testing.T) {
	records, err := Parse(readFixture(t, "base64-value.ldif"))

	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "Base64 User", records[0].FirstValue("cn"))
}

func TestParseRejectsInvalidFixtures(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    string
	}{
		{
			name:    "missing colon",
			fixture: "malformed-missing-colon.ldif",
			want:    "missing ':'",
		},
		{
			name:    "unsupported changetype",
			fixture: "unsupported-changetype.ldif",
			want:    "changetype records are not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(readFixture(t, tt.fixture))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.Contains(t, err.Error(), "ldif line")
		})
	}
}

func TestParseRejectsMissingAndDuplicateDN(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "missing dn",
			input: `objectClass: inetOrgPerson
uid: missing`,
			want: "record is missing dn",
		},
		{
			name: "duplicate dn",
			input: `dn: uid=one,dc=example,dc=com
dn: uid=two,dc=example,dc=com
objectClass: inetOrgPerson`,
			want: "duplicate dn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestParseRejectsURLValueAndOrphanFold(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "url value",
			input: `dn: uid=url,dc=example,dc=com
jpegPhoto:< file:///tmp/photo.jpg`,
			want: "URL values are not supported",
		},
		{
			name:  "orphan folded line",
			input: " orphan",
			want:  "folded line has no previous line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return string(data)
}
