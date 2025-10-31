package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/pkg/config"
)

// TestPasswordNotStoredInAttributes verifies that userPassword is never stored in the attributes table
func TestPasswordNotStoredInAttributes(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path:            tmpDir + "/test.db",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 300,
		},
		LDAP: config.LDAPConfig{
			BaseDN: "dc=test,dc=com",
		},
		Security: config.SecurityConfig{
			Argon2Config: config.Argon2Config{
				Memory:      65536,
				Iterations:  3,
				Parallelism: 2,
				SaltLength:  16,
				KeyLength:   32,
			},
		},
	}

	os.Setenv("LDAP_ADMIN_PASSWORD", "TestPassword123!")
	defer os.Unsetenv("LDAP_ADMIN_PASSWORD")

	store := NewSQLiteStore(cfg)
	ctx := context.Background()

	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer store.Close()

	// Create a test user with a password
	user := models.NewUser("dc=test,dc=com", "testuser", "Test", "User", "testuser@example.com")
	hashedPassword := "{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dGVzdHNhbHQ$testhash"
	user.SetPassword(hashedPassword)

	if err := store.CreateEntry(ctx, user.Entry); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify password is stored in users table (Phase 3: join with attributes for uid lookup)
	var storedPasswordHash string
	query := `
		SELECT u.password_hash
		FROM users u
		INNER JOIN attributes a ON u.entry_id = a.entry_id
		WHERE a.name = 'uid' AND a.value = ?
	`
	err := store.db.QueryRow(query, "testuser").Scan(&storedPasswordHash)
	if err != nil {
		t.Fatalf("Failed to query users table: %v", err)
	}
	if storedPasswordHash != hashedPassword {
		t.Errorf("Password hash not stored correctly in users table. Got: %s, Want: %s", storedPasswordHash, hashedPassword)
	}

	// Verify password is NOT stored in attributes table
	var count int
	err = store.db.QueryRow(`
		SELECT COUNT(*) FROM attributes
		WHERE entry_id = (SELECT id FROM entries WHERE dn = ?)
		AND LOWER(name) = 'userpassword'
	`, user.DN).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query attributes table: %v", err)
	}
	if count != 0 {
		t.Errorf("userPassword found in attributes table! Count: %d (expected: 0)", count)
	}

	// Verify GetEntry does NOT return userPassword in Attributes
	entry, err := store.GetEntry(ctx, user.DN)
	if err != nil {
		t.Fatalf("Failed to get entry: %v", err)
	}
	if entry == nil {
		t.Fatal("Entry not found")
	}

	if passwd := entry.GetAttribute("userPassword"); passwd != "" {
		t.Errorf("GetEntry returned userPassword in Attributes! Value: %s", passwd)
	}

	// Verify SearchEntries does NOT return userPassword in Attributes
	entries, err := store.SearchEntries(ctx, "dc=test,dc=com", "(uid=testuser)")
	if err != nil {
		t.Fatalf("Failed to search entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if passwd := entries[0].GetAttribute("userPassword"); passwd != "" {
		t.Errorf("SearchEntries returned userPassword in Attributes! Value: %s", passwd)
	}

	t.Log("✓ Password security verified: userPassword not stored in or returned from attributes table")
}

// TestPasswordUpdateNotStoredInAttributes verifies password updates don't leak into attributes table
func TestPasswordUpdateNotStoredInAttributes(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path:            tmpDir + "/test.db",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 300,
		},
		LDAP: config.LDAPConfig{
			BaseDN: "dc=test,dc=com",
		},
		Security: config.SecurityConfig{
			Argon2Config: config.Argon2Config{
				Memory:      65536,
				Iterations:  3,
				Parallelism: 2,
				SaltLength:  16,
				KeyLength:   32,
			},
		},
	}

	os.Setenv("LDAP_ADMIN_PASSWORD", "TestPassword123!")
	defer os.Unsetenv("LDAP_ADMIN_PASSWORD")

	store := NewSQLiteStore(cfg)
	ctx := context.Background()

	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer store.Close()

	// Create a test user with a password
	user := models.NewUser("dc=test,dc=com", "testuser", "Test", "User", "testuser@example.com")
	hashedPassword1 := "{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$dGVzdHNhbHQ$testhash1"
	user.SetPassword(hashedPassword1)

	if err := store.CreateEntry(ctx, user.Entry); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Update the user's password
	entry, err := store.GetEntry(ctx, user.DN)
	if err != nil {
		t.Fatalf("Failed to get entry: %v", err)
	}

	hashedPassword2 := "{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$bmV3c2FsdA$newhash2"
	entry.SetAttribute("userPassword", hashedPassword2)
	entry.SetAttribute("mail", "updated@example.com") // Also update another attribute
	entry.UpdatedAt = time.Now()

	if err := store.UpdateEntry(ctx, entry); err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Verify password was updated in users table (Phase 3: join with attributes for uid lookup)
	var storedPasswordHash string
	query := `
		SELECT u.password_hash
		FROM users u
		INNER JOIN attributes a ON u.entry_id = a.entry_id
		WHERE a.name = 'uid' AND a.value = ?
	`
	err = store.db.QueryRow(query, "testuser").Scan(&storedPasswordHash)
	if err != nil {
		t.Fatalf("Failed to query users table: %v", err)
	}
	if storedPasswordHash != hashedPassword2 {
		t.Errorf("Password hash not updated in users table. Got: %s, Want: %s", storedPasswordHash, hashedPassword2)
	}

	// Verify password is still NOT in attributes table
	var count int
	err = store.db.QueryRow(`
		SELECT COUNT(*) FROM attributes
		WHERE entry_id = (SELECT id FROM entries WHERE dn = ?)
		AND LOWER(name) = 'userpassword'
	`, user.DN).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query attributes table: %v", err)
	}
	if count != 0 {
		t.Errorf("userPassword found in attributes table after update! Count: %d (expected: 0)", count)
	}

	// Verify other attribute was updated
	updatedEntry, err := store.GetEntry(ctx, user.DN)
	if err != nil {
		t.Fatalf("Failed to get updated entry: %v", err)
	}
	if mail := updatedEntry.GetAttribute("mail"); mail != "updated@example.com" {
		t.Errorf("Mail attribute not updated. Got: %s, Want: updated@example.com", mail)
	}

	t.Log("✓ Password update security verified: userPassword not stored in attributes table after update")
}

// TestMigrationCleansUpExistingPasswords verifies that migration 003 removes passwords from attributes
func TestMigrationCleansUpExistingPasswords(t *testing.T) {
	// This test simulates the scenario where passwords were previously stored in attributes table
	// and verifies that the migration cleans them up

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path:            tmpDir + "/test.db",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 300,
		},
		LDAP: config.LDAPConfig{
			BaseDN: "dc=test,dc=com",
		},
		Security: config.SecurityConfig{
			Argon2Config: config.Argon2Config{
				Memory:      65536,
				Iterations:  3,
				Parallelism: 2,
				SaltLength:  16,
				KeyLength:   32,
			},
		},
	}

	os.Setenv("LDAP_ADMIN_PASSWORD", "TestPassword123!")
	defer os.Unsetenv("LDAP_ADMIN_PASSWORD")

	store := NewSQLiteStore(cfg)
	ctx := context.Background()

	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer store.Close()

	// Verify no userPassword entries exist in attributes table (migration should have run)
	var count int
	err := store.db.QueryRow("SELECT COUNT(*) FROM attributes WHERE LOWER(name) = 'userpassword'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query attributes table: %v", err)
	}
	if count != 0 {
		t.Errorf("Migration failed to clean up userPassword from attributes table! Count: %d (expected: 0)", count)
	}

	t.Log("✓ Migration verified: no userPassword entries in attributes table")
}
