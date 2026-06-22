package ldapmsg

type MessageID int

type Message struct {
	ID MessageID
	Op Operation
}

type Operation interface {
	isOperation()
}

type ResultCode int

const (
	ResultCodeSuccess                  ResultCode = 0
	ResultCodeOperationsError          ResultCode = 1
	ResultCodeProtocolError            ResultCode = 2
	ResultCodeCompareFalse             ResultCode = 5
	ResultCodeCompareTrue              ResultCode = 6
	ResultCodeInvalidCredentials       ResultCode = 49
	ResultCodeInsufficientAccessRights ResultCode = 50
	ResultCodeUnavailable              ResultCode = 52
	ResultCodeUnwillingToPerform       ResultCode = 53
	ResultCodeNoSuchObject             ResultCode = 32
	ResultCodeEntryAlreadyExists       ResultCode = 68
	ResultCodeObjectClassViolation     ResultCode = 65
	ResultCodeConstraintViolation      ResultCode = 19
)

type Attribute struct {
	Name   string
	Values []string
}

type AttributeValueAssertion struct {
	Attribute string
	Value     string
}
