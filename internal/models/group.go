package models

import (
	"fmt"
	"strings"
)

// Group represents an LDAP group (groupOfNames)
type Group struct {
	*Entry
	CN      string
	Members []string // DNs of members
}

// NewGroup creates a new group entry
func NewGroup(baseDN, cn, description string) *Group {
	groupDN := fmt.Sprintf("cn=%s,ou=groups,%s", cn, baseDN)
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

// RemoveMember removes a member from the group
func (g *Group) RemoveMember(memberDN string) error {
	for i, m := range g.Members {
		if m == memberDN {
			g.Members = append(g.Members[:i], g.Members[i+1:]...)
			return g.Entry.RemoveAttributeValue("member", memberDN)
		}
	}
	return fmt.Errorf("member %s not found in group", memberDN)
}

// HasMember checks if a DN is a member of the group (direct membership only)
func (g *Group) HasMember(memberDN string) bool {
	for _, m := range g.Members {
		if m == memberDN {
			return true
		}
	}
	return false
}

// GetMembers returns all direct members of the group
func (g *Group) GetMembers() []string {
	return g.Members
}

// LoadMembersFromAttributes loads members from the entry attributes
func (g *Group) LoadMembersFromAttributes() {
	g.Members = g.Entry.GetAttributes("member")
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

// SetDescription sets the group's description
func (g *Group) SetDescription(description string) {
	g.Entry.SetAttribute("description", description)
}

// GetDescription returns the group's description
func (g *Group) GetDescription() string {
	return g.Entry.GetAttribute("description")
}

// ExtractCNFromDN extracts the CN from a DN
// e.g., "cn=developers,ou=groups,dc=example,dc=com" -> "developers"
func ExtractCNFromDN(dn string) (string, error) {
	parts := strings.SplitN(dn, ",", 2)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid DN format: %s", dn)
	}

	rdnParts := strings.SplitN(parts[0], "=", 2)
	if len(rdnParts) != 2 || strings.ToLower(rdnParts[0]) != "cn" {
		return "", fmt.Errorf("DN does not contain cn: %s", dn)
	}

	return rdnParts[1], nil
}

// ExtractOUFromDN extracts the OU from a DN
// e.g., "cn=developers,ou=groups,dc=example,dc=com" -> "groups"
func ExtractOUFromDN(dn string) (string, error) {
	// Split by comma and find the first "ou=" component
	parts := strings.Split(dn, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "ou=") {
			ouParts := strings.SplitN(part, "=", 2)
			if len(ouParts) == 2 {
				return ouParts[1], nil
			}
		}
	}
	return "", fmt.Errorf("no OU found in DN: %s", dn)
}
