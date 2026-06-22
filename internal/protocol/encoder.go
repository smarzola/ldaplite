package protocol

import (
	"fmt"
	"net"

	"github.com/smarzola/ldaplite/internal/protocol/ber"
	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
)

const (
	tagBindResponse      byte = 0x61
	tagSearchResultEntry byte = 0x64
	tagSearchResultDone  byte = 0x65
	tagModifyResponse    byte = 0x67
	tagAddResponse       byte = 0x69
	tagDelResponse       byte = 0x6b
	tagCompareResponse   byte = 0x6f
	tagExtendedResponse  byte = 0x78
	tagResponseName      byte = 0x8a
	tagResponseValue     byte = 0x8b
)

func WriteLDAPResponse(conn net.Conn, messageID ldapmsg.MessageID, op ldapmsg.Operation) error {
	data, err := EncodeLDAPResponse(messageID, op)
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to connection: %w", err)
	}
	return nil
}

func EncodeLDAPResponse(messageID ldapmsg.MessageID, op ldapmsg.Operation) ([]byte, error) {
	protocolOp, err := encodeResponseProtocolOp(op)
	if err != nil {
		return nil, err
	}
	return ber.Sequence(
		ber.Integer(int(messageID)),
		protocolOp,
	), nil
}

func encodeResponseProtocolOp(op ldapmsg.Operation) ([]byte, error) {
	switch resp := op.(type) {
	case ldapmsg.BindResponse:
		return encodeLDAPResult(tagBindResponse, resp.LDAPResult), nil
	case ldapmsg.SearchResultEntry:
		return encodeSearchResultEntry(resp), nil
	case ldapmsg.SearchResultDone:
		return encodeLDAPResult(tagSearchResultDone, resp.LDAPResult), nil
	case ldapmsg.AddResponse:
		return encodeLDAPResult(tagAddResponse, resp.LDAPResult), nil
	case ldapmsg.ModifyResponse:
		return encodeLDAPResult(tagModifyResponse, resp.LDAPResult), nil
	case ldapmsg.DeleteResponse:
		return encodeLDAPResult(tagDelResponse, resp.LDAPResult), nil
	case ldapmsg.CompareResponse:
		return encodeLDAPResult(tagCompareResponse, resp.LDAPResult), nil
	case ldapmsg.ExtendedResponse:
		return encodeExtendedResponse(resp), nil
	default:
		return nil, fmt.Errorf("unsupported LDAP response operation %T", op)
	}
}

func encodeLDAPResult(tag byte, result ldapmsg.LDAPResult) []byte {
	return ber.TLV(tag, concatBER(
		ber.Enumerated(int(result.ResultCode)),
		ber.OctetString(result.MatchedDN),
		ber.OctetString(result.DiagnosticMessage),
	))
}

func encodeSearchResultEntry(entry ldapmsg.SearchResultEntry) []byte {
	attrs := make([][]byte, 0, len(entry.Attributes))
	for _, attr := range entry.Attributes {
		values := make([][]byte, 0, len(attr.Values))
		for _, value := range attr.Values {
			values = append(values, ber.OctetString(value))
		}
		attrs = append(attrs, ber.Sequence(
			ber.OctetString(attr.Name),
			ber.Set(values...),
		))
	}

	return ber.TLV(tagSearchResultEntry, concatBER(
		ber.OctetString(entry.ObjectName),
		ber.Sequence(attrs...),
	))
}

func encodeExtendedResponse(resp ldapmsg.ExtendedResponse) []byte {
	fields := [][]byte{
		ber.Enumerated(int(resp.ResultCode)),
		ber.OctetString(resp.MatchedDN),
		ber.OctetString(resp.DiagnosticMessage),
	}
	if resp.ResponseName != "" {
		fields = append(fields, ber.TLV(tagResponseName, []byte(resp.ResponseName)))
	}
	if resp.ResponseValue != nil {
		fields = append(fields, ber.TLV(tagResponseValue, []byte(*resp.ResponseValue)))
	}
	return ber.TLV(tagExtendedResponse, concatBER(fields...))
}

func concatBER(parts ...[]byte) []byte {
	size := 0
	for _, part := range parts {
		size += len(part)
	}
	out := make([]byte, 0, size)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}
