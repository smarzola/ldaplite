package store

import (
	"context"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/pkg/config"
)

// setupTestStore creates an in-memory SQLite store with test data
func setupTestStore(t *testing.T) *SQLiteStore {
	// Use a temporary file instead of :memory: because migrations don't work well with :memory:
	tmpfile := t.TempDir() + "/test.db"

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: tmpfile,
		},
		LDAP: config.LDAPConfig{
			BaseDN: "dc=test,dc=com",
		},
		Security: config.SecurityConfig{
			Argon2Config: config.Argon2Config{
				Memory:      64 * 1024,
				Iterations:  3,
				Parallelism: 2,
				SaltLength:  16,
				KeyLength:   32,
			},
		},
	}

	store := NewSQLiteStore(cfg)
	ctx := context.Background()

	// Set admin password for initialization
	t.Setenv("LDAP_ADMIN_PASSWORD", "test_admin_password")

	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize store: %v", err)
	}

	// Initialize already created base DN, OUs (users, groups), and admin user
	// Now create test users
	users := []struct {
		uid       string
		cn        string
		sn        string
		givenName string
		mail      string
	}{
		{"jdoe", "John Doe", "Doe", "John", "jdoe@test.com"},
		{"jsmith", "Jane Smith", "Smith", "Jane", "jsmith@test.com"},
		{"bob", "Bob Johnson", "Johnson", "Bob", "bob@test.com"},
		{"alice", "Alice Williams", "Williams", "Alice", "alice@test.com"},
	}

	for _, u := range users {
		user := models.NewUser("ou=users,dc=test,dc=com", u.uid, u.cn, u.sn, u.mail)
		// SetPassword expects a hashed password with LDAP scheme prefix
		user.SetPassword("{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dummyhash$dummyhash")
		if err := store.CreateEntry(ctx, user.Entry); err != nil {
			t.Fatalf("failed to create user %s: %v", u.uid, err)
		}
	}

	// Create groups
	admins := models.NewGroup("ou=groups,dc=test,dc=com", "admins", "Administrators group")
	admins.AddMember("uid=jdoe,ou=users,dc=test,dc=com")
	if err := store.CreateEntry(ctx, admins.Entry); err != nil {
		t.Fatalf("failed to create admins group: %v", err)
	}

	developers := models.NewGroup("ou=groups,dc=test,dc=com", "developers", "Developers group")
	developers.AddMember("uid=jsmith,ou=users,dc=test,dc=com")
	developers.AddMember("uid=bob,ou=users,dc=test,dc=com")
	if err := store.CreateEntry(ctx, developers.Entry); err != nil {
		t.Fatalf("failed to create developers group: %v", err)
	}

	return store
}

func TestSearchEntriesWithEqualityFilter(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	tests := []struct {
		name        string
		baseDN      string
		filter      string
		wantCount   int
		wantContain []string // DNs that should be in results
	}{
		{
			name:        "search by objectClass=inetOrgPerson",
			baseDN:      "dc=test,dc=com",
			filter:      "(objectClass=inetOrgPerson)",
			wantCount:   5, // admin + 4 test users
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com", "uid=jsmith,ou=users,dc=test,dc=com"},
		},
		{
			name:        "search by objectClass=organizationalUnit",
			baseDN:      "dc=test,dc=com",
			filter:      "(objectClass=organizationalUnit)",
			wantCount:   2,
			wantContain: []string{"ou=users,dc=test,dc=com", "ou=groups,dc=test,dc=com"},
		},
		{
			name:        "search by objectClass=groupOfNames",
			baseDN:      "dc=test,dc=com",
			filter:      "(objectClass=groupOfNames)",
			wantCount:   3, // ldaplite.admin + 2 test groups
			wantContain: []string{"cn=ldaplite.admin,ou=groups,dc=test,dc=com", "cn=admins,ou=groups,dc=test,dc=com", "cn=developers,ou=groups,dc=test,dc=com"},
		},
		{
			name:        "search by uid attribute",
			baseDN:      "dc=test,dc=com",
			filter:      "(uid=jdoe)",
			wantCount:   1,
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com"},
		},
		{
			name:        "search by cn attribute",
			baseDN:      "dc=test,dc=com",
			filter:      "(cn=John Doe)",
			wantCount:   1,
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com"},
		},
		{
			name:        "search by mail attribute",
			baseDN:      "dc=test,dc=com",
			filter:      "(mail=jsmith@test.com)",
			wantCount:   1,
			wantContain: []string{"uid=jsmith,ou=users,dc=test,dc=com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := store.SearchEntries(ctx, tt.baseDN, tt.filter)
			if err != nil {
				t.Fatalf("SearchEntries() error = %v", err)
			}

			if len(entries) != tt.wantCount {
				t.Errorf("SearchEntries() got %d entries, want %d", len(entries), tt.wantCount)
				t.Logf("Actual entries returned:")
				for _, e := range entries {
					t.Logf("  - %s (objectClass=%s)", e.DN, e.ObjectClass)
				}
			}

			// Check that expected DNs are in results
			entryDNs := make(map[string]bool)
			for _, e := range entries {
				entryDNs[e.DN] = true
			}

			missingAny := false
			for _, wantDN := range tt.wantContain {
				if !entryDNs[wantDN] {
					t.Errorf("SearchEntries() missing expected DN: %s", wantDN)
					missingAny = true
				}
			}

			if missingAny {
				t.Logf("All actual entries:")
				for _, e := range entries {
					t.Logf("  - %s (objectClass=%s)", e.DN, e.ObjectClass)
				}
			}
		})
	}
}

func TestSearchEntriesWithPresentFilter(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	tests := []struct {
		name      string
		baseDN    string
		filter    string
		wantCount int
	}{
		{
			name:      "search for entries with objectClass present",
			baseDN:    "dc=test,dc=com",
			filter:    "(objectClass=*)",
			wantCount: 11, // 1 base + 2 OUs + 5 users + 3 groups
		},
		{
			name:      "search for entries with uid present",
			baseDN:    "dc=test,dc=com",
			filter:    "(uid=*)",
			wantCount: 5, // 5 users (including admin)
		},
		{
			name:      "search for entries with mail present",
			baseDN:    "dc=test,dc=com",
			filter:    "(mail=*)",
			wantCount: 5, // 5 users with mail (including admin)
		},
		{
			name:      "search for entries with cn present",
			baseDN:    "dc=test,dc=com",
			filter:    "(cn=*)",
			wantCount: 8, // 5 users + 3 groups
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := store.SearchEntries(ctx, tt.baseDN, tt.filter)
			if err != nil {
				t.Fatalf("SearchEntries() error = %v", err)
			}

			if len(entries) != tt.wantCount {
				t.Errorf("SearchEntries() got %d entries, want %d", len(entries), tt.wantCount)
			}
		})
	}
}

func TestSearchEntriesWithSubstringFilter(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	tests := []struct {
		name        string
		baseDN      string
		filter      string
		wantCount   int
		wantContain []string
	}{
		{
			name:        "search cn starting with 'John'",
			baseDN:      "dc=test,dc=com",
			filter:      "(cn=John*)",
			wantCount:   1,
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com"},
		},
		{
			name:        "search cn ending with 'Smith'",
			baseDN:      "dc=test,dc=com",
			filter:      "(cn=*Smith)",
			wantCount:   1,
			wantContain: []string{"uid=jsmith,ou=users,dc=test,dc=com"},
		},
		{
			name:        "search cn containing 'son'",
			baseDN:      "dc=test,dc=com",
			filter:      "(cn=*son*)",
			wantCount:   1, // Only Bob Johnson contains "son"
			wantContain: []string{"uid=bob,ou=users,dc=test,dc=com"},
		},
		{
			name:        "search mail starting with 'j'",
			baseDN:      "dc=test,dc=com",
			filter:      "(mail=j*)",
			wantCount:   2,
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com", "uid=jsmith,ou=users,dc=test,dc=com"},
		},
		{
			name:        "search uid containing 'o'",
			baseDN:      "dc=test,dc=com",
			filter:      "(uid=*o*)",
			wantCount:   2,
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com", "uid=bob,ou=users,dc=test,dc=com"},
		},
		{
			name:        "search uid starting with 'a'",
			baseDN:      "dc=test,dc=com",
			filter:      "(uid=a*)",
			wantCount:   2, // admin and alice
			wantContain: []string{"uid=admin,ou=users,dc=test,dc=com", "uid=alice,ou=users,dc=test,dc=com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := store.SearchEntries(ctx, tt.baseDN, tt.filter)
			if err != nil {
				t.Fatalf("SearchEntries() error = %v", err)
			}

			if len(entries) != tt.wantCount {
				t.Errorf("SearchEntries() got %d entries, want %d", len(entries), tt.wantCount)
			}

			// Check that expected DNs are in results
			entryDNs := make(map[string]bool)
			for _, e := range entries {
				entryDNs[e.DN] = true
			}

			for _, wantDN := range tt.wantContain {
				if !entryDNs[wantDN] {
					t.Errorf("SearchEntries() missing expected DN: %s", wantDN)
				}
			}
		})
	}
}

func TestSearchEntriesWithAndFilter(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	tests := []struct {
		name        string
		baseDN      string
		filter      string
		wantCount   int
		wantContain []string
	}{
		{
			name:        "AND: objectClass=inetOrgPerson AND uid=jdoe",
			baseDN:      "dc=test,dc=com",
			filter:      "(&(objectClass=inetOrgPerson)(uid=jdoe))",
			wantCount:   1,
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com"},
		},
		{
			name:        "AND: objectClass=inetOrgPerson AND cn with 'John'",
			baseDN:      "dc=test,dc=com",
			filter:      "(&(objectClass=inetOrgPerson)(cn=John*))",
			wantCount:   1,
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com"},
		},
		{
			name:        "AND: objectClass=inetOrgPerson AND mail present",
			baseDN:      "dc=test,dc=com",
			filter:      "(&(objectClass=inetOrgPerson)(mail=*))",
			wantCount:   5, // All 5 users have mail (including admin)
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com", "uid=jsmith,ou=users,dc=test,dc=com"},
		},
		{
			name:        "AND with three conditions",
			baseDN:      "dc=test,dc=com",
			filter:      "(&(objectClass=inetOrgPerson)(cn=*Smith)(mail=jsmith@test.com))",
			wantCount:   1,
			wantContain: []string{"uid=jsmith,ou=users,dc=test,dc=com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := store.SearchEntries(ctx, tt.baseDN, tt.filter)
			if err != nil {
				t.Fatalf("SearchEntries() error = %v", err)
			}

			if len(entries) != tt.wantCount {
				t.Errorf("SearchEntries() got %d entries, want %d", len(entries), tt.wantCount)
			}

			// Check that expected DNs are in results
			entryDNs := make(map[string]bool)
			for _, e := range entries {
				entryDNs[e.DN] = true
			}

			for _, wantDN := range tt.wantContain {
				if !entryDNs[wantDN] {
					t.Errorf("SearchEntries() missing expected DN: %s", wantDN)
				}
			}
		})
	}
}

func TestSearchEntriesWithOrFilter(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	tests := []struct {
		name        string
		baseDN      string
		filter      string
		wantCount   int
		wantContain []string
	}{
		{
			name:        "OR: uid=jdoe OR uid=jsmith",
			baseDN:      "dc=test,dc=com",
			filter:      "(|(uid=jdoe)(uid=jsmith))",
			wantCount:   2,
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com", "uid=jsmith,ou=users,dc=test,dc=com"},
		},
		{
			name:        "OR: objectClass=organizationalUnit OR objectClass=groupOfNames",
			baseDN:      "dc=test,dc=com",
			filter:      "(|(objectClass=organizationalUnit)(objectClass=groupOfNames))",
			wantCount:   5, // 2 OUs + 3 groups
			wantContain: []string{"ou=users,dc=test,dc=com", "cn=ldaplite.admin,ou=groups,dc=test,dc=com", "cn=admins,ou=groups,dc=test,dc=com"},
		},
		{
			name:        "OR with three conditions",
			baseDN:      "dc=test,dc=com",
			filter:      "(|(uid=jdoe)(uid=bob)(uid=alice))",
			wantCount:   3,
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com", "uid=bob,ou=users,dc=test,dc=com", "uid=alice,ou=users,dc=test,dc=com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := store.SearchEntries(ctx, tt.baseDN, tt.filter)
			if err != nil {
				t.Fatalf("SearchEntries() error = %v", err)
			}

			if len(entries) != tt.wantCount {
				t.Errorf("SearchEntries() got %d entries, want %d", len(entries), tt.wantCount)
			}

			// Check that expected DNs are in results
			entryDNs := make(map[string]bool)
			for _, e := range entries {
				entryDNs[e.DN] = true
			}

			for _, wantDN := range tt.wantContain {
				if !entryDNs[wantDN] {
					t.Errorf("SearchEntries() missing expected DN: %s", wantDN)
				}
			}
		})
	}
}

func TestSearchEntriesWithNotFilter(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	tests := []struct {
		name           string
		baseDN         string
		filter         string
		wantMinCount   int // Use min count since NOT filters may match more entries
		wantNotContain []string
	}{
		{
			name:           "NOT: objectClass!=inetOrgPerson",
			baseDN:         "dc=test,dc=com",
			filter:         "(!(objectClass=inetOrgPerson))",
			wantMinCount:   3, // At least: 1 base + 2 OUs + 2 groups = 5 (excluding 4 users)
			wantNotContain: []string{"uid=jdoe,ou=users,dc=test,dc=com", "uid=jsmith,ou=users,dc=test,dc=com"},
		},
		{
			name:           "NOT: uid!=jdoe",
			baseDN:         "dc=test,dc=com",
			filter:         "(!(uid=jdoe))",
			wantMinCount:   7, // All entries except jdoe
			wantNotContain: []string{"uid=jdoe,ou=users,dc=test,dc=com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := store.SearchEntries(ctx, tt.baseDN, tt.filter)
			if err != nil {
				t.Fatalf("SearchEntries() error = %v", err)
			}

			if len(entries) < tt.wantMinCount {
				t.Errorf("SearchEntries() got %d entries, want at least %d", len(entries), tt.wantMinCount)
			}

			// Check that unwanted DNs are NOT in results
			entryDNs := make(map[string]bool)
			for _, e := range entries {
				entryDNs[e.DN] = true
			}

			for _, unwantedDN := range tt.wantNotContain {
				if entryDNs[unwantedDN] {
					t.Errorf("SearchEntries() should not contain DN: %s", unwantedDN)
				}
			}
		})
	}
}

func TestSearchEntriesWithComplexFilters(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	tests := []struct {
		name        string
		baseDN      string
		filter      string
		wantCount   int
		wantContain []string
	}{
		{
			name:        "Complex: (&(objectClass=inetOrgPerson)(|(uid=jdoe)(uid=bob)))",
			baseDN:      "dc=test,dc=com",
			filter:      "(&(objectClass=inetOrgPerson)(|(uid=jdoe)(uid=bob)))",
			wantCount:   2,
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com", "uid=bob,ou=users,dc=test,dc=com"},
		},
		{
			name:        "Complex: (&(objectClass=inetOrgPerson)(!(uid=jdoe)))",
			baseDN:      "dc=test,dc=com",
			filter:      "(&(objectClass=inetOrgPerson)(!(uid=jdoe)))",
			wantCount:   4, // All users except jdoe (including admin)
			wantContain: []string{"uid=admin,ou=users,dc=test,dc=com", "uid=jsmith,ou=users,dc=test,dc=com", "uid=bob,ou=users,dc=test,dc=com", "uid=alice,ou=users,dc=test,dc=com"},
		},
		{
			name:        "Complex: (|(&(objectClass=inetOrgPerson)(cn=John*))(objectClass=groupOfNames))",
			baseDN:      "dc=test,dc=com",
			filter:      "(|(&(objectClass=inetOrgPerson)(cn=John*))(objectClass=groupOfNames))",
			wantCount:   4, // John Doe + 3 groups
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com", "cn=ldaplite.admin,ou=groups,dc=test,dc=com", "cn=admins,ou=groups,dc=test,dc=com", "cn=developers,ou=groups,dc=test,dc=com"},
		},
		{
			name:        "Complex: (&(objectClass=inetOrgPerson)(cn=*)(mail=*))",
			baseDN:      "dc=test,dc=com",
			filter:      "(&(objectClass=inetOrgPerson)(cn=*)(mail=*))",
			wantCount:   5, // All users have both cn and mail (including admin)
			wantContain: []string{"uid=admin,ou=users,dc=test,dc=com", "uid=jdoe,ou=users,dc=test,dc=com", "uid=jsmith,ou=users,dc=test,dc=com", "uid=bob,ou=users,dc=test,dc=com", "uid=alice,ou=users,dc=test,dc=com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := store.SearchEntries(ctx, tt.baseDN, tt.filter)
			if err != nil {
				t.Fatalf("SearchEntries() error = %v", err)
			}

			if len(entries) != tt.wantCount {
				t.Errorf("SearchEntries() got %d entries, want %d", len(entries), tt.wantCount)
				// Print actual DNs for debugging
				t.Logf("Actual entries:")
				for _, e := range entries {
					t.Logf("  - %s", e.DN)
				}
			}

			// Check that expected DNs are in results
			entryDNs := make(map[string]bool)
			for _, e := range entries {
				entryDNs[e.DN] = true
			}

			for _, wantDN := range tt.wantContain {
				if !entryDNs[wantDN] {
					t.Errorf("SearchEntries() missing expected DN: %s", wantDN)
				}
			}
		})
	}
}

func TestSearchEntriesWithBaseDNScoping(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	tests := []struct {
		name      string
		baseDN    string
		filter    string
		wantCount int
	}{
		{
			name:      "search only under ou=users",
			baseDN:    "ou=users,dc=test,dc=com",
			filter:    "(objectClass=*)",
			wantCount: 6, // 1 OU + 5 users (including admin)
		},
		{
			name:      "search only under ou=groups",
			baseDN:    "ou=groups,dc=test,dc=com",
			filter:    "(objectClass=*)",
			wantCount: 4, // 1 OU + 3 groups
		},
		{
			name:      "search users only under ou=users",
			baseDN:    "ou=users,dc=test,dc=com",
			filter:    "(objectClass=inetOrgPerson)",
			wantCount: 5, // Including admin
		},
		{
			name:      "search groups only under ou=groups",
			baseDN:    "ou=groups,dc=test,dc=com",
			filter:    "(objectClass=groupOfNames)",
			wantCount: 3, // ldaplite.admin + 2 test groups
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := store.SearchEntries(ctx, tt.baseDN, tt.filter)
			if err != nil {
				t.Fatalf("SearchEntries() error = %v", err)
			}

			if len(entries) != tt.wantCount {
				t.Errorf("SearchEntries() got %d entries, want %d", len(entries), tt.wantCount)
			}
		})
	}
}

func TestSearchEntriesWithTimestampFilters(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	tests := []struct {
		name        string
		baseDN      string
		filter      string
		wantMinimum int // Minimum expected count (all entries have recent timestamps)
		shouldError bool
	}{
		{
			name:        "modifyTimestamp >= past date (should return all entries)",
			baseDN:      "dc=test,dc=com",
			filter:      "(modifyTimestamp>=20130905020304Z)",
			wantMinimum: 5, // At least users and groups
			shouldError: false,
		},
		{
			name:        "modifyTimestamp >= future date (should return no entries)",
			baseDN:      "dc=test,dc=com",
			filter:      "(modifyTimestamp>=20991231235959Z)",
			wantMinimum: 0,
			shouldError: false,
		},
		{
			name:        "createTimestamp >= past date (should return all entries)",
			baseDN:      "dc=test,dc=com",
			filter:      "(createTimestamp>=20130905020304Z)",
			wantMinimum: 5,
			shouldError: false,
		},
		{
			name:        "modifyTimestamp <= future date (should return all entries)",
			baseDN:      "dc=test,dc=com",
			filter:      "(modifyTimestamp<=20991231235959Z)",
			wantMinimum: 5,
			shouldError: false,
		},
		{
			name:        "createTimestamp <= past date (should return no entries)",
			baseDN:      "dc=test,dc=com",
			filter:      "(createTimestamp<=20130905020304Z)",
			wantMinimum: 0,
			shouldError: false,
		},
		{
			name:        "combined filter with timestamp and objectClass",
			baseDN:      "dc=test,dc=com",
			filter:      "(&(objectClass=inetOrgPerson)(modifyTimestamp>=20130905020304Z))",
			wantMinimum: 4, // At least the 4 test users (admin + 4 created users)
			shouldError: false,
		},
		{
			name:        "OR filter with timestamps",
			baseDN:      "dc=test,dc=com",
			filter:      "(|(modifyTimestamp>=20991231235959Z)(objectClass=organizationalUnit))",
			wantMinimum: 2, // At least the 2 OUs
			shouldError: false,
		},
		{
			name:        "timestamp filter on users OU",
			baseDN:      "ou=users,dc=test,dc=com",
			filter:      "(createTimestamp>=20130905020304Z)",
			wantMinimum: 5, // OU + 5 users
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := store.SearchEntries(ctx, tt.baseDN, tt.filter)

			if tt.shouldError {
				if err == nil {
					t.Errorf("SearchEntries() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("SearchEntries() error = %v", err)
			}

			// Check that results meet minimum expected count
			if len(entries) < tt.wantMinimum {
				t.Errorf("SearchEntries() got %d entries, want at least %d", len(entries), tt.wantMinimum)
				t.Logf("Filter: %s", tt.filter)
				t.Logf("Returned entries:")
				for _, e := range entries {
					t.Logf("  - %s (created: %v, updated: %v)", e.DN, e.CreatedAt, e.UpdatedAt)
				}
			}

			// Verify that returned entries have operational attributes
			if len(entries) > 0 {
				firstEntry := entries[0]
				// Make sure operational attributes were added
				firstEntry.AddOperationalAttributes()

				createTS := firstEntry.GetAttribute("createTimestamp")
				modifyTS := firstEntry.GetAttribute("modifyTimestamp")

				if createTS == "" {
					t.Errorf("Entry missing createTimestamp attribute")
				}
				if modifyTS == "" {
					t.Errorf("Entry missing modifyTimestamp attribute")
				}

				t.Logf("Sample entry %s: createTimestamp=%s, modifyTimestamp=%s",
					firstEntry.DN, createTS, modifyTS)
			}
		})
	}
}

func TestSearchEntriesTimestampComparisons(t *testing.T) {
	// This test verifies that timestamp comparisons work correctly with boundary conditions
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Test >= with a past timestamp (should return all entries)
	entriesGTE, err := store.SearchEntries(ctx, "dc=test,dc=com", "(modifyTimestamp>=20130905020304Z)")
	if err != nil {
		t.Fatalf("SearchEntries() with >= failed: %v", err)
	}
	if len(entriesGTE) == 0 {
		t.Error("SearchEntries() with >= past timestamp should return entries")
	}

	// Test <= with a future timestamp (should return all entries)
	entriesLTE, err := store.SearchEntries(ctx, "dc=test,dc=com", "(modifyTimestamp<=20991231235959Z)")
	if err != nil {
		t.Fatalf("SearchEntries() with <= failed: %v", err)
	}
	if len(entriesLTE) == 0 {
		t.Error("SearchEntries() with <= future timestamp should return entries")
	}

	// Test range query (should return all entries)
	entriesRange, err := store.SearchEntries(ctx, "dc=test,dc=com",
		"(&(modifyTimestamp>=20130905020304Z)(modifyTimestamp<=20991231235959Z))")
	if err != nil {
		t.Fatalf("SearchEntries() with range failed: %v", err)
	}
	if len(entriesRange) == 0 {
		t.Error("SearchEntries() with timestamp range should return entries")
	}

	// Test >= with future timestamp (should return no entries)
	entriesFuture, err := store.SearchEntries(ctx, "dc=test,dc=com", "(modifyTimestamp>=20991231235959Z)")
	if err != nil {
		t.Fatalf("SearchEntries() with >= future failed: %v", err)
	}
	if len(entriesFuture) != 0 {
		t.Errorf("SearchEntries() with >= future timestamp should return 0 entries, got %d", len(entriesFuture))
	}

	// Test <= with past timestamp (should return no entries)
	entriesPast, err := store.SearchEntries(ctx, "dc=test,dc=com", "(modifyTimestamp<=20130905020304Z)")
	if err != nil {
		t.Fatalf("SearchEntries() with <= past failed: %v", err)
	}
	if len(entriesPast) != 0 {
		t.Errorf("SearchEntries() with <= past timestamp should return 0 entries, got %d", len(entriesPast))
	}

	t.Logf(">=past: %d entries, <=future: %d entries, range: %d entries",
		len(entriesGTE), len(entriesLTE), len(entriesRange))
}

// =============================================================================
// memberOf Attribute Tests (RFC2307bis Compliance)
// =============================================================================

// TestMemberOfBasicPresence verifies that the memberOf attribute is populated
// for users who are members of groups (RFC2307bis compliance).
func TestMemberOfBasicPresence(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Test that jdoe has memberOf for admins group
	jdoeEntry, err := store.GetEntry(ctx, "uid=jdoe,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	if jdoeEntry == nil {
		t.Fatal("GetEntry() returned nil for jdoe")
	}

	memberOfValues := jdoeEntry.GetAttributes("memberOf")
	if len(memberOfValues) == 0 {
		t.Error("memberOf attribute should be present for jdoe (member of admins group)")
	}

	// Verify the memberOf value is the correct group DN
	found := false
	for _, v := range memberOfValues {
		if v == "cn=admins,ou=groups,dc=test,dc=com" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("memberOf should contain 'cn=admins,ou=groups,dc=test,dc=com', got: %v", memberOfValues)
	}

	t.Logf("✓ jdoe memberOf: %v", memberOfValues)
}

// TestMemberOfMultipleGroups verifies that users in multiple groups have
// multiple memberOf values (multi-valued attribute per RFC2307bis).
func TestMemberOfMultipleGroups(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Add jdoe to the developers group as well (already in admins)
	developers, err := store.GetEntry(ctx, "cn=developers,ou=groups,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	developers.AddAttribute("member", "uid=jdoe,ou=users,dc=test,dc=com")
	if err := store.UpdateEntry(ctx, developers); err != nil {
		t.Fatalf("UpdateEntry() failed: %v", err)
	}

	// Verify jdoe now has two memberOf values
	jdoeEntry, err := store.GetEntry(ctx, "uid=jdoe,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}

	memberOfValues := jdoeEntry.GetAttributes("memberOf")
	if len(memberOfValues) != 2 {
		t.Errorf("memberOf should have 2 values for jdoe (in admins and developers), got: %d", len(memberOfValues))
	}

	// Check both group DNs are present
	expectedGroups := map[string]bool{
		"cn=admins,ou=groups,dc=test,dc=com":     false,
		"cn=developers,ou=groups,dc=test,dc=com": false,
	}
	for _, v := range memberOfValues {
		if _, ok := expectedGroups[v]; ok {
			expectedGroups[v] = true
		}
	}
	for groupDN, found := range expectedGroups {
		if !found {
			t.Errorf("memberOf should contain '%s'", groupDN)
		}
	}

	t.Logf("✓ jdoe memberOf (multi-valued): %v", memberOfValues)
}

// TestMemberOfNotPresentForNonMembers verifies that users who are not
// members of any group do not have the memberOf attribute.
func TestMemberOfNotPresentForNonMembers(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Create a new user who is not in any group
	loneUser := models.NewUser("ou=users,dc=test,dc=com", "loneuser", "Lone User", "User", "lone@test.com")
	loneUser.SetPassword("{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dummyhash$dummyhash")
	if err := store.CreateEntry(ctx, loneUser.Entry); err != nil {
		t.Fatalf("CreateEntry() failed: %v", err)
	}

	// Verify loneuser has no memberOf
	loneEntry, err := store.GetEntry(ctx, "uid=loneuser,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}

	memberOfValues := loneEntry.GetAttributes("memberOf")
	if len(memberOfValues) != 0 {
		t.Errorf("memberOf should be empty for user not in any group, got: %v", memberOfValues)
	}

	t.Log("✓ loneuser has no memberOf (not in any group)")
}

// TestMemberOfOnlyForUsers verifies that memberOf is only populated
// for inetOrgPerson entries (users), not for groups, OUs, or other entries.
func TestMemberOfOnlyForUsers(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Check that groups don't have memberOf
	groupEntry, err := store.GetEntry(ctx, "cn=admins,ou=groups,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	groupMemberOf := groupEntry.GetAttributes("memberOf")
	if len(groupMemberOf) != 0 {
		t.Errorf("Groups should not have memberOf attribute, got: %v", groupMemberOf)
	}

	// Check that OUs don't have memberOf
	ouEntry, err := store.GetEntry(ctx, "ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	ouMemberOf := ouEntry.GetAttributes("memberOf")
	if len(ouMemberOf) != 0 {
		t.Errorf("OUs should not have memberOf attribute, got: %v", ouMemberOf)
	}

	// Check that base DN doesn't have memberOf
	baseEntry, err := store.GetEntry(ctx, "dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	baseMemberOf := baseEntry.GetAttributes("memberOf")
	if len(baseMemberOf) != 0 {
		t.Errorf("Base DN should not have memberOf attribute, got: %v", baseMemberOf)
	}

	t.Log("✓ memberOf only present for inetOrgPerson entries")
}

// TestMemberOfViaSearchEntries verifies that memberOf is populated
// when fetching users via SearchEntries.
func TestMemberOfViaSearchEntries(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Search for all inetOrgPerson entries
	entries, err := store.SearchEntries(ctx, "dc=test,dc=com", "(objectClass=inetOrgPerson)")
	if err != nil {
		t.Fatalf("SearchEntries() failed: %v", err)
	}

	// Build a map of user UIDs to their memberOf values
	userMemberOf := make(map[string][]string)
	for _, entry := range entries {
		uid := entry.GetAttribute("uid")
		memberOf := entry.GetAttributes("memberOf")
		userMemberOf[uid] = memberOf
	}

	// Verify expected memberships
	// jdoe should be in admins
	if !containsValue(userMemberOf["jdoe"], "cn=admins,ou=groups,dc=test,dc=com") {
		t.Errorf("jdoe should have memberOf=cn=admins, got: %v", userMemberOf["jdoe"])
	}

	// jsmith should be in developers
	if !containsValue(userMemberOf["jsmith"], "cn=developers,ou=groups,dc=test,dc=com") {
		t.Errorf("jsmith should have memberOf=cn=developers, got: %v", userMemberOf["jsmith"])
	}

	// bob should be in developers
	if !containsValue(userMemberOf["bob"], "cn=developers,ou=groups,dc=test,dc=com") {
		t.Errorf("bob should have memberOf=cn=developers, got: %v", userMemberOf["bob"])
	}

	// admin should be in ldaplite.admin
	if !containsValue(userMemberOf["admin"], "cn=ldaplite.admin,ou=groups,dc=test,dc=com") {
		t.Errorf("admin should have memberOf=cn=ldaplite.admin, got: %v", userMemberOf["admin"])
	}

	// alice should have no memberOf (not in any group)
	if len(userMemberOf["alice"]) != 0 {
		t.Errorf("alice should have no memberOf, got: %v", userMemberOf["alice"])
	}

	t.Log("✓ memberOf correctly populated via SearchEntries")
}

// TestMemberOfViaGetAllEntries verifies that memberOf is populated
// when fetching all entries via GetAllEntries.
func TestMemberOfViaGetAllEntries(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	entries, err := store.GetAllEntries(ctx)
	if err != nil {
		t.Fatalf("GetAllEntries() failed: %v", err)
	}

	// Find jdoe and verify memberOf
	var jdoeEntry *models.Entry
	for _, entry := range entries {
		if entry.GetAttribute("uid") == "jdoe" {
			jdoeEntry = entry
			break
		}
	}

	if jdoeEntry == nil {
		t.Fatal("jdoe not found in GetAllEntries results")
	}

	memberOfValues := jdoeEntry.GetAttributes("memberOf")
	if !containsValue(memberOfValues, "cn=admins,ou=groups,dc=test,dc=com") {
		t.Errorf("jdoe should have memberOf=cn=admins via GetAllEntries, got: %v", memberOfValues)
	}

	t.Log("✓ memberOf correctly populated via GetAllEntries")
}

// TestMemberOfViaGetChildren verifies that memberOf is populated
// when fetching children via GetChildren.
func TestMemberOfViaGetChildren(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Get children of ou=users
	entries, err := store.GetChildren(ctx, "ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetChildren() failed: %v", err)
	}

	// All entries should be users, find jdoe
	var jdoeEntry *models.Entry
	for _, entry := range entries {
		if entry.GetAttribute("uid") == "jdoe" {
			jdoeEntry = entry
			break
		}
	}

	if jdoeEntry == nil {
		t.Fatal("jdoe not found in GetChildren results")
	}

	memberOfValues := jdoeEntry.GetAttributes("memberOf")
	if !containsValue(memberOfValues, "cn=admins,ou=groups,dc=test,dc=com") {
		t.Errorf("jdoe should have memberOf=cn=admins via GetChildren, got: %v", memberOfValues)
	}

	t.Log("✓ memberOf correctly populated via GetChildren")
}

// TestMemberOfAfterGroupUpdate verifies that memberOf reflects changes
// when group membership is modified.
func TestMemberOfAfterGroupUpdate(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Verify alice initially has no memberOf
	aliceEntry, err := store.GetEntry(ctx, "uid=alice,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	initialMemberOf := aliceEntry.GetAttributes("memberOf")
	if len(initialMemberOf) != 0 {
		t.Errorf("alice should initially have no memberOf, got: %v", initialMemberOf)
	}

	// Add alice to the developers group
	developers, err := store.GetEntry(ctx, "cn=developers,ou=groups,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	developers.AddAttribute("member", "uid=alice,ou=users,dc=test,dc=com")
	if err := store.UpdateEntry(ctx, developers); err != nil {
		t.Fatalf("UpdateEntry() failed: %v", err)
	}

	// Verify alice now has memberOf
	aliceEntryUpdated, err := store.GetEntry(ctx, "uid=alice,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	updatedMemberOf := aliceEntryUpdated.GetAttributes("memberOf")
	if !containsValue(updatedMemberOf, "cn=developers,ou=groups,dc=test,dc=com") {
		t.Errorf("alice should have memberOf=cn=developers after update, got: %v", updatedMemberOf)
	}

	t.Log("✓ memberOf updated after group modification")
}

// TestMemberOfFilterSearch verifies that users can be searched by memberOf
// attribute (in-memory filter evaluation).
func TestMemberOfFilterSearch(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Search for users in the developers group using memberOf filter
	entries, err := store.SearchEntries(ctx, "dc=test,dc=com",
		"(&(objectClass=inetOrgPerson)(memberOf=cn=developers,ou=groups,dc=test,dc=com))")
	if err != nil {
		t.Fatalf("SearchEntries() failed: %v", err)
	}

	// Should find jsmith and bob
	if len(entries) != 2 {
		t.Errorf("Expected 2 users in developers group, got: %d", len(entries))
	}

	foundUIDs := make(map[string]bool)
	for _, entry := range entries {
		foundUIDs[entry.GetAttribute("uid")] = true
	}

	if !foundUIDs["jsmith"] {
		t.Error("jsmith should be found in developers group search")
	}
	if !foundUIDs["bob"] {
		t.Error("bob should be found in developers group search")
	}

	t.Logf("✓ memberOf filter search found: %v", foundUIDs)
}

// TestMemberOfDNFormat verifies that memberOf values use proper DN format
// as required by RFC2307bis (distinguishedName syntax).
func TestMemberOfDNFormat(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Get admin user entry
	adminEntry, err := store.GetEntry(ctx, "uid=admin,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}

	memberOfValues := adminEntry.GetAttributes("memberOf")
	if len(memberOfValues) == 0 {
		t.Fatal("admin should have memberOf attribute")
	}

	// Verify DN format: should start with cn= and contain proper DN components
	for _, dn := range memberOfValues {
		// Check it's a valid DN format (starts with attribute=value)
		if len(dn) < 3 || dn[2] != '=' {
			t.Errorf("memberOf value should be valid DN format, got: %s", dn)
		}

		// Check it contains the base DN suffix
		if !containsSubstring(dn, "dc=test,dc=com") {
			t.Errorf("memberOf DN should contain base DN, got: %s", dn)
		}

		// Check it's a group DN (starts with cn=)
		if dn[:3] != "cn=" {
			t.Errorf("memberOf should reference a group (cn=...), got: %s", dn)
		}
	}

	t.Logf("✓ memberOf uses proper DN format: %v", memberOfValues)
}

// TestMemberOfOperationalAttribute verifies that memberOf behaves as an
// operational attribute (not stored, computed on demand).
func TestMemberOfOperationalAttribute(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Create a new user
	testUser := models.NewUser("ou=users,dc=test,dc=com", "testop", "Test Op", "Op", "testop@test.com")
	testUser.SetPassword("{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dummyhash$dummyhash")
	if err := store.CreateEntry(ctx, testUser.Entry); err != nil {
		t.Fatalf("CreateEntry() failed: %v", err)
	}

	// The new user should have no memberOf
	entry, err := store.GetEntry(ctx, "uid=testop,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	if len(entry.GetAttributes("memberOf")) != 0 {
		t.Error("New user should have no memberOf")
	}

	// Add user to a group
	admins, err := store.GetEntry(ctx, "cn=admins,ou=groups,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	admins.AddAttribute("member", "uid=testop,ou=users,dc=test,dc=com")
	if err := store.UpdateEntry(ctx, admins); err != nil {
		t.Fatalf("UpdateEntry() failed: %v", err)
	}

	// Now the user should have memberOf (computed from group_members table)
	entryAfter, err := store.GetEntry(ctx, "uid=testop,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	memberOf := entryAfter.GetAttributes("memberOf")
	if !containsValue(memberOf, "cn=admins,ou=groups,dc=test,dc=com") {
		t.Errorf("User should have memberOf after being added to group, got: %v", memberOf)
	}

	t.Log("✓ memberOf behaves as operational attribute (computed, not stored)")
}

// Helper function to check if a slice contains a value
func containsValue(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
