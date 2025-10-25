package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/smarzola/ldaplite/internal/models"
)

// GetGroup retrieves a group by DN
func (s *SQLiteStore) GetGroup(ctx context.Context, dn string) (*models.Group, error) {
	entry, err := s.GetEntry(ctx, dn)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	if !entry.IsGroup() {
		return nil, fmt.Errorf("entry is not a group: %s", dn)
	}

	cn := entry.GetAttribute("cn")
	if cn == "" {
		return nil, fmt.Errorf("group missing cn attribute: %s", dn)
	}

	group := &models.Group{
		Entry:   entry,
		CN:      cn,
		Members: entry.GetAttributes("member"),
	}

	return group, nil
}

// GetGroupByName retrieves a group by name (CN)
func (s *SQLiteStore) GetGroupByName(ctx context.Context, name string) (*models.Group, error) {
	// Use JSON aggregation to fetch group with attributes in a single query
	query := `
		SELECT
			e.id,
			e.dn,
			e.parent_dn,
			e.object_class,
			e.created_at,
			e.updated_at,
			json_group_array(
				CASE WHEN a.name IS NOT NULL
				THEN json_object('name', a.name, 'value', a.value)
				ELSE NULL END
			) as attributes_json
		FROM entries e
		INNER JOIN groups g ON e.id = g.entry_id
		LEFT JOIN attributes a ON e.id = a.entry_id
		WHERE g.cn = ?
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
	`

	var entry models.Entry
	var attrsJSON string

	err := s.db.QueryRowContext(ctx, query, name).Scan(
		&entry.ID,
		&entry.DN,
		&entry.ParentDN,
		&entry.ObjectClass,
		&entry.CreatedAt,
		&entry.UpdatedAt,
		&attrsJSON,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get group by name: %w", err)
	}

	// Decode attributes from JSON
	entry.Attributes, err = decodeAttributesJSON(attrsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
	}

	group := &models.Group{
		Entry:   &entry,
		CN:      name,
		Members: entry.GetAttributes("member"),
	}

	return group, nil
}

// CreateGroup creates a new group
func (s *SQLiteStore) CreateGroup(ctx context.Context, group *models.Group) error {
	if err := group.ValidateGroup(); err != nil {
		return err
	}

	return s.CreateEntry(ctx, group.Entry)
}

// UpdateGroup updates an existing group
func (s *SQLiteStore) UpdateGroup(ctx context.Context, group *models.Group) error {
	if err := group.ValidateGroup(); err != nil {
		return err
	}

	return s.UpdateEntry(ctx, group.Entry)
}

// DeleteGroup deletes a group
func (s *SQLiteStore) DeleteGroup(ctx context.Context, dn string) error {
	return s.DeleteEntry(ctx, dn)
}

// SearchGroups searches for groups matching a filter
func (s *SQLiteStore) SearchGroups(ctx context.Context, baseDN string, filter string) ([]*models.Group, error) {
	entries, err := s.SearchEntries(ctx, baseDN, filter)
	if err != nil {
		return nil, err
	}

	var groups []*models.Group
	for _, entry := range entries {
		if entry.IsGroup() {
			group := &models.Group{
				Entry:   entry,
				CN:      entry.GetAttribute("cn"),
				Members: entry.GetAttributes("member"),
			}
			groups = append(groups, group)
		}
	}

	return groups, nil
}

// AddGroupMember adds a member to a group
func (s *SQLiteStore) AddGroupMember(ctx context.Context, groupDN, memberDN string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get group entry ID
	var groupID int64
	groupQuery := `SELECT id FROM entries WHERE dn = ?`
	err = tx.QueryRowContext(ctx, groupQuery, groupDN).Scan(&groupID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("group not found: %s", groupDN)
	}
	if err != nil {
		return fmt.Errorf("failed to get group: %w", err)
	}

	// Get member entry ID
	var memberID int64
	memberQuery := `SELECT id FROM entries WHERE dn = ?`
	err = tx.QueryRowContext(ctx, memberQuery, memberDN).Scan(&memberID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("member not found: %s", memberDN)
	}
	if err != nil {
		return fmt.Errorf("failed to get member: %w", err)
	}

	// Add to group_members table
	insertQuery := `INSERT OR IGNORE INTO group_members (group_entry_id, member_entry_id) VALUES (?, ?)`
	if _, err := tx.ExecContext(ctx, insertQuery, groupID, memberID); err != nil {
		return fmt.Errorf("failed to add group member: %w", err)
	}

	// Add to attributes table
	attrQuery := `INSERT INTO attributes (entry_id, name, value) VALUES (?, ?, ?)`
	if _, err := tx.ExecContext(ctx, attrQuery, groupID, "member", memberDN); err != nil {
		return fmt.Errorf("failed to add member attribute: %w", err)
	}

	return tx.Commit()
}

// RemoveGroupMember removes a member from a group
func (s *SQLiteStore) RemoveGroupMember(ctx context.Context, groupDN, memberDN string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get group entry ID
	var groupID int64
	groupQuery := `SELECT id FROM entries WHERE dn = ?`
	err = tx.QueryRowContext(ctx, groupQuery, groupDN).Scan(&groupID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("group not found: %s", groupDN)
	}
	if err != nil {
		return fmt.Errorf("failed to get group: %w", err)
	}

	// Get member entry ID
	var memberID int64
	memberQuery := `SELECT id FROM entries WHERE dn = ?`
	err = tx.QueryRowContext(ctx, memberQuery, memberDN).Scan(&memberID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("member not found: %s", memberDN)
	}
	if err != nil {
		return fmt.Errorf("failed to get member: %w", err)
	}

	// Remove from group_members table
	deleteGroupMembersQuery := `DELETE FROM group_members WHERE group_entry_id = ? AND member_entry_id = ?`
	if _, err := tx.ExecContext(ctx, deleteGroupMembersQuery, groupID, memberID); err != nil {
		return fmt.Errorf("failed to remove group member: %w", err)
	}

	// Remove from attributes table
	deleteAttrQuery := `DELETE FROM attributes WHERE entry_id = ? AND name = ? AND value = ?`
	if _, err := tx.ExecContext(ctx, deleteAttrQuery, groupID, "member", memberDN); err != nil {
		return fmt.Errorf("failed to remove member attribute: %w", err)
	}

	return tx.Commit()
}

// GetGroupMembers returns direct members of a group
func (s *SQLiteStore) GetGroupMembers(ctx context.Context, groupDN string) ([]*models.Entry, error) {
	// Use JSON aggregation to fetch members with attributes in a single query
	query := `
		SELECT
			e.id,
			e.dn,
			e.parent_dn,
			e.object_class,
			e.created_at,
			e.updated_at,
			json_group_array(
				CASE WHEN a.name IS NOT NULL
				THEN json_object('name', a.name, 'value', a.value)
				ELSE NULL END
			) as attributes_json
		FROM entries e
		INNER JOIN group_members gm ON e.id = gm.member_entry_id
		LEFT JOIN attributes a ON e.id = a.entry_id
		WHERE gm.group_entry_id = (SELECT id FROM entries WHERE dn = ?)
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
	`

	rows, err := s.db.QueryContext(ctx, query, groupDN)
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}
	defer rows.Close()

	var members []*models.Entry
	for rows.Next() {
		entry := &models.Entry{}
		var attrsJSON string

		if err := rows.Scan(
			&entry.ID,
			&entry.DN,
			&entry.ParentDN,
			&entry.ObjectClass,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&attrsJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		// Decode attributes from JSON
		entry.Attributes, err = decodeAttributesJSON(attrsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
		}

		members = append(members, entry)
	}

	return members, nil
}

// GetGroupMembersRecursive returns all members including nested groups
func (s *SQLiteStore) GetGroupMembersRecursive(ctx context.Context, groupDN string, maxDepth int) ([]*models.Entry, error) {
	var members []*models.Entry
	visited := make(map[string]bool)

	if err := s.resolveGroupMembersRecursive(ctx, groupDN, maxDepth, &visited, &members); err != nil {
		return nil, err
	}

	return members, nil
}

// resolveGroupMembersRecursive recursively resolves group members
func (s *SQLiteStore) resolveGroupMembersRecursive(ctx context.Context, groupDN string, depth int, visited *map[string]bool, members *[]*models.Entry) error {
	if depth <= 0 {
		return nil
	}

	// Avoid circular references
	if (*visited)[groupDN] {
		return nil
	}
	(*visited)[groupDN] = true

	// Get direct members
	directMembers, err := s.GetGroupMembers(ctx, groupDN)
	if err != nil {
		return err
	}

	for _, member := range directMembers {
		*members = append(*members, member)

		// If member is a group, recursively get its members
		if member.IsGroup() {
			if err := s.resolveGroupMembersRecursive(ctx, member.DN, depth-1, visited, members); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetUserGroups returns direct groups a user belongs to
func (s *SQLiteStore) GetUserGroups(ctx context.Context, userDN string) ([]*models.Group, error) {
	// Use JSON aggregation to fetch groups with attributes in a single query
	query := `
		SELECT
			e.id,
			e.dn,
			e.parent_dn,
			e.object_class,
			e.created_at,
			e.updated_at,
			g.cn,
			json_group_array(
				CASE WHEN a.name IS NOT NULL
				THEN json_object('name', a.name, 'value', a.value)
				ELSE NULL END
			) as attributes_json
		FROM entries e
		INNER JOIN group_members gm ON e.id = gm.group_entry_id
		INNER JOIN groups g ON e.id = g.entry_id
		LEFT JOIN attributes a ON e.id = a.entry_id
		WHERE gm.member_entry_id = (SELECT id FROM entries WHERE dn = ?)
		AND e.object_class = ?
		GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at, g.cn
	`

	rows, err := s.db.QueryContext(ctx, query, userDN, string(models.ObjectClassGroupOfNames))
	if err != nil {
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}
	defer rows.Close()

	var groups []*models.Group
	for rows.Next() {
		entry := &models.Entry{}
		var cn string
		var attrsJSON string

		if err := rows.Scan(
			&entry.ID,
			&entry.DN,
			&entry.ParentDN,
			&entry.ObjectClass,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&cn,
			&attrsJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		// Decode attributes from JSON
		entry.Attributes, err = decodeAttributesJSON(attrsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
		}

		group := &models.Group{
			Entry:   entry,
			CN:      entry.GetAttribute("cn"),
			Members: entry.GetAttributes("member"),
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// GetUserGroupsRecursive returns all groups a user belongs to including nested
func (s *SQLiteStore) GetUserGroupsRecursive(ctx context.Context, userDN string, maxDepth int) ([]*models.Group, error) {
	var groups []*models.Group
	visited := make(map[string]bool)

	if err := s.resolveUserGroupsRecursive(ctx, userDN, maxDepth, &visited, &groups); err != nil {
		return nil, err
	}

	return groups, nil
}

// resolveUserGroupsRecursive recursively resolves user groups
func (s *SQLiteStore) resolveUserGroupsRecursive(ctx context.Context, memberDN string, depth int, visited *map[string]bool, groups *[]*models.Group) error {
	if depth <= 0 {
		return nil
	}

	// Get direct groups
	directGroups, err := s.GetUserGroups(ctx, memberDN)
	if err != nil {
		return err
	}

	for _, group := range directGroups {
		// Avoid circular references
		if (*visited)[group.DN] {
			continue
		}
		(*visited)[group.DN] = true

		*groups = append(*groups, group)

		// Recursively get parent groups
		if err := s.resolveUserGroupsRecursive(ctx, group.DN, depth-1, visited, groups); err != nil {
			return err
		}
	}

	return nil
}

// IsMemberOf checks if a user/group is member of a group (direct or indirect)
func (s *SQLiteStore) IsMemberOf(ctx context.Context, memberDN, groupDN string) (bool, error) {
	// Check direct membership
	query := `
		SELECT COUNT(*) FROM group_members
		WHERE member_entry_id = (SELECT id FROM entries WHERE dn = ?)
		AND group_entry_id = (SELECT id FROM entries WHERE dn = ?)
	`

	var count int
	err := s.db.QueryRowContext(ctx, query, memberDN, groupDN).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check membership: %w", err)
	}

	if count > 0 {
		return true, nil
	}

	// Check if memberDN is a group with groups that contain groupDN
	memberEntry, err := s.GetEntry(ctx, memberDN)
	if err != nil {
		return false, err
	}

	if memberEntry == nil {
		return false, nil
	}

	if memberEntry.IsGroup() {
		// Recursively check if this group is a member of the target group
		groups, err := s.GetUserGroupsRecursive(ctx, memberDN, 10)
		if err != nil {
			return false, err
		}

		for _, group := range groups {
			if group.DN == groupDN {
				return true, nil
			}
		}
	}

	return false, nil
}
