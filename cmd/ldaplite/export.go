package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/smarzola/ldaplite/internal/ldif"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/spf13/cobra"
)

type exportLDIFOptions struct {
	file                        string
	includeOperational          bool
	includePasswordPlaceholders bool
}

func newExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export directory data",
	}
	cmd.AddCommand(newExportLDIFCommand())
	return cmd
}

func newExportLDIFCommand() *cobra.Command {
	options := &exportLDIFOptions{file: "-"}
	cmd := &cobra.Command{
		Use:   "ldif",
		Short: "Export directory entries as LDIF",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExportLDIF(cmd, options)
		},
	}
	cmd.Flags().StringVar(&options.file, "file", "-", "LDIF destination file, or - for stdout")
	cmd.Flags().BoolVar(&options.includeOperational, "include-operational", false, "Include safe operational attributes")
	cmd.Flags().BoolVar(&options.includePasswordPlaceholders, "include-password-placeholders", false, "Emit redacted userPassword placeholders")
	return cmd
}

func runExportLDIF(cmd *cobra.Command, options *exportLDIFOptions) error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	st := store.NewSQLiteStore(cfg)
	if err := st.Initialize(cmd.Context()); err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer st.Close()

	records, err := ldif.BuildExportRecords(cmd.Context(), st, ldif.ExportOptions{
		BaseDN:                      cfg.LDAP.BaseDN,
		IncludeOperational:          options.includeOperational,
		IncludePasswordPlaceholders: options.includePasswordPlaceholders,
	})
	if err != nil {
		return fmt.Errorf("failed to export LDIF: %w", err)
	}

	output := ldif.Format(records)
	if options.file == "-" {
		_, err := fmt.Fprint(cmd.OutOrStdout(), output)
		return err
	}
	if err := writeOutputFile(options.file, []byte(output)); err != nil {
		return fmt.Errorf("failed to write LDIF file %s: %w", options.file, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "LDIF export successful: records=%d file=%s\n", len(records), options.file)
	return nil
}

func writeOutputFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ldaplite-export-*.ldif")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
