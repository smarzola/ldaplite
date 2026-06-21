package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smarzola/ldaplite/internal/server"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/internal/web"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var (
	version = "0.8.2"
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
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}
	cfg.Print()

	// Initialize structured logging (slog only, no unstructured logs)
	initLogging(cfg.Logging.Level, cfg.Logging.Format)

	slog.Info("Starting LDAPLite", "version", version, "commit", commit)

	ctx := context.Background()

	// Initialize SQLite store
	st := store.NewSQLiteStore(cfg)
	if err := st.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer st.Close()

	slog.Info("Database initialized successfully")

	// Create and start LDAP server
	srv := server.NewServer(cfg, st, version)
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	slog.Info("LDAPLite server is running", "address", fmt.Sprintf("%s:%d", cfg.Server.BindAddress, cfg.Server.Port))

	// Start web UI if enabled
	var webSrv *web.Server
	if cfg.WebUI.Enabled {
		var err error
		webSrv, err = web.NewServer(cfg, st)
		if err != nil {
			return fmt.Errorf("failed to create web server: %w", err)
		}

		// Start web server in goroutine
		go func() {
			if err := webSrv.Start(); err != nil {
				slog.Error("Web server failed", "error", err)
			}
		}()

		slog.Info("Web UI server is running", "address", fmt.Sprintf("%s:%d", cfg.WebUI.BindAddress, cfg.WebUI.Port))
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down servers")

	// Stop LDAP server
	srv.Stop()

	// Stop web server if it was started
	if webSrv != nil {
		if err := webSrv.Stop(); err != nil {
			slog.Error("Failed to stop web server", "error", err)
		}
	}

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
		cfg, err := config.LoadFromEnv()
		if err != nil {
			return err
		}
		if err := runHealthcheck(cmd.Context(), cfg); err != nil {
			return err
		}
		fmt.Println("Health check passed")
		return nil
	},
}

func runHealthcheck(ctx context.Context, cfg *config.Config) error {
	if cfg.Database.Path == "" {
		return fmt.Errorf("LDAP_DATABASE_PATH is required")
	}
	if cfg.LDAP.BaseDN == "" {
		return fmt.Errorf("LDAP_BASE_DN is required")
	}

	info, err := os.Stat(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("database is not accessible at %s: %w", cfg.Database.Path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("database path is a directory: %s", cfg.Database.Path)
	}

	db, err := sql.Open("sqlite", cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	requiredTables := []string{
		"entries",
		"attributes",
		"users",
		"groups",
		"group_members",
		"organizational_units",
	}
	for _, table := range requiredTables {
		var exists bool
		err := db.QueryRowContext(
			ctx,
			`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)`,
			table,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("database schema is missing required table: %s", table)
		}
	}

	var baseExists bool
	err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM entries WHERE dn = ?)`, cfg.LDAP.BaseDN).Scan(&baseExists)
	if err != nil {
		return fmt.Errorf("failed to check base DN: %w", err)
	}
	if !baseExists {
		return fmt.Errorf("base DN does not exist in database: %s", cfg.LDAP.BaseDN)
	}

	if cfg.Server.Port <= 0 {
		return fmt.Errorf("LDAP_PORT must be greater than zero")
	}
	address := healthcheckAddress(cfg.Server.BindAddress, cfg.Server.Port)
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("LDAP listener is not reachable at %s: %w", address, err)
	}
	conn.Close()

	return nil
}

func healthcheckAddress(bindAddress string, port int) string {
	host := bindAddress
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}
