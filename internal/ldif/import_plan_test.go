package ldif

import (
	"context"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanImportValidatesAndOrdersBatch(t *testing.T) {
	records, err := Parse(readFixture(t, "valid-bootstrap.ldif"))
	require.NoError(t, err)

	plan, err := PlanImport(context.Background(), fakeLookup{}, records, ImportPlanOptions{
		BaseDN: "dc=example,dc=com",
		Hasher: testHasher(),
	})

	require.NoError(t, err)
	require.Len(t, plan.Entries, 7)
	assert.Equal(t, "dc=example,dc=com", plan.Entries[0].DN)
	assert.Equal(t, string(models.ObjectClassTop), plan.Entries[0].ObjectClass)
	assert.Equal(t, "ou=users,dc=example,dc=com", plan.Entries[1].DN)
	assert.Equal(t, "ou=groups,dc=example,dc=com", plan.Entries[2].DN)

	jane := findPlannedEntry(t, plan, "uid=jane,ou=users,dc=example,dc=com")
	assert.Equal(t, string(models.ObjectClassInetOrgPerson), jane.ObjectClass)
	assert.Equal(t, []string{"jane@example.com", "jane.alt@example.com"}, jane.GetAttributes("mail"))
	assert.True(t, strings.HasPrefix(jane.GetAttribute("userPassword"), crypto.SchemeArgon2ID))

	engineering := findPlannedEntry(t, plan, "cn=engineering,ou=groups,dc=example,dc=com")
	assert.Equal(t, []string{
		"uid=jane,ou=users,dc=example,dc=com",
		"uid=appbind,ou=users,dc=example,dc=com",
	}, engineering.GetAttributes("member"))
}

func TestPlanImportRejectsInvalidFixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		existing []string
		want     string
	}{
		{
			name:    "outside base",
			fixture: "outside-base.ldif",
			want:    "outside base DN",
		},
		{
			name:    "missing parent",
			fixture: "missing-parent.ldif",
			want:    "parent DN does not exist",
		},
		{
			name:     "missing group member",
			fixture:  "missing-group-member.ldif",
			existing: []string{"ou=groups,dc=example,dc=com"},
			want:     "group member does not exist",
		},
		{
			name:     "protected attributes",
			fixture:  "protected-attributes.ldif",
			existing: []string{"ou=users,dc=example,dc=com"},
			want:     "protected attribute entryUUID is not importable",
		},
		{
			name:     "unsupported password scheme",
			fixture:  "unsupported-password-scheme.ldif",
			existing: []string{"ou=users,dc=example,dc=com"},
			want:     "unsupported password scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records, err := Parse(readFixture(t, tt.fixture))
			require.NoError(t, err)

			_, err = PlanImport(context.Background(), fakeLookupWith(tt.existing...), records, ImportPlanOptions{
				BaseDN: "dc=example,dc=com",
				Hasher: testHasher(),
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestPlanImportRejectsMissingPasswordAndUnsupportedObjectClass(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "missing password",
			input: `dn: uid=missing-password,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: missing-password
cn: Missing Password
sn: Password`,
			want: "userPassword is required",
		},
		{
			name: "unsupported object class",
			input: `dn: uid=posix,ou=users,dc=example,dc=com
objectClass: posixAccount
uid: posix
cn: Posix User
sn: User
userPassword: ChangeMe123!`,
			want: "unsupported objectClass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records, err := Parse(tt.input)
			require.NoError(t, err)

			_, err = PlanImport(context.Background(), fakeLookupWith("ou=users,dc=example,dc=com"), records, ImportPlanOptions{
				BaseDN: "dc=example,dc=com",
				Hasher: testHasher(),
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestPlanImportAllowsTopAlongsideStructuralObjectClass(t *testing.T) {
	records, err := Parse(`dn: uid=top-user,ou=users,dc=example,dc=com
objectClass: top
objectClass: inetOrgPerson
uid: top-user
cn: Top User
sn: User
userPassword: ChangeMe123!`)
	require.NoError(t, err)

	plan, err := PlanImport(context.Background(), fakeLookupWith("ou=users,dc=example,dc=com"), records, ImportPlanOptions{
		BaseDN: "dc=example,dc=com",
		Hasher: testHasher(),
	})

	require.NoError(t, err)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, string(models.ObjectClassInetOrgPerson), plan.Entries[0].ObjectClass)
}

func TestPlanImportDryRunDoesNotMutateSQLiteStore(t *testing.T) {
	ctx := context.Background()
	st := setupLDIFPlanStore(t)
	defer st.Close()

	beforeUserExists, err := st.EntryExists(ctx, "uid=dryrun,ou=users,dc=example,dc=com")
	require.NoError(t, err)
	require.False(t, beforeUserExists)

	records, err := Parse(`dn: uid=dryrun,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: dryrun
cn: Dry Run
sn: Run
userPassword: ChangeMe123!

dn: cn=dryrun,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: dryrun
member: uid=dryrun,ou=users,dc=example,dc=com`)
	require.NoError(t, err)

	plan, err := PlanImport(ctx, st, records, ImportPlanOptions{
		BaseDN: "dc=example,dc=com",
		Hasher: testHasher(),
	})

	require.NoError(t, err)
	require.Len(t, plan.Entries, 2)
	afterUserExists, err := st.EntryExists(ctx, "uid=dryrun,ou=users,dc=example,dc=com")
	require.NoError(t, err)
	assert.False(t, afterUserExists)
}

type fakeLookup map[string]struct{}

func fakeLookupWith(dns ...string) fakeLookup {
	lookup := fakeLookup{}
	for _, dn := range dns {
		lookup[strings.ToLower(dn)] = struct{}{}
	}
	return lookup
}

func (f fakeLookup) EntryExists(_ context.Context, dn string) (bool, error) {
	_, ok := f[strings.ToLower(dn)]
	return ok, nil
}

func findPlannedEntry(t *testing.T, plan *ImportPlan, dn string) *models.Entry {
	t.Helper()
	for _, entry := range plan.Entries {
		if strings.EqualFold(entry.DN, dn) {
			return entry
		}
	}
	t.Fatalf("planned entry %s not found", dn)
	return nil
}

func testHasher() *crypto.PasswordHasher {
	return crypto.NewPasswordHasher(config.Argon2Config{
		Memory:      64,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  8,
		KeyLength:   16,
	})
}

func setupLDIFPlanStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	t.Setenv("LDAP_ADMIN_PASSWORD", "TestPassword123!")
	cfg := &config.Config{
		LDAP: config.LDAPConfig{
			BaseDN: "dc=example,dc=com",
		},
		Database: config.DatabaseConfig{
			Path: t.TempDir() + "/ldaplite.db",
		},
		Security: config.SecurityConfig{
			Argon2Config: config.Argon2Config{
				Memory:      64,
				Iterations:  1,
				Parallelism: 1,
				SaltLength:  8,
				KeyLength:   16,
			},
		},
	}
	st := store.NewSQLiteStore(cfg)
	require.NoError(t, st.Initialize(context.Background()))
	return st
}
