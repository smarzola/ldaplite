package protocol

import (
	"net"
	"testing"
	"time"

	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
)

func TestDecodeLDAPMessageRejectsMalformedBER(t *testing.T) {
	tests := []struct {
		name string
		wire []byte
	}{
		{
			name: "truncated outer message",
			wire: []byte{0x30, 0x0c, 0x02, 0x01, 0x01},
		},
		{
			name: "indefinite length",
			wire: []byte{0x30, 0x80},
		},
		{
			name: "unsupported protocol op",
			wire: []byte{
				0x30, 0x05,
				0x02, 0x01, 0x01,
				0x50, 0x00,
			},
		},
		{
			name: "search missing fields",
			wire: []byte{
				0x30, 0x07,
				0x02, 0x01, 0x02,
				0x63, 0x02,
				0x04, 0x00,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeLDAPMessage(tt.wire); err == nil {
				t.Fatal("DecodeLDAPMessage() succeeded, want error")
			}
		})
	}
}

func TestReadLDAPMessageReadsPartialTCPMessage(t *testing.T) {
	wire := []byte{
		0x30, 0x0c,
		0x02, 0x01, 0x01,
		0x60, 0x07,
		0x02, 0x01, 0x03,
		0x04, 0x00,
		0x80, 0x00,
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	writeDone := make(chan error, 1)
	go func() {
		_, err := clientConn.Write(wire[:5])
		if err == nil {
			_, err = clientConn.Write(wire[5:])
		}
		if closeErr := clientConn.Close(); err == nil {
			err = closeErr
		}
		writeDone <- err
	}()

	msg, err := ReadLDAPMessage(serverConn)
	if err != nil {
		t.Fatalf("ReadLDAPMessage() failed: %v", err)
	}
	if msg.ID != 1 {
		t.Fatalf("ID = %d, want 1", msg.ID)
	}
	if _, ok := msg.Op.(ldapmsg.BindRequest); !ok {
		t.Fatalf("Op = %T, want ldapmsg.BindRequest", msg.Op)
	}

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("client write fixture failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("fixture writer did not finish")
	}
}
