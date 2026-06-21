package store

import (
	"errors"
	"fmt"

	"github.com/smarzola/ldaplite/internal/models"
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

	if errors.Is(err, models.ErrRequiredAttributeEmpty) ||
		errors.Is(err, models.ErrObjectClassRequired) {
		return fmt.Errorf("%w: %w", ErrObjectClassViolation, err)
	}

	return err
}
