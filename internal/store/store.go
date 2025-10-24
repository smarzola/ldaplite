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

	// User operations
	GetUser(ctx context.Context, dn string) (*models.User, error)
	GetUserByUID(ctx context.Context, uid string) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) error
	UpdateUser(ctx context.Context, user *models.User) error
	DeleteUser(ctx context.Context, dn string) error
	SearchUsers(ctx context.Context, baseDN string, filter string) ([]*models.User, error)

	// Group operations
	GetGroup(ctx context.Context, dn string) (*models.Group, error)
	GetGroupByName(ctx context.Context, name string) (*models.Group, error)
	CreateGroup(ctx context.Context, group *models.Group) error
	UpdateGroup(ctx context.Context, group *models.Group) error
	DeleteGroup(ctx context.Context, dn string) error
	SearchGroups(ctx context.Context, baseDN string, filter string) ([]*models.Group, error)

	// Group membership operations
	AddGroupMember(ctx context.Context, groupDN, memberDN string) error
	RemoveGroupMember(ctx context.Context, groupDN, memberDN string) error
	GetGroupMembers(ctx context.Context, groupDN string) ([]*models.Entry, error)
	GetGroupMembersRecursive(ctx context.Context, groupDN string, maxDepth int) ([]*models.Entry, error)
	GetUserGroups(ctx context.Context, userDN string) ([]*models.Group, error)
	GetUserGroupsRecursive(ctx context.Context, userDN string, maxDepth int) ([]*models.Group, error)
	IsMemberOf(ctx context.Context, memberDN, groupDN string) (bool, error)

	// Organizational Unit operations
	GetOU(ctx context.Context, dn string) (*models.OrganizationalUnit, error)
	CreateOU(ctx context.Context, ou *models.OrganizationalUnit) error
	UpdateOU(ctx context.Context, ou *models.OrganizationalUnit) error
	DeleteOU(ctx context.Context, dn string) error
	SearchOUs(ctx context.Context, baseDN string) ([]*models.OrganizationalUnit, error)

	// Miscellaneous
	GetAllEntries(ctx context.Context) ([]*models.Entry, error)
	GetChildren(ctx context.Context, dn string) ([]*models.Entry, error)
}
