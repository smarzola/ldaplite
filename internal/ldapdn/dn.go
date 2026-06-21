package ldapdn

import "strings"

// Split returns the first RDN and parent DN, splitting on the first unescaped comma.
func Split(dn string) (string, string) {
	idx := firstUnescapedComma(dn)
	if idx < 0 {
		return strings.TrimSpace(dn), ""
	}
	return strings.TrimSpace(dn[:idx]), strings.TrimSpace(dn[idx+1:])
}

// RDN returns the first relative distinguished name.
func RDN(dn string) string {
	rdn, _ := Split(dn)
	return rdn
}

// Parent returns the parent DN.
func Parent(dn string) string {
	_, parent := Split(dn)
	return parent
}

// FirstRDNValue returns the first RDN value when its attribute name matches attr.
func FirstRDNValue(dn, attr string) string {
	name, value, ok := SplitRDN(RDN(dn))
	if !ok || !strings.EqualFold(name, attr) {
		return ""
	}
	return value
}

// SplitRDN splits an RDN into attribute name and value.
func SplitRDN(rdn string) (string, string, bool) {
	idx := firstUnescapedEqual(rdn)
	if idx < 0 {
		return "", "", false
	}
	name := strings.TrimSpace(rdn[:idx])
	value := strings.TrimSpace(rdn[idx+1:])
	if name == "" {
		return "", "", false
	}
	return name, value, true
}

// Equal compares DNs using the repository's current case-insensitive matching.
func Equal(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// WithinBase reports whether dn is the base DN itself or a descendant of it.
func WithinBase(dn, baseDN string) bool {
	dn = strings.TrimSpace(dn)
	baseDN = strings.TrimSpace(baseDN)
	if baseDN == "" {
		return false
	}
	if Equal(dn, baseDN) {
		return true
	}
	for parent := Parent(dn); parent != ""; parent = Parent(parent) {
		if Equal(parent, baseDN) {
			return true
		}
	}
	return false
}

func firstUnescapedComma(dn string) int {
	return firstUnescaped(dn, ',')
}

func firstUnescapedEqual(rdn string) int {
	return firstUnescaped(rdn, '=')
}

func firstUnescaped(s string, target byte) int {
	escaped := false
	for i := 0; i < len(s); i++ {
		switch {
		case escaped:
			escaped = false
		case s[i] == '\\':
			escaped = true
		case s[i] == target:
			return i
		}
	}
	return -1
}
