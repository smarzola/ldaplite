package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/ldif"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportLDIFWritesSafeStdout(t *testing.T) {
	setupImportCommandEnv(t)
	importFixtureForExportTest(t)
	var out bytes.Buffer
	cmd := newExportCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ldif", "--file", "-"})

	err := cmd.Execute()

	require.NoError(t, err)
	output := out.String()
	assert.True(t, strings.HasPrefix(output, "dn: dc=example,dc=com\n"))
	assert.Contains(t, output, "dn: uid=imported,ou=users,dc=example,dc=com")
	assert.Contains(t, output, "dn: cn=imported,ou=groups,dc=example,dc=com")
	assert.NotContains(t, output, "userPassword:")
	assert.NotContains(t, output, "{ARGON2ID}")
	assert.NotContains(t, output, "memberOf:")

	records, err := ldif.Parse(output)
	require.NoError(t, err)
	require.NotEmpty(t, records)
}

func TestExportLDIFWritesFileWithOptionalFields(t *testing.T) {
	setupImportCommandEnv(t)
	importFixtureForExportTest(t)
	outputPath := filepath.Join(t.TempDir(), "export.ldif")
	var out bytes.Buffer
	cmd := newExportCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ldif", "--file", outputPath, "--include-operational", "--include-password-placeholders"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, out.String(), "LDIF export successful:")
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	output := string(data)
	assert.Contains(t, output, "entryuuid:")
	assert.Contains(t, output, "createTimestamp:")
	assert.Contains(t, output, "modifyTimestamp:")
	assert.Contains(t, output, "userPassword: {REDACTED}")
	assert.NotContains(t, output, "{ARGON2ID}")
}

func importFixtureForExportTest(t *testing.T) {
	t.Helper()
	ldifPath := writeImportFixture(t, validCommandImportLDIF())
	cmd := newImportCommand()
	cmd.SetArgs([]string{"ldif", "--file", ldifPath})
	require.NoError(t, cmd.Execute())
}
