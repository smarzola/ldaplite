package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/smarzola/ldaplite/internal/models"
)

func syncGroupMembers(ctx context.Context, tx *sql.Tx, groupEntryID int64, groupDN string, memberDNs []string, replace bool) error {
	if len(memberDNs) == 0 {
		if replace {
			if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_entry_id = ?`, groupEntryID); err != nil {
				return fmt.Errorf("failed to delete group members: %w", err)
			}
		}
		return nil
	}

	memberEntryIDs := make([]int64, 0, len(memberDNs))
	for _, memberDN := range memberDNs {
		var memberEntryID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM entries WHERE dn = ?`, memberDN).Scan(&memberEntryID)
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: group member does not exist: %s", ErrConstraintViolation, memberDN)
		}
		if err != nil {
			return fmt.Errorf("failed to verify group member %s: %w", memberDN, err)
		}
		memberEntryIDs = append(memberEntryIDs, memberEntryID)
	}

	if replace {
		if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_entry_id = ?`, groupEntryID); err != nil {
			return fmt.Errorf("failed to delete group members: %w", err)
		}
	}

	for i, memberEntryID := range memberEntryIDs {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO group_members (group_entry_id, member_entry_id) VALUES (?, ?)`,
			groupEntryID,
			memberEntryID,
		); err != nil {
			return fmt.Errorf("failed to add member %s to group %s: %w", memberDNs[i], groupDN, err)
		}
	}

	return nil
}

// IsUserInGroup checks if a user is a member of a group, including membership
// through nested groups. A recursive CTE walks from the user's direct groups up
// through parent groups with cycle protection.
func (s *SQLiteStore) IsUserInGroup(ctx context.Context, userDN, groupDN string) (bool, error) {
	query := `
		WITH RECURSIVE user_groups(group_id, depth, path) AS (
			-- Direct groups containing the user
			SELECT gm.group_entry_id, 0, printf(',%d,', gm.group_entry_id)
			FROM group_members gm
			INNER JOIN entries user_entry ON gm.member_entry_id = user_entry.id
			WHERE user_entry.dn = ?

			UNION ALL

			-- Parent groups containing one of the user's groups
			SELECT gm.group_entry_id, ug.depth + 1, ug.path || gm.group_entry_id || ','
			FROM group_members gm
			INNER JOIN user_groups ug ON gm.member_entry_id = ug.group_id
			WHERE ug.depth < 100
			  AND instr(ug.path, printf(',%d,', gm.group_entry_id)) = 0
		)
		SELECT EXISTS(
			SELECT 1
			FROM user_groups ug
			INNER JOIN entries group_entry ON ug.group_id = group_entry.id
			WHERE group_entry.dn = ?
		)
	`
	var isMember bool
	err := s.db.QueryRowContext(ctx, query, userDN, groupDN).Scan(&isMember)
	if err != nil {
		return false, fmt.Errorf("failed to check group membership: %w", err)
	}
	return isMember, nil
}

// populateMemberOf adds the memberOf attribute to user entries (inetOrgPerson).
// This is a virtual attribute computed from the group_members table. It
// includes direct and nested group memberships with cycle protection.
//
// This function efficiently batches the lookup to minimize database queries:
// 1. Collect all user entry IDs
// 2. Single query to get all group memberships for those users
// 3. Populate memberOf attribute for each user entry
func (s *SQLiteStore) populateMemberOf(ctx context.Context, entries []*models.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	// Collect user entry IDs
	userEntryIDs := make([]int64, 0)
	userEntriesByID := make(map[int64]*models.Entry)
	for _, entry := range entries {
		if entry.IsUser() && entry.ID > 0 {
			userEntryIDs = append(userEntryIDs, entry.ID)
			userEntriesByID[entry.ID] = entry
		}
	}

	if len(userEntryIDs) == 0 {
		return nil
	}

	// Build query with placeholders for user entry IDs
	placeholders := make([]string, len(userEntryIDs))
	args := make([]interface{}, len(userEntryIDs))
	for i, id := range userEntryIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	// Query all direct and nested group memberships for these users.
	// Returns: user entry_id, group_dn.
	query := `
		WITH RECURSIVE memberships(user_id, group_id, group_dn, depth, path) AS (
			-- Direct groups containing each user
			SELECT gm.member_entry_id, gm.group_entry_id, g_entry.dn, 0, printf(',%d,', gm.group_entry_id)
			FROM group_members gm
			INNER JOIN entries g_entry ON gm.group_entry_id = g_entry.id
			WHERE gm.member_entry_id IN (` + strings.Join(placeholders, ",") + `)

			UNION ALL

			-- Parent groups containing one of the user's groups
			SELECT m.user_id, gm.group_entry_id, g_entry.dn, m.depth + 1, m.path || gm.group_entry_id || ','
			FROM memberships m
			INNER JOIN group_members gm ON gm.member_entry_id = m.group_id
			INNER JOIN entries g_entry ON gm.group_entry_id = g_entry.id
			WHERE m.depth < 100
			  AND instr(m.path, printf(',%d,', gm.group_entry_id)) = 0
		)
		SELECT DISTINCT user_id, group_dn
		FROM memberships
	`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to query group memberships: %w", err)
	}
	defer rows.Close()

	// Populate memberOf attribute for each user
	for rows.Next() {
		var memberEntryID int64
		var groupDN string
		if err := rows.Scan(&memberEntryID, &groupDN); err != nil {
			return fmt.Errorf("failed to scan group membership: %w", err)
		}

		if entry, ok := userEntriesByID[memberEntryID]; ok {
			entry.Attributes["memberof"] = append(entry.Attributes["memberof"], groupDN)
		}
	}

	return rows.Err()
}
