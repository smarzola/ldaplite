-- Remove all group memberships (rollback to unsynchronized state)
-- Note: This doesn't remove the member attributes, only the group_members junction table entries
DELETE FROM group_members;
