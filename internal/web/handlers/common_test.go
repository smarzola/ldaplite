package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
)

func TestReplaceExtraAttributesReplacesStaleExtrasAndDropsComputedAttributes(t *testing.T) {
	entry := models.NewEntry("uid=jdoe,ou=users,dc=test,dc=com", "inetOrgPerson")
	entry.SetAttribute("uid", "jdoe")
	entry.SetAttribute("cn", "John Doe")
	entry.SetAttribute("title", "Old Title")
	entry.SetAttribute("objectClass", "inetOrgPerson")
	entry.SetAttribute("memberOf", "cn=admins,ou=groups,dc=test,dc=com")
	entry.SetAttribute("createTimestamp", "20260102030405Z")
	entry.SetAttribute("modifyTimestamp", "20260102030405Z")

	ReplaceExtraAttributes(entry, []string{"uid", "cn"}, map[string][]string{
		"title":      {"New Title"},
		"department": {"Engineering"},
	})

	if got := entry.GetAttribute("uid"); got != "jdoe" {
		t.Fatalf("uid should be preserved, got %q", got)
	}
	if got := entry.GetAttribute("cn"); got != "John Doe" {
		t.Fatalf("cn should be preserved, got %q", got)
	}
	if got := entry.GetAttribute("title"); got != "New Title" {
		t.Fatalf("title should be replaced, got %q", got)
	}
	if got := entry.GetAttribute("department"); got != "Engineering" {
		t.Fatalf("department should be added, got %q", got)
	}

	for _, attr := range []string{"objectClass", "memberOf", "createTimestamp", "modifyTimestamp"} {
		if entry.HasAttribute(attr) {
			t.Fatalf("%s should be removed before update persistence", attr)
		}
	}
}

func TestRedirectWithMessageEscapesQueryValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/users/delete", nil)
	rr := httptest.NewRecorder()

	redirectWithMessage(rr, req, "/users", "error", "Failed to delete User: cn=a b,dc=test")

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}

	want := "/users?error=Failed+to+delete+User%3A+cn%3Da+b%2Cdc%3Dtest"
	if got := rr.Header().Get("Location"); got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestSetOptionalAttributeRemovesBlankValues(t *testing.T) {
	entry := models.NewEntry("uid=jdoe,ou=users,dc=test,dc=com", "inetOrgPerson")
	entry.SetAttribute("mail", "jdoe@test.com")

	setOptionalAttribute(entry, "mail", "   ")

	if entry.HasAttribute("mail") {
		t.Fatal("blank optional value should remove the attribute")
	}
}

func TestParseNonEmptyLinesTrimsAndDropsBlanks(t *testing.T) {
	got := parseNonEmptyLines(" uid=one,dc=test,dc=com \n\nuid=two,dc=test,dc=com\r\n   ")
	want := []string{"uid=one,dc=test,dc=com", "uid=two,dc=test,dc=com"}

	if len(got) != len(want) {
		t.Fatalf("parseNonEmptyLines() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseNonEmptyLines()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAddExtraAttributesPreservesMultiValues(t *testing.T) {
	entry := models.NewEntry("uid=jdoe,ou=users,dc=test,dc=com", "inetOrgPerson")

	addExtraAttributes(entry, map[string][]string{
		"mail":  {"jdoe@test.com", "john@test.com"},
		"title": {"Engineer"},
	})

	if got := entry.GetAttributes("mail"); len(got) != 2 || got[0] != "jdoe@test.com" || got[1] != "john@test.com" {
		t.Fatalf("mail values = %#v", got)
	}
	if got := entry.GetAttribute("title"); got != "Engineer" {
		t.Fatalf("title = %q, want Engineer", got)
	}
}

func TestExtractUIDFromDNUsesFirstRDN(t *testing.T) {
	tests := []struct {
		name string
		dn   string
		want string
	}{
		{
			name: "uid RDN",
			dn:   "uid=admin,ou=users,dc=example,dc=com",
			want: "admin",
		},
		{
			name: "case-insensitive uid attribute",
			dn:   "UID=admin,ou=users,dc=example,dc=com",
			want: "admin",
		},
		{
			name: "escaped comma in uid value",
			dn:   `uid=Doe\, Jane,ou=users,dc=example,dc=com`,
			want: `Doe\, Jane`,
		},
		{
			name: "non uid first RDN",
			dn:   "cn=admin,ou=users,dc=example,dc=com",
			want: "",
		},
		{
			name: "empty DN",
			dn:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractUIDFromDN(tt.dn); got != tt.want {
				t.Fatalf("ExtractUIDFromDN() = %q, want %q", got, tt.want)
			}
		})
	}
}
