package store

import (
	"context"

	"github.com/smarzola/ldaplite/internal/models"
)

type SearchScope int

const (
	SearchScopeBaseObject SearchScope = iota
	SearchScopeSingleLevel
	SearchScopeWholeSubtree
)

type SearchOptions struct {
	BaseDN          string
	Filter          string
	Scope           SearchScope
	IncludeMemberOf bool
}

type EntryOptions struct {
	IncludeMemberOf bool
}

// Store defines the interface for LDAP data storage
type Store interface {
	// Initialize sets up the database and runs migrations
	Initialize(ctx context.Context) error

	// Close closes the database connection
	Close() error

	// Entry operations
	GetEntry(ctx context.Context, dn string) (*models.Entry, error)
	GetEntryWithOptions(ctx context.Context, dn string, options EntryOptions) (*models.Entry, error)
	CreateEntry(ctx context.Context, entry *models.Entry) error
	UpdateEntry(ctx context.Context, entry *models.Entry) error
	DeleteEntry(ctx context.Context, dn string) error
	SearchEntries(ctx context.Context, baseDN string, filter string) ([]*models.Entry, error)
	SearchEntriesWithOptions(ctx context.Context, options SearchOptions) ([]*models.Entry, error)
	EntryExists(ctx context.Context, dn string) (bool, error)

	// Authentication and Authorization
	GetUserPasswordHash(ctx context.Context, uid string) (passwordHash string, dn string, err error)
	GetUserPasswordHashByDN(ctx context.Context, dn string) (passwordHash string, canonicalDN string, err error)
	IsUserInGroup(ctx context.Context, userDN, groupDN string) (bool, error)
}
