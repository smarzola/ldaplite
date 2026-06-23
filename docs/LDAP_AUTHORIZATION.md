# LDAP Authorization

LDAPLite supports a small LDAP write-authorization model intended for app bind
users.

## Default Behavior

- Unbound connections cannot search or write normal directory entries.
- Anonymous binds can search normal entries only when `LDAP_ALLOW_ANONYMOUS_BIND`
  is enabled.
- Authenticated non-anonymous users can bind, search, compare, and write unless
  they are members of the reserved read-only group.
- RootDSE and schema discovery remain public.

## Read-Only Service Accounts

Create application bind users under `ou=users,<baseDN>` and add them to:

```text
cn=ldaplite.readonly,ou=groups,<baseDN>
```

Members of this group can bind, search, and compare. They cannot Add, Modify,
or Delete; those operations return LDAP `insufficientAccessRights` (`50`).
Nested group membership is honored by the same membership check used elsewhere
in LDAPLite.

Example LDIF for `dc=example,dc=com`:

```ldif
dn: uid=appbind,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: appbind
cn: Application Bind
sn: Bind
userPassword: change-me

dn: cn=ldaplite.readonly,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: ldaplite.readonly
member: uid=appbind,ou=users,dc=example,dc=com
```

Use `uid=appbind,ou=users,dc=example,dc=com` as the bind DN in applications
that only need directory reads.

## Non-Goals

- This is not a full ACL system.
- It does not grant per-subtree or per-attribute permissions.
- It does not change Web UI authorization; the Web UI still requires membership
  in `cn=ldaplite.admin,ou=groups,<baseDN>`.
- It does not make anonymous bind writable.
