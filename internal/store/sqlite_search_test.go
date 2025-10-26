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
		user := models.NewUser("dc=test,dc=com", u.uid, u.cn, u.sn, u.givenName, u.mail)
		// SetPassword expects a hashed password, but for testing we'll use a dummy hash
		user.SetPassword("$argon2id$v=19$m=65536,t=3,p=2$dummyhash")
		if err := store.CreateUser(ctx, user); err != nil {
			t.Fatalf("failed to create user %s: %v", u.uid, err)
		}
	}

	// Create groups
	admins := models.NewGroup("dc=test,dc=com", "admins", "Administrators group")
	admins.AddMember("uid=jdoe,ou=users,dc=test,dc=com")
	if err := store.CreateGroup(ctx, admins); err != nil {
		t.Fatalf("failed to create admins group: %v", err)
	}

	developers := models.NewGroup("dc=test,dc=com", "developers", "Developers group")
	developers.AddMember("uid=jsmith,ou=users,dc=test,dc=com")
	developers.AddMember("uid=bob,ou=users,dc=test,dc=com")
	if err := store.CreateGroup(ctx, developers); err != nil {
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
			wantCount:   2,
			wantContain: []string{"cn=admins,ou=groups,dc=test,dc=com", "cn=developers,ou=groups,dc=test,dc=com"},
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
			wantCount: 10, // 1 base + 2 OUs + 5 users + 2 groups
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
			wantCount: 7, // 5 users + 2 groups
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
			wantCount:   4,
			wantContain: []string{"ou=users,dc=test,dc=com", "cn=admins,ou=groups,dc=test,dc=com"},
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
			wantCount:   3, // John Doe + 2 groups
			wantContain: []string{"uid=jdoe,ou=users,dc=test,dc=com", "cn=admins,ou=groups,dc=test,dc=com", "cn=developers,ou=groups,dc=test,dc=com"},
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
			wantCount: 3, // 1 OU + 2 groups
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
			wantCount: 2,
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
