-- Remove ldaplite.admin group
DELETE FROM entries WHERE dn LIKE 'cn=ldaplite.admin,ou=groups,%';
