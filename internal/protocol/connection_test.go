package protocol

import (
	"context"
	"testing"

	"github.com/lor00x/goldap/message"
)

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

func TestDispatchPassesContextToHandler(t *testing.T) {
	type contextKey string
	key := contextKey("request-id")
	ctx := context.WithValue(context.Background(), key, "abc123")

	var got string
	conn := NewConnection(nil, OperationHandlers{
		OnBind: func(ctx context.Context, conn *Connection, msg *message.LDAPMessage) error {
			got, _ = ctx.Value(key).(string)
			return nil
		},
	})

	msg := message.NewLDAPMessageWithProtocolOp(message.BindRequest{})
	if err := conn.dispatch(ctx, msg); err != nil {
		t.Fatalf("dispatch() failed: %v", err)
	}
	if got != "abc123" {
		t.Fatalf("handler context value = %q, want abc123", got)
	}
}
