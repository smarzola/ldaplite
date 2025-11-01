-- Backfill group_members table from attributes table
-- This ensures consistency between member attributes and the group_members junction table

-- Insert all group memberships from attributes into group_members
-- This handles any groups created before the sync logic was implemented
INSERT OR IGNORE INTO group_members (group_entry_id, member_entry_id)
SELECT
    g.id as group_entry_id,
    m.id as member_entry_id
FROM entries g
INNER JOIN attributes a ON g.id = a.entry_id
INNER JOIN entries m ON a.value = m.dn
WHERE g.object_class = 'groupOfNames'
  AND a.name = 'member'
  AND m.id IS NOT NULL;  -- Ensure member DN exists in entries table
