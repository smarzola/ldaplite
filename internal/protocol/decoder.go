package protocol

import (
	"fmt"

	"github.com/smarzola/ldaplite/internal/protocol/ber"
	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
)

const (
	tagBindRequest     byte = 0x60
	tagSearchRequest   byte = 0x63
	tagModifyRequest   byte = 0x66
	tagAddRequest      byte = 0x68
	tagDelRequest      byte = 0x4a
	tagCompareRequest  byte = 0x6e
	tagUnbindRequest   byte = 0x42
	tagExtendedRequest byte = 0x77

	tagSimpleAuth           byte = 0x80
	tagExtendedRequestName  byte = 0x80
	tagExtendedRequestValue byte = 0x81

	tagFilterAnd            byte = 0xa0
	tagFilterOr             byte = 0xa1
	tagFilterNot            byte = 0xa2
	tagFilterEqualityMatch  byte = 0xa3
	tagFilterSubstrings     byte = 0xa4
	tagFilterGreaterOrEqual byte = 0xa5
	tagFilterLessOrEqual    byte = 0xa6
	tagFilterPresent        byte = 0x87
	tagFilterApproxMatch    byte = 0xa8

	tagSubstringInitial byte = 0x80
	tagSubstringAny     byte = 0x81
	tagSubstringFinal   byte = 0x82
)

func DecodeLDAPMessage(data []byte) (*ldapmsg.Message, error) {
	packet, n, err := ber.ReadPacket(data)
	if err != nil {
		return nil, err
	}
	if n != len(data) {
		return nil, fmt.Errorf("LDAP message has %d trailing bytes", len(data)-n)
	}
	if err := packet.RequireTag(ber.ClassUniversal | ber.Constructed | ber.TagSequence); err != nil {
		return nil, fmt.Errorf("LDAP message: %w", err)
	}
	if len(packet.Children) < 2 {
		return nil, fmt.Errorf("LDAP message has %d fields, want at least 2", len(packet.Children))
	}
	if err := packet.Children[0].RequireTag(ber.ClassUniversal | ber.TagInteger); err != nil {
		return nil, fmt.Errorf("LDAP messageID: %w", err)
	}
	messageID, err := packet.Children[0].Int()
	if err != nil {
		return nil, fmt.Errorf("LDAP messageID: %w", err)
	}

	op, err := decodeProtocolOp(packet.Children[1])
	if err != nil {
		return nil, err
	}

	return &ldapmsg.Message{ID: ldapmsg.MessageID(messageID), Op: op}, nil
}

func decodeProtocolOp(packet ber.Packet) (ldapmsg.Operation, error) {
	switch packet.Tag {
	case tagBindRequest:
		return decodeBindRequest(packet)
	case tagSearchRequest:
		return decodeSearchRequest(packet)
	case tagAddRequest:
		return decodeAddRequest(packet)
	case tagModifyRequest:
		return decodeModifyRequest(packet)
	case tagDelRequest:
		return ldapmsg.DeleteRequest{DN: packet.String()}, nil
	case tagCompareRequest:
		return decodeCompareRequest(packet)
	case tagExtendedRequest:
		return decodeExtendedRequest(packet)
	case tagUnbindRequest:
		if len(packet.Value) != 0 {
			return nil, fmt.Errorf("unbind request value length %d, want 0", len(packet.Value))
		}
		return ldapmsg.UnbindRequest{}, nil
	default:
		return nil, fmt.Errorf("unsupported LDAP protocol op tag 0x%02x", packet.Tag)
	}
}

func decodeBindRequest(packet ber.Packet) (ldapmsg.BindRequest, error) {
	if len(packet.Children) != 3 {
		return ldapmsg.BindRequest{}, fmt.Errorf("bind request has %d fields, want 3", len(packet.Children))
	}
	if err := packet.Children[0].RequireTag(ber.ClassUniversal | ber.TagInteger); err != nil {
		return ldapmsg.BindRequest{}, fmt.Errorf("bind version: %w", err)
	}
	if err := packet.Children[1].RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
		return ldapmsg.BindRequest{}, fmt.Errorf("bind name: %w", err)
	}
	if err := packet.Children[2].RequireTag(tagSimpleAuth); err != nil {
		return ldapmsg.BindRequest{}, fmt.Errorf("bind simple auth: %w", err)
	}
	return ldapmsg.BindRequest{
		Name:     packet.Children[1].String(),
		Password: packet.Children[2].String(),
	}, nil
}

func decodeSearchRequest(packet ber.Packet) (ldapmsg.SearchRequest, error) {
	if len(packet.Children) != 8 {
		return ldapmsg.SearchRequest{}, fmt.Errorf("search request has %d fields, want 8", len(packet.Children))
	}
	if err := packet.Children[0].RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
		return ldapmsg.SearchRequest{}, fmt.Errorf("search base object: %w", err)
	}
	if err := packet.Children[1].RequireTag(ber.ClassUniversal | ber.TagEnumerated); err != nil {
		return ldapmsg.SearchRequest{}, fmt.Errorf("search scope: %w", err)
	}
	scope, err := packet.Children[1].Int()
	if err != nil {
		return ldapmsg.SearchRequest{}, fmt.Errorf("search scope: %w", err)
	}
	if err := packet.Children[5].RequireTag(ber.ClassUniversal | ber.TagBoolean); err != nil {
		return ldapmsg.SearchRequest{}, fmt.Errorf("search typesOnly: %w", err)
	}
	typesOnly, err := packet.Children[5].Bool()
	if err != nil {
		return ldapmsg.SearchRequest{}, fmt.Errorf("search typesOnly: %w", err)
	}
	filter, err := decodeFilter(packet.Children[6])
	if err != nil {
		return ldapmsg.SearchRequest{}, fmt.Errorf("search filter: %w", err)
	}
	attrs, err := decodeStringSequence(packet.Children[7])
	if err != nil {
		return ldapmsg.SearchRequest{}, fmt.Errorf("search attributes: %w", err)
	}
	return ldapmsg.SearchRequest{
		BaseObject: packet.Children[0].String(),
		Scope:      ldapmsg.SearchScope(scope),
		TypesOnly:  typesOnly,
		Filter:     filter,
		Attributes: attrs,
	}, nil
}

func decodeAddRequest(packet ber.Packet) (ldapmsg.AddRequest, error) {
	if len(packet.Children) != 2 {
		return ldapmsg.AddRequest{}, fmt.Errorf("add request has %d fields, want 2", len(packet.Children))
	}
	if err := packet.Children[0].RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
		return ldapmsg.AddRequest{}, fmt.Errorf("add entry: %w", err)
	}
	attrs, err := decodeAttributes(packet.Children[1])
	if err != nil {
		return ldapmsg.AddRequest{}, err
	}
	return ldapmsg.AddRequest{Entry: packet.Children[0].String(), Attributes: attrs}, nil
}

func decodeModifyRequest(packet ber.Packet) (ldapmsg.ModifyRequest, error) {
	if len(packet.Children) != 2 {
		return ldapmsg.ModifyRequest{}, fmt.Errorf("modify request has %d fields, want 2", len(packet.Children))
	}
	if err := packet.Children[0].RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
		return ldapmsg.ModifyRequest{}, fmt.Errorf("modify object: %w", err)
	}
	if err := packet.Children[1].RequireTag(ber.ClassUniversal | ber.Constructed | ber.TagSequence); err != nil {
		return ldapmsg.ModifyRequest{}, fmt.Errorf("modify changes: %w", err)
	}
	changes := make([]ldapmsg.ModifyChange, 0, len(packet.Children[1].Children))
	for _, child := range packet.Children[1].Children {
		if len(child.Children) != 2 {
			return ldapmsg.ModifyRequest{}, fmt.Errorf("modify change has %d fields, want 2", len(child.Children))
		}
		if err := child.Children[0].RequireTag(ber.ClassUniversal | ber.TagEnumerated); err != nil {
			return ldapmsg.ModifyRequest{}, fmt.Errorf("modify operation: %w", err)
		}
		op, err := child.Children[0].Int()
		if err != nil {
			return ldapmsg.ModifyRequest{}, fmt.Errorf("modify operation: %w", err)
		}
		attr, err := decodeAttribute(child.Children[1])
		if err != nil {
			return ldapmsg.ModifyRequest{}, err
		}
		changes = append(changes, ldapmsg.ModifyChange{
			Operation:    ldapmsg.ModifyOperation(op),
			Modification: attr,
		})
	}
	return ldapmsg.ModifyRequest{Object: packet.Children[0].String(), Changes: changes}, nil
}

func decodeCompareRequest(packet ber.Packet) (ldapmsg.CompareRequest, error) {
	if len(packet.Children) != 2 {
		return ldapmsg.CompareRequest{}, fmt.Errorf("compare request has %d fields, want 2", len(packet.Children))
	}
	if err := packet.Children[0].RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
		return ldapmsg.CompareRequest{}, fmt.Errorf("compare entry: %w", err)
	}
	ava, err := decodeAVA(packet.Children[1])
	if err != nil {
		return ldapmsg.CompareRequest{}, err
	}
	return ldapmsg.CompareRequest{Entry: packet.Children[0].String(), AVA: ava}, nil
}

func decodeExtendedRequest(packet ber.Packet) (ldapmsg.ExtendedRequest, error) {
	if len(packet.Children) == 0 {
		return ldapmsg.ExtendedRequest{}, fmt.Errorf("extended request missing requestName")
	}
	if err := packet.Children[0].RequireTag(tagExtendedRequestName); err != nil {
		return ldapmsg.ExtendedRequest{}, fmt.Errorf("extended requestName: %w", err)
	}
	var value *string
	if len(packet.Children) > 1 {
		if err := packet.Children[1].RequireTag(tagExtendedRequestValue); err != nil {
			return ldapmsg.ExtendedRequest{}, fmt.Errorf("extended requestValue: %w", err)
		}
		v := packet.Children[1].String()
		value = &v
	}
	return ldapmsg.ExtendedRequest{RequestName: packet.Children[0].String(), RequestValue: value}, nil
}

func decodeAttributes(packet ber.Packet) ([]ldapmsg.Attribute, error) {
	if err := packet.RequireTag(ber.ClassUniversal | ber.Constructed | ber.TagSequence); err != nil {
		return nil, fmt.Errorf("attributes: %w", err)
	}
	attrs := make([]ldapmsg.Attribute, 0, len(packet.Children))
	for _, child := range packet.Children {
		attr, err := decodeAttribute(child)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, attr)
	}
	return attrs, nil
}

func decodeAttribute(packet ber.Packet) (ldapmsg.Attribute, error) {
	if err := packet.RequireTag(ber.ClassUniversal | ber.Constructed | ber.TagSequence); err != nil {
		return ldapmsg.Attribute{}, fmt.Errorf("attribute: %w", err)
	}
	if len(packet.Children) != 2 {
		return ldapmsg.Attribute{}, fmt.Errorf("attribute has %d fields, want 2", len(packet.Children))
	}
	if err := packet.Children[0].RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
		return ldapmsg.Attribute{}, fmt.Errorf("attribute name: %w", err)
	}
	values, err := decodeStringSet(packet.Children[1])
	if err != nil {
		return ldapmsg.Attribute{}, err
	}
	return ldapmsg.Attribute{Name: packet.Children[0].String(), Values: values}, nil
}

func decodeAVA(packet ber.Packet) (ldapmsg.AttributeValueAssertion, error) {
	if err := packet.RequireTag(ber.ClassUniversal | ber.Constructed | ber.TagSequence); err != nil {
		return ldapmsg.AttributeValueAssertion{}, fmt.Errorf("attribute value assertion: %w", err)
	}
	return decodeAVAChildren(packet.Children)
}

func decodeTaggedAVA(packet ber.Packet) (ldapmsg.AttributeValueAssertion, error) {
	return decodeAVAChildren(packet.Children)
}

func decodeAVAChildren(children []ber.Packet) (ldapmsg.AttributeValueAssertion, error) {
	if len(children) != 2 {
		return ldapmsg.AttributeValueAssertion{}, fmt.Errorf("attribute value assertion has %d fields, want 2", len(children))
	}
	if err := children[0].RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
		return ldapmsg.AttributeValueAssertion{}, fmt.Errorf("assertion attribute: %w", err)
	}
	if err := children[1].RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
		return ldapmsg.AttributeValueAssertion{}, fmt.Errorf("assertion value: %w", err)
	}
	return ldapmsg.AttributeValueAssertion{Attribute: children[0].String(), Value: children[1].String()}, nil
}

func decodeStringSequence(packet ber.Packet) ([]string, error) {
	if err := packet.RequireTag(ber.ClassUniversal | ber.Constructed | ber.TagSequence); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(packet.Children))
	for _, child := range packet.Children {
		if err := child.RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
			return nil, err
		}
		out = append(out, child.String())
	}
	return out, nil
}

func decodeStringSet(packet ber.Packet) ([]string, error) {
	if err := packet.RequireTag(ber.ClassUniversal | ber.Constructed | ber.TagSet); err != nil {
		return nil, fmt.Errorf("attribute values: %w", err)
	}
	out := make([]string, 0, len(packet.Children))
	for _, child := range packet.Children {
		if err := child.RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
			return nil, fmt.Errorf("attribute value: %w", err)
		}
		out = append(out, child.String())
	}
	return out, nil
}

func decodeFilter(packet ber.Packet) (ldapmsg.Filter, error) {
	switch packet.Tag {
	case tagFilterAnd:
		filters, err := decodeFilterList(packet.Children)
		if err != nil {
			return nil, err
		}
		return ldapmsg.AndFilter{Filters: filters}, nil
	case tagFilterOr:
		filters, err := decodeFilterList(packet.Children)
		if err != nil {
			return nil, err
		}
		return ldapmsg.OrFilter{Filters: filters}, nil
	case tagFilterNot:
		if len(packet.Children) != 1 {
			return nil, fmt.Errorf("not filter has %d children, want 1", len(packet.Children))
		}
		filter, err := decodeFilter(packet.Children[0])
		if err != nil {
			return nil, err
		}
		return ldapmsg.NotFilter{Filter: filter}, nil
	case tagFilterEqualityMatch:
		ava, err := decodeTaggedAVA(packet)
		if err != nil {
			return nil, err
		}
		return ldapmsg.EqualityMatchFilter{Attribute: ava.Attribute, Value: ava.Value}, nil
	case tagFilterSubstrings:
		return decodeSubstringsFilter(packet)
	case tagFilterGreaterOrEqual:
		ava, err := decodeTaggedAVA(packet)
		if err != nil {
			return nil, err
		}
		return ldapmsg.GreaterOrEqualFilter{Attribute: ava.Attribute, Value: ava.Value}, nil
	case tagFilterLessOrEqual:
		ava, err := decodeTaggedAVA(packet)
		if err != nil {
			return nil, err
		}
		return ldapmsg.LessOrEqualFilter{Attribute: ava.Attribute, Value: ava.Value}, nil
	case tagFilterPresent:
		return ldapmsg.PresentFilter{Attribute: packet.String()}, nil
	case tagFilterApproxMatch:
		ava, err := decodeTaggedAVA(packet)
		if err != nil {
			return nil, err
		}
		return ldapmsg.ApproxMatchFilter{Attribute: ava.Attribute, Value: ava.Value}, nil
	default:
		return nil, fmt.Errorf("unsupported filter tag 0x%02x", packet.Tag)
	}
}

func decodeFilterList(children []ber.Packet) ([]ldapmsg.Filter, error) {
	filters := make([]ldapmsg.Filter, 0, len(children))
	for _, child := range children {
		filter, err := decodeFilter(child)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	return filters, nil
}

func decodeSubstringsFilter(packet ber.Packet) (ldapmsg.SubstringsFilter, error) {
	if len(packet.Children) != 2 {
		return ldapmsg.SubstringsFilter{}, fmt.Errorf("substring filter has %d fields, want 2", len(packet.Children))
	}
	if err := packet.Children[0].RequireTag(ber.ClassUniversal | ber.TagOctet); err != nil {
		return ldapmsg.SubstringsFilter{}, fmt.Errorf("substring attribute: %w", err)
	}
	if err := packet.Children[1].RequireTag(ber.ClassUniversal | ber.Constructed | ber.TagSequence); err != nil {
		return ldapmsg.SubstringsFilter{}, fmt.Errorf("substring list: %w", err)
	}

	substrings := make([]ldapmsg.Substring, 0, len(packet.Children[1].Children))
	for _, child := range packet.Children[1].Children {
		switch child.Tag {
		case tagSubstringInitial:
			substrings = append(substrings, ldapmsg.Substring{Kind: ldapmsg.SubstringInitial, Value: child.String()})
		case tagSubstringAny:
			substrings = append(substrings, ldapmsg.Substring{Kind: ldapmsg.SubstringAny, Value: child.String()})
		case tagSubstringFinal:
			substrings = append(substrings, ldapmsg.Substring{Kind: ldapmsg.SubstringFinal, Value: child.String()})
		default:
			return ldapmsg.SubstringsFilter{}, fmt.Errorf("unsupported substring tag 0x%02x", child.Tag)
		}
	}
	return ldapmsg.SubstringsFilter{Attribute: packet.Children[0].String(), Substrings: substrings}, nil
}
