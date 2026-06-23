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
		INSERT INTO attributes (entry_id, name, value) VALUES
			(2, 'uuid', '1d84d1af-89ef-4cc2-98fb-f868b84f10e1');
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
		compatUUID := attrs["uuid"]
		if entryUUID == "" || compatUUID == "" {
			t.Fatalf("entry %d attrs = %#v, want entryuuid and uuid", entryID, attrs)
		}
		if entryUUID != compatUUID {
			t.Fatalf("entry %d entryuuid = %q, uuid = %q, want same value", entryID, entryUUID, compatUUID)
		}
		if _, err := uuid.Parse(entryUUID); err != nil {
			t.Fatalf("entry %d entryuuid = %q, want valid UUID: %v", entryID, entryUUID, err)
		}
	}

	if got := valuesByEntry[2]["entryuuid"]; got != "1d84d1af-89ef-4cc2-98fb-f868b84f10e1" {
		t.Fatalf("existing uuid was not preserved as entryUUID: got %q", got)
	}
}
