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
