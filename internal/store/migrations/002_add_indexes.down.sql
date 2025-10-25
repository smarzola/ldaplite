-- Drop indexes
DROP INDEX IF EXISTS idx_entries_parent_dn;
DROP INDEX IF EXISTS idx_entries_object_class;
DROP INDEX IF EXISTS idx_attributes_entry_id;
DROP INDEX IF EXISTS idx_attributes_name;
DROP INDEX IF EXISTS idx_attributes_name_value;
DROP INDEX IF EXISTS idx_users_entry_id;
DROP INDEX IF EXISTS idx_users_uid;
DROP INDEX IF EXISTS idx_groups_entry_id;
DROP INDEX IF EXISTS idx_groups_cn;
DROP INDEX IF EXISTS idx_organizational_units_entry_id;
DROP INDEX IF EXISTS idx_organizational_units_ou;
DROP INDEX IF EXISTS idx_group_members_group_entry_id;
DROP INDEX IF EXISTS idx_group_members_member_entry_id;
