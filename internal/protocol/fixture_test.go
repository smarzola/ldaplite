package protocol

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
)

func TestReadLDAPMessageDecodesRepresentativeRequestFixtures(t *testing.T) {
	tests := []struct {
		name      string
		wire      []byte
		assertion func(*testing.T, *ldapmsg.Message)
	}{
		{
			name: "bind simple empty credentials",
			wire: []byte{
				0x30, 0x0c,
				0x02, 0x01, 0x01,
				0x60, 0x07,
				0x02, 0x01, 0x03,
				0x04, 0x00,
				0x80, 0x00,
			},
			assertion: func(t *testing.T, msg *ldapmsg.Message) {
				t.Helper()
				req, ok := msg.Op.(ldapmsg.BindRequest)
				if !ok {
					t.Fatalf("Op = %T, want ldapmsg.BindRequest", msg.Op)
				}
				if got := msg.ID; got != 1 {
					t.Fatalf("ID = %d, want 1", got)
				}
				if got := req.Name; got != "" {
					t.Fatalf("Name = %q, want empty", got)
				}
				if got := req.Password; got != "" {
					t.Fatalf("Password = %q, want empty", got)
				}
			},
		},
		{
			name: "search subtree present objectClass",
			wire: []byte{
				0x30, 0x36,
				0x02, 0x01, 0x02,
				0x63, 0x31,
				0x04, 0x11,
				'd', 'c', '=', 'e', 'x', 'a', 'm', 'p', 'l', 'e', ',', 'd', 'c', '=', 'c', 'o', 'm',
				0x0a, 0x01, 0x02,
				0x0a, 0x01, 0x00,
				0x02, 0x01, 0x00,
				0x02, 0x01, 0x00,
				0x01, 0x01, 0x00,
				0x87, 0x0b,
				'o', 'b', 'j', 'e', 'c', 't', 'C', 'l', 'a', 's', 's',
				0x30, 0x00,
			},
			assertion: func(t *testing.T, msg *ldapmsg.Message) {
				t.Helper()
				req, ok := msg.Op.(ldapmsg.SearchRequest)
				if !ok {
					t.Fatalf("Op = %T, want ldapmsg.SearchRequest", msg.Op)
				}
				if got := msg.ID; got != 2 {
					t.Fatalf("ID = %d, want 2", got)
				}
				if got := req.BaseObject; got != "dc=example,dc=com" {
					t.Fatalf("BaseObject = %q, want dc=example,dc=com", got)
				}
				if got := req.Scope; got != ldapmsg.SearchScopeWholeSubtree {
					t.Fatalf("Scope = %d, want subtree", got)
				}
				if _, ok := req.Filter.(ldapmsg.PresentFilter); !ok {
					t.Fatalf("Filter = %T, want ldapmsg.PresentFilter", req.Filter)
				}
				if req.TypesOnly {
					t.Fatalf("TypesOnly = true, want false")
				}
				if got := len(req.Attributes); got != 0 {
					t.Fatalf("len(Attributes) = %d, want 0", got)
				}
			},
		},
		{
			name: "add single objectClass attribute",
			wire: []byte{
				0x30, 0x4c,
				0x02, 0x01, 0x03,
				0x68, 0x47,
				0x04, 0x23,
				'u', 'i', 'd', '=', 'j', 'a', 'n', 'e', ',', 'o', 'u', '=', 'u', 's', 'e', 'r', 's',
				',', 'd', 'c', '=', 'e', 'x', 'a', 'm', 'p', 'l', 'e', ',', 'd', 'c', '=', 'c', 'o', 'm',
				0x30, 0x20,
				0x30, 0x1e,
				0x04, 0x0b,
				'o', 'b', 'j', 'e', 'c', 't', 'C', 'l', 'a', 's', 's',
				0x31, 0x0f,
				0x04, 0x0d,
				'i', 'n', 'e', 't', 'O', 'r', 'g', 'P', 'e', 'r', 's', 'o', 'n',
			},
			assertion: func(t *testing.T, msg *ldapmsg.Message) {
				t.Helper()
				req, ok := msg.Op.(ldapmsg.AddRequest)
				if !ok {
					t.Fatalf("Op = %T, want ldapmsg.AddRequest", msg.Op)
				}
				if got := req.Entry; got != "uid=jane,ou=users,dc=example,dc=com" {
					t.Fatalf("Entry = %q, want jane DN", got)
				}
				attrs := req.Attributes
				if len(attrs) != 1 {
					t.Fatalf("len(Attributes) = %d, want 1", len(attrs))
				}
				if got := attrs[0].Name; got != "objectClass" {
					t.Fatalf("attribute Name = %q, want objectClass", got)
				}
				if got := attrs[0].Values; len(got) != 1 || got[0] != "inetOrgPerson" {
					t.Fatalf("attribute values = %v, want [inetOrgPerson]", got)
				}
			},
		},
		{
			name: "delete jane",
			wire: []byte{
				0x30, 0x28,
				0x02, 0x01, 0x04,
				0x4a, 0x23,
				'u', 'i', 'd', '=', 'j', 'a', 'n', 'e', ',', 'o', 'u', '=', 'u', 's', 'e', 'r', 's',
				',', 'd', 'c', '=', 'e', 'x', 'a', 'm', 'p', 'l', 'e', ',', 'd', 'c', '=', 'c', 'o', 'm',
			},
			assertion: func(t *testing.T, msg *ldapmsg.Message) {
				t.Helper()
				req, ok := msg.Op.(ldapmsg.DeleteRequest)
				if !ok {
					t.Fatalf("Op = %T, want ldapmsg.DeleteRequest", msg.Op)
				}
				if got := req.DN; got != "uid=jane,ou=users,dc=example,dc=com" {
					t.Fatalf("DeleteRequest DN = %q, want jane DN", got)
				}
			},
		},
		{
			name: "compare uid jane",
			wire: []byte{
				0x30, 0x37,
				0x02, 0x01, 0x05,
				0x6e, 0x32,
				0x04, 0x23,
				'u', 'i', 'd', '=', 'j', 'a', 'n', 'e', ',', 'o', 'u', '=', 'u', 's', 'e', 'r', 's',
				',', 'd', 'c', '=', 'e', 'x', 'a', 'm', 'p', 'l', 'e', ',', 'd', 'c', '=', 'c', 'o', 'm',
				0x30, 0x0b,
				0x04, 0x03, 'u', 'i', 'd',
				0x04, 0x04, 'j', 'a', 'n', 'e',
			},
			assertion: func(t *testing.T, msg *ldapmsg.Message) {
				t.Helper()
				if _, ok := msg.Op.(ldapmsg.CompareRequest); !ok {
					t.Fatalf("Op = %T, want ldapmsg.CompareRequest", msg.Op)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := readLDAPFixture(t, tt.wire)
			tt.assertion(t, msg)
		})
	}
}

func TestResponseHelpersEncodeExactBERFixtures(t *testing.T) {
	tests := []struct {
		name     string
		response ldapmsg.Operation
		want     []byte
	}{
		{
			name:     "bind success",
			response: NewBindResponse(ldapmsg.ResultCodeSuccess),
			want: []byte{
				0x30, 0x0c,
				0x02, 0x01, 0x01,
				0x61, 0x07,
				0x0a, 0x01, 0x00,
				0x04, 0x00,
				0x04, 0x00,
			},
		},
		{
			name:     "search done success",
			response: NewSearchResultDone(ldapmsg.ResultCodeSuccess),
			want: []byte{
				0x30, 0x0c,
				0x02, 0x01, 0x01,
				0x65, 0x07,
				0x0a, 0x01, 0x00,
				0x04, 0x00,
				0x04, 0x00,
			},
		},
		{
			name:     "add success",
			response: NewAddResponse(ldapmsg.ResultCodeSuccess),
			want: []byte{
				0x30, 0x0c,
				0x02, 0x01, 0x01,
				0x69, 0x07,
				0x0a, 0x01, 0x00,
				0x04, 0x00,
				0x04, 0x00,
			},
		},
		{
			name:     "modify success",
			response: NewModifyResponse(ldapmsg.ResultCodeSuccess),
			want: []byte{
				0x30, 0x0c,
				0x02, 0x01, 0x01,
				0x67, 0x07,
				0x0a, 0x01, 0x00,
				0x04, 0x00,
				0x04, 0x00,
			},
		},
		{
			name:     "delete success",
			response: NewDelResponse(ldapmsg.ResultCodeSuccess),
			want: []byte{
				0x30, 0x0c,
				0x02, 0x01, 0x01,
				0x6b, 0x07,
				0x0a, 0x01, 0x00,
				0x04, 0x00,
				0x04, 0x00,
			},
		},
		{
			name:     "compare false",
			response: NewCompareResponse(ldapmsg.ResultCodeCompareFalse),
			want: []byte{
				0x30, 0x0c,
				0x02, 0x01, 0x01,
				0x6f, 0x07,
				0x0a, 0x01, 0x05,
				0x04, 0x00,
				0x04, 0x00,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeProtocolOpFixture(t, tt.response)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("encoded BER = %x, want %x", got, tt.want)
			}
		})
	}
}

func TestWhoAmIResponseEncodesExactBERFixture(t *testing.T) {
	resp := NewWhoAmIResponse("dn:uid=admin,ou=users,dc=example,dc=com")

	want := []byte{
		0x30, 0x4e,
		0x02, 0x01, 0x01,
		0x78, 0x49,
		0x0a, 0x01, 0x00,
		0x04, 0x00,
		0x04, 0x00,
		0x8a, 0x17,
		'1', '.', '3', '.', '6', '.', '1', '.', '4', '.', '1', '.', '4', '2', '0', '3', '.', '1', '.', '1', '1', '.', '3',
		0x8b, 0x27,
		'd', 'n', ':', 'u', 'i', 'd', '=', 'a', 'd', 'm', 'i', 'n', ',', 'o', 'u', '=', 'u', 's', 'e', 'r', 's',
		',', 'd', 'c', '=', 'e', 'x', 'a', 'm', 'p', 'l', 'e', ',', 'd', 'c', '=', 'c', 'o', 'm',
	}

	got := encodeProtocolOpFixture(t, resp)
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded WhoAmI BER = %x, want %x", got, want)
	}
}

func readLDAPFixture(t *testing.T, wire []byte) *ldapmsg.Message {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	writeDone := make(chan error, 1)
	go func() {
		_, err := clientConn.Write(wire)
		if closeErr := clientConn.Close(); err == nil {
			err = closeErr
		}
		writeDone <- err
	}()

	goldapMsg, err := ReadLDAPMessage(serverConn)
	if err != nil {
		t.Fatalf("ReadLDAPMessage() failed: %v", err)
	}
	msg, err := FromGoldapMessage(goldapMsg)
	if err != nil {
		t.Fatalf("FromGoldapMessage() failed: %v", err)
	}

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("client write fixture failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("fixture writer did not finish")
	}

	return msg
}

func encodeProtocolOpFixture(t *testing.T, op ldapmsg.Operation) []byte {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	writeDone := make(chan error, 1)
	go func() {
		writeDone <- WriteLDAPResponse(serverConn, 1, op)
	}()

	gotCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		var buf bytes.Buffer
		_, err := buf.ReadFrom(clientConn)
		if closeErr := clientConn.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			errCh <- err
			return
		}
		gotCh <- buf.Bytes()
	}()

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("WriteLDAPResponse() failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WriteLDAPResponse() did not finish")
	}

	_ = serverConn.Close()

	select {
	case err := <-errCh:
		t.Fatalf("client read failed: %v", err)
	case got := <-gotCh:
		return got
	case <-time.After(time.Second):
		t.Fatal("fixture reader did not finish")
	}

	panic("unreachable")
}
