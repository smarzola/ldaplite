//go:build functional

package functional

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/url"
	"testing"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
)

func TestLDAPSTLSSidecarCompatibility(t *testing.T) {
	srv := startTestServer(t)

	ldapsURL := startTLSSidecar(t, srv.URL)
	conn, err := ldap.DialURL(ldapsURL, ldap.DialWithTLSConfig(&tls.Config{
		InsecureSkipVerify: true,
	}))
	if err != nil {
		t.Fatalf("dial TLS sidecar %s: %v", ldapsURL, err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	bindAdmin(t, conn)
	createMilestoneFixture(t, conn)

	res := search(t, conn, "(uid=jane)", []string{"uid", "mail"})
	assertDNs(t, res, []string{janeDN})
	assertAttrValues(t, requireEntry(t, res, janeDN), "mail", []string{"jane@example.com"})
}

func startTLSSidecar(t *testing.T, upstreamURL string) string {
	t.Helper()

	parsed, err := url.Parse(upstreamURL)
	if err != nil {
		t.Fatalf("parse upstream URL %q: %v", upstreamURL, err)
	}
	if parsed.Host == "" {
		t.Fatalf("upstream URL %q has no host", upstreamURL)
	}

	cert := selfSignedLocalhostCert(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("listen TLS sidecar: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
	})

	go func() {
		for {
			clientConn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					return
				}
			}
			go proxyLDAPConnection(ctx, clientConn, parsed.Host)
		}
	}()

	return "ldaps://" + ln.Addr().String()
}

func proxyLDAPConnection(ctx context.Context, clientConn net.Conn, upstreamAddr string) {
	defer clientConn.Close()

	upstreamConn, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", upstreamAddr)
	if err != nil {
		return
	}
	defer upstreamConn.Close()

	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(upstreamConn, clientConn)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(clientConn, upstreamConn)
		errCh <- err
	}()

	select {
	case <-ctx.Done():
	case <-errCh:
	}
}

func selfSignedLocalhostCert(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate TLS sidecar key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate TLS sidecar serial: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create TLS sidecar certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load TLS sidecar key pair: %v", err)
	}
	return cert
}
