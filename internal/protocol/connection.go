package protocol

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/lor00x/goldap/message"
)

// Connection represents an LDAP client connection
type Connection struct {
	conn     net.Conn
	mu       sync.Mutex
	closed   bool
	boundDN  string
	handlers OperationHandlers
}

// OperationHandlers defines callbacks for LDAP operations
type OperationHandlers struct {
	OnBind     func(*Connection, *message.LDAPMessage) error
	OnSearch   func(*Connection, *message.LDAPMessage) error
	OnAdd      func(*Connection, *message.LDAPMessage) error
	OnModify   func(*Connection, *message.LDAPMessage) error
	OnDelete   func(*Connection, *message.LDAPMessage) error
	OnCompare  func(*Connection, *message.LDAPMessage) error
	OnExtended func(*Connection, *message.LDAPMessage) error
	OnUnbind   func(*Connection, *message.LDAPMessage) error
}

// NewConnection creates a new LDAP connection wrapper
func NewConnection(conn net.Conn, handlers OperationHandlers) *Connection {
	return &Connection{
		conn:     conn,
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
			if err.Error() == "EOF" {
				slog.Debug("Client disconnected", "remote", c.conn.RemoteAddr())
				return nil
			}
			slog.Error("Failed to read LDAP message", "error", err, "remote", c.conn.RemoteAddr())
			return err
		}

		// Dispatch to appropriate handler based on operation type
		if err := c.dispatch(msg); err != nil {
			slog.Error("Failed to handle LDAP operation", "error", err, "operation", msg.ProtocolOpName())
			// Continue processing other messages even if one fails
		}
	}
}

// dispatch routes the message to the appropriate handler
func (c *Connection) dispatch(msg *message.LDAPMessage) error {
	switch msg.ProtocolOp().(type) {
	case message.BindRequest:
		if c.handlers.OnBind != nil {
			return c.handlers.OnBind(c, msg)
		}

	case message.SearchRequest:
		if c.handlers.OnSearch != nil {
			return c.handlers.OnSearch(c, msg)
		}

	case message.AddRequest:
		if c.handlers.OnAdd != nil {
			return c.handlers.OnAdd(c, msg)
		}

	case message.ModifyRequest:
		if c.handlers.OnModify != nil {
			return c.handlers.OnModify(c, msg)
		}

	case message.DelRequest:
		if c.handlers.OnDelete != nil {
			return c.handlers.OnDelete(c, msg)
		}

	case message.CompareRequest:
		if c.handlers.OnCompare != nil {
			return c.handlers.OnCompare(c, msg)
		}

	case message.ExtendedRequest:
		if c.handlers.OnExtended != nil {
			return c.handlers.OnExtended(c, msg)
		}

	case message.UnbindRequest:
		if c.handlers.OnUnbind != nil {
			return c.handlers.OnUnbind(c, msg)
		}
		return c.Close()

	default:
		slog.Warn("Unsupported LDAP operation", "operation", msg.ProtocolOpName())
		return c.WriteError(msg.MessageID(), message.ResultCodeProtocolError, "Unsupported operation")
	}

	return nil
}

// WriteResponse writes an LDAP response message
func (c *Connection) WriteResponse(messageID message.MessageID, response message.ProtocolOp) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("connection closed")
	}

	msg := message.NewLDAPMessageWithProtocolOp(response)
	msg.SetMessageID(int(messageID))

	return WriteLDAPMessage(c.conn, msg)
}

// WriteError writes an error response
func (c *Connection) WriteError(messageID message.MessageID, resultCode int, diagnosticMessage string) error {
	// Create a generic error response using BindResponse structure
	resp := NewBindResponse(resultCode)
	resp.SetDiagnosticMessage(diagnosticMessage)
	return c.WriteResponse(messageID, resp)
}

// RemoteAddr returns the remote address of the connection
func (c *Connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// SetBoundDN sets the bound DN for this connection after successful authentication
func (c *Connection) SetBoundDN(dn string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.boundDN = dn
}

// GetBoundDN returns the currently bound DN for this connection
func (c *Connection) GetBoundDN() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.boundDN
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
