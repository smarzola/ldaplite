package protocol

import (
	"testing"

	"github.com/lor00x/goldap/message"

	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
)

func TestFromGoldapMessageConvertsRequestFixtures(t *testing.T) {
	tests := []struct {
		name      string
		wire      []byte
		assertion func(*testing.T, *ldapmsg.Message)
	}{
		{
			name: "bind",
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
				if msg.ID != 1 || req.Name != "" || req.Password != "" {
					t.Fatalf("converted bind = %#v, message = %#v", req, msg)
				}
			},
		},
		{
			name: "search present",
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
				filter, ok := req.Filter.(ldapmsg.PresentFilter)
				if !ok {
					t.Fatalf("Filter = %T, want ldapmsg.PresentFilter", req.Filter)
				}
				if req.BaseObject != "dc=example,dc=com" || req.Scope != ldapmsg.SearchScopeWholeSubtree || filter.Attribute != "objectClass" {
					t.Fatalf("converted search = %#v", req)
				}
			},
		},
		{
			name: "add",
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
				if req.Entry != "uid=jane,ou=users,dc=example,dc=com" || len(req.Attributes) != 1 {
					t.Fatalf("converted add = %#v", req)
				}
				attr := req.Attributes[0]
				if attr.Name != "objectClass" || len(attr.Values) != 1 || attr.Values[0] != "inetOrgPerson" {
					t.Fatalf("converted add attribute = %#v", attr)
				}
			},
		},
		{
			name: "delete",
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
				if req.DN != "uid=jane,ou=users,dc=example,dc=com" {
					t.Fatalf("converted delete = %#v", req)
				}
			},
		},
		{
			name: "compare",
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
				req, ok := msg.Op.(ldapmsg.CompareRequest)
				if !ok {
					t.Fatalf("Op = %T, want ldapmsg.CompareRequest", msg.Op)
				}
				if req.Entry != "uid=jane,ou=users,dc=example,dc=com" || req.AVA.Attribute != "uid" || req.AVA.Value != "jane" {
					t.Fatalf("converted compare = %#v", req)
				}
			},
		},
		{
			name: "modify replace mail",
			wire: []byte{
				0x30, 0x4d,
				0x02, 0x01, 0x06,
				0x66, 0x48,
				0x04, 0x23,
				'u', 'i', 'd', '=', 'j', 'a', 'n', 'e', ',', 'o', 'u', '=', 'u', 's', 'e', 'r', 's',
				',', 'd', 'c', '=', 'e', 'x', 'a', 'm', 'p', 'l', 'e', ',', 'd', 'c', '=', 'c', 'o', 'm',
				0x30, 0x21,
				0x30, 0x1f,
				0x0a, 0x01, 0x02,
				0x30, 0x1a,
				0x04, 0x04, 'm', 'a', 'i', 'l',
				0x31, 0x12,
				0x04, 0x10, 'j', 'a', 'n', 'e', '@', 'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm',
			},
			assertion: func(t *testing.T, msg *ldapmsg.Message) {
				t.Helper()
				req, ok := msg.Op.(ldapmsg.ModifyRequest)
				if !ok {
					t.Fatalf("Op = %T, want ldapmsg.ModifyRequest", msg.Op)
				}
				if req.Object != "uid=jane,ou=users,dc=example,dc=com" || len(req.Changes) != 1 {
					t.Fatalf("converted modify = %#v", req)
				}
				change := req.Changes[0]
				if change.Operation != ldapmsg.ModifyOperationReplace || change.Modification.Name != "mail" {
					t.Fatalf("converted modify change = %#v", change)
				}
				if len(change.Modification.Values) != 1 || change.Modification.Values[0] != "jane@example.com" {
					t.Fatalf("converted modify values = %#v", change.Modification.Values)
				}
			},
		},
		{
			name: "extended whoami",
			wire: []byte{
				0x30, 0x1e,
				0x02, 0x01, 0x07,
				0x77, 0x19,
				0x80, 0x17,
				'1', '.', '3', '.', '6', '.', '1', '.', '4', '.', '1', '.', '4', '2', '0', '3', '.', '1', '.', '1', '1', '.', '3',
			},
			assertion: func(t *testing.T, msg *ldapmsg.Message) {
				t.Helper()
				req, ok := msg.Op.(ldapmsg.ExtendedRequest)
				if !ok {
					t.Fatalf("Op = %T, want ldapmsg.ExtendedRequest", msg.Op)
				}
				if req.RequestName != WhoAmIOID || req.RequestValue != nil {
					t.Fatalf("converted extended = %#v", req)
				}
			},
		},
		{
			name: "unbind",
			wire: []byte{
				0x30, 0x05,
				0x02, 0x01, 0x08,
				0x42, 0x00,
			},
			assertion: func(t *testing.T, msg *ldapmsg.Message) {
				t.Helper()
				if _, ok := msg.Op.(ldapmsg.UnbindRequest); !ok {
					t.Fatalf("Op = %T, want ldapmsg.UnbindRequest", msg.Op)
				}
			},
		},
		{
			name: "search and filter",
			wire: []byte{
				0x30, 0x45,
				0x02, 0x01, 0x09,
				0x63, 0x40,
				0x04, 0x11,
				'd', 'c', '=', 'e', 'x', 'a', 'm', 'p', 'l', 'e', ',', 'd', 'c', '=', 'c', 'o', 'm',
				0x0a, 0x01, 0x02,
				0x0a, 0x01, 0x00,
				0x02, 0x01, 0x00,
				0x02, 0x01, 0x00,
				0x01, 0x01, 0x00,
				0xa0, 0x1a,
				0xa3, 0x0b,
				0x04, 0x03, 'u', 'i', 'd',
				0x04, 0x04, 'j', 'a', 'n', 'e',
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
				filter, ok := req.Filter.(ldapmsg.AndFilter)
				if !ok {
					t.Fatalf("Filter = %T, want ldapmsg.AndFilter", req.Filter)
				}
				if len(filter.Filters) != 2 {
					t.Fatalf("len(and filters) = %d, want 2", len(filter.Filters))
				}
				eq, ok := filter.Filters[0].(ldapmsg.EqualityMatchFilter)
				if !ok || eq.Attribute != "uid" || eq.Value != "jane" {
					t.Fatalf("first and filter = %#v, want uid equality", filter.Filters[0])
				}
				present, ok := filter.Filters[1].(ldapmsg.PresentFilter)
				if !ok || present.Attribute != "objectClass" {
					t.Fatalf("second and filter = %#v, want objectClass present", filter.Filters[1])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goldapMsg := readGoldapMessageFixture(t, tt.wire)
			msg, err := FromGoldapMessage(goldapMsg)
			if err != nil {
				t.Fatalf("FromGoldapMessage() failed: %v", err)
			}
			tt.assertion(t, msg)
		})
	}
}

func readGoldapMessageFixture(t *testing.T, wire []byte) *message.LDAPMessage {
	t.Helper()

	bytes := message.NewBytes(0, wire)
	msg, err := message.ReadLDAPMessage(bytes)
	if err != nil {
		t.Fatalf("goldap ReadLDAPMessage() failed: %v", err)
	}

	return &msg
}
