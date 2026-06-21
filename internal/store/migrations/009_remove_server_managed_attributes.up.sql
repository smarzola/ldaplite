-- Remove server-managed attributes from the generic EAV table.
-- These values are owned by entries, users, and group_members instead.
DELETE FROM attributes
WHERE LOWER(name) IN (
  'userpassword',
  'objectclass',
  'createtimestamp',
  'modifytimestamp',
  'memberof'
);
