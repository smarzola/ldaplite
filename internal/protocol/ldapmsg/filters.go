package ldapmsg

type Filter interface {
	isFilter()
}

type EqualityMatchFilter struct {
	Attribute string
	Value     string
}

func (EqualityMatchFilter) isFilter() {}

type PresentFilter struct {
	Attribute string
}

func (PresentFilter) isFilter() {}

type AndFilter struct {
	Filters []Filter
}

func (AndFilter) isFilter() {}

type OrFilter struct {
	Filters []Filter
}

func (OrFilter) isFilter() {}

type NotFilter struct {
	Filter Filter
}

func (NotFilter) isFilter() {}

type GreaterOrEqualFilter struct {
	Attribute string
	Value     string
}

func (GreaterOrEqualFilter) isFilter() {}

type LessOrEqualFilter struct {
	Attribute string
	Value     string
}

func (LessOrEqualFilter) isFilter() {}

type ApproxMatchFilter struct {
	Attribute string
	Value     string
}

func (ApproxMatchFilter) isFilter() {}

type SubstringsFilter struct {
	Attribute  string
	Substrings []Substring
}

func (SubstringsFilter) isFilter() {}

type SubstringKind int

const (
	SubstringInitial SubstringKind = iota
	SubstringAny
	SubstringFinal
)

type Substring struct {
	Kind  SubstringKind
	Value string
}
