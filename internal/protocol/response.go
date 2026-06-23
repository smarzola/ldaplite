package protocol

import (
	"strings"

	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
)

// Helper functions for creating LDAP responses

// NewBindResponse creates a bind response with the given result code
func NewBindResponse(resultCode ldapmsg.ResultCode) ldapmsg.BindResponse {
	return ldapmsg.BindResponse{LDAPResult: ldapmsg.LDAPResult{ResultCode: resultCode}}
}

// NewSearchResultEntry creates a search result entry with the given DN
func NewSearchResultEntry(dn string) ldapmsg.SearchResultEntry {
	return ldapmsg.SearchResultEntry{ObjectName: dn}
}

// AddAttribute adds an attribute to a search result entry
func AddAttribute(entry *ldapmsg.SearchResultEntry, name string, values ...string) {
	entry.Attributes = append(entry.Attributes, ldapmsg.Attribute{
		Name:   CanonicalAttributeName(name),
		Values: append([]string(nil), values...),
	})
}

// CanonicalAttributeName returns the preferred display casing for known LDAP
// attributes while keeping unknown/custom attributes unchanged.
func CanonicalAttributeName(name string) string {
	switch strings.ToLower(name) {
	case "objectclass":
		return "objectClass"
	case "createtimestamp":
		return "createTimestamp"
	case "modifytimestamp":
		return "modifyTimestamp"
	case "memberof":
		return "memberOf"
	case "givenname":
		return "givenName"
	case "displayname":
		return "displayName"
	case "entryuuid":
		return "entryUUID"
	case "telephonenumber":
		return "telephoneNumber"
	case "userpassword":
		return "userPassword"
	case "namingcontexts":
		return "namingContexts"
	case "subschemasubentry":
		return "subschemaSubentry"
	case "supportedldapversion":
		return "supportedLDAPVersion"
	case "supportedextension":
		return "supportedExtension"
	case "vendorname":
		return "vendorName"
	case "vendorversion":
		return "vendorVersion"
	default:
		return name
	}
}

// NewSearchResultDone creates a search done response
func NewSearchResultDone(resultCode ldapmsg.ResultCode) ldapmsg.SearchResultDone {
	return ldapmsg.SearchResultDone{LDAPResult: ldapmsg.LDAPResult{ResultCode: resultCode}}
}

// NewAddResponse creates an add response
func NewAddResponse(resultCode ldapmsg.ResultCode) ldapmsg.AddResponse {
	return ldapmsg.AddResponse{LDAPResult: ldapmsg.LDAPResult{ResultCode: resultCode}}
}

// NewModifyResponse creates a modify response
func NewModifyResponse(resultCode ldapmsg.ResultCode) ldapmsg.ModifyResponse {
	return ldapmsg.ModifyResponse{LDAPResult: ldapmsg.LDAPResult{ResultCode: resultCode}}
}

// NewDelResponse creates a delete response
func NewDelResponse(resultCode ldapmsg.ResultCode) ldapmsg.DeleteResponse {
	return ldapmsg.DeleteResponse{LDAPResult: ldapmsg.LDAPResult{ResultCode: resultCode}}
}

// NewCompareResponse creates a compare response
func NewCompareResponse(resultCode ldapmsg.ResultCode) ldapmsg.CompareResponse {
	return ldapmsg.CompareResponse{LDAPResult: ldapmsg.LDAPResult{ResultCode: resultCode}}
}

// NewExtendedResponse creates an extended response
func NewExtendedResponse(resultCode ldapmsg.ResultCode) ldapmsg.ExtendedResponse {
	return ldapmsg.ExtendedResponse{LDAPResult: ldapmsg.LDAPResult{ResultCode: resultCode}}
}
