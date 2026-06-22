package ldapmsg

type BindRequest struct {
	Name     string
	Password string
}

func (BindRequest) isOperation() {}

type SearchScope int

const (
	SearchScopeBaseObject SearchScope = iota
	SearchScopeSingleLevel
	SearchScopeWholeSubtree
)

type SearchRequest struct {
	BaseObject string
	Scope      SearchScope
	TypesOnly  bool
	Filter     Filter
	Attributes []string
}

func (SearchRequest) isOperation() {}

type AddRequest struct {
	Entry      string
	Attributes []Attribute
}

func (AddRequest) isOperation() {}

type ModifyOperation int

const (
	ModifyOperationAdd ModifyOperation = iota
	ModifyOperationDelete
	ModifyOperationReplace
)

type ModifyRequest struct {
	Object  string
	Changes []ModifyChange
}

func (ModifyRequest) isOperation() {}

type ModifyChange struct {
	Operation    ModifyOperation
	Modification Attribute
}

type DeleteRequest struct {
	DN string
}

func (DeleteRequest) isOperation() {}

type CompareRequest struct {
	Entry string
	AVA   AttributeValueAssertion
}

func (CompareRequest) isOperation() {}

type ExtendedRequest struct {
	RequestName  string
	RequestValue *string
}

func (ExtendedRequest) isOperation() {}

type UnbindRequest struct{}

func (UnbindRequest) isOperation() {}
