package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/smarzola/ldaplite/pkg/config"
)

type memberOfBenchmarkFixture struct {
	store       *SQLiteStore
	ctx         context.Context
	baseDN      string
	usersDN     string
	targetUser  string
	directGroup string
	nestedGroup string
}

// Run the full matrix with:
// GOCACHE=/private/tmp/ldaplite-gocache go test -run '^$' -bench='BenchmarkMemberOf' -benchmem -benchtime=1x ./internal/store
//
// The 10k-entry cases are intended as explicit scale probes. Using 1x avoids
// rebuilding the large fixture repeatedly during Go's benchmark calibration.
func setupMemberOfBenchmarkFixture(b *testing.B, users, groups, membersPerGroup, nestedDepth int) memberOfBenchmarkFixture {
	b.Helper()

	baseDN := "dc=bench,dc=com"
	usersDN := "ou=users," + baseDN
	groupsDN := "ou=groups," + baseDN

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: b.TempDir() + "/bench.db",
		},
		LDAP: config.LDAPConfig{
			BaseDN: baseDN,
		},
		Security: config.SecurityConfig{
			Argon2Config: config.Argon2Config{
				Memory:      64,
				Iterations:  1,
				Parallelism: 1,
				SaltLength:  16,
				KeyLength:   32,
			},
		},
	}

	store := NewSQLiteStore(cfg)
	ctx := context.Background()
	b.Setenv("LDAP_ADMIN_PASSWORD", "benchmark_admin_password")

	if err := store.Initialize(ctx); err != nil {
		b.Fatalf("Initialize() failed: %v", err)
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		b.Fatalf("BeginTx() failed: %v", err)
	}
	defer tx.Rollback()

	entryStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO entries (dn, parent_dn, object_class, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		b.Fatalf("prepare entries insert failed: %v", err)
	}
	defer entryStmt.Close()

	attrStmt, err := tx.PrepareContext(ctx, `INSERT INTO attributes (entry_id, name, value) VALUES (?, ?, ?)`)
	if err != nil {
		b.Fatalf("prepare attributes insert failed: %v", err)
	}
	defer attrStmt.Close()

	userStmt, err := tx.PrepareContext(ctx, `INSERT INTO users (entry_id, password_hash) VALUES (?, ?)`)
	if err != nil {
		b.Fatalf("prepare users insert failed: %v", err)
	}
	defer userStmt.Close()

	groupStmt, err := tx.PrepareContext(ctx, `INSERT INTO groups (entry_id) VALUES (?)`)
	if err != nil {
		b.Fatalf("prepare groups insert failed: %v", err)
	}
	defer groupStmt.Close()

	groupMemberStmt, err := tx.PrepareContext(ctx, `INSERT INTO group_members (group_entry_id, member_entry_id) VALUES (?, ?)`)
	if err != nil {
		b.Fatalf("prepare group_members insert failed: %v", err)
	}
	defer groupMemberStmt.Close()

	now := time.Now()
	insertEntry := func(dn, parentDN, objectClass string) int64 {
		b.Helper()
		result, err := entryStmt.ExecContext(ctx, dn, parentDN, objectClass, now, now)
		if err != nil {
			b.Fatalf("insert entry %s failed: %v", dn, err)
		}
		entryID, err := result.LastInsertId()
		if err != nil {
			b.Fatalf("LastInsertId(%s) failed: %v", dn, err)
		}
		return entryID
	}
	insertAttr := func(entryID int64, name, value string) {
		b.Helper()
		if _, err := attrStmt.ExecContext(ctx, entryID, name, value); err != nil {
			b.Fatalf("insert attr %s=%s failed: %v", name, value, err)
		}
	}

	userEntryIDs := make([]int64, users)
	for i := 0; i < users; i++ {
		uid := fmt.Sprintf("user-%06d", i)
		userDN := "uid=" + uid + "," + usersDN
		entryID := insertEntry(userDN, usersDN, "inetOrgPerson")
		userEntryIDs[i] = entryID
		insertAttr(entryID, "uid", uid)
		insertAttr(entryID, "cn", uid)
		insertAttr(entryID, "sn", uid)
		insertAttr(entryID, "mail", uid+"@bench.test")
		if _, err := userStmt.ExecContext(ctx, entryID, "{ARGON2ID}$argon2id$v=19$m=64,t=1,p=1$dummyhash$dummyhash"); err != nil {
			b.Fatalf("insert user %s failed: %v", userDN, err)
		}
	}

	groupEntryIDs := make([]int64, groups)
	for groupIndex := 0; groupIndex < groups; groupIndex++ {
		groupCN := fmt.Sprintf("group-%06d", groupIndex)
		groupDN := "cn=" + groupCN + "," + groupsDN
		groupEntryID := insertEntry(groupDN, groupsDN, "groupOfNames")
		groupEntryIDs[groupIndex] = groupEntryID
		insertAttr(groupEntryID, "cn", groupCN)
		insertAttr(groupEntryID, "description", "benchmark group")
		if _, err := groupStmt.ExecContext(ctx, groupEntryID); err != nil {
			b.Fatalf("insert group %s failed: %v", groupDN, err)
		}
		for memberIndex := 0; memberIndex < membersPerGroup; memberIndex++ {
			userIndex := (groupIndex*membersPerGroup + memberIndex) % users
			memberDN := fmt.Sprintf("uid=user-%06d,%s", userIndex, usersDN)
			insertAttr(groupEntryID, "member", memberDN)
			if _, err := groupMemberStmt.ExecContext(ctx, groupEntryID, userEntryIDs[userIndex]); err != nil {
				b.Fatalf("insert membership %s -> %s failed: %v", groupDN, memberDN, err)
			}
		}
	}

	directGroup := fmt.Sprintf("cn=group-%06d,%s", 0, groupsDN)
	nestedGroupEntryID := groupEntryIDs[0]
	nestedGroup := directGroup
	for depth := 0; depth < nestedDepth; depth++ {
		groupCN := fmt.Sprintf("nested-%06d", depth)
		groupDN := "cn=" + groupCN + "," + groupsDN
		groupEntryID := insertEntry(groupDN, groupsDN, "groupOfNames")
		insertAttr(groupEntryID, "cn", groupCN)
		insertAttr(groupEntryID, "description", "nested benchmark group")
		insertAttr(groupEntryID, "member", nestedGroup)
		if _, err := groupStmt.ExecContext(ctx, groupEntryID); err != nil {
			b.Fatalf("insert nested group %s failed: %v", groupDN, err)
		}
		if _, err := groupMemberStmt.ExecContext(ctx, groupEntryID, nestedGroupEntryID); err != nil {
			b.Fatalf("insert nested membership %s -> %s failed: %v", groupDN, nestedGroup, err)
		}
		nestedGroup = groupDN
		nestedGroupEntryID = groupEntryID
	}

	if err := tx.Commit(); err != nil {
		b.Fatalf("benchmark fixture commit failed: %v", err)
	}

	return memberOfBenchmarkFixture{
		store:       store,
		ctx:         ctx,
		baseDN:      baseDN,
		usersDN:     usersDN,
		targetUser:  "uid=user-000000," + usersDN,
		directGroup: directGroup,
		nestedGroup: nestedGroup,
	}
}

func BenchmarkMemberOfSkipProjection(b *testing.B) {
	for _, users := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("users=%d", users), func(b *testing.B) {
			fixture := setupMemberOfBenchmarkFixture(b, users, 100, 20, 0)
			defer fixture.store.Close()

			options := SearchOptions{
				BaseDN:          fixture.usersDN,
				Filter:          "(uid=user-000000)",
				Scope:           SearchScopeWholeSubtree,
				IncludeMemberOf: false,
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entries, err := fixture.store.SearchEntriesWithOptions(fixture.ctx, options)
				if err != nil {
					b.Fatalf("SearchEntriesWithOptions() failed: %v", err)
				}
				if len(entries) != 1 || entries[0].HasAttribute("memberOf") {
					b.Fatalf("unexpected result count/memberOf state: len=%d attrs=%v", len(entries), entries)
				}
			}
		})
	}
}

func BenchmarkMemberOfSingleResultProjection(b *testing.B) {
	for _, users := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("users=%d", users), func(b *testing.B) {
			fixture := setupMemberOfBenchmarkFixture(b, users, 100, 20, 0)
			defer fixture.store.Close()

			options := SearchOptions{
				BaseDN:          fixture.usersDN,
				Filter:          "(uid=user-000000)",
				Scope:           SearchScopeWholeSubtree,
				IncludeMemberOf: true,
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entries, err := fixture.store.SearchEntriesWithOptions(fixture.ctx, options)
				if err != nil {
					b.Fatalf("SearchEntriesWithOptions() failed: %v", err)
				}
				if len(entries) != 1 || !containsValue(entries[0].GetAttributes("memberOf"), fixture.directGroup) {
					b.Fatalf("unexpected result count/memberOf state: len=%d memberOf=%v", len(entries), entries[0].GetAttributes("memberOf"))
				}
			}
		})
	}
}

func BenchmarkMemberOfAllUsersProjection(b *testing.B) {
	for _, users := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("users=%d", users), func(b *testing.B) {
			fixture := setupMemberOfBenchmarkFixture(b, users, 100, 20, 0)
			defer fixture.store.Close()

			options := SearchOptions{
				BaseDN:          fixture.usersDN,
				Filter:          "(objectClass=inetOrgPerson)",
				Scope:           SearchScopeSingleLevel,
				IncludeMemberOf: true,
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entries, err := fixture.store.SearchEntriesWithOptions(fixture.ctx, options)
				if err != nil {
					b.Fatalf("SearchEntriesWithOptions() failed: %v", err)
				}
				if want := users + 1; len(entries) != want {
					b.Fatalf("SearchEntriesWithOptions() got %d entries, want %d", len(entries), want)
				}
			}
		})
	}
}

func BenchmarkMemberOfDirectFilter(b *testing.B) {
	for _, users := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("users=%d", users), func(b *testing.B) {
			fixture := setupMemberOfBenchmarkFixture(b, users, 100, 20, 0)
			defer fixture.store.Close()

			options := SearchOptions{
				BaseDN:          fixture.baseDN,
				Filter:          "(memberOf=" + fixture.directGroup + ")",
				Scope:           SearchScopeWholeSubtree,
				IncludeMemberOf: false,
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entries, err := fixture.store.SearchEntriesWithOptions(fixture.ctx, options)
				if err != nil {
					b.Fatalf("SearchEntriesWithOptions() failed: %v", err)
				}
				if len(entries) != 20 || entries[0].HasAttribute("memberOf") {
					b.Fatalf("unexpected result count/memberOf state: len=%d firstMemberOf=%v", len(entries), entries[0].GetAttributes("memberOf"))
				}
			}
		})
	}
}

func BenchmarkMemberOfNestedProjection(b *testing.B) {
	for _, depth := range []int{1, 3, 10, 50} {
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			fixture := setupMemberOfBenchmarkFixture(b, 1000, 100, 20, depth)
			defer fixture.store.Close()

			options := SearchOptions{
				BaseDN:          fixture.targetUser,
				Filter:          "(objectClass=inetOrgPerson)",
				Scope:           SearchScopeBaseObject,
				IncludeMemberOf: true,
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entries, err := fixture.store.SearchEntriesWithOptions(fixture.ctx, options)
				if err != nil {
					b.Fatalf("SearchEntriesWithOptions() failed: %v", err)
				}
				if len(entries) != 1 || !containsValue(entries[0].GetAttributes("memberOf"), fixture.nestedGroup) {
					b.Fatalf("nested memberOf missing: len=%d memberOf=%v", len(entries), entries[0].GetAttributes("memberOf"))
				}
			}
		})
	}
}

func BenchmarkMemberOfNestedFilter(b *testing.B) {
	for _, depth := range []int{1, 3, 10, 50} {
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			fixture := setupMemberOfBenchmarkFixture(b, 1000, 100, 20, depth)
			defer fixture.store.Close()

			options := SearchOptions{
				BaseDN:          fixture.baseDN,
				Filter:          "(memberOf=" + fixture.nestedGroup + ")",
				Scope:           SearchScopeWholeSubtree,
				IncludeMemberOf: false,
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entries, err := fixture.store.SearchEntriesWithOptions(fixture.ctx, options)
				if err != nil {
					b.Fatalf("SearchEntriesWithOptions() failed: %v", err)
				}
				if len(entries) != 20 || entries[0].HasAttribute("memberOf") {
					b.Fatalf("unexpected nested result state: len=%d firstMemberOf=%v", len(entries), entries[0].GetAttributes("memberOf"))
				}
			}
		})
	}
}
