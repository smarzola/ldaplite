package models

import (
	"fmt"
	"strings"
)

// User represents an LDAP user (inetOrgPerson)
type User struct {
	*Entry
	UID      string
	Password string // Hashed password, never store plaintext
}

// NewUser creates a new user entry
func NewUser(baseDN, uid, cn, sn, givenName, mail string) *User {
	userDN := fmt.Sprintf("uid=%s,ou=users,%s", uid, baseDN)
	entry := NewEntry(userDN, string(ObjectClassInetOrgPerson))

	// Set required attributes
	entry.SetAttribute("uid", uid)
	entry.SetAttribute("cn", cn)
	entry.SetAttribute("sn", sn)
	entry.SetAttribute("givenName", givenName)

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

// GetPassword returns the password hash
func (u *User) GetPassword() string {
	if u.Password != "" {
		return u.Password
	}
	return u.Entry.GetAttribute("userPassword")
}

// ValidateUser validates that a user has all required attributes
func (u *User) ValidateUser() error {
	if err := u.Entry.Validate(); err != nil {
		return err
	}

	requiredAttrs := []string{"uid", "cn", "sn", "givenName"}
	for _, attr := range requiredAttrs {
		if u.Entry.GetAttribute(attr) == "" {
			return fmt.Errorf("required attribute %s is missing", attr)
		}
	}

	return nil
}

// SetEmail sets the user's email
func (u *User) SetEmail(email string) {
	u.Entry.SetAttribute("mail", email)
}

// GetEmail returns the user's email
func (u *User) GetEmail() string {
	return u.Entry.GetAttribute("mail")
}

// SetTelephoneNumber sets the user's telephone number
func (u *User) SetTelephoneNumber(phone string) {
	u.Entry.SetAttribute("telephoneNumber", phone)
}

// GetTelephoneNumber returns the user's telephone number
func (u *User) GetTelephoneNumber() string {
	return u.Entry.GetAttribute("telephoneNumber")
}

// SetDisplayName sets the user's display name
func (u *User) SetDisplayName(displayName string) {
	u.Entry.SetAttribute("displayName", displayName)
}

// GetDisplayName returns the user's display name
func (u *User) GetDisplayName() string {
	return u.Entry.GetAttribute("displayName")
}

// ExtractUIDFromDN extracts the UID from a DN
// e.g., "uid=john,ou=users,dc=example,dc=com" -> "john"
func ExtractUIDFromDN(dn string) (string, error) {
	parts := strings.SplitN(dn, ",", 2)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid DN format: %s", dn)
	}

	rdnParts := strings.SplitN(parts[0], "=", 2)
	if len(rdnParts) != 2 || strings.ToLower(rdnParts[0]) != "uid" {
		return "", fmt.Errorf("DN does not contain uid: %s", dn)
	}

	return rdnParts[1], nil
}
