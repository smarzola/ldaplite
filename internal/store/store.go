package store

import (
	"context"
	"github.com/smarzola/ldaplite/internal/models"
)

// Store defines the interface for LDAP data storage
type Store interface {
	// Initialize sets up the database and runs migrations
	Initialize(ctx context.Context) error

	// Close closes the database connection
	Close() error

	// Entry operations
	GetEntry(ctx context.Context, dn string) (*models.Entry, error)
	CreateEntry(ctx context.Context, entry *models.Entry) error
	UpdateEntry(ctx context.Context, entry *models.Entry) error
	DeleteEntry(ctx context.Context, dn string) error
	SearchEntries(ctx context.Context, baseDN string, filter string) ([]*models.Entry, error)
	EntryExists(ctx context.Context, dn string) (bool, error)

	// Miscellaneous
	GetAllEntries(ctx context.Context) ([]*models.Entry, error)
	GetChildren(ctx context.Context, dn string) ([]*models.Entry, error)
}
