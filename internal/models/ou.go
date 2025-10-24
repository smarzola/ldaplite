package models

import (
	"fmt"
	"strings"
)

// OrganizationalUnit represents an LDAP organizational unit (ou)
type OrganizationalUnit struct {
	*Entry
	OU string
}

// NewOrganizationalUnit creates a new OU entry
func NewOrganizationalUnit(baseDN, ou, description string) *OrganizationalUnit {
	ouDN := fmt.Sprintf("ou=%s,%s", ou, baseDN)
	entry := NewEntry(ouDN, string(ObjectClassOrganizationalUnit))

	// Set required attributes
	entry.SetAttribute("ou", ou)
	if description != "" {
		entry.SetAttribute("description", description)
	}

	return &OrganizationalUnit{
		Entry: entry,
		OU:    ou,
	}
}

// ValidateOU validates that an OU has all required attributes
func (o *OrganizationalUnit) ValidateOU() error {
	if err := o.Entry.Validate(); err != nil {
		return err
	}

	if o.Entry.GetAttribute("ou") == "" {
		return fmt.Errorf("required attribute ou is missing")
	}

	return nil
}

// SetDescription sets the OU's description
func (o *OrganizationalUnit) SetDescription(description string) {
	o.Entry.SetAttribute("description", description)
}

// GetDescription returns the OU's description
func (o *OrganizationalUnit) GetDescription() string {
	return o.Entry.GetAttribute("description")
}

// ExtractOUFromDN extracts the OU from a DN
// e.g., "ou=users,dc=example,dc=com" -> "users"
func ExtractOUNameFromDN(dn string) (string, error) {
	parts := strings.SplitN(dn, ",", 2)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid DN format: %s", dn)
	}

	rdnParts := strings.SplitN(parts[0], "=", 2)
	if len(rdnParts) != 2 || strings.ToLower(rdnParts[0]) != "ou" {
		return "", fmt.Errorf("DN does not contain ou: %s", dn)
	}

	return rdnParts[1], nil
}
