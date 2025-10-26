package models

import (
	"fmt"
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
