-- Phase 3: Remove redundant columns from specialized tables
-- After this migration, specialized tables contain ONLY:
-- - users: entry_id, password_hash (security-sensitive data)
-- - groups: entry_id (for group_members foreign key)
-- - organizational_units: entry_id (for referential integrity)
--
-- All other attributes (uid, cn, ou) live in the attributes table with optimized indexes

-- Add specialized composite indexes BEFORE dropping columns
-- These indexes replace the dedicated column indexes with similar performance
CREATE INDEX IF NOT EXISTS idx_attributes_uid_lookup ON attributes(name, value) WHERE name = 'uid';
CREATE INDEX IF NOT EXISTS idx_attributes_cn_lookup ON attributes(name, value) WHERE name = 'cn';
CREATE INDEX IF NOT EXISTS idx_attributes_ou_lookup ON attributes(name, value) WHERE name = 'ou';

-- Drop redundant indexes on columns we're about to remove
DROP INDEX IF EXISTS idx_users_uid;
DROP INDEX IF EXISTS idx_groups_cn;
DROP INDEX IF EXISTS idx_organizational_units_ou;

-- SQLite doesn't support DROP COLUMN directly, so we recreate tables
-- This is safe because we're in a transaction and data is preserved

-- Recreate users table (keep only entry_id and password_hash)
CREATE TABLE users_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER UNIQUE NOT NULL,
    password_hash TEXT,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

INSERT INTO users_new (id, entry_id, password_hash)
SELECT id, entry_id, password_hash FROM users;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

-- Recreate index for users table
CREATE INDEX IF NOT EXISTS idx_users_entry_id ON users(entry_id);

-- Recreate groups table (keep only entry_id)
CREATE TABLE groups_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER UNIQUE NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

INSERT INTO groups_new (id, entry_id)
SELECT id, entry_id FROM groups;

DROP TABLE groups;
ALTER TABLE groups_new RENAME TO groups;

-- Recreate index for groups table
CREATE INDEX IF NOT EXISTS idx_groups_entry_id ON groups(entry_id);

-- Recreate organizational_units table (keep only entry_id)
CREATE TABLE organizational_units_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER UNIQUE NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

INSERT INTO organizational_units_new (id, entry_id)
SELECT id, entry_id FROM organizational_units;

DROP TABLE organizational_units;
ALTER TABLE organizational_units_new RENAME TO organizational_units;

-- Recreate index for organizational_units table
CREATE INDEX IF NOT EXISTS idx_organizational_units_entry_id ON organizational_units(entry_id);
