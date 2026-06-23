# Gitea And Forgejo LDAP Integration

This recipe configures Gitea or Forgejo LDAP authentication against LDAPLite
using a bind DN, user search, and optional group-based admin filter.

Reference: https://docs.gitea.com/administration/authentication

## LDAPLite Assumptions

```bash
LDAP_BASE_DN=dc=example,dc=com
LDAP_ADMIN_PASSWORD=change-me
LDAP_APP_BIND_PASSWORD=app-bind-change-me
LDAP_PORT=3389
```

Default directory layout:

| Purpose | DN |
| --- | --- |
| User search base | `ou=users,dc=example,dc=com` |
| Group search base | `ou=groups,dc=example,dc=com` |
| Bind DN | `uid=appbind,ou=users,dc=example,dc=com` |

Create the bind user and add it to
`cn=ldaplite.readonly,ou=groups,dc=example,dc=com`.

## Authentication Source

In the Gitea or Forgejo admin UI, add an LDAP authentication source with these
values:

| Setting | LDAPLite value |
| --- | --- |
| Authentication Name | `LDAPLite` |
| Security Protocol | Unencrypted LDAP, or LDAPS through the [LDAPS TLS sidecar guide](../deployment/ldaps-tls-sidecar.md) |
| Host | `ldaplite` |
| Port | `3389` |
| Bind DN | `uid=appbind,ou=users,dc=example,dc=com` |
| Bind Password | Value of `LDAP_APP_BIND_PASSWORD` |
| User Search Base | `ou=users,dc=example,dc=com` |
| User Filter | `(&(objectClass=inetOrgPerson)(uid=%s))` |
| Username Attribute | `uid` |
| First Name Attribute | `givenName` |
| Surname Attribute | `sn` |
| Email Attribute | `mail` |

For login by username or email, use this user filter if your Gitea/Forgejo
version accepts LDAP OR filters in the UI:

```ldap
(&(objectClass=inetOrgPerson)(|(uid=%s)(mail=%s)))
```

## Admin Group Filter

Create a group for administrators:

```ldif
dn: cn=gitea_admins,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: gitea_admins
member: uid=admin,ou=users,dc=example,dc=com
```

Use `memberOf` for an admin filter:

```ldap
(memberOf=cn=gitea_admins,ou=groups,dc=example,dc=com)
```

LDAPLite computes `memberOf`, including nested group membership.

## Smoke Tests

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD" \
  -b "ou=users,dc=example,dc=com" \
  "(&(objectClass=inetOrgPerson)(uid=admin))" \
  uid givenName sn mail memberOf
```

## Known Limitations

- Read-only app bind users must be members of
  `cn=ldaplite.readonly,ou=groups,dc=example,dc=com`.
- LDAPLite does not currently terminate native LDAPS or StartTLS. Use private
  networking, VPN, or the [LDAPS TLS sidecar guide](../deployment/ldaps-tls-sidecar.md) for production traffic.
- Kerberos, SASL, and Windows SPNEGO/SSPI flows are out of scope for LDAPLite.
