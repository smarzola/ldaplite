//go:build functional

package functional

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	ldap "github.com/go-ldap/ldap/v3"
)

const (
	ldifImportedUserDN = "uid=ldifuser," + usersOUDN
	ldifAppBindDN      = "uid=ldifapp," + usersOUDN
)

func TestLDIFImportFeedsRealLDAPServer(t *testing.T) {
	repoRoot := findRepoRoot(t)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "ldaplite.db")
	fixturePath := filepath.Join(tmpDir, "import.ldif")
	if err := os.WriteFile(fixturePath, []byte(`dn: uid=ldifuser,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: ldifuser
cn: LDIF User
sn: User
mail: ldifuser@example.com
userPassword: UserPassword123!

dn: uid=ldifapp,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: ldifapp
cn: LDIF App Bind
sn: Bind
userPassword: AppBindPassword123!

dn: cn=ldaplite.readonly,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: ldaplite.readonly
member: uid=ldifapp,ou=users,dc=example,dc=com`), 0600); err != nil {
		t.Fatalf("write LDIF fixture: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/ldaplite", "import", "ldif", "--file", fixturePath)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"LDAP_BASE_DN="+baseDN,
		"LDAP_ADMIN_PASSWORD="+adminPassword,
		"LDAP_DATABASE_PATH="+dbPath,
		"LDAP_ARGON2_MEMORY=64",
		"LDAP_ARGON2_ITERATIONS=1",
		"LDAP_ARGON2_PARALLELISM=1",
		"LDAP_ARGON2_SALT_LENGTH=8",
		"LDAP_ARGON2_KEY_LENGTH=16",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("import LDIF fixture: %v\n%s", err, output)
	}

	srv := startTestServerWithEnv(t, map[string]string{
		"LDAP_DATABASE_PATH":      dbPath,
		"LDAP_ARGON2_MEMORY":      "64",
		"LDAP_ARGON2_ITERATIONS":  "1",
		"LDAP_ARGON2_PARALLELISM": "1",
		"LDAP_ARGON2_SALT_LENGTH": "8",
		"LDAP_ARGON2_KEY_LENGTH":  "16",
	}, "ldap")

	assertBindSucceeds(t, srv, ldifImportedUserDN, "UserPassword123!")
	assertBindSucceeds(t, srv, ldifAppBindDN, "AppBindPassword123!")

	readOnlyConn := srv.dial(t)
	if err := readOnlyConn.Bind(ldifAppBindDN, "AppBindPassword123!"); err != nil {
		t.Fatalf("read-only app bind: %v", err)
	}
	res := search(t, readOnlyConn, "(uid=ldifuser)", []string{"cn", "mail"})
	entry := requireEntry(t, res, ldifImportedUserDN)
	assertAttrValues(t, entry, "cn", []string{"LDIF User"})
	assertAttrValues(t, entry, "mail", []string{"ldifuser@example.com"})

	add := ldap.NewAddRequest("uid=blocked,"+usersOUDN, nil)
	add.Attribute("objectClass", []string{"inetOrgPerson"})
	add.Attribute("uid", []string{"blocked"})
	add.Attribute("cn", []string{"Blocked User"})
	add.Attribute("sn", []string{"User"})
	add.Attribute("userPassword", []string{"Blocked123!"})
	assertLDAPResultCode(t, readOnlyConn.Add(add), ldap.LDAPResultInsufficientAccessRights)
}
