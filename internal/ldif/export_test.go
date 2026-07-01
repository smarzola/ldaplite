package ldif

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildExportRecordsOmitsSensitiveAttributesByDefault(t *testing.T) {
	ctx := context.Background()
	st := setupLDIFPlanStore(t)
	defer st.Close()
	seedExportEntry(t, st)

	records, err := BuildExportRecords(ctx, st, ExportOptions{BaseDN: "dc=example,dc=com"})

	require.NoError(t, err)
	require.NotEmpty(t, records)
	assert.Equal(t, "dc=example,dc=com", records[0].DN)
	output := Format(records)
	assert.Contains(t, output, "dn: uid=exported,ou=users,dc=example,dc=com")
	assert.Contains(t, output, "dn: cn=exported,ou=groups,dc=example,dc=com")
	assert.NotContains(t, output, "userPassword:")
	assert.NotContains(t, output, "{ARGON2ID}")
	assert.NotContains(t, output, "memberOf:")
	assert.NotContains(t, output, "entryuuid:")
	assert.NotContains(t, output, "entryUUID:")
}

func TestBuildExportRecordsIncludesRequestedOperationalAndPlaceholders(t *testing.T) {
	ctx := context.Background()
	st := setupLDIFPlanStore(t)
	defer st.Close()
	seedExportEntry(t, st)

	records, err := BuildExportRecords(ctx, st, ExportOptions{
		BaseDN:                      "dc=example,dc=com",
		IncludeOperational:          true,
		IncludePasswordPlaceholders: true,
	})

	require.NoError(t, err)
	output := Format(records)
	assert.Contains(t, output, "entryuuid:")
	assert.Contains(t, output, "createTimestamp:")
	assert.Contains(t, output, "modifyTimestamp:")
	assert.Contains(t, output, "userPassword: {REDACTED}")
	assert.NotContains(t, output, "{ARGON2ID}")
	assert.NotContains(t, output, "memberOf:")
}

func TestBuildExportRecordsOrdersParentBeforeChild(t *testing.T) {
	ctx := context.Background()
	st := setupLDIFPlanStore(t)
	defer st.Close()
	seedExportEntry(t, st)

	records, err := BuildExportRecords(ctx, st, ExportOptions{BaseDN: "dc=example,dc=com"})

	require.NoError(t, err)
	lastDepth := 0
	for _, record := range records {
		depth := strings.Count(record.DN, ",") + 1
		assert.GreaterOrEqual(t, depth, lastDepth)
		lastDepth = depth
	}
}

func seedExportEntry(t *testing.T, st EntryStore) {
	t.Helper()
	records, err := Parse(`dn: uid=exported,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: exported
cn: Exported User
sn: User
userPassword: ChangeMe123!

dn: cn=exported,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: exported
member: uid=exported,ou=users,dc=example,dc=com`)
	require.NoError(t, err)
	plan, err := PlanImport(context.Background(), st, records, ImportPlanOptions{
		BaseDN: "dc=example,dc=com",
		Hasher: testHasher(),
	})
	require.NoError(t, err)
	require.NoError(t, ApplyImport(context.Background(), st, plan))
}

type EntryStore interface {
	EntryLookup
	EntryWriter
	EntrySearcher
}
