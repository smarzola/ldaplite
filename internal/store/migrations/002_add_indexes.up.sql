-- Indexes for entries table
CREATE INDEX IF NOT EXISTS idx_entries_parent_dn ON entries(parent_dn);
CREATE INDEX IF NOT EXISTS idx_entries_object_class ON entries(object_class);

-- Indexes for attributes table
CREATE INDEX IF NOT EXISTS idx_attributes_entry_id ON attributes(entry_id);
CREATE INDEX IF NOT EXISTS idx_attributes_name ON attributes(name);
CREATE INDEX IF NOT EXISTS idx_attributes_name_value ON attributes(name, value);

-- Indexes for users table
CREATE INDEX IF NOT EXISTS idx_users_entry_id ON users(entry_id);
CREATE INDEX IF NOT EXISTS idx_users_uid ON users(uid);

-- Indexes for groups table
CREATE INDEX IF NOT EXISTS idx_groups_entry_id ON groups(entry_id);
CREATE INDEX IF NOT EXISTS idx_groups_cn ON groups(cn);

-- Indexes for organizational_units table
CREATE INDEX IF NOT EXISTS idx_organizational_units_entry_id ON organizational_units(entry_id);
CREATE INDEX IF NOT EXISTS idx_organizational_units_ou ON organizational_units(ou);

-- Indexes for group_members table
CREATE INDEX IF NOT EXISTS idx_group_members_group_entry_id ON group_members(group_entry_id);
CREATE INDEX IF NOT EXISTS idx_group_members_member_entry_id ON group_members(member_entry_id);
