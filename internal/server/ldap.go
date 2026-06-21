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

	"github.com/lor00x/goldap/message"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/protocol"
	"github.com/smarzola/ldaplite/internal/store"
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
	"modifytimestamp",
}

// modifyProtectedAttributes lists LDAP operational/structural attributes that
// cannot be changed after entry creation.
var modifyProtectedAttributes = []string{
	"createtimestamp",
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
func (s *Server) handleBind(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	conn.ClearBoundDN()

	bindReq := msg.ProtocolOp().(message.BindRequest)
	bindDN := string(bindReq.Name())
	password := string(bindReq.AuthenticationSimple())

	slog.Debug("Bind request", "dn", bindDN)

	// Handle anonymous bind
	if bindDN == "" || password == "" {
		if s.cfg.Security.AllowAnonymousBind {
			conn.SetBoundDN("") // Anonymous bind
			slog.Debug("Anonymous bind allowed")
			return conn.WriteResponse(msg.MessageID(), protocol.NewBindResponse(message.ResultCodeSuccess))
		}
		slog.Info("Anonymous bind rejected - not allowed by configuration")
		return conn.WriteResponse(msg.MessageID(), protocol.NewBindResponse(message.ResultCodeInvalidCredentials))
	}

	// Extract UID from the bind DN
	uid := extractUID(bindDN)
	if uid == "" {
		slog.Debug("Failed to extract UID from DN", "dn", bindDN)
		return conn.WriteResponse(msg.MessageID(), protocol.NewBindResponse(message.ResultCodeInvalidCredentials))
	}

	// Look up user by UID to get password hash and DN from database
	passwordHash, dn, err := s.store.GetUserPasswordHash(ctx, uid)
	if err != nil {
		slog.Debug("Error retrieving user", "uid", uid, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewBindResponse(message.ResultCodeInvalidCredentials))
	}

	if passwordHash == "" || dn == "" {
		slog.Debug("User not found", "uid", uid)
		return conn.WriteResponse(msg.MessageID(), protocol.NewBindResponse(message.ResultCodeInvalidCredentials))
	}

	// Validate that client's bind DN matches the DN in database
	if !dnEqual(bindDN, dn) {
		slog.Debug("Bind DN does not match database DN", "bind_dn", bindDN, "db_dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewBindResponse(message.ResultCodeInvalidCredentials))
	}

	// Verify password
	valid, err := s.hasher.Verify(password, passwordHash)
	if err != nil || !valid {
		slog.Debug("Password verification failed", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewBindResponse(message.ResultCodeInvalidCredentials))
	}

	// Bind successful - set the DN on the connection
	conn.SetBoundDN(dn)

	slog.Debug("Bind successful", "dn", dn)
	return conn.WriteResponse(msg.MessageID(), protocol.NewBindResponse(message.ResultCodeSuccess))
}

// handleSearch handles search operations
func (s *Server) handleSearch(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	searchReq := msg.ProtocolOp().(message.SearchRequest)
	baseDN := string(searchReq.BaseObject())
	scope := ldapSearchScope(searchReq.Scope())
	selection := newSearchAttributeSelection(searchReq.Attributes())

	// Handle RootDSE queries (empty base DN)
	if baseDN == "" {
		slog.Debug("RootDSE query")
		return s.handleRootDSE(conn, msg)
	}

	// Handle schema queries
	if baseDN == "cn=Subschema" || baseDN == "cn=subschema" {
		slog.Debug("Schema query")
		return s.handleSchema(conn, msg)
	}

	if !s.canSearch(conn, baseDN) {
		slog.Info("Search rejected - bind required", "baseDN", baseDN)
		return conn.WriteResponse(msg.MessageID(), protocol.NewSearchResultDone(message.ResultCodeInsufficientAccessRights))
	}

	// Get filter from request
	filterStr := serializeFilter(searchReq.Filter())
	if filterStr == "" {
		filterStr = "(objectClass=*)"
	}

	slog.Debug("Search request", "baseDN", baseDN, "scope", scope, "filter", filterStr)

	entries, err := s.store.SearchEntriesWithOptions(ctx, store.SearchOptions{
		BaseDN:          baseDN,
		Filter:          filterStr,
		Scope:           scope,
		IncludeMemberOf: selection.includes("memberOf"),
	})
	if err != nil {
		slog.Error("Search error", "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewSearchResultDone(message.ResultCodeOperationsError))
	}

	// Return matching entries
	for _, entry := range entries {
		// Build search result entry
		result := protocol.NewSearchResultEntry(entry.DN)

		for _, attr := range searchResponseAttributes(entry, selection) {
			addSearchAttribute(&result, attr.name, attr.values, bool(searchReq.TypesOnly()))
		}

		// Write entry
		if err := conn.WriteResponse(msg.MessageID(), result); err != nil {
			return err
		}
	}

	slog.Debug("Search completed", "baseDN", baseDN, "results", len(entries))
	return conn.WriteResponse(msg.MessageID(), protocol.NewSearchResultDone(message.ResultCodeSuccess))
}

func ldapSearchScope(scope message.ENUMERATED) store.SearchScope {
	switch int(scope) {
	case 0:
		return store.SearchScopeBaseObject
	case 1:
		return store.SearchScopeSingleLevel
	default:
		return store.SearchScopeWholeSubtree
	}
}

// handleAdd handles add operations
func (s *Server) handleAdd(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	addReq := msg.ProtocolOp().(message.AddRequest)

	dn := string(addReq.Entry())
	slog.Debug("Add request", "dn", dn)

	if !s.canWrite(conn) {
		slog.Info("Add rejected - authenticated bind required", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeInsufficientAccessRights))
	}

	// Check if entry already exists
	exists, err := s.store.EntryExists(ctx, dn)
	if err != nil {
		slog.Error("Failed to check entry existence", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeOperationsError))
	}

	if exists {
		slog.Debug("Entry already exists", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeEntryAlreadyExists))
	}

	// Extract parent DN
	parentDN := ""
	if idx := findFirstComma(dn); idx >= 0 && idx+1 < len(dn) {
		parentDN = dn[idx+1:]
	}

	// Create new entry
	entry := &models.Entry{
		DN:         dn,
		ParentDN:   parentDN,
		Attributes: make(map[string][]string),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Parse attributes
	attrs := addReq.Attributes()
	for _, attr := range attrs {
		name := string(attr.Type_())

		// Check protected attributes
		if isAddProtectedAttribute(name) {
			slog.Debug("Attempt to set protected attribute", "dn", dn, "attribute", name)
			return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeUnwillingToPerform))
		}

		values := attr.Vals()
		for _, val := range values {
			entry.AddAttribute(name, string(val))
		}
	}

	// Process userPassword
	if userPassword := entry.GetAttribute("userPassword"); userPassword != "" {
		processedPassword, err := s.hasher.ProcessPassword(userPassword)
		if err != nil {
			slog.Debug("Invalid password format", "dn", dn, "error", err)
			return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeConstraintViolation))
		}
		entry.SetAttribute("userPassword", processedPassword)
	}

	// Determine object class
	objectClasses := entry.GetAttribute("objectClass")
	if objectClasses == "" {
		slog.Debug("No objectClass provided", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeObjectClassViolation))
	}

	entry.ObjectClass = objectClasses
	allClasses := entry.GetAttributes("objectClass")
	if len(allClasses) > 0 {
		entry.ObjectClass = allClasses[0]
	}

	slog.Debug("Creating entry", "dn", dn, "objectClass", objectClasses)

	// Store entry
	if err := s.store.CreateEntry(ctx, entry); err != nil {
		slog.Error("Failed to create entry", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(entryWriteResultCode(err)))
	}

	slog.Info("Entry created", "dn", dn)
	return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeSuccess))
}

// handleDelete handles delete operations
func (s *Server) handleDelete(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	delReq := msg.ProtocolOp().(message.DelRequest)

	dn := string(delReq)
	slog.Debug("Delete request", "dn", dn)

	if !s.canWrite(conn) {
		slog.Info("Delete rejected - authenticated bind required", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewDelResponse(message.ResultCodeInsufficientAccessRights))
	}

	// Check if entry exists
	exists, err := s.store.EntryExists(ctx, dn)
	if err != nil {
		slog.Error("Failed to check entry existence", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewDelResponse(message.ResultCodeOperationsError))
	}

	if !exists {
		slog.Debug("Entry not found", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewDelResponse(message.ResultCodeNoSuchObject))
	}

	if err := s.store.DeleteEntry(ctx, dn); err != nil {
		slog.Error("Failed to delete entry", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewDelResponse(message.ResultCodeOperationsError))
	}

	slog.Info("Entry deleted", "dn", dn)
	return conn.WriteResponse(msg.MessageID(), protocol.NewDelResponse(message.ResultCodeSuccess))
}

// handleModify handles modify operations
func (s *Server) handleModify(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	modReq := msg.ProtocolOp().(message.ModifyRequest)

	dn := string(modReq.Object())
	slog.Debug("Modify request", "dn", dn)

	if !s.canWrite(conn) {
		slog.Info("Modify rejected - authenticated bind required", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeInsufficientAccessRights))
	}

	// Get entry
	entry, err := s.store.GetEntry(ctx, dn)
	if err != nil {
		slog.Error("Failed to get entry", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeOperationsError))
	}

	if entry == nil {
		slog.Debug("Entry not found", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeNoSuchObject))
	}

	// Apply modifications
	changes := modReq.Changes()
	for _, change := range changes {
		modification := change.Modification()
		attrType := string(modification.Type_())

		// Check protected attributes
		if isModifyProtectedAttribute(attrType) {
			slog.Debug("Attempt to modify protected attribute", "dn", dn, "attribute", attrType)
			return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeUnwillingToPerform))
		}

		vals := modification.Vals()
		opType := int(change.Operation())

		switch opType {
		case 0: // Add
			slog.Debug("Add attribute", "attr", attrType)
			if attrType == "userPassword" {
				for _, val := range vals {
					processedPassword, err := s.hasher.ProcessPassword(string(val))
					if err != nil {
						slog.Debug("Invalid password format", "dn", dn, "error", err)
						return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeConstraintViolation))
					}
					entry.AddAttribute(attrType, processedPassword)
				}
			} else {
				for _, val := range vals {
					entry.AddAttribute(attrType, string(val))
				}
			}

		case 1: // Delete
			slog.Debug("Delete attribute", "attr", attrType)
			if len(vals) == 0 {
				entry.RemoveAttribute(attrType)
			} else {
				existing := entry.GetAttributes(attrType)
				newVals := []string{}
				for _, v := range existing {
					shouldKeep := true
					for _, val := range vals {
						if v == string(val) {
							shouldKeep = false
							break
						}
					}
					if shouldKeep {
						newVals = append(newVals, v)
					}
				}
				if len(newVals) == 0 {
					entry.RemoveAttribute(attrType)
				} else {
					entry.RemoveAttribute(attrType)
					for _, v := range newVals {
						entry.AddAttribute(attrType, v)
					}
				}
			}

		case 2: // Replace
			slog.Debug("Replace attribute", "attr", attrType)
			entry.RemoveAttribute(attrType)
			if attrType == "userPassword" {
				for _, val := range vals {
					processedPassword, err := s.hasher.ProcessPassword(string(val))
					if err != nil {
						slog.Debug("Invalid password format", "dn", dn, "error", err)
						return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeConstraintViolation))
					}
					entry.AddAttribute(attrType, processedPassword)
				}
			} else {
				for _, val := range vals {
					entry.AddAttribute(attrType, string(val))
				}
			}
		}
	}

	// Update entry
	if err := s.store.UpdateEntry(ctx, entry); err != nil {
		slog.Error("Failed to update entry", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(entryWriteResultCode(err)))
	}

	slog.Info("Entry modified", "dn", dn)
	return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeSuccess))
}

// handleRootDSE handles RootDSE queries
func (s *Server) handleRootDSE(conn *protocol.Connection, msg *message.LDAPMessage) error {
	entry := protocol.NewSearchResultEntry("")
	protocol.AddAttribute(&entry, "objectClass", "top")
	protocol.AddAttribute(&entry, "namingContexts", s.cfg.LDAP.BaseDN)
	protocol.AddAttribute(&entry, "subschemaSubentry", "cn=Subschema")
	protocol.AddAttribute(&entry, "supportedLDAPVersion", "3")
	protocol.AddAttribute(&entry, "vendorName", "LDAPLite")
	protocol.AddAttribute(&entry, "vendorVersion", s.version)

	if err := conn.WriteResponse(msg.MessageID(), entry); err != nil {
		return err
	}
	return conn.WriteResponse(msg.MessageID(), protocol.NewSearchResultDone(message.ResultCodeSuccess))
}

// handleSchema handles schema queries
func (s *Server) handleSchema(conn *protocol.Connection, msg *message.LDAPMessage) error {
	entry := protocol.NewSearchResultEntry("cn=Subschema")
	protocol.AddAttribute(&entry, "objectClass", "top", "subschema")
	protocol.AddAttribute(&entry, "objectClasses",
		"( 2.5.6.0 NAME 'top' DESC 'top of the superclass chain' ABSTRACT MUST objectClass )",
		"( 2.5.6.6 NAME 'person' DESC 'RFC2256: a person' SUP top STRUCTURAL MUST ( sn $ cn ) MAY ( userPassword $ telephoneNumber $ seeAlso $ description ) )",
		"( 2.5.6.7 NAME 'organizationalPerson' DESC 'RFC2256: an organizational person' SUP person STRUCTURAL MAY ( title $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ ou $ st $ l ) )",
		"( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' DESC 'RFC2798: Internet Organizational Person' SUP organizationalPerson STRUCTURAL MAY ( audio $ businessCategory $ carLicense $ departmentNumber $ displayName $ employeeNumber $ employeeType $ givenName $ homePhone $ homePostalAddress $ initials $ jpegPhoto $ labeledURI $ mail $ manager $ mobile $ o $ pager $ photo $ roomNumber $ secretary $ uid $ userCertificate $ x500uniqueIdentifier $ preferredLanguage $ userSMIMECertificate $ userPKCS12 ) )",
		"( 2.5.6.9 NAME 'groupOfNames' DESC 'RFC2256: a group of names (DNs)' SUP top STRUCTURAL MUST ( member $ cn ) MAY ( businessCategory $ seeAlso $ owner $ ou $ o $ description ) )",
		"( 2.5.6.5 NAME 'organizationalUnit' DESC 'RFC2256: an organizational unit' SUP top STRUCTURAL MUST ou MAY ( userPassword $ searchGuide $ seeAlso $ businessCategory $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ st $ l $ description ) )",
	)
	protocol.AddAttribute(&entry, "attributeTypes",
		"( 2.5.4.0 NAME 'objectClass' DESC 'RFC2256: object classes of the entity' EQUALITY objectIdentifierMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.38 )",
		"( 2.5.4.41 NAME 'name' DESC 'RFC2256: common supertype of name attributes' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{32768} )",
		"( 2.5.4.3 NAME 'cn' SUP name DESC 'RFC2256: common name(s) for which the entity is known by' )",
		"( 2.5.4.4 NAME 'sn' SUP name DESC 'RFC2256: last (family) name(s) for which the entity is known by' )",
		"( 0.9.2342.19200300.100.1.1 NAME 'uid' DESC 'RFC1274: user identifier' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{256} )",
		"( 0.9.2342.19200300.100.1.3 NAME 'mail' DESC 'RFC1274: RFC822 Mailbox' EQUALITY caseIgnoreIA5Match SUBSTR caseIgnoreIA5SubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.26{256} )",
		"( 2.5.4.35 NAME 'userPassword' DESC 'RFC2256/2307: password of user' EQUALITY octetStringMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.40{128} )",
		"( 2.5.4.31 NAME 'member' DESC 'RFC2256: member of a group' SUP distinguishedName )",
		"( 2.5.4.11 NAME 'ou' SUP name DESC 'RFC2256: organizational unit this object belongs to' )",
		"( 1.2.840.113556.1.2.102 NAME 'memberOf' DESC 'RFC2307bis: Groups to which the entry belongs' EQUALITY distinguishedNameMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 NO-USER-MODIFICATION USAGE directoryOperation )",
	)

	if err := conn.WriteResponse(msg.MessageID(), entry); err != nil {
		return err
	}
	return conn.WriteResponse(msg.MessageID(), protocol.NewSearchResultDone(message.ResultCodeSuccess))
}

// handleCompare handles compare operations
func (s *Server) handleCompare(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	slog.Debug("Compare request")
	return conn.WriteResponse(msg.MessageID(), protocol.NewCompareResponse(message.ResultCodeCompareFalse))
}

// handleExtended handles extended operations
func (s *Server) handleExtended(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	extReq := msg.ProtocolOp().(message.ExtendedRequest)
	reqOID := string(extReq.RequestName())

	slog.Debug("Extended request", "oid", reqOID)

	// Handle "Who am I?" extended operation (RFC 4532)
	// OID: 1.3.6.1.4.1.4203.1.11.3
	if reqOID == protocol.WhoAmIOID {
		boundDN := conn.GetBoundDN()

		// Set the response value to the bound DN
		// RFC 4532 specifies the format as "dn:<distinguished-name>" or empty for anonymous
		var authzID string
		if boundDN == "" {
			authzID = "" // Anonymous
		} else {
			authzID = "dn:" + boundDN
		}

		resp, err := protocol.NewWhoAmIResponse(authzID)
		if err != nil {
			slog.Error("Failed to build Who am I response", "error", err)
			return conn.WriteResponse(msg.MessageID(), protocol.NewExtendedResponse(message.ResultCodeOperationsError))
		}

		slog.Debug("Who am I response", "authzID", authzID)
		return conn.WriteResponse(msg.MessageID(), resp)
	}

	// Unsupported extended operation
	slog.Debug("Unsupported extended operation", "oid", reqOID)
	return conn.WriteResponse(msg.MessageID(), protocol.NewExtendedResponse(message.ResultCodeUnavailable))
}

// handleUnbind handles unbind operations
func (s *Server) handleUnbind(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	slog.Debug("Unbind request")
	conn.ClearBoundDN()
	return nil // Connection will be closed by handler
}

func (s *Server) canSearch(conn *protocol.Connection, baseDN string) bool {
	if isPublicSearchBase(baseDN) {
		return true
	}
	if !conn.IsBound() {
		return false
	}
	if conn.GetBoundDN() == "" {
		return s.cfg.Security.AllowAnonymousBind
	}
	return true
}

func (s *Server) canWrite(conn *protocol.Connection) bool {
	return conn.IsBound() && conn.GetBoundDN() != ""
}

func isPublicSearchBase(baseDN string) bool {
	return baseDN == "" || strings.EqualFold(baseDN, "cn=Subschema")
}

type searchAttributeSelection struct {
	noAttributes       bool
	includeAll         bool
	includeOperational bool
	names              map[string]bool
}

type searchResponseAttribute struct {
	name   string
	values []string
}

func newSearchAttributeSelection(selection message.AttributeSelection) searchAttributeSelection {
	if len(selection) == 0 {
		return searchAttributeSelection{includeAll: true, includeOperational: true}
	}

	result := searchAttributeSelection{names: make(map[string]bool)}
	for _, selector := range selection {
		name := strings.TrimSpace(string(selector))
		if name == "" {
			continue
		}

		switch strings.ToLower(name) {
		case "1.1":
			result.noAttributes = true
		case "*":
			result.includeAll = true
			result.noAttributes = false
		case "+":
			result.includeOperational = true
			result.noAttributes = false
		default:
			result.names[strings.ToLower(name)] = true
			result.noAttributes = false
		}
	}

	return result
}

func (s searchAttributeSelection) includes(attrName string) bool {
	if s.noAttributes {
		return false
	}
	attrLower := strings.ToLower(attrName)
	if s.names[attrLower] {
		return true
	}
	if s.includeOperational && isOperationalAttribute(attrLower) {
		return true
	}
	return s.includeAll && !isOperationalAttribute(attrLower)
}

func isOperationalAttribute(attrName string) bool {
	switch strings.ToLower(attrName) {
	case "createtimestamp", "modifytimestamp", "memberof":
		return true
	default:
		return false
	}
}

func searchResponseAttributes(entry *models.Entry, selection searchAttributeSelection) []searchResponseAttribute {
	attrs := make([]searchResponseAttribute, 0, len(entry.Attributes)+1)
	if entry.ObjectClass != "" && selection.includes("objectClass") {
		attrs = append(attrs, searchResponseAttribute{
			name:   "objectClass",
			values: []string{entry.ObjectClass},
		})
	}

	for attrName, attrValues := range entry.Attributes {
		if strings.EqualFold(attrName, "objectClass") {
			continue
		}
		if selection.includes(attrName) {
			attrs = append(attrs, searchResponseAttribute{
				name:   attrName,
				values: attrValues,
			})
		}
	}

	return attrs
}

func addSearchAttribute(result *message.SearchResultEntry, name string, values []string, typesOnly bool) {
	if typesOnly {
		protocol.AddAttribute(result, name)
		return
	}
	protocol.AddAttribute(result, name, values...)
}

func entryWriteResultCode(err error) int {
	if err == nil {
		return message.ResultCodeSuccess
	}

	if errors.Is(err, store.ErrEntryAlreadyExists) {
		return message.ResultCodeEntryAlreadyExists
	}
	if errors.Is(err, store.ErrNoSuchObject) {
		return message.ResultCodeNoSuchObject
	}
	if errors.Is(err, store.ErrObjectClassViolation) {
		return message.ResultCodeObjectClassViolation
	}
	if errors.Is(err, store.ErrConstraintViolation) {
		return message.ResultCodeConstraintViolation
	}

	return message.ResultCodeOperationsError
}

// extractUID extracts UID from a DN
func extractUID(dn string) string {
	if len(dn) > 4 && dn[:4] == "uid=" {
		for i, c := range dn[4:] {
			if c == ',' {
				return dn[4 : 4+i]
			}
		}
		return dn[4:]
	}
	return ""
}

// dnEqual compares two DNs for equality (case-insensitive)
func dnEqual(dn1, dn2 string) bool {
	return strings.EqualFold(strings.TrimSpace(dn1), strings.TrimSpace(dn2))
}

// serializeFilter converts a goldap Filter to LDAP filter string
func serializeFilter(f interface{}) string {
	if f == nil {
		return ""
	}

	switch filter := f.(type) {
	case message.FilterEqualityMatch:
		return fmt.Sprintf("(%s=%s)", filter.AttributeDesc(), filter.AssertionValue())

	case message.FilterPresent:
		return fmt.Sprintf("(%s=*)", string(filter))

	case message.FilterAnd:
		if len(filter) == 0 {
			return ""
		}
		var parts []string
		for _, subFilter := range filter {
			parts = append(parts, serializeFilter(subFilter))
		}
		return "(&" + strings.Join(parts, "") + ")"

	case message.FilterOr:
		if len(filter) == 0 {
			return ""
		}
		var parts []string
		for _, subFilter := range filter {
			parts = append(parts, serializeFilter(subFilter))
		}
		return "(|" + strings.Join(parts, "") + ")"

	case message.FilterNot:
		return "(!" + serializeFilter(filter.Filter) + ")"

	case message.FilterGreaterOrEqual:
		return fmt.Sprintf("(%s>=%s)", filter.AttributeDesc(), filter.AssertionValue())

	case message.FilterLessOrEqual:
		return fmt.Sprintf("(%s<=%s)", filter.AttributeDesc(), filter.AssertionValue())

	case message.FilterApproxMatch:
		return fmt.Sprintf("(%s~=%s)", filter.AttributeDesc(), filter.AssertionValue())

	case message.FilterSubstrings:
		attr := string(filter.Type_())
		var sb strings.Builder
		sb.WriteString("(")
		sb.WriteString(attr)
		sb.WriteString("=")

		for _, sub := range filter.Substrings() {
			switch s := sub.(type) {
			case message.SubstringInitial:
				sb.WriteString(string(s))
				sb.WriteString("*")
			case message.SubstringAny:
				sb.WriteString(string(s))
				sb.WriteString("*")
			case message.SubstringFinal:
				sb.WriteString(string(s))
			}
		}
		sb.WriteString(")")
		return sb.String()

	default:
		str := fmt.Sprintf("%v", f)
		if str != "" && str[0] == '(' {
			return str
		}
		return "(objectClass=*)"
	}
}

// findFirstComma finds the first unescaped comma in a DN string
func findFirstComma(dn string) int {
	for i := 0; i < len(dn); i++ {
		if dn[i] == ',' {
			if i > 0 && dn[i-1] == '\\' {
				continue
			}
			return i
		}
	}
	return -1
}
