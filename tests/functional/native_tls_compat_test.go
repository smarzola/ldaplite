//go:build functional

package functional

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeLDAPSCompatibility(t *testing.T) {
	certFile, keyFile := writeTestTLSFiles(t)
	srv := startTestServerWithEnv(t, map[string]string{
		"LDAP_TLS_ENABLED":   "true",
		"LDAP_TLS_CERT_FILE": certFile,
		"LDAP_TLS_KEY_FILE":  keyFile,
	}, "ldaps")

	conn := srv.dial(t)
	bindAdmin(t, conn)
	createMilestoneFixture(t, conn)

	res := search(t, conn, "(uid=jane)", []string{"uid", "mail"})
	assertDNs(t, res, []string{janeDN})
	assertAttrValues(t, requireEntry(t, res, janeDN), "mail", []string{"jane@example.com"})
}

func TestNativeStartTLSCompatibility(t *testing.T) {
	certFile, keyFile := writeTestTLSFiles(t)
	srv := startTestServerWithEnv(t, map[string]string{
		"LDAP_STARTTLS_ENABLED": "true",
		"LDAP_TLS_CERT_FILE":    certFile,
		"LDAP_TLS_KEY_FILE":     keyFile,
	}, "ldap")

	conn := srv.dial(t)
	if err := conn.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		t.Fatalf("StartTLS: %v", err)
	}
	bindAdmin(t, conn)
	createMilestoneFixture(t, conn)

	res := search(t, conn, "(uid=jane)", []string{"uid", "mail"})
	assertDNs(t, res, []string{janeDN})
	assertAttrValues(t, requireEntry(t, res, janeDN), "mail", []string{"jane@example.com"})
}

func writeTestTLSFiles(t *testing.T) (string, string) {
	t.Helper()

	cert := selfSignedLocalhostCert(t)
	key, ok := cert.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		t.Fatalf("test certificate private key has type %T, want *rsa.PrivateKey", cert.PrivateKey)
	}

	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "ldap.crt")
	keyFile := filepath.Join(tmpDir, "ldap.key")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatalf("write TLS certificate: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatalf("write TLS key: %v", err)
	}
	return certFile, keyFile
}
