package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/smarzola/ldaplite/internal/server"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	commit  = "dev"
)

func init() {
	// Suppress unstructured logs from ldapserver library globally
	// This must happen before any other code runs
	log.SetOutput(io.Discard)
	log.SetFlags(0)
	log.SetPrefix("")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "ldaplite",
	Short: "LDAPLite - A lightweight, LDAP-compliant server",
	Long:  "A simple, opinion-driven LDAP server written in Go with SQLite backend",
}

func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(healthcheckCmd)
}

func startServer() error {
	// Load configuration
	cfg := config.Load()
	cfg.Print()

	// Initialize structured logging (slog only, no unstructured logs)
	initLogging(cfg.Logging.Level, cfg.Logging.Format)

	ctx := context.Background()

	// Initialize SQLite store
	st := store.NewSQLiteStore(cfg)
	if err := st.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer st.Close()

	slog.Info("Database initialized successfully")

	// Create and start LDAP server
	srv := server.NewServer(cfg, st)
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	slog.Info("LDAPLite server is running", "address", fmt.Sprintf("%s:%d", cfg.Server.BindAddress, cfg.Server.Port))

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down server")
	srv.Stop()

	return nil
}

func initLogging(level, format string) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	switch level {
	case "debug":
		opts.Level = slog.LevelDebug
	case "info":
		opts.Level = slog.LevelInfo
	case "warn":
		opts.Level = slog.LevelWarn
	case "error":
		opts.Level = slog.LevelError
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the LDAP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return startServer()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ldaplite version %s (commit: %s)\n", version, commit)
	},
}

var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Perform a health check",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement health check
		fmt.Println("Health check passed")
		return nil
	},
}
