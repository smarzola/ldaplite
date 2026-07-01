package ldif

import (
	"context"
	"fmt"

	"github.com/smarzola/ldaplite/internal/models"
)

// EntryWriter is the store capability needed to apply an import plan.
type EntryWriter interface {
	CreateEntry(ctx context.Context, entry *models.Entry) error
}

type entryReplacer interface {
	EntryExists(ctx context.Context, dn string) (bool, error)
	UpdateEntry(ctx context.Context, entry *models.Entry) error
}

// ApplyImport writes a validated import plan in plan order.
func ApplyImport(ctx context.Context, writer EntryWriter, plan *ImportPlan) error {
	if writer == nil {
		return &ImportPlanError{Msg: "entry writer is required"}
	}
	if plan == nil {
		return &ImportPlanError{Msg: "import plan is required"}
	}
	if plan.ReplaceExisting {
		replacer, ok := writer.(entryReplacer)
		if !ok {
			return &ImportPlanError{Msg: "replace-existing import requires entry update support"}
		}
		for _, entry := range plan.Entries {
			exists, err := replacer.EntryExists(ctx, entry.DN)
			if err != nil {
				return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("failed to check existing entry: %v", err)}
			}
			if exists {
				if err := replacer.UpdateEntry(ctx, entry); err != nil {
					return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("failed to replace entry: %v", err)}
				}
				continue
			}
			if err := writer.CreateEntry(ctx, entry); err != nil {
				return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("failed to create entry: %v", err)}
			}
		}
		return nil
	}
	for _, entry := range plan.Entries {
		if err := writer.CreateEntry(ctx, entry); err != nil {
			return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("failed to create entry: %v", err)}
		}
	}
	return nil
}
