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

// ApplyImport writes a validated import plan in plan order.
func ApplyImport(ctx context.Context, writer EntryWriter, plan *ImportPlan) error {
	if writer == nil {
		return &ImportPlanError{Msg: "entry writer is required"}
	}
	if plan == nil {
		return &ImportPlanError{Msg: "import plan is required"}
	}
	for _, entry := range plan.Entries {
		if err := writer.CreateEntry(ctx, entry); err != nil {
			return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("failed to create entry: %v", err)}
		}
	}
	return nil
}
