package store

import (
	"context"
	"errors"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
)

func TestStoreReadOperationsHonorCanceledContext(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := store.GetEntryWithOptions(ctx, "uid=jdoe,ou=users,dc=test,dc=com", EntryOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetEntryWithOptions() error = %v, want context.Canceled", err)
	}

	if _, err := store.SearchEntriesWithOptions(ctx, SearchOptions{
		BaseDN: "dc=test,dc=com",
		Filter: "(uid=jdoe)",
		Scope:  SearchScopeWholeSubtree,
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("SearchEntriesWithOptions() error = %v, want context.Canceled", err)
	}
}

func TestStoreWriteOperationHonorsCanceledContext(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	entry := models.NewUser("ou=users,dc=test,dc=com", "canceled", "Canceled User", "User", "canceled@test.com")
	entry.SetPassword("{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dummyhash$dummyhash")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := store.CreateEntry(ctx, entry.Entry); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateEntry() error = %v, want context.Canceled", err)
	}
}
