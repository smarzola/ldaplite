//go:build functional

package functional

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
)

const (
	baseDN        = "dc=example,dc=com"
	adminPassword = "ChangeMe123!"
	adminDN       = "uid=admin,ou=users," + baseDN
)

type testServer struct {
	URL     string
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	logs    *bytes.Buffer
	done    chan struct{}
	waitMu  sync.Mutex
	waitErr error
}

func startTestServer(t *testing.T) *testServer {
	t.Helper()

	repoRoot := findRepoRoot(t)
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "ldaplite")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/ldaplite")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build ldaplite: %v\n%s", err, output)
	}

	port := freeTCPPort(t)
	ctx, cancel := context.WithCancel(context.Background())
	logs := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, binaryPath, "server")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"LDAP_BASE_DN="+baseDN,
		"LDAP_ADMIN_PASSWORD="+adminPassword,
		fmt.Sprintf("LDAP_DATABASE_PATH=%s", filepath.Join(tmpDir, "ldaplite.db")),
		fmt.Sprintf("LDAP_PORT=%d", port),
		"LDAP_BIND_ADDRESS=127.0.0.1",
		"LDAP_LOG_LEVEL=debug",
		"LDAP_LOG_FORMAT=text",
		"LDAP_WEB_UI_ENABLED=false",
	)
	cmd.Stdout = logs
	cmd.Stderr = logs

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start ldaplite: %v", err)
	}

	done := make(chan struct{})
	srv := &testServer{
		URL:    fmt.Sprintf("ldap://127.0.0.1:%d", port),
		cmd:    cmd,
		cancel: cancel,
		logs:   logs,
		done:   done,
	}
	go func() {
		srv.setWaitErr(cmd.Wait())
		close(done)
	}()
	t.Cleanup(func() {
		srv.stop(t)
	})

	srv.waitReady(t)
	return srv
}

func (s *testServer) stop(t *testing.T) {
	t.Helper()

	if s.cmd == nil || s.cmd.Process == nil {
		return
	}

	s.cancel()

	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
		_ = s.cmd.Process.Kill()
		<-s.done
	}
}

func (s *testServer) setWaitErr(err error) {
	s.waitMu.Lock()
	defer s.waitMu.Unlock()
	s.waitErr = err
}

func (s *testServer) waitError() error {
	s.waitMu.Lock()
	defer s.waitMu.Unlock()
	return s.waitErr
}

func (s *testServer) waitReady(t *testing.T) {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		conn, err := ldap.DialURL(s.URL, ldap.DialWithDialer(&net.Dialer{Timeout: 500 * time.Millisecond}))
		if err == nil {
			conn.Close()
			return
		}
		lastErr = err

		select {
		case <-s.done:
			t.Fatalf("ldaplite exited before becoming ready: %v\nlast dial error: %v\nlogs:\n%s", s.waitError(), lastErr, s.logs.String())
		case <-ticker.C:
			if time.Now().After(deadline) {
				t.Fatalf("ldaplite did not become ready: %v\nlogs:\n%s", lastErr, s.logs.String())
			}
		}
	}
}

func (s *testServer) dial(t *testing.T) *ldap.Conn {
	t.Helper()

	conn, err := ldap.DialURL(s.URL, ldap.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}))
	if err != nil {
		t.Fatalf("dial %s: %v\nlogs:\n%s", s.URL, err, s.logs.String())
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})
	return conn
}

func bindAdmin(t *testing.T, conn *ldap.Conn) {
	t.Helper()
	if err := conn.Bind(adminDN, adminPassword); err != nil {
		t.Fatalf("admin bind: %v", err)
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free TCP port: %v", err)
	}
	defer ln.Close()

	return ln.Addr().(*net.TCPAddr).Port
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
