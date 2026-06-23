# LDAP Authorization

LDAPLite supports a small capability-based authorization model intended for
simple directory administration and app bind users.

This is a breaking change from older LDAPLite versions. Authenticated users are
no longer writable by default.

## Default Behavior

- Unbound connections cannot search or write normal directory entries.
- Anonymous binds can search normal entries only when `LDAP_ALLOW_ANONYMOUS_BIND`
  is enabled.
- Authenticated non-anonymous users can bind, search, compare, and change their
  own password.
- Authenticated non-admin users cannot Add, Modify, or Delete arbitrary
  directory entries.
- Members of `cn=ldaplite.admin,ou=groups,<baseDN>` can Add, Modify, and Delete
  directory entries.
- RootDSE and schema discovery remain public.

LDAP Add, Modify, and Delete operations return `insufficientAccessRights` (`50`)
when the bound user does not have the required capability.

Self-service password changes are intentionally narrow: a user may modify only
their own `userPassword` value through the password-processing path. They cannot
combine that with other attribute changes or reset another user's password.

## Read-Only Service Accounts

Create application bind users under `ou=users,<baseDN>` and add them to:

```text
cn=ldaplite.readonly,ou=groups,<baseDN>
```

Authenticated users are read-only by default, so this group is no longer needed
to remove write access from ordinary users. Keep using it for application bind
users because it makes intent explicit in the directory and in integration
recipes.

Members of this group can bind, search, and compare. Like other non-admin users,
they cannot Add, Modify, or Delete arbitrary entries; those operations return
LDAP `insufficientAccessRights` (`50`). Nested group membership is honored by
the same membership check used elsewhere in LDAPLite.

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

## Built-In Capability Groups

LDAPLite keeps authorization intentionally coarse and group-based:

- `cn=ldaplite.admin,ou=groups,<baseDN>` grants directory write,
  group-management, password-reset, and full Web UI administration capability.
- `cn=ldaplite.readonly,ou=groups,<baseDN>` is the recommended marker for
  application bind users that should only read directory data. Authenticated
  users are already read-only by default, but this group keeps service-account
  intent visible.
- `cn=ldaplite.password,ou=groups,<baseDN>` grants account-only Web UI access
  for self-service password changes when the user is not also an admin or
  ordinary directory-read user. Admin membership takes precedence when a user is
  in multiple built-in groups.

## Web UI Roles

The embedded Web UI resolves capabilities on the server and shows only the
surfaces the actor can use:

- Admin users land in the directory console and can search, filter, paginate,
  inspect entry details, copy DNs and attribute values, create users/groups/OUs,
  edit attributes, reset user passwords, manage group members, and delete
  entries where the server allows it.
- Read-only users land in lookup-only directory views. They can search,
  paginate, inspect detail sheets, and copy DNs/attribute values, but they do
  not see write controls and cannot reach mutating Web UI/API endpoints.
- Password-only users land on the account surface. They can change their own
  password, but cannot access directory search, detail, or administration
  routes.

Hidden controls are not the security boundary. Mutating Web UI/API routes are
also protected by server-side capabilities and same-origin checks. Password-only
users are denied directory API access server-side, even if they request those
routes directly.

## Non-Goals

- This is not a full ACL system.
- It does not grant per-subtree or per-attribute permissions.
- It does not make anonymous bind writable.
