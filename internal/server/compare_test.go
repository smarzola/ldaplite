package server

import (
	"testing"
	"time"

	"github.com/smarzola/ldaplite/internal/models"
)

func TestCompareEntryAttribute(t *testing.T) {
	entry := models.NewEntry("uid=jane,ou=users,dc=example,dc=com", "inetOrgPerson")
	entry.SetAttributes("cn", []string{"Jane Doe", "J. Doe"})
	entry.SetAttribute("userPassword", "{ARGON2ID}redacted")
	entry.SetComputedAttributes("memberOf", []string{"cn=engineering,ou=groups,dc=example,dc=com"})
	entry.CreatedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	entry.UpdatedAt = time.Date(2026, 1, 2, 4, 5, 6, 0, time.UTC)

	tests := []struct {
		name  string
		attr  string
		value string
		want  bool
	}{
		{name: "ordinary true", attr: "cn", value: "Jane Doe", want: true},
		{name: "ordinary case insensitive", attr: "CN", value: "jane doe", want: true},
		{name: "ordinary false", attr: "cn", value: "Jane Other", want: false},
		{name: "missing attribute false", attr: "mail", value: "jane@example.com", want: false},
		{name: "objectClass true", attr: "objectClass", value: "inetOrgPerson", want: true},
		{name: "createTimestamp true", attr: "createTimestamp", value: "20260102030405Z", want: true},
		{name: "modifyTimestamp true", attr: "modifyTimestamp", value: "20260102040506Z", want: true},
		{name: "computed memberOf true", attr: "memberOf", value: "cn=engineering,ou=groups,dc=example,dc=com", want: true},
		{name: "password always false", attr: "userPassword", value: "{ARGON2ID}redacted", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareEntryAttribute(entry, tt.attr, tt.value); got != tt.want {
				t.Fatalf("compareEntryAttribute(%q, %q) = %v, want %v", tt.attr, tt.value, got, tt.want)
			}
		})
	}
}
