package store

import (
	"context"
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
	if !strings.Contains(err.Error(), "parent DN does not exist") {
		t.Fatalf("CreateEntry() error = %v, want parent DN does not exist", err)
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
