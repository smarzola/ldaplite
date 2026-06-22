package ldapmsg

type LDAPResult struct {
	ResultCode        ResultCode
	MatchedDN         string
	DiagnosticMessage string
}

type BindResponse struct {
	LDAPResult
}

func (BindResponse) isOperation() {}

type SearchResultEntry struct {
	ObjectName string
	Attributes []Attribute
}

func (SearchResultEntry) isOperation() {}

type SearchResultDone struct {
	LDAPResult
}

func (SearchResultDone) isOperation() {}

type AddResponse struct {
	LDAPResult
}

func (AddResponse) isOperation() {}

type ModifyResponse struct {
	LDAPResult
}

func (ModifyResponse) isOperation() {}

type DeleteResponse struct {
	LDAPResult
}

func (DeleteResponse) isOperation() {}

type CompareResponse struct {
	LDAPResult
}

func (CompareResponse) isOperation() {}

type ExtendedResponse struct {
	LDAPResult
	ResponseName  string
	ResponseValue *string
}

func (ExtendedResponse) isOperation() {}
