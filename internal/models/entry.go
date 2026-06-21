package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/smarzola/ldaplite/internal/ldapdn"
)

// ObjectClass represents LDAP object classes we support
type ObjectClass string

const (
	ObjectClassOrganizationalUnit ObjectClass = "organizationalUnit"
	ObjectClassInetOrgPerson      ObjectClass = "inetOrgPerson"
	ObjectClassGroupOfNames       ObjectClass = "groupOfNames"
	ObjectClassTop                ObjectClass = "top"
)

// Entry represents an LDAP entry (object)
type Entry struct {
	ID          int64
	DN          string              // Distinguished Name
	ParentDN    string              // Parent DN for hierarchy
	ObjectClass string              // Primary object class
	Attributes  map[string][]string // Multi-valued attributes
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewEntry creates a new LDAP entry
func NewEntry(dn string, objectClass string) *Entry {
	return &Entry{
		DN:          dn,
		ParentDN:    ldapdn.Parent(dn),
		ObjectClass: objectClass,
		Attributes:  make(map[string][]string),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// SetAttribute sets a single-valued attribute
func (e *Entry) SetAttribute(name, value string) {
	e.Attributes[strings.ToLower(name)] = []string{value}
	e.UpdatedAt = time.Now()
}

// AddAttribute adds a value to a multi-valued attribute
func (e *Entry) AddAttribute(name, value string) {
	name = strings.ToLower(name)
	e.Attributes[name] = append(e.Attributes[name], value)
	e.UpdatedAt = time.Now()
}

// GetAttribute gets the first value of an attribute
func (e *Entry) GetAttribute(name string) string {
	name = strings.ToLower(name)
	if values, exists := e.Attributes[name]; exists && len(values) > 0 {
		return values[0]
	}
	return ""
}

// GetAttributes gets all values of an attribute
func (e *Entry) GetAttributes(name string) []string {
	name = strings.ToLower(name)
	if values, exists := e.Attributes[name]; exists {
		return values
	}
	return []string{}
}

// HasAttribute checks if an attribute exists
func (e *Entry) HasAttribute(name string) bool {
	name = strings.ToLower(name)
	_, exists := e.Attributes[name]
	return exists
}

// RemoveAttribute removes an attribute
func (e *Entry) RemoveAttribute(name string) {
	name = strings.ToLower(name)
	delete(e.Attributes, name)
	e.UpdatedAt = time.Now()
}

// RemoveAttributeValue removes a specific value from an attribute
func (e *Entry) RemoveAttributeValue(name, value string) error {
	name = strings.ToLower(name)
	values, exists := e.Attributes[name]
	if !exists {
		return fmt.Errorf("attribute %s does not exist", name)
	}

	for i, v := range values {
		if v == value {
			e.Attributes[name] = append(values[:i], values[i+1:]...)
			e.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("value %s not found in attribute %s", value, name)
}

// IsOrganizationalUnit checks if entry is an OU
func (e *Entry) IsOrganizationalUnit() bool {
	return e.ObjectClass == string(ObjectClassOrganizationalUnit)
}

// IsUser checks if entry is a user (inetOrgPerson)
func (e *Entry) IsUser() bool {
	return e.ObjectClass == string(ObjectClassInetOrgPerson)
}

// IsGroup checks if entry is a group
func (e *Entry) IsGroup() bool {
	return e.ObjectClass == string(ObjectClassGroupOfNames)
}

// GetRDN returns the Relative Distinguished Name (first component)
// e.g., "cn=admin" from "cn=admin,ou=users,dc=example,dc=com"
func (e *Entry) GetRDN() string {
	return ldapdn.RDN(e.DN)
}

// Validate checks if the entry has required attributes
func (e *Entry) Validate() error {
	if e.DN == "" {
		return fmt.Errorf("DN is required")
	}
	if e.ObjectClass == "" {
		return fmt.Errorf("ObjectClass is required")
	}
	return nil
}

// ToLDIF converts the entry to LDIF format
func (e *Entry) ToLDIF() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("dn: %s", e.DN))
	lines = append(lines, fmt.Sprintf("objectClass: %s", e.ObjectClass))

	// Add all other attributes
	for name, values := range e.Attributes {
		for _, value := range values {
			lines = append(lines, fmt.Sprintf("%s: %s", name, value))
		}
	}

	return strings.Join(lines, "\n")
}

// FormatLDAPTimestamp formats a time.Time into LDAP Generalized Time format
// Format: YYYYMMDDHHMMSSz (UTC)
// Example: 20250125143045Z
// This format is defined in RFC 4517 (LDAP Syntaxes and Matching Rules)
func FormatLDAPTimestamp(t time.Time) string {
	return t.UTC().Format("20060102150405Z")
}
