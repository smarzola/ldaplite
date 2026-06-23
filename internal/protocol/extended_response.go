package protocol

import "github.com/smarzola/ldaplite/internal/protocol/ldapmsg"

const WhoAmIOID = "1.3.6.1.4.1.4203.1.11.3"
const StartTLSOID = "1.3.6.1.4.1.1466.20037"

func NewWhoAmIResponse(authzID string) ldapmsg.ExtendedResponse {
	return ldapmsg.ExtendedResponse{
		LDAPResult:    ldapmsg.LDAPResult{ResultCode: ldapmsg.ResultCodeSuccess},
		ResponseName:  WhoAmIOID,
		ResponseValue: &authzID,
	}
}
