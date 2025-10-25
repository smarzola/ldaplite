package store

import "embed"

// migrationsFS embeds the migration files into the binary
//
//go:embed migrations/*.sql
var migrationsFS embed.FS
