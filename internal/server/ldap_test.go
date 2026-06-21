package server

import (
	"net"
	"testing"
	"time"

	"github.com/smarzola/ldaplite/pkg/config"
)

func TestHandleConnectionClosesIdleConnOnServerCancel(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	srv := NewServer(&config.Config{
		Security: config.SecurityConfig{
			Argon2Config: config.Argon2Config{
				Memory:      1,
				Iterations:  1,
				Parallelism: 1,
				SaltLength:  1,
				KeyLength:   1,
			},
		},
	}, nil, "test")

	done := make(chan struct{})
	go func() {
		srv.handleConnection(serverConn)
		close(done)
	}()

	srv.cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleConnection did not exit after server context cancellation")
	}
}
