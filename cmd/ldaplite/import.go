package main

import (
	"fmt"
	"os"

	"github.com/smarzola/ldaplite/internal/ldif"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
	"github.com/spf13/cobra"
)

type importLDIFOptions struct {
	file   string
	dryRun bool
}

func newImportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import directory data",
	}
	cmd.AddCommand(newImportLDIFCommand())
	return cmd
}

func newImportLDIFCommand() *cobra.Command {
	options := &importLDIFOptions{}
	cmd := &cobra.Command{
		Use:   "ldif",
		Short: "Import directory entries from LDIF",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportLDIF(cmd, options)
		},
	}
	cmd.Flags().StringVar(&options.file, "file", "", "LDIF file to import")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Parse and validate without writing")
	return cmd
}

func runImportLDIF(cmd *cobra.Command, options *importLDIFOptions) error {
	if options.file == "" {
		return fmt.Errorf("--file is required")
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(options.file)
	if err != nil {
		return fmt.Errorf("failed to read LDIF file %s: %w", options.file, err)
	}
	records, err := ldif.Parse(string(data))
	if err != nil {
		return fmt.Errorf("failed to parse LDIF: %w", err)
	}

	st := store.NewSQLiteStore(cfg)
	if err := st.Initialize(cmd.Context()); err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer st.Close()

	plan, err := ldif.PlanImport(cmd.Context(), st, records, ldif.ImportPlanOptions{
		BaseDN: cfg.LDAP.BaseDN,
		Hasher: crypto.NewPasswordHasher(cfg.Security.Argon2Config),
	})
	if err != nil {
		return fmt.Errorf("failed to validate LDIF import: %w", err)
	}

	if options.dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "LDIF dry-run successful: records=%d planned=%d\n", len(records), len(plan.Entries))
		return nil
	}

	if err := ldif.ApplyImport(cmd.Context(), st, plan); err != nil {
		return fmt.Errorf("failed to import LDIF: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "LDIF import successful: records=%d imported=%d\n", len(records), len(plan.Entries))
	return nil
}
