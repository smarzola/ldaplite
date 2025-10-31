package protocol

import (
	"github.com/lor00x/goldap/message"
)

// Helper functions for creating LDAP responses

// NewBindResponse creates a bind response with the given result code
func NewBindResponse(resultCode int) message.BindResponse {
	r := message.BindResponse{}
	r.SetResultCode(resultCode)
	return r
}

// NewSearchResultEntry creates a search result entry with the given DN
func NewSearchResultEntry(dn string) message.SearchResultEntry {
	r := message.SearchResultEntry{}
	r.SetObjectName(dn)
	return r
}

// AddAttribute adds an attribute to a search result entry
func AddAttribute(entry *message.SearchResultEntry, name string, values ...string) {
	attrValues := make([]message.AttributeValue, len(values))
	for i, v := range values {
		attrValues[i] = message.AttributeValue(v)
	}
	entry.AddAttribute(message.AttributeDescription(name), attrValues...)
}

// NewSearchResultDone creates a search done response
func NewSearchResultDone(resultCode int) message.SearchResultDone {
	r := message.SearchResultDone{}
	r.SetResultCode(resultCode)
	return r
}

// NewAddResponse creates an add response
func NewAddResponse(resultCode int) message.AddResponse {
	r := message.AddResponse{}
	r.SetResultCode(resultCode)
	return r
}

// NewModifyResponse creates a modify response
func NewModifyResponse(resultCode int) message.ModifyResponse {
	r := message.ModifyResponse{}
	r.SetResultCode(resultCode)
	return r
}

// NewDelResponse creates a delete response
func NewDelResponse(resultCode int) message.DelResponse {
	r := message.DelResponse{}
	r.SetResultCode(resultCode)
	return r
}

// NewCompareResponse creates a compare response
func NewCompareResponse(resultCode int) message.CompareResponse {
	r := message.CompareResponse{}
	r.SetResultCode(resultCode)
	return r
}

// NewExtendedResponse creates an extended response
func NewExtendedResponse(resultCode int) message.ExtendedResponse {
	r := message.ExtendedResponse{}
	r.SetResultCode(resultCode)
	return r
}
