package protocol

import (
	"bytes"
	"testing"

	"github.com/lor00x/goldap/message"
)

func TestCanonicalAttributeName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "objectclass", want: "objectClass"},
		{name: "createTimestamp", want: "createTimestamp"},
		{name: "createtimestamp", want: "createTimestamp"},
		{name: "modifytimestamp", want: "modifyTimestamp"},
		{name: "memberof", want: "memberOf"},
		{name: "givenname", want: "givenName"},
		{name: "displayname", want: "displayName"},
		{name: "telephonenumber", want: "telephoneNumber"},
		{name: "userpassword", want: "userPassword"},
		{name: "namingcontexts", want: "namingContexts"},
		{name: "subschemasubentry", want: "subschemaSubentry"},
		{name: "supportedldapversion", want: "supportedLDAPVersion"},
		{name: "vendorname", want: "vendorName"},
		{name: "vendorversion", want: "vendorVersion"},
		{name: "customattr", want: "customattr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanonicalAttributeName(tt.name); got != tt.want {
				t.Fatalf("CanonicalAttributeName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestSearchResultEntryBERUsesCanonicalKnownAttributeNames(t *testing.T) {
	entry := NewSearchResultEntry("uid=jdoe,ou=users,dc=example,dc=com")
	AddAttribute(&entry, "objectclass", "inetOrgPerson")
	AddAttribute(&entry, "memberof", "cn=developers,ou=groups,dc=example,dc=com")
	AddAttribute(&entry, "createtimestamp", "20260102030405Z")
	AddAttribute(&entry, "modifytimestamp", "20260102040506Z")
	AddAttribute(&entry, "givenname", "John")
	AddAttribute(&entry, "displayname", "John Doe")
	AddAttribute(&entry, "telephonenumber", "+15555550100")
	AddAttribute(&entry, "namingcontexts", "dc=example,dc=com")
	AddAttribute(&entry, "subschemasubentry", "cn=Subschema")
	AddAttribute(&entry, "supportedldapversion", "3")
	AddAttribute(&entry, "vendorname", "LDAPLite")
	AddAttribute(&entry, "vendorversion", "test")

	msg := message.NewLDAPMessageWithProtocolOp(entry)
	msg.SetMessageID(1)
	encoded, err := msg.Write()
	if err != nil {
		t.Fatalf("failed to encode search result entry: %v", err)
	}
	ber := encoded.Bytes()

	for _, attr := range []string{
		"objectClass",
		"memberOf",
		"createTimestamp",
		"modifyTimestamp",
		"givenName",
		"displayName",
		"telephoneNumber",
		"namingContexts",
		"subschemaSubentry",
		"supportedLDAPVersion",
		"vendorName",
		"vendorVersion",
	} {
		if !bytes.Contains(ber, []byte(attr)) {
			t.Fatalf("encoded BER search result is missing canonical attribute %q: %x", attr, ber)
		}
	}

	for _, attr := range []string{"objectclass", "memberof", "createtimestamp", "modifytimestamp", "givenname"} {
		if bytes.Contains(ber, []byte(attr)) {
			t.Fatalf("encoded BER search result contains non-canonical attribute %q: %x", attr, ber)
		}
	}
}
