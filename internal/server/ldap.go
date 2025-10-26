package server

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/lor00x/goldap/message"
	"github.com/vjeantet/ldapserver"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

// NullWriter discards all writes
type NullWriter struct{}

func (n *NullWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

// Server represents an LDAP server
type Server struct {
	cfg     *config.Config
	store   store.Store
	hasher  *crypto.PasswordHasher
	srv     *ldapserver.Server
	version string
}

// NewServer creates a new LDAP server
func NewServer(cfg *config.Config, st store.Store, version string) *Server {
	return &Server{
		cfg:     cfg,
		store:   st,
		hasher:  crypto.NewPasswordHasher(cfg.Security.Argon2Config),
		version: version,
	}
}

// protectedAttributes lists LDAP operational attributes that cannot be modified by clients
// Note: objectClass is NOT protected - it's a required user attribute that clients must provide
var protectedAttributes = []string{
	"createtimestamp",
	"modifytimestamp",
}

// isProtectedAttribute checks if an attribute name is protected from modification
func isProtectedAttribute(attrName string) bool {
	attrLower := strings.ToLower(attrName)
	for _, protected := range protectedAttributes {
		if attrLower == protected {
			return true
		}
	}
	return false
}

// Start starts the LDAP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.BindAddress, s.cfg.Server.Port)

	// Create route mux for handling LDAP operations
	routeMux := ldapserver.NewRouteMux()
	routeMux.Bind(s.handleBind)
	routeMux.Search(s.handleSearch)
	routeMux.Add(s.handleAdd)
	routeMux.Delete(s.handleDelete)
	routeMux.Modify(s.handleModify)
	routeMux.Compare(s.handleCompare)
	routeMux.Extended(s.handleExtended)
	routeMux.NotFound(s.handleNotFound)

	// Create LDAP server
	ldapserver.Logger = log.New(&NullWriter{}, "", 0) // Redirect ldapserver logs to slog
	s.srv = ldapserver.NewServer()
	s.srv.Handle(routeMux)

	// Start server
	slog.Info("LDAP server starting", "address", addr)
	go func() {
		if err := s.srv.ListenAndServe(addr); err != nil {
			slog.Error("LDAP server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the LDAP server
func (s *Server) Stop() error {
	if s.srv != nil {
		s.srv.Stop()
	}
	return nil
}

// handleBind handles bind operations
func (s *Server) handleBind(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	ctx := context.Background()

	// Get bind request details from message
	bindReq := m.GetBindRequest()
	dn := string(bindReq.Name())
	password := string(bindReq.AuthenticationSimple())

	slog.Debug("Bind request", "dn", dn)

	// Handle anonymous bind
	if dn == "" || password == "" {
		if s.cfg.Security.AllowAnonymousBind {
			slog.Debug("Anonymous bind allowed")
			w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultSuccess))
			return
		}
		slog.Info("Anonymous bind rejected - not allowed by configuration")
		w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultInvalidCredentials))
		return
	}

	// Find user by UID using search
	uid := extractUID(dn)
	if uid == "" {
		slog.Debug("Failed to extract UID from DN", "dn", dn)
		w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultInvalidCredentials))
		return
	}

	// Search for user entry with matching uid
	entries, err := s.store.SearchEntries(ctx, s.cfg.LDAP.BaseDN, fmt.Sprintf("(uid=%s)", uid))
	if err != nil {
		slog.Debug("Search error during bind", "dn", dn, "error", err)
		w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultInvalidCredentials))
		return
	}

	// Should find exactly one user
	if len(entries) != 1 {
		slog.Debug("User not found or multiple users found", "dn", dn, "count", len(entries))
		w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultInvalidCredentials))
		return
	}

	entry := entries[0]

	// Get password from entry
	userPassword := entry.GetAttribute("userPassword")
	if userPassword == "" {
		w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultInvalidCredentials))
		return
	}

	// Verify password
	verified, err := s.hasher.Verify(password, userPassword)
	if err != nil || !verified {
		slog.Debug("Password verification failed", "dn", dn)
		w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultInvalidCredentials))
		return
	}

	slog.Debug("Bind successful", "dn", dn)
	w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultSuccess))
}

// handleSearch handles search operations
func (s *Server) handleSearch(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	ctx := context.Background()

	searchReq := m.GetSearchRequest()
	baseDN := string(searchReq.BaseObject())
	scope := int(searchReq.Scope())

	// Handle RootDSE queries (empty base DN)
	if baseDN == "" {
		slog.Debug("RootDSE query")
		s.handleRootDSE(w, m)
		return
	}

	// Handle schema queries
	if baseDN == "cn=Subschema" || baseDN == "cn=subschema" {
		slog.Debug("Schema query")
		s.handleSchema(w, m)
		return
	}

	// Get filter from request and serialize it to string
	filterStr := serializeFilter(searchReq.Filter())
	if filterStr == "" {
		// Default to matching everything if no filter provided
		filterStr = "(objectClass=*)"
	}

	slog.Debug("Search request", "baseDN", baseDN, "scope", scope, "filter", filterStr)

	entries, err := s.store.SearchEntries(ctx, baseDN, filterStr)
	if err != nil {
		slog.Error("Search error", "error", err)
		w.Write(ldapserver.NewSearchResultDoneResponse(ldapserver.LDAPResultOperationsError))
		return
	}

	// Return matching entries with attributes
	// Note: entries are already filtered by SearchEntries (SQL or in-memory)
	for _, entry := range entries {
		// Apply scope filtering
		// Scope: 0=base (only baseDN), 1=one level (immediate children), 2=subtree (all descendants)
		switch scope {
		case 0: // base - only return exact match
			if !strings.EqualFold(entry.DN, baseDN) {
				continue
			}
		case 1: // one level - only immediate children
			if !strings.EqualFold(entry.ParentDN, baseDN) {
				continue
			}
			// case 2: subtree - return all (default behavior)
		}

		// Build and send the LDAP search result entry
		result := ldapserver.NewSearchResultEntry(entry.DN)

		// Add objectClass attribute
		if entry.ObjectClass != "" {
			result.AddAttribute(
				message.AttributeDescription("objectClass"),
				message.AttributeValue(entry.ObjectClass),
			)
		}

		// Add all other attributes from the entry
		for attrName, attrValues := range entry.Attributes {
			// Convert string values to goldap AttributeValue types
			goldapValues := make([]message.AttributeValue, len(attrValues))
			for i, val := range attrValues {
				goldapValues[i] = message.AttributeValue(val)
			}

			// Add the attribute with all its values
			result.AddAttribute(
				message.AttributeDescription(attrName),
				goldapValues...,
			)
		}

		w.Write(result)
	}

	slog.Debug("Search completed", "baseDN", baseDN, "results", len(entries))
	w.Write(ldapserver.NewSearchResultDoneResponse(ldapserver.LDAPResultSuccess))
}

// handleAdd handles add operations
func (s *Server) handleAdd(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	ctx := context.Background()
	addReq := m.GetAddRequest()

	dn := string(addReq.Entry())
	slog.Debug("Add request", "dn", dn)

	// Check if entry already exists
	exists, err := s.store.EntryExists(ctx, dn)
	if err != nil {
		slog.Error("Failed to check entry existence", "dn", dn, "error", err)
		w.Write(ldapserver.NewAddResponse(ldapserver.LDAPResultOperationsError))
		return
	}

	if exists {
		slog.Debug("Entry already exists", "dn", dn)
		w.Write(ldapserver.NewAddResponse(ldapserver.LDAPResultEntryAlreadyExists))
		return
	}

	// Extract parent DN from the full DN
	// e.g., "uid=john,ou=users,dc=example,dc=com" -> "ou=users,dc=example,dc=com"
	parentDN := ""
	if idx := findFirstComma(dn); idx >= 0 && idx+1 < len(dn) {
		parentDN = dn[idx+1:]
	}

	// Create new entry with proper timestamps
	entry := &models.Entry{
		DN:         dn,
		ParentDN:   parentDN,
		Attributes: make(map[string][]string),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Parse and add attributes from request
	attrs := addReq.Attributes()
	for _, attr := range attrs {
		name := string(attr.Type_())

		// Check if trying to set protected operational attributes
		if isProtectedAttribute(name) {
			slog.Debug("Attempt to set protected attribute", "dn", dn, "attribute", name)
			w.Write(ldapserver.NewAddResponse(ldapserver.LDAPResultUnwillingToPerform))
			return
		}

		values := attr.Vals()
		for _, val := range values {
			entry.AddAttribute(name, string(val))
		}
	}

	// Special handling for userPassword - must be hashed with scheme prefix
	if userPassword := entry.GetAttribute("userPassword"); userPassword != "" {
		processedPassword, err := s.hasher.ProcessPassword(userPassword)
		if err != nil {
			slog.Debug("Invalid password format", "dn", dn, "error", err)
			w.Write(ldapserver.NewAddResponse(ldapserver.LDAPResultConstraintViolation))
			return
		}
		entry.SetAttribute("userPassword", processedPassword)
	}

	// Determine object class and handle accordingly
	objectClasses := entry.GetAttribute("objectClass")
	if objectClasses == "" {
		slog.Debug("No objectClass provided", "dn", dn)
		w.Write(ldapserver.NewAddResponse(ldapserver.LDAPResultObjectClassViolation))
		return
	}

	// Set the primary object class (use first objectClass value if multiple)
	entry.ObjectClass = objectClasses
	allClasses := entry.GetAttributes("objectClass")
	if len(allClasses) > 0 {
		entry.ObjectClass = allClasses[0]
	}

	slog.Debug("Creating entry", "dn", dn, "objectClass", objectClasses)

	// Store the entry
	if err := s.store.CreateEntry(ctx, entry); err != nil {
		slog.Error("Failed to create entry", "dn", dn, "error", err)
		w.Write(ldapserver.NewAddResponse(ldapserver.LDAPResultOperationsError))
		return
	}

	slog.Info("Entry created", "dn", dn)
	w.Write(ldapserver.NewAddResponse(ldapserver.LDAPResultSuccess))
}

// handleDelete handles delete operations
func (s *Server) handleDelete(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	ctx := context.Background()
	delReq := m.GetDeleteRequest()

	dn := string(delReq)
	slog.Debug("Delete request", "dn", dn)

	// Check if entry exists
	exists, err := s.store.EntryExists(ctx, dn)
	if err != nil {
		slog.Error("Failed to check entry existence", "dn", dn, "error", err)
		w.Write(ldapserver.NewDeleteResponse(ldapserver.LDAPResultOperationsError))
		return
	}

	if !exists {
		slog.Debug("Entry not found", "dn", dn)
		w.Write(ldapserver.NewDeleteResponse(ldapserver.LDAPResultNoSuchObject))
		return
	}

	if err := s.store.DeleteEntry(ctx, dn); err != nil {
		slog.Error("Failed to delete entry", "dn", dn, "error", err)
		w.Write(ldapserver.NewDeleteResponse(ldapserver.LDAPResultOperationsError))
		return
	}

	slog.Info("Entry deleted", "dn", dn)
	w.Write(ldapserver.NewDeleteResponse(ldapserver.LDAPResultSuccess))
}

// handleModify handles modify operations
func (s *Server) handleModify(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	ctx := context.Background()
	modReq := m.GetModifyRequest()

	dn := string(modReq.Object())
	slog.Debug("Modify request", "dn", dn)

	// Get the entry
	entry, err := s.store.GetEntry(ctx, dn)
	if err != nil {
		slog.Error("Failed to get entry", "dn", dn, "error", err)
		w.Write(ldapserver.NewModifyResponse(ldapserver.LDAPResultOperationsError))
		return
	}

	if entry == nil {
		slog.Debug("Entry not found", "dn", dn)
		w.Write(ldapserver.NewModifyResponse(ldapserver.LDAPResultNoSuchObject))
		return
	}

	// Apply modifications
	changes := modReq.Changes()
	for _, change := range changes {
		modification := change.Modification()
		attrType := string(modification.Type_())

		// Check if trying to modify protected operational attributes
		if isProtectedAttribute(attrType) {
			slog.Debug("Attempt to modify protected attribute", "dn", dn, "attribute", attrType)
			w.Write(ldapserver.NewModifyResponse(ldapserver.LDAPResultUnwillingToPerform))
			return
		}

		vals := modification.Vals()

		// Get operation type
		// 0 = Add, 1 = Delete, 2 = Replace
		opType := int(change.Operation())

		switch opType {
		case 0: // Add
			slog.Debug("Add attribute", "attr", attrType)
			if attrType == "userPassword" {
				// Process password values
				for _, val := range vals {
					processedPassword, err := s.hasher.ProcessPassword(string(val))
					if err != nil {
						slog.Debug("Invalid password format", "dn", dn, "error", err)
						w.Write(ldapserver.NewModifyResponse(ldapserver.LDAPResultConstraintViolation))
						return
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
				// Delete all values of this attribute
				entry.RemoveAttribute(attrType)
			} else {
				// Delete specific values
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
				// Process password values
				for _, val := range vals {
					processedPassword, err := s.hasher.ProcessPassword(string(val))
					if err != nil {
						slog.Debug("Invalid password format", "dn", dn, "error", err)
						w.Write(ldapserver.NewModifyResponse(ldapserver.LDAPResultConstraintViolation))
						return
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

	// Update the entry in database
	if err := s.store.UpdateEntry(ctx, entry); err != nil {
		slog.Error("Failed to update entry", "dn", dn, "error", err)
		w.Write(ldapserver.NewModifyResponse(ldapserver.LDAPResultOperationsError))
		return
	}

	slog.Info("Entry modified", "dn", dn)
	w.Write(ldapserver.NewModifyResponse(ldapserver.LDAPResultSuccess))
}

// handleRootDSE handles RootDSE queries (empty base DN)
func (s *Server) handleRootDSE(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	// Create RootDSE entry response
	entry := ldapserver.NewSearchResultEntry("")

	// Add standard RootDSE attributes using proper goldap types
	entry.AddAttribute(
		message.AttributeDescription("objectClass"),
		message.AttributeValue("top"),
	)
	entry.AddAttribute(
		message.AttributeDescription("namingContexts"),
		message.AttributeValue(s.cfg.LDAP.BaseDN),
	)
	entry.AddAttribute(
		message.AttributeDescription("subschemaSubentry"),
		message.AttributeValue("cn=Subschema"),
	)
	entry.AddAttribute(
		message.AttributeDescription("supportedLDAPVersion"),
		message.AttributeValue("3"),
	)
	entry.AddAttribute(
		message.AttributeDescription("vendorName"),
		message.AttributeValue("LDAPLite"),
	)
	entry.AddAttribute(
		message.AttributeDescription("vendorVersion"),
		message.AttributeValue(s.version),
	)

	// Write the entry
	w.Write(entry)

	// Write done response
	w.Write(ldapserver.NewSearchResultDoneResponse(ldapserver.LDAPResultSuccess))
}

// handleSchema handles schema queries (cn=Subschema)
func (s *Server) handleSchema(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	// Create schema entry
	entry := ldapserver.NewSearchResultEntry("cn=Subschema")

	// Add objectClass
	entry.AddAttribute(
		message.AttributeDescription("objectClass"),
		message.AttributeValue("top"),
		message.AttributeValue("subschema"),
	)

	// Add basic object classes that we support
	entry.AddAttribute(
		message.AttributeDescription("objectClasses"),
		message.AttributeValue("( 2.5.6.0 NAME 'top' DESC 'top of the superclass chain' ABSTRACT MUST objectClass )"),
		message.AttributeValue("( 2.5.6.6 NAME 'person' DESC 'RFC2256: a person' SUP top STRUCTURAL MUST ( sn $ cn ) MAY ( userPassword $ telephoneNumber $ seeAlso $ description ) )"),
		message.AttributeValue("( 2.5.6.7 NAME 'organizationalPerson' DESC 'RFC2256: an organizational person' SUP person STRUCTURAL MAY ( title $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ ou $ st $ l ) )"),
		message.AttributeValue("( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' DESC 'RFC2798: Internet Organizational Person' SUP organizationalPerson STRUCTURAL MAY ( audio $ businessCategory $ carLicense $ departmentNumber $ displayName $ employeeNumber $ employeeType $ givenName $ homePhone $ homePostalAddress $ initials $ jpegPhoto $ labeledURI $ mail $ manager $ mobile $ o $ pager $ photo $ roomNumber $ secretary $ uid $ userCertificate $ x500uniqueIdentifier $ preferredLanguage $ userSMIMECertificate $ userPKCS12 ) )"),
		message.AttributeValue("( 2.5.6.9 NAME 'groupOfNames' DESC 'RFC2256: a group of names (DNs)' SUP top STRUCTURAL MUST ( member $ cn ) MAY ( businessCategory $ seeAlso $ owner $ ou $ o $ description ) )"),
		message.AttributeValue("( 2.5.6.5 NAME 'organizationalUnit' DESC 'RFC2256: an organizational unit' SUP top STRUCTURAL MUST ou MAY ( userPassword $ searchGuide $ seeAlso $ businessCategory $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ st $ l $ description ) )"),
	)

	// Add basic attribute types
	entry.AddAttribute(
		message.AttributeDescription("attributeTypes"),
		message.AttributeValue("( 2.5.4.0 NAME 'objectClass' DESC 'RFC2256: object classes of the entity' EQUALITY objectIdentifierMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.38 )"),
		message.AttributeValue("( 2.5.4.41 NAME 'name' DESC 'RFC2256: common supertype of name attributes' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{32768} )"),
		message.AttributeValue("( 2.5.4.3 NAME 'cn' SUP name DESC 'RFC2256: common name(s) for which the entity is known by' )"),
		message.AttributeValue("( 2.5.4.4 NAME 'sn' SUP name DESC 'RFC2256: last (family) name(s) for which the entity is known by' )"),
		message.AttributeValue("( 0.9.2342.19200300.100.1.1 NAME 'uid' DESC 'RFC1274: user identifier' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{256} )"),
		message.AttributeValue("( 0.9.2342.19200300.100.1.3 NAME 'mail' DESC 'RFC1274: RFC822 Mailbox' EQUALITY caseIgnoreIA5Match SUBSTR caseIgnoreIA5SubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.26{256} )"),
		message.AttributeValue("( 2.5.4.35 NAME 'userPassword' DESC 'RFC2256/2307: password of user' EQUALITY octetStringMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.40{128} )"),
		message.AttributeValue("( 2.5.4.31 NAME 'member' DESC 'RFC2256: member of a group' SUP distinguishedName )"),
		message.AttributeValue("( 2.5.4.11 NAME 'ou' SUP name DESC 'RFC2256: organizational unit this object belongs to' )"),
	)

	// Write the entry
	w.Write(entry)

	// Write done response
	w.Write(ldapserver.NewSearchResultDoneResponse(ldapserver.LDAPResultSuccess))
}

// handleCompare handles compare operations
func (s *Server) handleCompare(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	slog.Debug("Compare request")
	w.Write(ldapserver.NewCompareResponse(ldapserver.LDAPResultCompareFalse))
}

// handleExtended handles extended operations
func (s *Server) handleExtended(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	slog.Debug("Extended request")
	w.Write(ldapserver.NewExtendedResponse(ldapserver.LDAPResultUnavailable))
}

// handleNotFound handles unknown operations
func (s *Server) handleNotFound(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	slog.Debug("Unknown operation", "operation", m.ProtocolOpName())
	w.Write(ldapserver.NewResponse(ldapserver.LDAPResultUnavailable))
}

// extractUID extracts UID from a DN
func extractUID(dn string) string {
	// Simple extraction: dn format is "uid=username,ou=users,dc=example,dc=com"
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

// serializeFilter converts a goldap Filter object to an LDAP filter string
// This is a best-effort function that handles common filter types
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
		// Substrings filter: (attr=initial*any*final)
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
		// Fallback: try string conversion
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
			// Check if it's escaped
			if i > 0 && dn[i-1] == '\\' {
				continue
			}
			return i
		}
	}
	return -1
}
