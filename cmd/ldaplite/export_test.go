package main

import (
	"bytes"
	"context"
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

func TestExportLDIFCanSeedFreshDatabaseWithGeneratedPasswords(t *testing.T) {
	sourceDBPath := setupImportCommandEnv(t)
	importFixtureForExportTest(t)

	exportPath := filepath.Join(t.TempDir(), "export.ldif")
	exportCmd := newExportCommand()
	exportCmd.SetArgs([]string{"ldif", "--file", exportPath})
	require.NoError(t, exportCmd.Execute())

	destinationDBPath := filepath.Join(t.TempDir(), "ldaplite-copy.db")
	t.Setenv("LDAP_DATABASE_PATH", destinationDBPath)
	importCmd := newImportCommand()
	importCmd.SetArgs([]string{"ldif", "--file", exportPath, "--replace-existing", "--allow-generated-passwords"})
	require.NoError(t, importCmd.Execute())

	t.Setenv("LDAP_DATABASE_PATH", sourceDBPath)
	sourceStore := openTestStore(t, sourceDBPath)
	defer sourceStore.Close()
	sourceExists, err := sourceStore.EntryExists(context.Background(), "uid=imported,ou=users,dc=example,dc=com")
	require.NoError(t, err)
	require.True(t, sourceExists)

	t.Setenv("LDAP_DATABASE_PATH", destinationDBPath)
	destinationStore := openTestStore(t, destinationDBPath)
	defer destinationStore.Close()
	destinationExists, err := destinationStore.EntryExists(context.Background(), "uid=imported,ou=users,dc=example,dc=com")
	require.NoError(t, err)
	assert.True(t, destinationExists)
}

func importFixtureForExportTest(t *testing.T) {
	t.Helper()
	ldifPath := writeImportFixture(t, validCommandImportLDIF())
	cmd := newImportCommand()
	cmd.SetArgs([]string{"ldif", "--file", ldifPath})
	require.NoError(t, cmd.Execute())
}
