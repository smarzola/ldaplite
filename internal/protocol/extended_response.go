package protocol

import (
	"fmt"
	"reflect"
	"unsafe"

	"github.com/lor00x/goldap/message"
)

const WhoAmIOID = "1.3.6.1.4.1.4203.1.11.3"

func NewWhoAmIResponse(authzID string) (message.ExtendedResponse, error) {
	resp := NewExtendedResponse(message.ResultCodeSuccess)
	resp.SetResponseName(message.LDAPOID(WhoAmIOID))

	if err := setExtendedResponseValue(&resp, authzID); err != nil {
		return message.ExtendedResponse{}, err
	}

	return resp, nil
}

func setExtendedResponseValue(resp *message.ExtendedResponse, value string) error {
	octetString := message.OCTETSTRING(value)
	respValue := reflect.ValueOf(resp).Elem()
	responseValueField := respValue.FieldByName("responseValue")
	if !responseValueField.IsValid() {
		return fmt.Errorf("goldap ExtendedResponse.responseValue field not found")
	}

	// goldap does not expose a responseValue setter. Keep the unsafe access
	// isolated here so dependency changes fail in one tested protocol helper.
	ptr := unsafe.Pointer(responseValueField.UnsafeAddr())
	*(**message.OCTETSTRING)(ptr) = &octetString
	return nil
}
