package models

import (
	"fmt"
)

// User represents an LDAP user (inetOrgPerson)
type User struct {
	*Entry
	UID      string
	Password string // Hashed password, never store plaintext
}

// NewUser creates a new user entry
func NewUser(parentDN, uid, cn, sn, mail string) *User {
	userDN := fmt.Sprintf("uid=%s,%s", uid, parentDN)
	entry := NewEntry(userDN, string(ObjectClassInetOrgPerson))

	// Set required attributes
	entry.SetAttribute("uid", uid)
	entry.SetAttribute("cn", cn)
	entry.SetAttribute("sn", sn)

	if mail != "" {
		entry.SetAttribute("mail", mail)
	}

	return &User{
		Entry: entry,
		UID:   uid,
	}
}

// SetPassword sets the hashed password for the user
func (u *User) SetPassword(hashedPassword string) {
	u.Password = hashedPassword
	u.Entry.SetAttribute("userPassword", hashedPassword)
}

// ValidateUser validates that a user has all required attributes
func (u *User) ValidateUser() error {
	if err := u.Entry.Validate(); err != nil {
		return err
	}

	requiredAttrs := []string{"uid", "cn", "sn"}
	for _, attr := range requiredAttrs {
		if u.Entry.GetAttribute(attr) == "" {
			return fmt.Errorf("required attribute %s is missing", attr)
		}
	}

	return nil
}
