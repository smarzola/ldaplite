package protocol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/smarzola/ldaplite/internal/audit"
	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
)

var nextConnectionID atomic.Uint64

// Connection represents an LDAP client connection
type Connection struct {
	conn     net.Conn
	id       string
	mu       sync.Mutex
	closed   bool
	bound    bool
	boundDN  string
	handlers OperationHandlers
}

// OperationHandlers defines callbacks for LDAP operations
type OperationHandlers struct {
	OnBind     func(context.Context, *Connection, *ldapmsg.Message) error
	OnSearch   func(context.Context, *Connection, *ldapmsg.Message) error
	OnAdd      func(context.Context, *Connection, *ldapmsg.Message) error
	OnModify   func(context.Context, *Connection, *ldapmsg.Message) error
	OnDelete   func(context.Context, *Connection, *ldapmsg.Message) error
	OnCompare  func(context.Context, *Connection, *ldapmsg.Message) error
	OnExtended func(context.Context, *Connection, *ldapmsg.Message) error
	OnUnbind   func(context.Context, *Connection, *ldapmsg.Message) error
}

// NewConnection creates a new LDAP connection wrapper
func NewConnection(conn net.Conn, handlers OperationHandlers) *Connection {
	return &Connection{
		conn:     conn,
		id:       "ldap-" + strconv.FormatUint(nextConnectionID.Add(1), 10),
		handlers: handlers,
	}
}

// Handle processes incoming LDAP messages in a loop
func (c *Connection) Handle(ctx context.Context) error {
	defer c.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read LDAP message from connection
		msg, err := ReadLDAPMessage(c.conn)
		if err != nil {
			// EOF is normal when client disconnects
			if errors.Is(err, io.EOF) {
				slog.Debug("Client disconnected", "remote", c.conn.RemoteAddr())
				return nil
			}
			slog.Error("Failed to read LDAP message", "error", err, "remote", c.conn.RemoteAddr())
			audit.LogLDAP(ctx, audit.LDAPEvent{
				Event:        audit.EventLDAPReadError,
				Operation:    "read",
				ConnectionID: c.ID(),
				RemoteAddr:   c.remoteAddrString(),
				ActorDN:      c.GetBoundDN(),
				ResultCode:   int(ldapmsg.ResultCodeProtocolError),
				Error:        err,
			})
			return err
		}

		if err := c.dispatch(ctx, msg); err != nil {
			slog.Error("Failed to handle LDAP operation", "error", err, "operation", fmt.Sprintf("%T", msg.Op))
			audit.LogLDAP(ctx, audit.LDAPEvent{
				Event:        audit.EventLDAPHandlerError,
				Operation:    OperationName(msg.Op),
				RequestID:    audit.RequestID(c.ID(), int(msg.ID)),
				ConnectionID: c.ID(),
				MessageID:    int(msg.ID),
				RemoteAddr:   c.remoteAddrString(),
				ActorDN:      c.GetBoundDN(),
				ResultCode:   int(ldapmsg.ResultCodeOperationsError),
				Error:        err,
			})
			// Continue processing other messages even if one fails
		}
	}
}

// dispatch routes the message to the appropriate handler
func (c *Connection) dispatch(ctx context.Context, msg *ldapmsg.Message) error {
	switch msg.Op.(type) {
	case ldapmsg.BindRequest:
		if c.handlers.OnBind != nil {
			return c.handlers.OnBind(ctx, c, msg)
		}

	case ldapmsg.SearchRequest:
		if c.handlers.OnSearch != nil {
			return c.handlers.OnSearch(ctx, c, msg)
		}

	case ldapmsg.AddRequest:
		if c.handlers.OnAdd != nil {
			return c.handlers.OnAdd(ctx, c, msg)
		}

	case ldapmsg.ModifyRequest:
		if c.handlers.OnModify != nil {
			return c.handlers.OnModify(ctx, c, msg)
		}

	case ldapmsg.DeleteRequest:
		if c.handlers.OnDelete != nil {
			return c.handlers.OnDelete(ctx, c, msg)
		}

	case ldapmsg.CompareRequest:
		if c.handlers.OnCompare != nil {
			return c.handlers.OnCompare(ctx, c, msg)
		}

	case ldapmsg.ExtendedRequest:
		if c.handlers.OnExtended != nil {
			return c.handlers.OnExtended(ctx, c, msg)
		}

	case ldapmsg.UnbindRequest:
		if c.handlers.OnUnbind != nil {
			return c.handlers.OnUnbind(ctx, c, msg)
		}
		return c.Close()

	default:
		slog.Warn("Unsupported LDAP operation", "operation", fmt.Sprintf("%T", msg.Op))
		audit.LogLDAP(ctx, audit.LDAPEvent{
			Operation:    "unsupported",
			RequestID:    audit.RequestID(c.ID(), int(msg.ID)),
			ConnectionID: c.ID(),
			MessageID:    int(msg.ID),
			RemoteAddr:   c.remoteAddrString(),
			ActorDN:      c.GetBoundDN(),
			ResultCode:   int(ldapmsg.ResultCodeProtocolError),
		})
		return c.WriteError(msg.ID, ldapmsg.ResultCodeProtocolError, "Unsupported operation")
	}

	return nil
}

// OperationName returns a stable lowercase operation name for an LDAP message operation.
func OperationName(op ldapmsg.Operation) string {
	switch op.(type) {
	case ldapmsg.BindRequest:
		return "bind"
	case ldapmsg.SearchRequest:
		return "search"
	case ldapmsg.AddRequest:
		return "add"
	case ldapmsg.ModifyRequest:
		return "modify"
	case ldapmsg.DeleteRequest:
		return "delete"
	case ldapmsg.CompareRequest:
		return "compare"
	case ldapmsg.ExtendedRequest:
		return "extended"
	case ldapmsg.UnbindRequest:
		return "unbind"
	default:
		return "unsupported"
	}
}

// WriteResponse writes an LDAP response message
func (c *Connection) WriteResponse(messageID ldapmsg.MessageID, response ldapmsg.Operation) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("connection closed")
	}

	return WriteLDAPResponse(c.conn, messageID, response)
}

// WriteError writes an error response
func (c *Connection) WriteError(messageID ldapmsg.MessageID, resultCode ldapmsg.ResultCode, diagnosticMessage string) error {
	// Create a generic error response using BindResponse structure
	resp := NewBindResponse(resultCode)
	resp.DiagnosticMessage = diagnosticMessage
	return c.WriteResponse(messageID, resp)
}

// RemoteAddr returns the remote address of the connection
func (c *Connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// RemoteAddrString returns the remote address string, or an empty string when
// the connection is not backed by a net.Conn.
func (c *Connection) RemoteAddrString() string {
	return c.remoteAddrString()
}

// ID returns the stable audit identifier for this LDAP connection.
func (c *Connection) ID() string {
	return c.id
}

// SetBoundDN marks this connection as bound after a successful bind. Anonymous
// binds are represented by bound=true with an empty DN.
func (c *Connection) SetBoundDN(dn string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bound = true
	c.boundDN = dn
}

// ClearBoundDN clears any previous bind state after a failed bind or unbind.
func (c *Connection) ClearBoundDN() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bound = false
	c.boundDN = ""
}

// IsBound reports whether this connection has completed a successful bind.
func (c *Connection) IsBound() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bound
}

// GetBoundDN returns the currently bound DN for this connection
func (c *Connection) GetBoundDN() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.boundDN
}

func (c *Connection) remoteAddrString() string {
	if c.conn == nil || c.conn.RemoteAddr() == nil {
		return ""
	}
	return c.conn.RemoteAddr().String()
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.conn.Close()
}
