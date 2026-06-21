package protocol

import "testing"

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
