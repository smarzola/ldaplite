package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportLDIFDryRunValidatesWithoutWriting(t *testing.T) {
	dbPath := setupImportCommandEnv(t)
	ldifPath := writeImportFixture(t, validCommandImportLDIF())
	var out bytes.Buffer
	cmd := newImportCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ldif", "--file", ldifPath, "--dry-run"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, out.String(), "LDIF dry-run successful: records=2 planned=2")
	st := openTestStore(t, dbPath)
	defer st.Close()
	exists, err := st.EntryExists(context.Background(), "uid=imported,ou=users,dc=example,dc=com")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestImportLDIFWritesEntriesAndKeepsPasswordOutOfAttributes(t *testing.T) {
	dbPath := setupImportCommandEnv(t)
	ldifPath := writeImportFixture(t, validCommandImportLDIF())
	var out bytes.Buffer
	cmd := newImportCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ldif", "--file", ldifPath})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, out.String(), "LDIF import successful: records=2 imported=2")

	st := openTestStore(t, dbPath)
	defer st.Close()
	ctx := context.Background()
	user, err := st.GetEntryWithOptions(ctx, "uid=imported,ou=users,dc=example,dc=com", store.EntryOptions{IncludeMemberOf: false})
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "Imported User", user.GetAttribute("cn"))
	assert.False(t, user.HasAttribute("userPassword"))

	group, err := st.GetEntry(ctx, "cn=imported,ou=groups,dc=example,dc=com")
	require.NoError(t, err)
	require.NotNil(t, group)
	assert.Equal(t, []string{"uid=imported,ou=users,dc=example,dc=com"}, group.GetAttributes("member"))

	hash, dn, err := st.GetUserPasswordHashByDN(ctx, "uid=imported,ou=users,dc=example,dc=com")
	require.NoError(t, err)
	assert.Equal(t, "uid=imported,ou=users,dc=example,dc=com", dn)
	assert.True(t, strings.HasPrefix(hash, crypto.SchemeArgon2ID))
}

func TestImportLDIFRejectsInvalidInputWithDN(t *testing.T) {
	setupImportCommandEnv(t)
	ldifPath := writeImportFixture(t, `dn: cn=bad,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: bad
member: uid=missing,ou=users,dc=example,dc=com`)
	cmd := newImportCommand()
	cmd.SetArgs([]string{"ldif", "--file", ldifPath})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cn=bad,ou=groups,dc=example,dc=com")
	assert.Contains(t, err.Error(), "group member does not exist")
}

func TestImportLDIFRejectsExistingEntryWithoutReplace(t *testing.T) {
	setupImportCommandEnv(t)
	ldifPath := writeImportFixture(t, validCommandImportLDIF())
	cmd := newImportCommand()
	cmd.SetArgs([]string{"ldif", "--file", ldifPath})
	require.NoError(t, cmd.Execute())

	cmd = newImportCommand()
	cmd.SetArgs([]string{"ldif", "--file", ldifPath})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "uid=imported,ou=users,dc=example,dc=com")
	assert.Contains(t, err.Error(), "entry already exists")
}

func TestImportLDIFReplaceExistingUpdatesEntry(t *testing.T) {
	dbPath := setupImportCommandEnv(t)
	ldifPath := writeImportFixture(t, validCommandImportLDIF())
	cmd := newImportCommand()
	cmd.SetArgs([]string{"ldif", "--file", ldifPath})
	require.NoError(t, cmd.Execute())

	replacePath := writeImportFixture(t, `dn: uid=imported,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: imported
cn: Imported User Replaced
sn: Replaced
userPassword: Changed123!

dn: cn=imported,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: imported
description: Replaced group
member: uid=imported,ou=users,dc=example,dc=com`)
	var out bytes.Buffer
	cmd = newImportCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ldif", "--file", replacePath, "--replace-existing"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, out.String(), "LDIF import successful: records=2 imported=2")
	st := openTestStore(t, dbPath)
	defer st.Close()
	user, err := st.GetEntryWithOptions(context.Background(), "uid=imported,ou=users,dc=example,dc=com", store.EntryOptions{})
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "Imported User Replaced", user.GetAttribute("cn"))
	group, err := st.GetEntry(context.Background(), "cn=imported,ou=groups,dc=example,dc=com")
	require.NoError(t, err)
	require.NotNil(t, group)
	assert.Equal(t, "Replaced group", group.GetAttribute("description"))
}

func TestImportLDIFAllowGeneratedPasswords(t *testing.T) {
	dbPath := setupImportCommandEnv(t)
	ldifPath := writeImportFixture(t, `dn: uid=generated,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: generated
cn: Generated User
sn: User`)
	var out bytes.Buffer
	cmd := newImportCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ldif", "--file", ldifPath, "--allow-generated-passwords"})

	err := cmd.Execute()

	require.NoError(t, err)
	output := out.String()
	assert.Contains(t, output, "Generated passwords:")
	assert.Contains(t, output, "uid=generated,ou=users,dc=example,dc=com:")
	st := openTestStore(t, dbPath)
	defer st.Close()
	hash, _, err := st.GetUserPasswordHashByDN(context.Background(), "uid=generated,ou=users,dc=example,dc=com")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hash, crypto.SchemeArgon2ID))
	assert.False(t, strings.Contains(output, hash))
}

func TestImportLDIFRequiresFile(t *testing.T) {
	setupImportCommandEnv(t)
	cmd := newImportCommand()
	cmd.SetArgs([]string{"ldif"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--file is required")
}

func setupImportCommandEnv(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ldaplite.db")
	t.Setenv("LDAP_BASE_DN", "dc=example,dc=com")
	t.Setenv("LDAP_ADMIN_PASSWORD", "TestPassword123!")
	t.Setenv("LDAP_DATABASE_PATH", dbPath)
	t.Setenv("LDAP_ARGON2_MEMORY", "64")
	t.Setenv("LDAP_ARGON2_ITERATIONS", "1")
	t.Setenv("LDAP_ARGON2_PARALLELISM", "1")
	t.Setenv("LDAP_ARGON2_SALT_LENGTH", "8")
	t.Setenv("LDAP_ARGON2_KEY_LENGTH", "16")
	return dbPath
}

func writeImportFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "import.ldif")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func validCommandImportLDIF() string {
	return `dn: uid=imported,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: imported
cn: Imported User
sn: User
userPassword: ChangeMe123!

dn: cn=imported,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: imported
member: uid=imported,ou=users,dc=example,dc=com`
}

func openTestStore(t *testing.T, dbPath string) *store.SQLiteStore {
	t.Helper()
	cfg, err := config.LoadFromEnv()
	require.NoError(t, err)
	cfg.Database.Path = dbPath
	st := store.NewSQLiteStore(cfg)
	require.NoError(t, st.Initialize(context.Background()))
	return st
}
