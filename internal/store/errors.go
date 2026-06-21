package store

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrConstraintViolation  = errors.New("constraint violation")
	ErrEntryAlreadyExists   = errors.New("entry already exists")
	ErrNoSuchObject         = errors.New("no such object")
	ErrObjectClassViolation = errors.New("object class violation")
)

func classifyModelValidationError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "required attribute") ||
		strings.Contains(errMsg, "objectclass is required") ||
		strings.Contains(errMsg, "objectclass is missing") {
		return fmt.Errorf("%w: %w", ErrObjectClassViolation, err)
	}

	return err
}
