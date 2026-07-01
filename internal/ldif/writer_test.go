package ldif

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatWritesRecordsInOrder(t *testing.T) {
	records := []Record{
		{
			DN: "dc=example,dc=com",
			Attributes: []Attribute{
				{Name: "objectClass", Value: "top"},
				{Name: "dc", Value: "example"},
			},
		},
		{
			DN: "ou=users,dc=example,dc=com",
			Attributes: []Attribute{
				{Name: "objectClass", Value: "organizationalUnit"},
				{Name: "ou", Value: "users"},
			},
		},
	}

	got := Format(records)

	assert.Equal(t, strings.Join([]string{
		"dn: dc=example,dc=com",
		"objectClass: top",
		"dc: example",
		"",
		"dn: ou=users,dc=example,dc=com",
		"objectClass: organizationalUnit",
		"ou: users",
		"",
	}, "\n"), got)
}

func TestFormatUsesBase64WhenRequired(t *testing.T) {
	records := []Record{{
		DN: "uid=encoded,ou=users,dc=example,dc=com",
		Attributes: []Attribute{
			{Name: "objectClass", Value: "inetOrgPerson"},
			{Name: "cn", Value: " Leading Space"},
			{Name: "sn", Value: "User"},
			{Name: "description", Value: "contains\nnewline"},
		},
	}}

	got := Format(records)

	assert.Contains(t, got, "cn:: IExlYWRpbmcgU3BhY2U=")
	assert.Contains(t, got, "description:: Y29udGFpbnMKbmV3bGluZQ==")

	parsed, err := Parse(got)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	assert.Equal(t, " Leading Space", parsed[0].FirstValue("cn"))
	assert.Equal(t, "contains\nnewline", parsed[0].FirstValue("description"))
}

func TestFormatFoldsLongLines(t *testing.T) {
	longValue := strings.Repeat("a", 120)
	records := []Record{{
		DN: "uid=long,ou=users,dc=example,dc=com",
		Attributes: []Attribute{
			{Name: "description", Value: longValue},
		},
	}}

	got := Format(records)

	assert.Contains(t, got, "\n ")
	for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		assert.LessOrEqual(t, len(line), maxLineLength)
	}

	parsed, err := Parse(got)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	assert.Equal(t, longValue, parsed[0].FirstValue("description"))
}

func TestWriteMatchesFormat(t *testing.T) {
	records := []Record{{
		DN: "uid=write,ou=users,dc=example,dc=com",
		Attributes: []Attribute{
			{Name: "objectClass", Value: "inetOrgPerson"},
		},
	}}
	var buf bytes.Buffer

	err := Write(&buf, records)

	require.NoError(t, err)
	assert.Equal(t, Format(records), buf.String())
}

func TestFormatRoundTripFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/valid-bootstrap.ldif")
	require.NoError(t, err)
	records, err := Parse(string(data))
	require.NoError(t, err)

	roundTripped, err := Parse(Format(records))

	require.NoError(t, err)
	require.Len(t, roundTripped, len(records))
	for i := range records {
		assert.Equal(t, records[i].DN, roundTripped[i].DN)
		assert.Equal(t, attributesWithoutLine(records[i].Attributes), attributesWithoutLine(roundTripped[i].Attributes))
	}
}

func attributesWithoutLine(attrs []Attribute) []Attribute {
	result := make([]Attribute, 0, len(attrs))
	for _, attr := range attrs {
		attr.Line = 0
		result = append(result, attr)
	}
	return result
}
