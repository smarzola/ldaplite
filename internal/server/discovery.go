package server

import (
	"context"
	"log/slog"

	"github.com/lor00x/goldap/message"

	"github.com/smarzola/ldaplite/internal/protocol"
)

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

// handleExtended handles extended operations
func (s *Server) handleExtended(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	extReq := msg.ProtocolOp().(message.ExtendedRequest)
	reqOID := string(extReq.RequestName())

	slog.Debug("Extended request", "oid", reqOID)

	// Handle "Who am I?" extended operation (RFC 4532)
	// OID: 1.3.6.1.4.1.4203.1.11.3
	if reqOID == protocol.WhoAmIOID {
		boundDN := conn.GetBoundDN()

		// RFC 4532 specifies the format as "dn:<distinguished-name>" or empty for anonymous.
		var authzID string
		if boundDN != "" {
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
