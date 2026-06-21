package ldapdn

import "testing"

func TestSplit(t *testing.T) {
	tests := []struct {
		name       string
		dn         string
		wantRDN    string
		wantParent string
	}{
		{
			name:       "simple DN",
			dn:         "uid=jane,ou=users,dc=example,dc=com",
			wantRDN:    "uid=jane",
			wantParent: "ou=users,dc=example,dc=com",
		},
		{
			name:       "escaped comma in RDN",
			dn:         `cn=Doe\, Jane,ou=users,dc=example,dc=com`,
			wantRDN:    `cn=Doe\, Jane`,
			wantParent: "ou=users,dc=example,dc=com",
		},
		{
			name:       "no parent",
			dn:         "dc=example",
			wantRDN:    "dc=example",
			wantParent: "",
		},
		{
			name:       "trims components",
			dn:         " uid=jane , ou=users,dc=example,dc=com ",
			wantRDN:    "uid=jane",
			wantParent: "ou=users,dc=example,dc=com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRDN, gotParent := Split(tt.dn)
			if gotRDN != tt.wantRDN {
				t.Fatalf("RDN = %q, want %q", gotRDN, tt.wantRDN)
			}
			if gotParent != tt.wantParent {
				t.Fatalf("Parent = %q, want %q", gotParent, tt.wantParent)
			}
		})
	}
}

func TestWithinBase(t *testing.T) {
	tests := []struct {
		name string
		dn   string
		base string
		want bool
	}{
		{
			name: "base itself",
			dn:   "dc=example,dc=com",
			base: "DC=EXAMPLE,DC=COM",
			want: true,
		},
		{
			name: "descendant",
			dn:   "uid=jane,ou=users,dc=example,dc=com",
			base: "dc=example,dc=com",
			want: true,
		},
		{
			name: "escaped comma descendant",
			dn:   `cn=Doe\, Jane,ou=users,dc=example,dc=com`,
			base: "ou=users,dc=example,dc=com",
			want: true,
		},
		{
			name: "similar suffix is not descendant",
			dn:   "uid=jane,ou=users,dc=badexample,dc=com",
			base: "dc=example,dc=com",
			want: false,
		},
		{
			name: "empty base",
			dn:   "uid=jane,dc=example,dc=com",
			base: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WithinBase(tt.dn, tt.base); got != tt.want {
				t.Fatalf("WithinBase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFirstRDNValue(t *testing.T) {
	tests := []struct {
		name string
		dn   string
		attr string
		want string
	}{
		{
			name: "matching uid",
			dn:   "uid=jane,ou=users,dc=example,dc=com",
			attr: "uid",
			want: "jane",
		},
		{
			name: "case-insensitive attribute",
			dn:   "UID=jane,ou=users,dc=example,dc=com",
			attr: "uid",
			want: "jane",
		},
		{
			name: "escaped comma in value",
			dn:   `uid=jane\,doe,ou=users,dc=example,dc=com`,
			attr: "uid",
			want: `jane\,doe`,
		},
		{
			name: "escaped equals in value",
			dn:   `uid=jane\=doe,ou=users,dc=example,dc=com`,
			attr: "uid",
			want: `jane\=doe`,
		},
		{
			name: "non-matching attribute",
			dn:   "cn=Jane Doe,ou=users,dc=example,dc=com",
			attr: "uid",
			want: "",
		},
		{
			name: "malformed RDN",
			dn:   "jane,ou=users,dc=example,dc=com",
			attr: "uid",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FirstRDNValue(tt.dn, tt.attr); got != tt.want {
				t.Fatalf("FirstRDNValue() = %q, want %q", got, tt.want)
			}
		})
	}
}
