//go:build functional

package functional

import (
	"regexp"
	"testing"

	ldap "github.com/go-ldap/ldap/v3"
)

const pocketIDAdminGroupDN = "cn=_pocket_id_admins," + groupsOUDN

func TestPocketIDLDAPCompatibility(t *testing.T) {
	srv := startTestServer(t)

	conn := srv.dial(t)
	bindAdmin(t, conn)
	createMilestoneFixture(t, conn)
	createPocketIDAdminGroupFixture(t, conn)

	t.Run("bind user can read sync attributes", func(t *testing.T) {
		res := search(t, conn, "(&(objectClass=inetOrgPerson)(uid=jane))", []string{
			"uuid",
			"uid",
			"mail",
			"givenName",
			"sn",
			"memberOf",
		})
		assertDNs(t, res, []string{janeDN})

		entry := requireEntry(t, res, janeDN)
		assertPocketIDUUID(t, entry, "uuid")
		assertAttrValues(t, entry, "uid", []string{"jane"})
		assertAttrValues(t, entry, "mail", []string{"jane@example.com"})
		assertAttrValues(t, entry, "givenName", []string{"Jane"})
		assertAttrValues(t, entry, "sn", []string{"Doe"})
		assertAttrValues(t, entry, "memberOf", []string{groupDN, pocketIDAdminGroupDN})
	})

	t.Run("group sync can resolve members", func(t *testing.T) {
		res := search(t, conn, "(objectClass=groupOfNames)", []string{
			"uuid",
			"cn",
			"member",
		})
		assertDNs(t, res, []string{"cn=ldaplite.admin," + groupsOUDN, groupDN, pocketIDAdminGroupDN})

		group := requireEntry(t, res, pocketIDAdminGroupDN)
		assertPocketIDUUID(t, group, "uuid")
		assertAttrValues(t, group, "cn", []string{"_pocket_id_admins"})
		assertAttrValues(t, group, "member", []string{janeDN})

		members := attrValues(group, "member")
		if len(members) != 1 {
			t.Fatalf("Pocket ID admin group members = %v, want exactly jane", members)
		}

		memberRes := search(t, conn, "(uid=jane)", []string{"uuid", "uid", "mail"})
		assertDNs(t, memberRes, []string{members[0]})
		requireEntry(t, memberRes, members[0])
	})
}

func createPocketIDAdminGroupFixture(t *testing.T, conn *ldap.Conn) {
	t.Helper()

	group := ldap.NewAddRequest(pocketIDAdminGroupDN, nil)
	group.Attribute("objectClass", []string{"groupOfNames"})
	group.Attribute("cn", []string{"_pocket_id_admins"})
	group.Attribute("member", []string{janeDN})
	if err := conn.Add(group); err != nil {
		t.Fatalf("add Pocket ID admin group fixture: %v", err)
	}
}

func assertPocketIDUUID(t *testing.T, entry *ldap.Entry, attr string) {
	t.Helper()
	values := attrValues(entry, attr)
	if len(values) != 1 {
		t.Fatalf("%s values = %v, want exactly one", attr, values)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(values[0]) {
		t.Fatalf("%s = %q, want RFC 4122 version 4 UUID", attr, values[0])
	}
}
