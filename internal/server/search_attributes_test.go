package server

import (
	"strings"
	"testing"

	"github.com/lor00x/goldap/message"
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
	selection := newSearchAttributeSelection(message.AttributeSelection{
		message.LDAPString("cn"),
	})

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
	selection := newSearchAttributeSelection(message.AttributeSelection{
		message.LDAPString("MeMbErOf"),
		message.LDAPString("GIVENNAME"),
	})

	if !selection.includes("memberOf") {
		t.Fatal("mixed-case selector should include memberOf")
	}
	if !selection.includes("givenName") {
		t.Fatal("upper-case selector should include givenName")
	}
}

func TestSearchAttributeSelectionNoAttributes(t *testing.T) {
	selection := newSearchAttributeSelection(message.AttributeSelection{
		message.LDAPString("1.1"),
	})

	for _, attr := range []string{"objectClass", "cn", "memberOf"} {
		t.Run(attr, func(t *testing.T) {
			if selection.includes(attr) {
				t.Fatalf("1.1 selection should not include %s", attr)
			}
		})
	}
}

func TestSearchAttributeSelectionWildcardAndOperational(t *testing.T) {
	selection := newSearchAttributeSelection(message.AttributeSelection{
		message.LDAPString("*"),
	})
	if !selection.includes("cn") {
		t.Fatal("* should include user attributes")
	}
	if selection.includes("memberOf") {
		t.Fatal("* should not include operational attributes")
	}

	selection = newSearchAttributeSelection(message.AttributeSelection{
		message.LDAPString("+"),
	})
	if !selection.includes("memberOf") {
		t.Fatal("+ should include operational attributes")
	}
	if selection.includes("cn") {
		t.Fatal("+ should not include user attributes")
	}

	selection = newSearchAttributeSelection(message.AttributeSelection{
		message.LDAPString("*"),
		message.LDAPString("+"),
	})
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

	attrs := searchResponseAttributes(entry, newSearchAttributeSelection(message.AttributeSelection{
		message.LDAPString("+"),
	}))

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
