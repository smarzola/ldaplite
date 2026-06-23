# Pocket ID LDAP Integration

This recipe configures Pocket ID to synchronize users and groups from
LDAPLite.

Pocket ID's LDAP settings are configurable enough to match LDAPLite's default
directory shape, but a few defaults should be changed:

- Use `inetOrgPerson` for user searches instead of Pocket ID's example
  `(objectClass=person)` filter.
- Use `cn` for group names instead of Pocket ID's default `uid` group-name
  attribute.
- Use plain `ldap://` on a private network or use the
  [LDAPS TLS sidecar guide](../deployment/ldaps-tls-sidecar.md).
  LDAPLite does not currently terminate native LDAPS or StartTLS.

Reference: https://pocket-id.org/docs/configuration/ldap

## LDAPLite Assumptions

Example LDAPLite environment:

```bash
LDAP_BASE_DN=dc=example,dc=com
LDAP_ADMIN_PASSWORD=change-me
LDAP_APP_BIND_PASSWORD=app-bind-change-me
LDAP_PORT=3389
LDAP_DATABASE_PATH=/var/lib/ldaplite/ldaplite.db
```

Default directory layout:

| Purpose | DN |
| --- | --- |
| Search base | `dc=example,dc=com` |
| User base | `ou=users,dc=example,dc=com` |
| Group base | `ou=groups,dc=example,dc=com` |
| App bind DN | `uid=appbind,ou=users,dc=example,dc=com` |

Create the bind user and add it to
`cn=ldaplite.readonly,ou=groups,dc=example,dc=com`.

## Pocket ID LDAP Client Configuration

Set these values in Pocket ID's LDAP Client Configuration:

| Pocket ID setting | LDAPLite value |
| --- | --- |
| LDAP URL | `ldap://ldaplite:3389` |
| LDAP Bind DN | `uid=appbind,ou=users,dc=example,dc=com` |
| LDAP Bind Password | Value of `LDAP_APP_BIND_PASSWORD` |
| LDAP Search Base | `dc=example,dc=com` |
| User Search Filter | `(&(objectClass=inetOrgPerson)(uid=*))` |
| User Group Search Filter | `(objectClass=groupOfNames)` |

If Pocket ID and LDAPLite run on the same host during development, use
`ldap://127.0.0.1:3389`. In container deployments, use the LDAPLite service
name on a private Docker or Kubernetes network.

## Pocket ID Attribute Configuration

Set these values in Pocket ID's LDAP Attribute Configuration:

| Pocket ID setting | LDAPLite attribute |
| --- | --- |
| User Unique Identifier Attribute | `uuid` |
| Username Attribute | `uid` |
| User Mail Attribute | `mail` |
| User First Name Attribute | `givenName` |
| User Last Name Attribute | `sn` |
| User Group Membership Attribute | `memberOf` |
| Group Members Attribute | `member` |
| Group Unique Identifier Attribute | `uuid` |
| Group Name Attribute | `cn` |
| Admin Group Name | `_pocket_id_admins` |

LDAPLite also exposes `entryUUID` as an operational stable identifier. Pocket ID
examples use `uuid`, so prefer `uuid` in Pocket ID.

## Admin Group

Pocket ID can make LDAP-synced users administrators when they are members of
the configured admin group. Create the group before enabling LDAP login if you
need an LDAP-backed Pocket ID administrator.

Example LDIF:

```ldif
dn: cn=_pocket_id_admins,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: _pocket_id_admins
member: uid=admin,ou=users,dc=example,dc=com
```

Use a real user DN as `member`. Group membership in LDAPLite is DN-based.

Pocket ID treats LDAP-synchronized users and groups as LDAP-owned. After sync,
manage those users and groups in LDAPLite rather than in Pocket ID's Web UI.

## Smoke Tests

Verify bind:

```bash
ldapwhoami -H ldap://localhost:3389 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD"
```

Verify user sync attributes:

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD" \
  -b "ou=users,dc=example,dc=com" \
  "(&(objectClass=inetOrgPerson)(uid=*))" \
  uuid uid mail givenName sn memberOf
```

Verify group sync attributes:

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD" \
  -b "ou=groups,dc=example,dc=com" \
  "(objectClass=groupOfNames)" \
  uuid cn member
```

## Known Limitations

- Pocket ID's example `(objectClass=person)` user filter may not match LDAPLite
  users. Use `(objectClass=inetOrgPerson)` until LDAPLite implements
  objectClass inheritance matching.
- Read-only app bind users must be members of
  `cn=ldaplite.readonly,ou=groups,dc=example,dc=com`.
- LDAPLite does not currently terminate native LDAPS or StartTLS. Use a private
  network, VPN, or the [LDAPS TLS sidecar guide](../deployment/ldaps-tls-sidecar.md)
  when Pocket ID is not on the same trusted network.
