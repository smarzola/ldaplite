-- Create ldaplite.admin group and add admin user to it
-- This migration works dynamically with any base DN

-- Step 1: Create admin group entry if it doesn't exist
-- Find the groups OU and construct the admin group DN from it
INSERT OR IGNORE INTO entries (dn, parent_dn, object_class, created_at, updated_at)
SELECT
    'cn=ldaplite.admin,' || groups_ou.dn as dn,
    groups_ou.dn as parent_dn,
    'groupOfNames' as object_class,
    CURRENT_TIMESTAMP as created_at,
    CURRENT_TIMESTAMP as updated_at
FROM (
    SELECT dn FROM entries
    WHERE dn LIKE 'ou=groups,%' AND object_class = 'organizationalUnit'
    LIMIT 1
) as groups_ou;

-- Step 2: Add cn attribute for the group
INSERT OR IGNORE INTO attributes (entry_id, name, value)
SELECT
    e.id,
    'cn' as name,
    'ldaplite.admin' as value
FROM entries e
WHERE e.dn LIKE 'cn=ldaplite.admin,ou=groups,%'
  AND e.object_class = 'groupOfNames';

-- Step 3: Add description attribute for the group
INSERT OR IGNORE INTO attributes (entry_id, name, value)
SELECT
    e.id,
    'description' as name,
    'LDAPLite administrators group - members have access to web UI' as value
FROM entries e
WHERE e.dn LIKE 'cn=ldaplite.admin,ou=groups,%'
  AND e.object_class = 'groupOfNames';

-- Step 4: Create groups table entry
INSERT OR IGNORE INTO groups (entry_id)
SELECT id FROM entries
WHERE dn LIKE 'cn=ldaplite.admin,ou=groups,%'
  AND object_class = 'groupOfNames';

-- Step 5: Add admin user as member of the group
-- Find both the admin user and the admin group, then create the membership
INSERT OR IGNORE INTO group_members (group_entry_id, member_entry_id)
SELECT
    g.id as group_entry_id,
    u.id as member_entry_id
FROM entries g
CROSS JOIN entries u
WHERE g.dn LIKE 'cn=ldaplite.admin,ou=groups,%'
  AND g.object_class = 'groupOfNames'
  AND u.dn LIKE 'uid=admin,ou=users,%'
  AND u.object_class = 'inetOrgPerson';

-- Step 6: Add member attribute to the group pointing to admin user
INSERT OR IGNORE INTO attributes (entry_id, name, value)
SELECT
    g.id,
    'member' as name,
    u.dn as value
FROM entries g
CROSS JOIN entries u
WHERE g.dn LIKE 'cn=ldaplite.admin,ou=groups,%'
  AND g.object_class = 'groupOfNames'
  AND u.dn LIKE 'uid=admin,ou=users,%'
  AND u.object_class = 'inetOrgPerson';
