package protocol

import "testing"

func TestConnectionBoundState(t *testing.T) {
	conn := NewConnection(nil, OperationHandlers{})

	if conn.IsBound() {
		t.Fatal("new connection should not be bound")
	}
	if got := conn.GetBoundDN(); got != "" {
		t.Fatalf("new connection bound DN = %q, want empty", got)
	}

	conn.SetBoundDN("")
	if !conn.IsBound() {
		t.Fatal("anonymous bind should mark connection as bound")
	}
	if got := conn.GetBoundDN(); got != "" {
		t.Fatalf("anonymous bind DN = %q, want empty", got)
	}

	conn.SetBoundDN("uid=admin,ou=users,dc=example,dc=com")
	if !conn.IsBound() {
		t.Fatal("authenticated bind should mark connection as bound")
	}
	if got := conn.GetBoundDN(); got != "uid=admin,ou=users,dc=example,dc=com" {
		t.Fatalf("authenticated bind DN = %q", got)
	}

	conn.ClearBoundDN()
	if conn.IsBound() {
		t.Fatal("cleared connection should not be bound")
	}
	if got := conn.GetBoundDN(); got != "" {
		t.Fatalf("cleared bound DN = %q, want empty", got)
	}
}
