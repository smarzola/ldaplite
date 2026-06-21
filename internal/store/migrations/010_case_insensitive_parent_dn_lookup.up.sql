CREATE INDEX IF NOT EXISTS idx_entries_lower_parent_dn
ON entries(LOWER(parent_dn));
