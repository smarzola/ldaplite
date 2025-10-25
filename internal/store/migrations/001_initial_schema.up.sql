-- Entries table: stores all LDAP entries (ou, user, group)
CREATE TABLE IF NOT EXISTS entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dn TEXT UNIQUE NOT NULL,
    parent_dn TEXT,
    object_class TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Attributes table: stores all attributes for entries
CREATE TABLE IF NOT EXISTS attributes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    value TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

-- Users table: optimized for user queries
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER UNIQUE NOT NULL,
    uid TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

-- Groups table: optimized for group queries
CREATE TABLE IF NOT EXISTS groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER UNIQUE NOT NULL,
    cn TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

-- Group members: many-to-many relationship (supports nested groups)
CREATE TABLE IF NOT EXISTS group_members (
    group_entry_id INTEGER NOT NULL,
    member_entry_id INTEGER NOT NULL,
    PRIMARY KEY (group_entry_id, member_entry_id),
    FOREIGN KEY (group_entry_id) REFERENCES entries(id) ON DELETE CASCADE,
    FOREIGN KEY (member_entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

-- Organizational Units table: optimized for OU queries
CREATE TABLE IF NOT EXISTS organizational_units (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER UNIQUE NOT NULL,
    ou TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);
