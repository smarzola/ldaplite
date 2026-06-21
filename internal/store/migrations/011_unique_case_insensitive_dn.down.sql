DROP INDEX IF EXISTS idx_entries_lower_dn;

CREATE INDEX IF NOT EXISTS idx_entries_lower_dn
ON entries(LOWER(dn));
