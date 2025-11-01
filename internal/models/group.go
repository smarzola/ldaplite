package models

import (
	"fmt"
)

// Group represents an LDAP group (groupOfNames)
type Group struct {
	*Entry
	CN      string
	Members []string // DNs of members
}

// NewGroup creates a new group entry
func NewGroup(parentDN, cn, description string) *Group {
	groupDN := fmt.Sprintf("cn=%s,%s", cn, parentDN)
	entry := NewEntry(groupDN, string(ObjectClassGroupOfNames))

	// Set required attributes
	entry.SetAttribute("cn", cn)
	if description != "" {
		entry.SetAttribute("description", description)
	}

	return &Group{
		Entry:   entry,
		CN:      cn,
		Members: []string{},
	}
}

// AddMember adds a member to the group
// member can be a user DN or another group DN
func (g *Group) AddMember(memberDN string) {
	// Check if member already exists
	for _, m := range g.Members {
		if m == memberDN {
			return // Already a member
		}
	}
	g.Members = append(g.Members, memberDN)
	g.Entry.AddAttribute("member", memberDN)
}

// ValidateGroup validates that a group has all required attributes
func (g *Group) ValidateGroup() error {
	if err := g.Entry.Validate(); err != nil {
		return err
	}

	if g.Entry.GetAttribute("cn") == "" {
		return fmt.Errorf("required attribute cn is missing")
	}

	return nil
}
