-- Rollback Phase 3: Restore redundant columns to specialized tables
-- This reverses the optimization by adding uid, cn, ou columns back

-- Drop specialized composite indexes
DROP INDEX IF EXISTS idx_attributes_uid_lookup;
DROP INDEX IF EXISTS idx_attributes_cn_lookup;
DROP INDEX IF EXISTS idx_attributes_ou_lookup;

-- Recreate users table with uid column
CREATE TABLE users_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER UNIQUE NOT NULL,
    uid TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

-- Populate uid from attributes table
INSERT INTO users_new (id, entry_id, uid, password_hash)
SELECT u.id, u.entry_id,
    (SELECT value FROM attributes WHERE entry_id = u.entry_id AND name = 'uid' LIMIT 1) as uid,
    u.password_hash
FROM users u;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

CREATE INDEX IF NOT EXISTS idx_users_entry_id ON users(entry_id);
CREATE INDEX IF NOT EXISTS idx_users_uid ON users(uid);

-- Recreate groups table with cn column
CREATE TABLE groups_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER UNIQUE NOT NULL,
    cn TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

INSERT INTO groups_new (id, entry_id, cn)
SELECT g.id, g.entry_id,
    (SELECT value FROM attributes WHERE entry_id = g.entry_id AND name = 'cn' LIMIT 1) as cn
FROM groups g;

DROP TABLE groups;
ALTER TABLE groups_new RENAME TO groups;

CREATE INDEX IF NOT EXISTS idx_groups_entry_id ON groups(entry_id);
CREATE INDEX IF NOT EXISTS idx_groups_cn ON groups(cn);

-- Recreate organizational_units table with ou column
CREATE TABLE organizational_units_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER UNIQUE NOT NULL,
    ou TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

INSERT INTO organizational_units_new (id, entry_id, ou)
SELECT o.id, o.entry_id,
    (SELECT value FROM attributes WHERE entry_id = o.entry_id AND name = 'ou' LIMIT 1) as ou
FROM organizational_units o;

DROP TABLE organizational_units;
ALTER TABLE organizational_units_new RENAME TO organizational_units;

CREATE INDEX IF NOT EXISTS idx_organizational_units_entry_id ON organizational_units(entry_id);
CREATE INDEX IF NOT EXISTS idx_organizational_units_ou ON organizational_units(ou);
