package protocol

import (
	"fmt"

	"github.com/lor00x/goldap/message"

	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
)

func FromGoldapMessage(msg *message.LDAPMessage) (*ldapmsg.Message, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil LDAP message")
	}

	op, err := fromGoldapOperation(msg.ProtocolOp())
	if err != nil {
		return nil, err
	}

	return &ldapmsg.Message{
		ID: ldapmsg.MessageID(msg.MessageID()),
		Op: op,
	}, nil
}

func fromGoldapOperation(op message.ProtocolOp) (ldapmsg.Operation, error) {
	switch req := op.(type) {
	case message.BindRequest:
		return ldapmsg.BindRequest{
			Name:     string(req.Name()),
			Password: string(req.AuthenticationSimple()),
		}, nil
	case message.SearchRequest:
		filter, err := fromGoldapFilter(req.Filter())
		if err != nil {
			return nil, err
		}
		return ldapmsg.SearchRequest{
			BaseObject: string(req.BaseObject()),
			Scope:      ldapmsg.SearchScope(req.Scope()),
			TypesOnly:  bool(req.TypesOnly()),
			Filter:     filter,
			Attributes: fromGoldapAttributeSelection(req.Attributes()),
		}, nil
	case message.AddRequest:
		return ldapmsg.AddRequest{
			Entry:      string(req.Entry()),
			Attributes: fromGoldapAttributes(req.Attributes()),
		}, nil
	case message.ModifyRequest:
		changes := req.Changes()
		out := make([]ldapmsg.ModifyChange, 0, len(changes))
		for i := range changes {
			modification := changes[i].Modification()
			out = append(out, ldapmsg.ModifyChange{
				Operation: ldapmsg.ModifyOperation(changes[i].Operation()),
				Modification: ldapmsg.Attribute{
					Name:   string(modification.Type_()),
					Values: fromGoldapAttributeValues(modification.Vals()),
				},
			})
		}
		return ldapmsg.ModifyRequest{
			Object:  string(req.Object()),
			Changes: out,
		}, nil
	case message.DelRequest:
		return ldapmsg.DeleteRequest{DN: string(req)}, nil
	case message.CompareRequest:
		ava := req.Ava()
		return ldapmsg.CompareRequest{
			Entry: string(req.Entry()),
			AVA: ldapmsg.AttributeValueAssertion{
				Attribute: string(ava.AttributeDesc()),
				Value:     string(ava.AssertionValue()),
			},
		}, nil
	case message.ExtendedRequest:
		return ldapmsg.ExtendedRequest{
			RequestName:  string(req.RequestName()),
			RequestValue: fromGoldapOptionalString(req.RequestValue()),
		}, nil
	case message.UnbindRequest:
		return ldapmsg.UnbindRequest{}, nil
	default:
		return nil, fmt.Errorf("unsupported goldap protocol operation %T", op)
	}
}

func fromGoldapAttributes(attrs []message.Attribute) []ldapmsg.Attribute {
	out := make([]ldapmsg.Attribute, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, ldapmsg.Attribute{
			Name:   string(attr.Type_()),
			Values: fromGoldapAttributeValues(attr.Vals()),
		})
	}
	return out
}

func fromGoldapAttributeValues(values []message.AttributeValue) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}

func fromGoldapAttributeSelection(selection message.AttributeSelection) []string {
	out := make([]string, 0, len(selection))
	for _, attr := range selection {
		out = append(out, string(attr))
	}
	return out
}

func fromGoldapOptionalString(value *message.OCTETSTRING) *string {
	if value == nil {
		return nil
	}
	out := string(*value)
	return &out
}

func fromGoldapFilter(filter interface{}) (ldapmsg.Filter, error) {
	switch f := filter.(type) {
	case nil:
		return nil, nil
	case message.FilterEqualityMatch:
		return ldapmsg.EqualityMatchFilter{
			Attribute: string(f.AttributeDesc()),
			Value:     string(f.AssertionValue()),
		}, nil
	case message.FilterPresent:
		return ldapmsg.PresentFilter{Attribute: string(f)}, nil
	case message.FilterAnd:
		filters, err := fromGoldapFilterList(f)
		if err != nil {
			return nil, err
		}
		return ldapmsg.AndFilter{Filters: filters}, nil
	case message.FilterOr:
		filters, err := fromGoldapFilterList(f)
		if err != nil {
			return nil, err
		}
		return ldapmsg.OrFilter{Filters: filters}, nil
	case message.FilterNot:
		nested, err := fromGoldapFilter(f.Filter)
		if err != nil {
			return nil, err
		}
		return ldapmsg.NotFilter{Filter: nested}, nil
	case message.FilterGreaterOrEqual:
		return ldapmsg.GreaterOrEqualFilter{
			Attribute: string(f.AttributeDesc()),
			Value:     string(f.AssertionValue()),
		}, nil
	case message.FilterLessOrEqual:
		return ldapmsg.LessOrEqualFilter{
			Attribute: string(f.AttributeDesc()),
			Value:     string(f.AssertionValue()),
		}, nil
	case message.FilterApproxMatch:
		return ldapmsg.ApproxMatchFilter{
			Attribute: string(f.AttributeDesc()),
			Value:     string(f.AssertionValue()),
		}, nil
	case message.FilterSubstrings:
		substrings := f.Substrings()
		out := make([]ldapmsg.Substring, 0, len(substrings))
		for _, substring := range substrings {
			switch s := substring.(type) {
			case message.SubstringInitial:
				out = append(out, ldapmsg.Substring{Kind: ldapmsg.SubstringInitial, Value: string(s)})
			case message.SubstringAny:
				out = append(out, ldapmsg.Substring{Kind: ldapmsg.SubstringAny, Value: string(s)})
			case message.SubstringFinal:
				out = append(out, ldapmsg.Substring{Kind: ldapmsg.SubstringFinal, Value: string(s)})
			default:
				return nil, fmt.Errorf("unsupported goldap substring %T", substring)
			}
		}
		return ldapmsg.SubstringsFilter{
			Attribute:  string(f.Type_()),
			Substrings: out,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported goldap filter %T", filter)
	}
}

func fromGoldapFilterList(filters []message.Filter) ([]ldapmsg.Filter, error) {
	out := make([]ldapmsg.Filter, 0, len(filters))
	for _, filter := range filters {
		converted, err := fromGoldapFilter(filter)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}
