package server

import (
	"testing"

	"github.com/lor00x/goldap/message"

	"github.com/smarzola/ldaplite/internal/models"
)

func TestAttributeValuesConvertsGoldapValues(t *testing.T) {
	got := attributeValues([]message.AttributeValue{
		message.AttributeValue("one"),
		message.AttributeValue("two"),
	})
	want := []string{"one", "two"}

	if len(got) != len(want) {
		t.Fatalf("attributeValues() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attributeValues()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNewAddEntryBuildsEntryFromAttributes(t *testing.T) {
	srv := &Server{}

	entry, resultCode, err := srv.newAddEntry("uid=jane,ou=users,dc=example,dc=com", map[string][]string{
		"objectClass": {"inetOrgPerson", "top"},
		"cn":          {"Jane Doe"},
		"mail":        {"jane@example.com", "j.doe@example.com"},
	})
	if err != nil {
		t.Fatalf("newAddEntry() failed: %v", err)
	}
	if resultCode != message.ResultCodeSuccess {
		t.Fatalf("resultCode = %d, want success", resultCode)
	}
	if entry.DN != "uid=jane,ou=users,dc=example,dc=com" {
		t.Fatalf("DN = %q", entry.DN)
	}
	if entry.ParentDN != "ou=users,dc=example,dc=com" {
		t.Fatalf("ParentDN = %q", entry.ParentDN)
	}
	if entry.ObjectClass != "inetOrgPerson" {
		t.Fatalf("ObjectClass = %q, want inetOrgPerson", entry.ObjectClass)
	}
	if got := entry.GetAttribute("cn"); got != "Jane Doe" {
		t.Fatalf("cn = %q, want Jane Doe", got)
	}
	if got := entry.GetAttributes("mail"); len(got) != 2 {
		t.Fatalf("mail values = %#v, want 2 values", got)
	}
}

func TestNewAddEntryRejectsProtectedAttributes(t *testing.T) {
	srv := &Server{}

	entry, resultCode, err := srv.newAddEntry("uid=jane,dc=example,dc=com", map[string][]string{
		"objectClass":     {"inetOrgPerson"},
		"createTimestamp": {"20260102030405Z"},
	})
	if err != nil {
		t.Fatalf("newAddEntry() error = %v", err)
	}
	if entry != nil {
		t.Fatalf("entry = %#v, want nil", entry)
	}
	if resultCode != message.ResultCodeUnwillingToPerform {
		t.Fatalf("resultCode = %d, want unwillingToPerform", resultCode)
	}
}

func TestNewAddEntryRequiresObjectClass(t *testing.T) {
	srv := &Server{}

	entry, resultCode, err := srv.newAddEntry("uid=jane,dc=example,dc=com", map[string][]string{
		"cn": {"Jane Doe"},
	})
	if err != nil {
		t.Fatalf("newAddEntry() error = %v", err)
	}
	if entry != nil {
		t.Fatalf("entry = %#v, want nil", entry)
	}
	if resultCode != message.ResultCodeObjectClassViolation {
		t.Fatalf("resultCode = %d, want objectClassViolation", resultCode)
	}
}

func TestDeleteModifyValues(t *testing.T) {
	entry := models.NewEntry("uid=jane,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.AddAttribute("mail", "jane@example.com")
	entry.AddAttribute("mail", "j.doe@example.com")
	entry.AddAttribute("mail", "jane@example.org")

	deleteModifyValues(entry, "mail", []string{"j.doe@example.com"})

	got := entry.GetAttributes("mail")
	want := []string{"jane@example.com", "jane@example.org"}
	if len(got) != len(want) {
		t.Fatalf("mail values = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mail[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDeleteModifyValuesWithoutSpecificValuesRemovesAttribute(t *testing.T) {
	entry := models.NewEntry("uid=jane,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.AddAttribute("mail", "jane@example.com")

	deleteModifyValues(entry, "mail", nil)

	if entry.HasAttribute("mail") {
		t.Fatal("mail should be removed")
	}
}

func TestReplaceModifyValues(t *testing.T) {
	entry := models.NewEntry("uid=jane,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.AddAttribute("mail", "old@example.com")

	srv := &Server{}
	if err := srv.replaceModifyValues(entry, "mail", []string{"new@example.com", "alt@example.com"}); err != nil {
		t.Fatalf("replaceModifyValues() failed: %v", err)
	}

	got := entry.GetAttributes("mail")
	want := []string{"new@example.com", "alt@example.com"}
	if len(got) != len(want) {
		t.Fatalf("mail values = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mail[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
