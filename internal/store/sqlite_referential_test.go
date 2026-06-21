package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
)

func TestCreateEntryRejectsOutsideBaseDN(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	user := models.NewUser("ou=users,dc=other,dc=com", "outsider", "Out Sider", "Sider", "outsider@test.com")
	user.SetPassword("{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dummyhash$dummyhash")

	err := store.CreateEntry(ctx, user.Entry)
	if err == nil {
		t.Fatal("CreateEntry() expected outside-base error, got nil")
	}
	if !strings.Contains(err.Error(), "outside base DN") {
		t.Fatalf("CreateEntry() error = %v, want outside base DN", err)
	}
}

func TestCreateEntryRejectsMissingParentDN(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	user := models.NewUser("ou=missing,dc=test,dc=com", "orphan", "Orphan User", "User", "orphan@test.com")
	user.SetPassword("{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dummyhash$dummyhash")

	err := store.CreateEntry(ctx, user.Entry)
	if err == nil {
		t.Fatal("CreateEntry() expected missing-parent error, got nil")
	}
	if !errors.Is(err, ErrNoSuchObject) {
		t.Fatalf("CreateEntry() error = %v, want ErrNoSuchObject", err)
	}
	if !strings.Contains(err.Error(), "parent DN does not exist") {
		t.Fatalf("CreateEntry() error = %v, want parent DN does not exist", err)
	}
}

func TestEntryExistsIsCaseInsensitive(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	exists, err := store.EntryExists(ctx, "UID=ADMIN,OU=USERS,DC=TEST,DC=COM")
	if err != nil {
		t.Fatalf("EntryExists() failed: %v", err)
	}
	if !exists {
		t.Fatal("EntryExists() should find entries case-insensitively")
	}
}

func TestCreateEntryRejectsCaseVariantDuplicateDN(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	user := models.NewUser("OU=USERS,DC=TEST,DC=COM", "ADMIN", "Admin Duplicate", "Duplicate", "admin2@test.com")
	user.SetPassword("{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dummyhash$dummyhash")

	err := store.CreateEntry(ctx, user.Entry)
	if !errors.Is(err, ErrEntryAlreadyExists) {
		t.Fatalf("CreateEntry() error = %v, want ErrEntryAlreadyExists", err)
	}
}

func TestCreateEntryRejectsExactDuplicateDN(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	user := models.NewUser("ou=users,dc=test,dc=com", "admin", "Admin Duplicate", "Duplicate", "admin2@test.com")
	user.SetPassword("{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dummyhash$dummyhash")

	err := store.CreateEntry(ctx, user.Entry)
	if !errors.Is(err, ErrEntryAlreadyExists) {
		t.Fatalf("CreateEntry() error = %v, want ErrEntryAlreadyExists", err)
	}
}

func TestSQLiteUniqueConstraintDetection(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	_, err := store.db.ExecContext(ctx, `
		INSERT INTO entries (dn, parent_dn, object_class)
		VALUES (?, ?, ?)
	`, "uid=admin,ou=users,dc=test,dc=com", "ou=users,dc=test,dc=com", "inetOrgPerson")
	if err == nil {
		t.Fatal("duplicate insert should fail")
	}
	if !isSQLiteUniqueConstraint(err) {
		t.Fatalf("isSQLiteUniqueConstraint() = false for %T: %v", err, err)
	}
}

func TestDatabaseRejectsCaseVariantDuplicateDN(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	_, err := store.db.ExecContext(ctx, `
		INSERT INTO entries (dn, parent_dn, object_class)
		VALUES (?, ?, ?)
	`, "UID=ADMIN,OU=USERS,DC=TEST,DC=COM", "ou=users,dc=test,dc=com", "inetOrgPerson")
	if err == nil {
		t.Fatal("case-variant duplicate insert should fail")
	}
	if !isSQLiteUniqueConstraint(err) {
		t.Fatalf("isSQLiteUniqueConstraint() = false for %T: %v", err, err)
	}
}

func TestCreateEntryAcceptsCaseVariantParentDN(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	user := models.NewUser("OU=USERS,DC=TEST,DC=COM", "mixedcaseparent", "Mixed Parent", "Parent", "mixed@test.com")
	user.SetPassword("{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dummyhash$dummyhash")

	if err := store.CreateEntry(ctx, user.Entry); err != nil {
		t.Fatalf("CreateEntry() with mixed-case parent failed: %v", err)
	}

	found, err := store.GetEntry(ctx, "uid=mixedcaseparent,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	if found == nil {
		t.Fatal("GetEntry() should find mixed-case parent entry by lower-case DN")
	}

	children, err := store.SearchEntriesWithOptions(ctx, SearchOptions{
		BaseDN:          "OU=USERS,DC=TEST,DC=COM",
		Filter:          "(uid=mixedcaseparent)",
		Scope:           SearchScopeSingleLevel,
		IncludeMemberOf: false,
	})
	if err != nil {
		t.Fatalf("SearchEntriesWithOptions() failed: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("single-level search should find child using a case-variant parent DN, got %d", len(children))
	}
}

func TestCreateGroupRejectsMissingMemberDN(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	group := models.NewGroup("ou=groups,dc=test,dc=com", "broken", "Broken group")
	group.AddMember("uid=missing,ou=users,dc=test,dc=com")

	err := store.CreateEntry(ctx, group.Entry)
	if err == nil {
		t.Fatal("CreateEntry() expected missing-member error, got nil")
	}
	if !errors.Is(err, ErrConstraintViolation) {
		t.Fatalf("CreateEntry() error = %v, want ErrConstraintViolation", err)
	}
	if !strings.Contains(err.Error(), "group member does not exist") {
		t.Fatalf("CreateEntry() error = %v, want group member does not exist", err)
	}

	exists, err := store.EntryExists(ctx, "cn=broken,ou=groups,dc=test,dc=com")
	if err != nil {
		t.Fatalf("EntryExists() failed: %v", err)
	}
	if exists {
		t.Fatal("group entry should have rolled back after missing member")
	}
}

func TestCreateGroupRejectsMissingMemberAttribute(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	group := models.NewGroup("ou=groups,dc=test,dc=com", "emptygroup", "Empty group")

	err := store.CreateEntry(ctx, group.Entry)
	if err == nil {
		t.Fatal("CreateEntry() expected missing-member-attribute error, got nil")
	}
	if !errors.Is(err, ErrObjectClassViolation) {
		t.Fatalf("CreateEntry() error = %v, want ErrObjectClassViolation", err)
	}
	if !strings.Contains(err.Error(), "required attribute member is missing") {
		t.Fatalf("CreateEntry() error = %v, want required attribute member is missing", err)
	}
}

func TestCreateGroupAcceptsCaseVariantMemberDN(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	group := models.NewGroup("ou=groups,dc=test,dc=com", "mixedmember", "Mixed member group")
	group.AddMember("UID=JDOE,OU=USERS,DC=TEST,DC=COM")

	if err := store.CreateEntry(ctx, group.Entry); err != nil {
		t.Fatalf("CreateEntry() with mixed-case member failed: %v", err)
	}

	jdoe, err := store.GetEntry(ctx, "uid=jdoe,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry(jdoe) failed: %v", err)
	}
	if !containsValue(jdoe.GetAttributes("memberOf"), "cn=mixedmember,ou=groups,dc=test,dc=com") {
		t.Fatalf("memberOf missing mixedmember group, got %v", jdoe.GetAttributes("memberOf"))
	}
}

func TestCreateGroupIgnoresDuplicateCaseVariantMembers(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	group := models.NewGroup("ou=groups,dc=test,dc=com", "duplicatemembers", "Duplicate members group")
	group.AddMember("uid=jdoe,ou=users,dc=test,dc=com")
	group.AddMember("UID=JDOE,OU=USERS,DC=TEST,DC=COM")

	if err := store.CreateEntry(ctx, group.Entry); err != nil {
		t.Fatalf("CreateEntry() with duplicate case-variant members failed: %v", err)
	}

	var count int
	err := store.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM group_members gm
		INNER JOIN entries g ON g.id = gm.group_entry_id
		INNER JOIN entries m ON m.id = gm.member_entry_id
		WHERE LOWER(g.dn) = LOWER(?)
		  AND LOWER(m.dn) = LOWER(?)
	`, "cn=duplicatemembers,ou=groups,dc=test,dc=com", "uid=jdoe,ou=users,dc=test,dc=com").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count duplicate memberships: %v", err)
	}
	if count != 1 {
		t.Fatalf("group_members row count = %d, want 1", count)
	}
}

func TestUpdateGroupRejectsMissingMemberDNAndRollsBack(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	developers, err := store.GetEntry(ctx, "cn=developers,ou=groups,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry() failed: %v", err)
	}
	developers.AddAttribute("member", "uid=missing,ou=users,dc=test,dc=com")

	err = store.UpdateEntry(ctx, developers)
	if err == nil {
		t.Fatal("UpdateEntry() expected missing-member error, got nil")
	}
	if !errors.Is(err, ErrConstraintViolation) {
		t.Fatalf("UpdateEntry() error = %v, want ErrConstraintViolation", err)
	}
	if !strings.Contains(err.Error(), "group member does not exist") {
		t.Fatalf("UpdateEntry() error = %v, want group member does not exist", err)
	}

	jsmith, err := store.GetEntry(ctx, "uid=jsmith,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry(jsmith) failed: %v", err)
	}
	if !containsValue(jsmith.GetAttributes("memberOf"), "cn=developers,ou=groups,dc=test,dc=com") {
		t.Fatalf("existing memberOf should survive failed update, got %v", jsmith.GetAttributes("memberOf"))
	}

	reloaded, err := store.GetEntry(ctx, "cn=developers,ou=groups,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry(developers) failed: %v", err)
	}
	if containsValue(reloaded.GetAttributes("member"), "uid=missing,ou=users,dc=test,dc=com") {
		t.Fatal("missing member attribute should not persist after failed update")
	}
}
