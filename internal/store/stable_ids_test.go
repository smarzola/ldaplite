package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestAddEntryUUIDMigrationBackfillsExistingEntries(t *testing.T) {
	db, err := sql.Open("sqlite", t.TempDir()+"/legacy.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			dn TEXT UNIQUE NOT NULL
		);
		CREATE TABLE attributes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			value TEXT NOT NULL
		);
		INSERT INTO entries (id, dn) VALUES
			(1, 'uid=legacy,ou=users,dc=test,dc=com'),
			(2, 'uid=existing,ou=users,dc=test,dc=com');
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	migration, err := migrationsFS.ReadFile("migrations/013_add_entry_uuid_attributes.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("run migration: %v", err)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT e.id, a.name, a.value
		FROM entries e
		INNER JOIN attributes a ON a.entry_id = e.id
		WHERE LOWER(a.name) IN ('entryuuid', 'uuid')
		ORDER BY e.id, a.name
	`)
	if err != nil {
		t.Fatalf("query migrated attributes: %v", err)
	}
	defer rows.Close()

	valuesByEntry := map[int64]map[string]string{}
	for rows.Next() {
		var entryID int64
		var name, value string
		if err := rows.Scan(&entryID, &name, &value); err != nil {
			t.Fatalf("scan migrated attribute: %v", err)
		}
		if valuesByEntry[entryID] == nil {
			valuesByEntry[entryID] = map[string]string{}
		}
		valuesByEntry[entryID][name] = value
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migrated attributes: %v", err)
	}

	for entryID, attrs := range valuesByEntry {
		entryUUID := attrs["entryuuid"]
		if entryUUID == "" {
			t.Fatalf("entry %d attrs = %#v, want entryuuid", entryID, attrs)
		}
		if attrs["uuid"] != "" {
			t.Fatalf("entry %d attrs = %#v, want no uuid alias", entryID, attrs)
		}
		if _, err := uuid.Parse(entryUUID); err != nil {
			t.Fatalf("entry %d entryuuid = %q, want valid UUID: %v", entryID, entryUUID, err)
		}
	}
}

func TestRemoveUUIDAliasMigrationDeletesPersistedAliases(t *testing.T) {
	db, err := sql.Open("sqlite", t.TempDir()+"/released.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE attributes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			value TEXT NOT NULL
		);
		INSERT INTO attributes (entry_id, name, value) VALUES
			(1, 'entryuuid', '1d84d1af-89ef-4cc2-98fb-f868b84f10e1'),
			(1, 'uuid', '1d84d1af-89ef-4cc2-98fb-f868b84f10e1'),
			(1, 'uid', 'jane');
	`); err != nil {
		t.Fatalf("seed released schema: %v", err)
	}

	migration, err := migrationsFS.ReadFile("migrations/014_remove_uuid_alias.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("run migration: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM attributes WHERE LOWER(name) = 'uuid'`).Scan(&count); err != nil {
		t.Fatalf("count uuid attrs: %v", err)
	}
	if count != 0 {
		t.Fatalf("uuid attr count = %d, want 0", count)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM attributes WHERE LOWER(name) = 'entryuuid'`).Scan(&count); err != nil {
		t.Fatalf("count entryuuid attrs: %v", err)
	}
	if count != 1 {
		t.Fatalf("entryuuid attr count = %d, want 1", count)
	}
}
