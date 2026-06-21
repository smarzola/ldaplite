package main

import (
	"context"
	"database/sql"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/pkg/config"
)

func TestRunHealthcheckSuccess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ldaplite.db")
	createHealthcheckDB(t, dbPath, "dc=test,dc=com", true)

	_, port := startTestLDAPListener(t)
	cfg := testHealthcheckConfig(dbPath, "dc=test,dc=com")
	cfg.Server.BindAddress = "127.0.0.1"
	cfg.Server.Port = port
	if err := runHealthcheck(context.Background(), cfg); err != nil {
		t.Fatalf("runHealthcheck() error = %v", err)
	}
}

func TestRunHealthcheckFailsForMissingDatabase(t *testing.T) {
	cfg := testHealthcheckConfig(filepath.Join(t.TempDir(), "missing.db"), "dc=test,dc=com")

	err := runHealthcheck(context.Background(), cfg)
	if err == nil {
		t.Fatal("runHealthcheck() expected missing database error, got nil")
	}
	if !strings.Contains(err.Error(), "database is not accessible") {
		t.Fatalf("runHealthcheck() error = %v, want database is not accessible", err)
	}
}

func TestRunHealthcheckFailsForMissingRequiredTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ldaplite.db")
	createHealthcheckDB(t, dbPath, "dc=test,dc=com", false)

	cfg := testHealthcheckConfig(dbPath, "dc=test,dc=com")
	err := runHealthcheck(context.Background(), cfg)
	if err == nil {
		t.Fatal("runHealthcheck() expected missing table error, got nil")
	}
	if !strings.Contains(err.Error(), "missing required table") {
		t.Fatalf("runHealthcheck() error = %v, want missing required table", err)
	}
}

func TestRunHealthcheckFailsForMissingBaseDN(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ldaplite.db")
	createHealthcheckDB(t, dbPath, "dc=other,dc=com", true)

	_, port := startTestLDAPListener(t)
	cfg := testHealthcheckConfig(dbPath, "dc=test,dc=com")
	cfg.Server.BindAddress = "127.0.0.1"
	cfg.Server.Port = port
	err := runHealthcheck(context.Background(), cfg)
	if err == nil {
		t.Fatal("runHealthcheck() expected missing base DN error, got nil")
	}
	if !strings.Contains(err.Error(), "base DN does not exist") {
		t.Fatalf("runHealthcheck() error = %v, want base DN does not exist", err)
	}
}

func TestRunHealthcheckFailsForUnreachableLDAPListener(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ldaplite.db")
	createHealthcheckDB(t, dbPath, "dc=test,dc=com", true)

	cfg := testHealthcheckConfig(dbPath, "dc=test,dc=com")
	cfg.Server.BindAddress = "127.0.0.1"
	cfg.Server.Port = unusedTCPPort(t)

	err := runHealthcheck(context.Background(), cfg)
	if err == nil {
		t.Fatal("runHealthcheck() expected unreachable listener error, got nil")
	}
	if !strings.Contains(err.Error(), "LDAP listener is not reachable") {
		t.Fatalf("runHealthcheck() error = %v, want LDAP listener is not reachable", err)
	}
}

func TestHealthcheckAddressUsesLoopbackForWildcardBind(t *testing.T) {
	if got := healthcheckAddress("0.0.0.0", 3389); got != "127.0.0.1:3389" {
		t.Fatalf("healthcheckAddress(0.0.0.0) = %q, want 127.0.0.1:3389", got)
	}
	if got := healthcheckAddress("::", 3389); got != "127.0.0.1:3389" {
		t.Fatalf("healthcheckAddress(::) = %q, want 127.0.0.1:3389", got)
	}
}

func testHealthcheckConfig(dbPath, baseDN string) *config.Config {
	return &config.Config{
		Database: config.DatabaseConfig{Path: dbPath},
		LDAP:     config.LDAPConfig{BaseDN: baseDN},
		Server:   config.ServerConfig{BindAddress: "127.0.0.1", Port: 3389},
	}
}

func createHealthcheckDB(t *testing.T, dbPath, baseDN string, includeAllTables bool) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() failed: %v", err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE entries (id INTEGER PRIMARY KEY AUTOINCREMENT, dn TEXT UNIQUE NOT NULL, parent_dn TEXT, object_class TEXT NOT NULL, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	if includeAllTables {
		statements = append(statements,
			`CREATE TABLE attributes (id INTEGER PRIMARY KEY AUTOINCREMENT, entry_id INTEGER NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL)`,
			`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, entry_id INTEGER UNIQUE NOT NULL, password_hash TEXT)`,
			`CREATE TABLE groups (id INTEGER PRIMARY KEY AUTOINCREMENT, entry_id INTEGER UNIQUE NOT NULL)`,
			`CREATE TABLE group_members (group_entry_id INTEGER NOT NULL, member_entry_id INTEGER NOT NULL)`,
			`CREATE TABLE organizational_units (id INTEGER PRIMARY KEY AUTOINCREMENT, entry_id INTEGER UNIQUE NOT NULL)`,
		)
	}
	statements = append(statements, `INSERT INTO entries (dn, parent_dn, object_class) VALUES (?, '', 'top')`)

	for _, stmt := range statements[:len(statements)-1] {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("db.Exec(%q) failed: %v", stmt, err)
		}
	}
	if _, err := db.Exec(statements[len(statements)-1], baseDN); err != nil {
		t.Fatalf("insert base DN failed: %v", err)
	}
}

func startTestLDAPListener(t *testing.T) (net.Listener, int) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() failed: %v", err)
	}
	t.Cleanup(func() {
		listener.Close()
	})

	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	return listener, port
}

func unusedTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() failed: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close() failed: %v", err)
	}

	_, portString, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("net.SplitHostPort(%q) failed: %v", addr, err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatalf("strconv.Atoi(%q) failed: %v", portString, err)
	}
	return port
}
