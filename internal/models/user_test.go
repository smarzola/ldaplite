package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUser(t *testing.T) {
	baseDN := "dc=example,dc=com"
	user := NewUser(baseDN, "john", "John Doe", "Doe", "John", "john@example.com")

	assert.NotNil(t, user)
	assert.Equal(t, "uid=john,ou=users,dc=example,dc=com", user.DN)
	assert.Equal(t, "john", user.UID)
	assert.True(t, user.IsUser())
}

func TestUserSetPassword(t *testing.T) {
	user := NewUser("dc=example,dc=com", "john", "John Doe", "Doe", "John", "john@example.com")
	hashedPassword := "$argon2id$v=19$m=65536,t=3,p=2$test$hash"

	user.SetPassword(hashedPassword)

	assert.Equal(t, hashedPassword, user.GetPassword())
	assert.Equal(t, hashedPassword, user.Entry.GetAttribute("userPassword"))
}

func TestUserSetEmail(t *testing.T) {
	user := NewUser("dc=example,dc=com", "john", "John Doe", "Doe", "John", "john@example.com")

	user.SetEmail("newemail@example.com")

	assert.Equal(t, "newemail@example.com", user.GetEmail())
}

func TestUserSetTelephoneNumber(t *testing.T) {
	user := NewUser("dc=example,dc=com", "john", "John Doe", "Doe", "John", "john@example.com")

	user.SetTelephoneNumber("+1-555-1234")

	assert.Equal(t, "+1-555-1234", user.GetTelephoneNumber())
}

func TestUserSetDisplayName(t *testing.T) {
	user := NewUser("dc=example,dc=com", "john", "John Doe", "Doe", "John", "john@example.com")

	user.SetDisplayName("John D.")

	assert.Equal(t, "John D.", user.GetDisplayName())
}

func TestValidateUser(t *testing.T) {
	user := NewUser("dc=example,dc=com", "john", "John Doe", "Doe", "John", "john@example.com")

	err := user.ValidateUser()
	assert.NoError(t, err)
}

func TestValidateUserMissingAttribute(t *testing.T) {
	user := NewUser("dc=example,dc=com", "john", "John Doe", "Doe", "John", "john@example.com")
	user.Entry.RemoveAttribute("uid")

	err := user.ValidateUser()
	assert.Error(t, err)
}

func TestExtractUIDFromDN(t *testing.T) {
	tests := []struct {
		name    string
		dn      string
		uid     string
		wantErr bool
	}{
		{
			name:    "valid dn",
			dn:      "uid=john,ou=users,dc=example,dc=com",
			uid:     "john",
			wantErr: false,
		},
		{
			name:    "simple dn",
			dn:      "uid=alice,dc=example,dc=com",
			uid:     "alice",
			wantErr: false,
		},
		{
			name:    "invalid dn",
			dn:      "cn=admin,dc=example,dc=com",
			uid:     "",
			wantErr: true,
		},
		{
			name:    "empty dn",
			dn:      "",
			uid:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid, err := ExtractUIDFromDN(tt.dn)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.uid, uid)
			}
		})
	}
}
