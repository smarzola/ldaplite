package models

import "errors"

var (
	ErrObjectClassRequired    = errors.New("objectClass is required")
	ErrRequiredAttributeEmpty = errors.New("required attribute is missing")
)
