package server

import (
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
)

func TestSearchAttributeSelectionDefaultIncludesExistingBehavior(t *testing.T) {
	selection := newSearchAttributeSelection(nil)

	for _, attr := range []string{"objectClass", "cn", "memberOf", "createTimestamp", "modifyTimestamp"} {
		t.Run(attr, func(t *testing.T) {
			if !selection.includes(attr) {
				t.Fatalf("default selection should include %s", attr)
			}
		})
	}
}

func TestSearchAttributeSelectionExplicitSingleAttribute(t *testing.T) {
	selection := newSearchAttributeSelection([]string{"cn"})

	if !selection.includes("cn") {
		t.Fatal("explicit selection should include cn")
	}
	if selection.includes("mail") {
		t.Fatal("explicit selection should not include unrequested mail")
	}
	if selection.includes("memberOf") {
		t.Fatal("explicit selection should not include unrequested operational memberOf")
	}
}

func TestSearchAttributeSelectionIsCaseInsensitive(t *testing.T) {
	selection := newSearchAttributeSelection([]string{"MeMbErOf", "GIVENNAME"})

	if !selection.includes("memberOf") {
		t.Fatal("mixed-case selector should include memberOf")
	}
	if !selection.includes("givenName") {
		t.Fatal("upper-case selector should include givenName")
	}
}

func TestSearchAttributeSelectionNoAttributes(t *testing.T) {
	selection := newSearchAttributeSelection([]string{"1.1"})

	for _, attr := range []string{"objectClass", "cn", "memberOf"} {
		t.Run(attr, func(t *testing.T) {
			if selection.includes(attr) {
				t.Fatalf("1.1 selection should not include %s", attr)
			}
		})
	}
}

func TestSearchAttributeSelectionWildcardAndOperational(t *testing.T) {
	selection := newSearchAttributeSelection([]string{"*"})
	if !selection.includes("cn") {
		t.Fatal("* should include user attributes")
	}
	if selection.includes("memberOf") {
		t.Fatal("* should not include operational attributes")
	}

	selection = newSearchAttributeSelection([]string{"+"})
	if !selection.includes("memberOf") {
		t.Fatal("+ should include operational attributes")
	}
	if selection.includes("cn") {
		t.Fatal("+ should not include user attributes")
	}

	selection = newSearchAttributeSelection([]string{"*", "+"})
	if !selection.includes("cn") || !selection.includes("memberOf") {
		t.Fatal("* + should include both user and operational attributes")
	}
}

func TestSearchResponseAttributesEmitsSingleObjectClass(t *testing.T) {
	entry := models.NewEntry("uid=jdoe,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("objectclass", "inetOrgPerson")
	entry.SetAttribute("cn", "John Doe")

	attrs := searchResponseAttributes(entry, newSearchAttributeSelection(nil))

	objectClassCount := 0
	for _, attr := range attrs {
		if attr.name == "objectClass" {
			objectClassCount++
		}
		if attr.name == "objectclass" {
			t.Fatalf("search response should not emit lower-case duplicate objectclass: %v", attrs)
		}
	}
	if objectClassCount != 1 {
		t.Fatalf("search response should emit exactly one objectClass, got %d in %v", objectClassCount, attrs)
	}
}

func TestSearchResponseAttributesProjectsOperationalTimestamps(t *testing.T) {
	entry := models.NewEntry("uid=jdoe,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("cn", "John Doe")
	entry.SetAttribute("createTimestamp", "stored-value-should-not-leak")
	entry.SetAttribute("modifyTimestamp", "stored-value-should-not-leak")

	attrs := searchResponseAttributes(entry, newSearchAttributeSelection([]string{"+"}))

	got := map[string][]string{}
	for _, attr := range attrs {
		got[strings.ToLower(attr.name)] = attr.values
	}

	if got["cn"] != nil {
		t.Fatalf("+ selection should not include user attribute cn: %v", attrs)
	}
	if got["createtimestamp"][0] != models.FormatLDAPTimestamp(entry.CreatedAt) {
		t.Fatalf("createTimestamp should be projected from Entry.CreatedAt, got %v", got["createtimestamp"])
	}
	if got["modifytimestamp"][0] != models.FormatLDAPTimestamp(entry.UpdatedAt) {
		t.Fatalf("modifyTimestamp should be projected from Entry.UpdatedAt, got %v", got["modifytimestamp"])
	}
}

func TestSearchResponseAttributesProjectsMemberOfOnce(t *testing.T) {
	entry := models.NewEntry("uid=jdoe,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttribute("cn", "John Doe")
	entry.SetAttributes("memberOf", []string{"cn=admins,ou=groups,dc=example,dc=com"})

	attrs := searchResponseAttributes(entry, newSearchAttributeSelection([]string{"+"}))

	memberOfCount := 0
	for _, attr := range attrs {
		if strings.EqualFold(attr.name, "memberOf") {
			memberOfCount++
			if len(attr.values) != 1 || attr.values[0] != "cn=admins,ou=groups,dc=example,dc=com" {
				t.Fatalf("memberOf values = %v", attr.values)
			}
		}
		if attr.name == "cn" {
			t.Fatalf("+ selection should not include user attribute cn: %v", attrs)
		}
	}
	if memberOfCount != 1 {
		t.Fatalf("search response should emit exactly one memberOf attribute, got %d in %v", memberOfCount, attrs)
	}
}

func TestEscapeLDAPFilterAssertionValue(t *testing.T) {
	got := escapeLDAPFilterAssertionValue(`A*B(C)\` + string(rune(0)))
	want := `A\2aB\28C\29\5c\00`
	if got != want {
		t.Fatalf("escapeLDAPFilterAssertionValue() = %q, want %q", got, want)
	}
}
