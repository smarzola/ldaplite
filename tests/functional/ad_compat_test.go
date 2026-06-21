//go:build functional

package functional

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"testing"

	ldap "github.com/go-ldap/ldap/v3"
)

const (
	usersOUDN  = "ou=users," + baseDN
	groupsOUDN = "ou=groups," + baseDN
	janeDN     = "uid=jane," + usersOUDN
	groupDN    = "cn=engineering," + groupsOUDN
)

func TestADLikeCompatibilityMilestone(t *testing.T) {
	srv := startTestServer(t)

	adminConn := srv.dial(t)
	bindAdmin(t, adminConn)
	createMilestoneFixture(t, adminConn)

	t.Run("bind compatibility", func(t *testing.T) {
		assertBindSucceeds(t, srv, adminDN, adminPassword)
		assertBindSucceeds(t, srv, janeDN, "Password123!")
		assertLDAPResultCode(t, bindErr(t, srv, janeDN, "WrongPassword123!"), ldap.LDAPResultInvalidCredentials)
		assertLDAPError(t, bindErr(t, srv, "uid=missing,"+usersOUDN, "Password123!"))
	})

	t.Run("search compatibility", func(t *testing.T) {
		conn := srv.dial(t)
		bindAdmin(t, conn)

		tests := []struct {
			name      string
			filter    string
			wantDNs   []string
			wantAttrs map[string][]string
		}{
			{
				name:    "all objects",
				filter:  "(objectClass=*)",
				wantDNs: []string{baseDN, usersOUDN, groupsOUDN, adminDN, "cn=ldaplite.admin," + groupsOUDN, janeDN, groupDN},
			},
			{
				name:    "uid",
				filter:  "(uid=jane)",
				wantDNs: []string{janeDN},
				wantAttrs: map[string][]string{
					"cn":                {"Jane Doe"},
					"sn":                {"Doe"},
					"mail":              {"jane@example.com"},
					"sAMAccountName":    {"jane"},
					"userPrincipalName": {"jane@example.com"},
				},
			},
			{
				name:    "cn",
				filter:  "(cn=Jane Doe)",
				wantDNs: []string{janeDN},
			},
			{
				name:    "mail",
				filter:  "(mail=jane@example.com)",
				wantDNs: []string{janeDN},
			},
			{
				name:    "sAMAccountName",
				filter:  "(sAMAccountName=jane)",
				wantDNs: []string{janeDN},
			},
			{
				name:    "userPrincipalName",
				filter:  "(userPrincipalName=jane@example.com)",
				wantDNs: []string{janeDN},
			},
			{
				name:    "and",
				filter:  "(&(objectClass=inetOrgPerson)(uid=jane))",
				wantDNs: []string{janeDN},
			},
			{
				name:    "or",
				filter:  "(|(uid=jane)(mail=jane@example.com))",
				wantDNs: []string{janeDN},
			},
			{
				name:    "not",
				filter:  "(!(uid=missing))",
				wantDNs: []string{baseDN, usersOUDN, groupsOUDN, adminDN, "cn=ldaplite.admin," + groupsOUDN, janeDN, groupDN},
			},
			{
				name:    "substring",
				filter:  "(cn=Jane*)",
				wantDNs: []string{janeDN},
			},
			{
				name:    "member",
				filter:  "(member=uid=jane,ou=users,dc=example,dc=com)",
				wantDNs: []string{groupDN},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				res := search(t, conn, tt.filter, []string{"*", "+"})
				assertDNs(t, res, tt.wantDNs)
				if len(tt.wantAttrs) > 0 {
					entry := requireEntry(t, res, janeDN)
					for attr, values := range tt.wantAttrs {
						assertAttrValues(t, entry, attr, values)
					}
				}
			})
		}

		literalStarDN := "uid=literalstar," + usersOUDN
		literalStarUser := ldap.NewAddRequest(literalStarDN, nil)
		literalStarUser.Attribute("objectClass", []string{"inetOrgPerson"})
		literalStarUser.Attribute("uid", []string{"literalstar"})
		literalStarUser.Attribute("cn", []string{"Literal * User"})
		literalStarUser.Attribute("sn", []string{"User"})
		literalStarUser.Attribute("userPassword", []string{"Password123!"})
		if err := conn.Add(literalStarUser); err != nil {
			t.Fatalf("add literal star user: %v", err)
		}
		t.Cleanup(func() {
			_ = conn.Del(ldap.NewDelRequest(literalStarDN, nil))
		})

		literalWildcardDN := "uid=literalwildcard," + usersOUDN
		literalWildcardUser := ldap.NewAddRequest(literalWildcardDN, nil)
		literalWildcardUser.Attribute("objectClass", []string{"inetOrgPerson"})
		literalWildcardUser.Attribute("uid", []string{"literalwildcard"})
		literalWildcardUser.Attribute("cn", []string{"Literal X User"})
		literalWildcardUser.Attribute("sn", []string{"User"})
		literalWildcardUser.Attribute("userPassword", []string{"Password123!"})
		if err := conn.Add(literalWildcardUser); err != nil {
			t.Fatalf("add literal wildcard comparison user: %v", err)
		}
		t.Cleanup(func() {
			_ = conn.Del(ldap.NewDelRequest(literalWildcardDN, nil))
		})

		res := search(t, conn, `(cn=Literal \2a User)`, []string{"cn"})
		assertDNs(t, res, []string{literalStarDN})

		res = search(t, conn, "(cn=Literal * User)", []string{"cn"})
		assertDNs(t, res, []string{literalStarDN, literalWildcardDN})
	})

	t.Run("attribute behavior", func(t *testing.T) {
		conn := srv.dial(t)
		bindAdmin(t, conn)
		res := search(t, conn, "(uid=jane)", []string{"*", "+"})
		entry := requireEntry(t, res, janeDN)

		assertNoAttr(t, entry, "userPassword")
		assertAttrValues(t, entry, "objectClass", []string{"inetOrgPerson"})
		assertTimestampAttr(t, entry, "createTimestamp")
		assertTimestampAttr(t, entry, "modifyTimestamp")
	})

	t.Run("escaped DN compatibility", func(t *testing.T) {
		conn := srv.dial(t)
		bindAdmin(t, conn)

		escapedDN := `uid=comma\,user,` + usersOUDN
		caseVariantDN := `UID=COMMA\,USER,OU=USERS,DC=EXAMPLE,DC=COM`

		user := ldap.NewAddRequest(escapedDN, nil)
		user.Attribute("objectClass", []string{"inetOrgPerson"})
		user.Attribute("uid", []string{"comma,user"})
		user.Attribute("cn", []string{"Comma User"})
		user.Attribute("sn", []string{"User"})
		user.Attribute("userPassword", []string{"CommaPassword123!"})
		if err := conn.Add(user); err != nil {
			t.Fatalf("add escaped DN user: %v", err)
		}

		res := search(t, conn, "(uid=comma,user)", []string{"*", "+"})
		assertDNs(t, res, []string{escapedDN})
		assertBindSucceeds(t, srv, caseVariantDN, "CommaPassword123!")

		if err := conn.Del(ldap.NewDelRequest(caseVariantDN, nil)); err != nil {
			t.Fatalf("delete escaped DN user by case variant: %v", err)
		}

		res = search(t, conn, "(uid=comma,user)", []string{"dn"})
		assertDNs(t, res, nil)
	})

	t.Run("modify compatibility", func(t *testing.T) {
		conn := srv.dial(t)
		bindAdmin(t, conn)

		modMail := ldap.NewModifyRequest(janeDN, nil)
		modMail.Replace("mail", []string{"jane.doe@example.com"})
		if err := conn.Modify(modMail); err != nil {
			t.Fatalf("modify mail: %v", err)
		}

		res := search(t, conn, "(uid=jane)", []string{"*", "+"})
		entry := requireEntry(t, res, janeDN)
		assertAttrValues(t, entry, "mail", []string{"jane.doe@example.com"})

		modPassword := ldap.NewModifyRequest(janeDN, nil)
		modPassword.Replace("userPassword", []string{"NewPassword123!"})
		if err := conn.Modify(modPassword); err != nil {
			t.Fatalf("modify password: %v", err)
		}

		assertLDAPResultCode(t, bindErr(t, srv, janeDN, "Password123!"), ldap.LDAPResultInvalidCredentials)
		assertBindSucceeds(t, srv, janeDN, "NewPassword123!")

		res = search(t, conn, "(uid=jane)", []string{"*", "+"})
		assertNoAttr(t, requireEntry(t, res, janeDN), "userPassword")
	})

	t.Run("error code compatibility", func(t *testing.T) {
		conn := srv.dial(t)
		bindAdmin(t, conn)

		unsupportedScheme := ldap.NewAddRequest("uid=ssha,"+usersOUDN, nil)
		unsupportedScheme.Attribute("objectClass", []string{"inetOrgPerson"})
		unsupportedScheme.Attribute("uid", []string{"ssha"})
		unsupportedScheme.Attribute("cn", []string{"Unsupported Scheme"})
		unsupportedScheme.Attribute("sn", []string{"Scheme"})
		unsupportedScheme.Attribute("userPassword", []string{"{SSHA}unsupported"})
		assertLDAPResultCode(t, conn.Add(unsupportedScheme), ldap.LDAPResultConstraintViolation)

		invalidUser := ldap.NewAddRequest("uid=invalid,"+usersOUDN, nil)
		invalidUser.Attribute("objectClass", []string{"inetOrgPerson"})
		invalidUser.Attribute("uid", []string{"invalid"})
		invalidUser.Attribute("cn", []string{"Invalid User"})
		invalidUser.Attribute("userPassword", []string{"Password123!"})
		assertLDAPResultCode(t, conn.Add(invalidUser), ldap.LDAPResultObjectClassViolation)

		invalidGroup := ldap.NewAddRequest("cn=invalid-group,"+groupsOUDN, nil)
		invalidGroup.Attribute("objectClass", []string{"groupOfNames"})
		invalidGroup.Attribute("cn", []string{"Invalid Group"})
		assertLDAPResultCode(t, conn.Add(invalidGroup), ldap.LDAPResultObjectClassViolation)
	})

	t.Run("delete compatibility", func(t *testing.T) {
		conn := srv.dial(t)
		bindAdmin(t, conn)

		if err := conn.Del(ldap.NewDelRequest(janeDN, nil)); err != nil {
			t.Fatalf("delete jane: %v", err)
		}

		res := search(t, conn, "(uid=jane)", []string{"*", "+"})
		assertDNs(t, res, nil)
		assertLDAPResultCode(t, bindErr(t, srv, janeDN, "NewPassword123!"), ldap.LDAPResultInvalidCredentials)
		assertLDAPResultCode(t, conn.Del(ldap.NewDelRequest(janeDN, nil)), ldap.LDAPResultNoSuchObject)
	})
}

func createMilestoneFixture(t *testing.T, conn *ldap.Conn) {
	t.Helper()

	user := ldap.NewAddRequest(janeDN, nil)
	user.Attribute("objectClass", []string{"inetOrgPerson"})
	user.Attribute("uid", []string{"jane"})
	user.Attribute("cn", []string{"Jane Doe"})
	user.Attribute("givenName", []string{"Jane"})
	user.Attribute("sn", []string{"Doe"})
	user.Attribute("mail", []string{"jane@example.com"})
	user.Attribute("sAMAccountName", []string{"jane"})
	user.Attribute("userPrincipalName", []string{"jane@example.com"})
	user.Attribute("userPassword", []string{"Password123!"})
	if err := conn.Add(user); err != nil {
		t.Fatalf("add jane fixture: %v", err)
	}

	group := ldap.NewAddRequest(groupDN, nil)
	group.Attribute("objectClass", []string{"groupOfNames"})
	group.Attribute("cn", []string{"engineering"})
	group.Attribute("member", []string{janeDN})
	if err := conn.Add(group); err != nil {
		t.Fatalf("add engineering group fixture: %v", err)
	}
}

func bindErr(t *testing.T, srv *testServer, dn, password string) error {
	t.Helper()
	conn := srv.dial(t)
	return conn.Bind(dn, password)
}

func assertBindSucceeds(t *testing.T, srv *testServer, dn, password string) {
	t.Helper()
	if err := bindErr(t, srv, dn, password); err != nil {
		t.Fatalf("bind %s: %v", dn, err)
	}
}

func search(t *testing.T, conn *ldap.Conn, filter string, attrs []string) *ldap.SearchResult {
	t.Helper()
	req := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		attrs,
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		t.Fatalf("search %s: %v", filter, err)
	}
	return res
}

func assertDNs(t *testing.T, res *ldap.SearchResult, want []string) {
	t.Helper()
	got := make([]string, 0, len(res.Entries))
	for _, entry := range res.Entries {
		got = append(got, strings.ToLower(entry.DN))
	}
	sort.Strings(got)

	wantLower := make([]string, 0, len(want))
	for _, dn := range want {
		wantLower = append(wantLower, strings.ToLower(dn))
	}
	sort.Strings(wantLower)

	if strings.Join(got, "\n") != strings.Join(wantLower, "\n") {
		t.Fatalf("DNs mismatch\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(wantLower, "\n"))
	}
}

func requireEntry(t *testing.T, res *ldap.SearchResult, dn string) *ldap.Entry {
	t.Helper()
	for _, entry := range res.Entries {
		if strings.EqualFold(entry.DN, dn) {
			return entry
		}
	}
	t.Fatalf("entry %s not found in search result", dn)
	return nil
}

func assertAttrValues(t *testing.T, entry *ldap.Entry, attr string, want []string) {
	t.Helper()
	got := attrValues(entry, attr)
	sort.Strings(got)
	want = append([]string(nil), want...)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%s values mismatch: got %v, want %v", attr, got, want)
	}
}

func assertNoAttr(t *testing.T, entry *ldap.Entry, attr string) {
	t.Helper()
	if values := attrValues(entry, attr); len(values) > 0 {
		t.Fatalf("attribute %s unexpectedly present with values %v", attr, values)
	}
}

func assertTimestampAttr(t *testing.T, entry *ldap.Entry, attr string) {
	t.Helper()
	values := attrValues(entry, attr)
	if len(values) != 1 {
		t.Fatalf("%s values = %v, want exactly one", attr, values)
	}
	if !regexp.MustCompile(`^\d{14}Z$`).MatchString(values[0]) {
		t.Fatalf("%s = %q, want LDAP generalized time YYYYMMDDHHMMSSZ", attr, values[0])
	}
}

func attrValues(entry *ldap.Entry, attr string) []string {
	for _, attribute := range entry.Attributes {
		if strings.EqualFold(attribute.Name, attr) {
			return append([]string(nil), attribute.Values...)
		}
	}
	return nil
}

func assertLDAPError(t *testing.T, err error) {
	t.Helper()
	var ldapErr *ldap.Error
	if !errors.As(err, &ldapErr) {
		t.Fatalf("error = %v, want *ldap.Error", err)
	}
}

func assertLDAPResultCode(t *testing.T, err error, want uint16) {
	t.Helper()
	var ldapErr *ldap.Error
	if !errors.As(err, &ldapErr) {
		t.Fatalf("error = %v, want LDAP result code %d", err, want)
	}
	if ldapErr.ResultCode != want {
		t.Fatalf("LDAP result code = %d, want %d; err=%v", ldapErr.ResultCode, want, err)
	}
}
