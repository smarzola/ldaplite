package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/smarzola/ldaplite/internal/audit"
	"github.com/smarzola/ldaplite/internal/protocol"
	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/internal/telemetry"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

// Server represents an LDAP server
type Server struct {
	cfg      *config.Config
	store    store.Store
	hasher   *crypto.PasswordHasher
	version  string
	listener net.Listener
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewServer creates a new LDAP server
func NewServer(cfg *config.Config, st store.Store, version string) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		cfg:     cfg,
		store:   st,
		hasher:  crypto.NewPasswordHasher(cfg.Security.Argon2Config),
		version: version,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// addProtectedAttributes lists LDAP operational attributes clients cannot set
// during Add. objectClass is structural and required during Add, so it is only
// protected from later Modify operations.
var addProtectedAttributes = []string{
	"createtimestamp",
	"memberof",
	"modifytimestamp",
}

// modifyProtectedAttributes lists LDAP operational/structural attributes that
// cannot be changed after entry creation.
var modifyProtectedAttributes = []string{
	"createtimestamp",
	"memberof",
	"modifytimestamp",
	"objectclass",
}

func isAddProtectedAttribute(attrName string) bool {
	return containsAttributeName(addProtectedAttributes, attrName)
}

func isModifyProtectedAttribute(attrName string) bool {
	return containsAttributeName(modifyProtectedAttributes, attrName)
}

func containsAttributeName(names []string, attrName string) bool {
	attrLower := strings.ToLower(attrName)
	for _, name := range names {
		if attrLower == name {
			return true
		}
	}
	return false
}

// Start starts the LDAP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.BindAddress, s.cfg.Server.Port)

	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}

	slog.Info("LDAP server starting", "address", addr)

	// Accept connections in a goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop()
	}()

	return nil
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				// Server is shutting down
				return
			default:
				slog.Error("Failed to accept connection", "error", err)
				continue
			}
		}

		slog.Debug("New connection", "remote", conn.RemoteAddr())
		telemetry.RecordLDAPConnectionAccepted(s.ctx)

		// Handle connection in a separate goroutine
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn)
		}()
	}
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	telemetry.AddActiveLDAPConnection(1)
	defer telemetry.AddActiveLDAPConnection(-1)

	done := make(chan struct{})
	go func() {
		select {
		case <-s.ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	defer close(done)

	// Create operation handlers
	handlers := protocol.OperationHandlers{
		OnBind:     s.handleBind,
		OnSearch:   s.handleSearch,
		OnAdd:      s.handleAdd,
		OnModify:   s.handleModify,
		OnDelete:   s.handleDelete,
		OnCompare:  s.handleCompare,
		OnExtended: s.handleExtended,
		OnUnbind:   s.handleUnbind,
	}

	// Create connection wrapper
	ldapConn := protocol.NewConnection(conn, handlers)

	// Handle the connection
	if err := ldapConn.Handle(s.ctx); err != nil {
		if err != context.Canceled {
			slog.Debug("Connection closed", "remote", conn.RemoteAddr(), "error", err)
		}
	}
}

// Stop stops the LDAP server
func (s *Server) Stop() error {
	if s.listener != nil {
		s.cancel()         // Cancel context to stop accept loop
		s.listener.Close() // Close listener to unblock Accept()
		s.wg.Wait()        // Wait for all connections to close
	}
	return nil
}

// handleBind handles bind operations
func (s *Server) handleBind(ctx context.Context, conn *protocol.Connection, msg *ldapmsg.Message) error {
	start := time.Now()
	resultCode := ldapmsg.ResultCodeOperationsError
	targetDN := ""
	defer func() {
		actorDN := ""
		if resultCode == ldapmsg.ResultCodeSuccess {
			actorDN = conn.GetBoundDN()
		}
		s.auditLDAPOperation(ctx, conn, msg, "bind", audit.LDAPEvent{
			ActorDN:    actorDN,
			TargetDN:   targetDN,
			ResultCode: int(resultCode),
			Duration:   time.Since(start),
		})
	}()

	conn.ClearBoundDN()

	bindReq := msg.Op.(ldapmsg.BindRequest)
	bindDN := bindReq.Name
	password := bindReq.Password
	targetDN = bindDN

	slog.Debug("Bind request", "dn", bindDN)

	// Handle anonymous bind
	if bindDN == "" || password == "" {
		if s.cfg.Security.AllowAnonymousBind {
			conn.SetBoundDN("") // Anonymous bind
			slog.Debug("Anonymous bind allowed")
			resultCode = ldapmsg.ResultCodeSuccess
			return conn.WriteResponse(msg.ID, protocol.NewBindResponse(ldapmsg.ResultCodeSuccess))
		}
		slog.Info("Anonymous bind rejected - not allowed by configuration")
		resultCode = ldapmsg.ResultCodeInvalidCredentials
		return conn.WriteResponse(msg.ID, protocol.NewBindResponse(ldapmsg.ResultCodeInvalidCredentials))
	}

	// Look up user by bind DN to get password hash and canonical DN from database
	passwordHash, dn, err := s.store.GetUserPasswordHashByDN(ctx, bindDN)
	if err != nil {
		slog.Debug("Error retrieving user", "dn", bindDN, "error", err)
		resultCode = ldapmsg.ResultCodeInvalidCredentials
		return conn.WriteResponse(msg.ID, protocol.NewBindResponse(ldapmsg.ResultCodeInvalidCredentials))
	}

	if passwordHash == "" || dn == "" {
		slog.Debug("User not found", "dn", bindDN)
		resultCode = ldapmsg.ResultCodeInvalidCredentials
		return conn.WriteResponse(msg.ID, protocol.NewBindResponse(ldapmsg.ResultCodeInvalidCredentials))
	}

	// Verify password
	valid, err := s.hasher.Verify(password, passwordHash)
	if err != nil || !valid {
		slog.Debug("Password verification failed", "dn", dn)
		targetDN = dn
		resultCode = ldapmsg.ResultCodeInvalidCredentials
		return conn.WriteResponse(msg.ID, protocol.NewBindResponse(ldapmsg.ResultCodeInvalidCredentials))
	}

	// Bind successful - set the DN on the connection
	conn.SetBoundDN(dn)

	slog.Debug("Bind successful", "dn", dn)
	targetDN = dn
	resultCode = ldapmsg.ResultCodeSuccess
	return conn.WriteResponse(msg.ID, protocol.NewBindResponse(ldapmsg.ResultCodeSuccess))
}

// handleCompare handles compare operations
func (s *Server) handleCompare(ctx context.Context, conn *protocol.Connection, msg *ldapmsg.Message) error {
	start := time.Now()
	compareReq := msg.Op.(ldapmsg.CompareRequest)
	resultCode := ldapmsg.ResultCodeCompareFalse
	defer func() {
		s.auditLDAPOperation(ctx, conn, msg, "compare", audit.LDAPEvent{
			ActorDN:    conn.GetBoundDN(),
			TargetDN:   compareReq.Entry,
			ResultCode: int(resultCode),
			Duration:   time.Since(start),
		})
	}()

	slog.Debug("Compare request")
	return conn.WriteResponse(msg.ID, protocol.NewCompareResponse(ldapmsg.ResultCodeCompareFalse))
}

// handleUnbind handles unbind operations
func (s *Server) handleUnbind(ctx context.Context, conn *protocol.Connection, msg *ldapmsg.Message) error {
	start := time.Now()
	actorDN := conn.GetBoundDN()
	defer func() {
		s.auditLDAPOperation(ctx, conn, msg, "unbind", audit.LDAPEvent{
			ActorDN:    actorDN,
			ResultCode: int(ldapmsg.ResultCodeSuccess),
			Duration:   time.Since(start),
		})
	}()

	slog.Debug("Unbind request")
	conn.ClearBoundDN()
	return nil // Connection will be closed by handler
}

func (s *Server) auditLDAPOperation(ctx context.Context, conn *protocol.Connection, msg *ldapmsg.Message, operation string, event audit.LDAPEvent) {
	event.Operation = operation
	event.RequestID = audit.RequestID(conn.ID(), int(msg.ID))
	event.ConnectionID = conn.ID()
	event.MessageID = int(msg.ID)
	event.RemoteAddr = conn.RemoteAddrString()
	audit.LogLDAP(ctx, event)
	telemetry.RecordLDAPOperation(ctx, operation, event.ResultCode, event.Duration)
}

func (s *Server) canWrite(conn *protocol.Connection) bool {
	return conn.IsBound() && conn.GetBoundDN() != ""
}

func entryWriteResultCode(err error) ldapmsg.ResultCode {
	if err == nil {
		return ldapmsg.ResultCodeSuccess
	}

	if errors.Is(err, store.ErrEntryAlreadyExists) {
		return ldapmsg.ResultCodeEntryAlreadyExists
	}
	if errors.Is(err, store.ErrNoSuchObject) {
		return ldapmsg.ResultCodeNoSuchObject
	}
	if errors.Is(err, store.ErrObjectClassViolation) {
		return ldapmsg.ResultCodeObjectClassViolation
	}
	if errors.Is(err, store.ErrConstraintViolation) {
		return ldapmsg.ResultCodeConstraintViolation
	}

	return ldapmsg.ResultCodeOperationsError
}
