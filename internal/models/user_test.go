package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUser(t *testing.T) {
	parentDN := "ou=users,dc=example,dc=com"
	user := NewUser(parentDN, "john", "John Doe", "Doe", "john@example.com")

	assert.NotNil(t, user)
	assert.Equal(t, "uid=john,ou=users,dc=example,dc=com", user.DN)
	assert.Equal(t, "john", user.UID)
	assert.True(t, user.IsUser())
}

func TestUserSetPassword(t *testing.T) {
	user := NewUser("ou=users,dc=example,dc=com", "john", "John Doe", "Doe", "john@example.com")
	hashedPassword := "{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$test$hash"

	user.SetPassword(hashedPassword)

	assert.Equal(t, hashedPassword, user.Password)
	assert.Equal(t, hashedPassword, user.Entry.GetAttribute("userPassword"))
}

func TestValidateUser(t *testing.T) {
	user := NewUser("ou=users,dc=example,dc=com", "john", "John Doe", "Doe", "john@example.com")

	err := user.ValidateUser()
	assert.NoError(t, err)
}

func TestValidateUserMissingUID(t *testing.T) {
	user := NewUser("ou=users,dc=example,dc=com", "john", "John Doe", "Doe", "john@example.com")
	user.Entry.RemoveAttribute("uid")

	err := user.ValidateUser()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uid")
}

func TestValidateUserMissingCN(t *testing.T) {
	user := NewUser("ou=users,dc=example,dc=com", "john", "John Doe", "Doe", "john@example.com")
	user.Entry.RemoveAttribute("cn")

	err := user.ValidateUser()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cn")
}

func TestValidateUserMissingSN(t *testing.T) {
	user := NewUser("ou=users,dc=example,dc=com", "john", "John Doe", "Doe", "john@example.com")
	user.Entry.RemoveAttribute("sn")

	err := user.ValidateUser()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sn")
}
