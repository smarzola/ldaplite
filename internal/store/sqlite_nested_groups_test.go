package store

import (
	"context"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
)

func TestNestedGroupMemberOfIsTransitive(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	engineering := models.NewGroup("ou=groups,dc=test,dc=com", "engineering", "Engineering group")
	engineering.AddMember("cn=developers,ou=groups,dc=test,dc=com")
	if err := store.CreateEntry(ctx, engineering.Entry); err != nil {
		t.Fatalf("CreateEntry(engineering) failed: %v", err)
	}

	bob, err := store.GetEntry(ctx, "uid=bob,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry(bob) failed: %v", err)
	}

	memberOf := bob.GetAttributes("memberOf")
	if !containsValue(memberOf, "cn=developers,ou=groups,dc=test,dc=com") {
		t.Fatalf("bob should have direct developers memberOf, got %v", memberOf)
	}
	if !containsValue(memberOf, "cn=engineering,ou=groups,dc=test,dc=com") {
		t.Fatalf("bob should have transitive engineering memberOf, got %v", memberOf)
	}
}

func TestIsUserInGroupIsTransitive(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	engineering := models.NewGroup("ou=groups,dc=test,dc=com", "engineering", "Engineering group")
	engineering.AddMember("cn=developers,ou=groups,dc=test,dc=com")
	if err := store.CreateEntry(ctx, engineering.Entry); err != nil {
		t.Fatalf("CreateEntry(engineering) failed: %v", err)
	}

	isMember, err := store.IsUserInGroup(ctx, "uid=jsmith,ou=users,dc=test,dc=com", "cn=engineering,ou=groups,dc=test,dc=com")
	if err != nil {
		t.Fatalf("IsUserInGroup() failed: %v", err)
	}
	if !isMember {
		t.Fatal("jsmith should be a transitive member of engineering via developers")
	}
}

func TestNestedGroupCycleDoesNotLoop(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	engineering := models.NewGroup("ou=groups,dc=test,dc=com", "engineering", "Engineering group")
	engineering.AddMember("cn=developers,ou=groups,dc=test,dc=com")
	if err := store.CreateEntry(ctx, engineering.Entry); err != nil {
		t.Fatalf("CreateEntry(engineering) failed: %v", err)
	}

	developers, err := store.GetEntry(ctx, "cn=developers,ou=groups,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry(developers) failed: %v", err)
	}
	developers.AddAttribute("member", "cn=engineering,ou=groups,dc=test,dc=com")
	if err := store.UpdateEntry(ctx, developers); err != nil {
		t.Fatalf("UpdateEntry(developers cycle) failed: %v", err)
	}

	isMember, err := store.IsUserInGroup(ctx, "uid=bob,ou=users,dc=test,dc=com", "cn=engineering,ou=groups,dc=test,dc=com")
	if err != nil {
		t.Fatalf("IsUserInGroup() failed: %v", err)
	}
	if !isMember {
		t.Fatal("bob should still be a member of engineering in cyclic group graph")
	}

	bob, err := store.GetEntry(ctx, "uid=bob,ou=users,dc=test,dc=com")
	if err != nil {
		t.Fatalf("GetEntry(bob) failed: %v", err)
	}
	memberOf := bob.GetAttributes("memberOf")
	if countValue(memberOf, "cn=engineering,ou=groups,dc=test,dc=com") != 1 {
		t.Fatalf("engineering should appear once in memberOf despite cycle, got %v", memberOf)
	}
}

func countValue(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}
