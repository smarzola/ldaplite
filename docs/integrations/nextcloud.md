# Nextcloud LDAP Integration

This recipe configures Nextcloud's LDAP user backend against LDAPLite.

Reference: https://docs.nextcloud.com/server/latest/admin_manual/configuration_user/user_auth_ldap.html

## LDAPLite Assumptions

```bash
LDAP_BASE_DN=dc=example,dc=com
LDAP_ADMIN_PASSWORD=change-me
LDAP_PORT=3389
```

Default directory layout:

| Purpose | DN |
| --- | --- |
| Base DN | `dc=example,dc=com` |
| User base | `ou=users,dc=example,dc=com` |
| Group base | `ou=groups,dc=example,dc=com` |
| Bind DN | `uid=admin,ou=users,dc=example,dc=com` |

Use the admin bind DN only until LDAPLite has read-only service accounts.

## Server Tab

| Nextcloud setting | LDAPLite value |
| --- | --- |
| Host | `ldap://ldaplite:3389` |
| Port | `3389` |
| User DN | `uid=admin,ou=users,dc=example,dc=com` |
| Password | Value of `LDAP_ADMIN_PASSWORD` |
| Base DN | `dc=example,dc=com` |

Use `ldap://127.0.0.1:3389` for local development. Use an LDAPS sidecar/proxy
or private network for production.

## Users Tab

Use raw filter mode:

```ldap
(objectClass=inetOrgPerson)
```

If you want only a specific group to appear in Nextcloud, create the group and
use `memberOf`:

```ldap
(&(objectClass=inetOrgPerson)(memberOf=cn=nextcloud_users,ou=groups,dc=example,dc=com))
```

## Login Attributes Tab

Allow username or email login with raw filter mode:

```ldap
(&(objectClass=inetOrgPerson)(|(uid=%uid)(mail=%uid)))
```

## Groups Tab

Use raw filter mode:

```ldap
(objectClass=groupOfNames)
```

Group display name:

```text
cn
```

Group membership is DN-based through the `member` attribute. LDAPLite also
computes `memberOf` for user entries.

## Expert Settings

Use these LDAP attributes where Nextcloud asks for stable IDs or display
attributes:

| Purpose | LDAPLite attribute |
| --- | --- |
| Internal username / stable user id | `uuid` |
| Username | `uid` |
| Email | `mail` |
| Display name | `displayName` |
| First name | `givenName` |
| Last name | `sn` |
| Group name | `cn` |

LDAPLite also exposes `entryUUID` as an operational stable identifier, but
`uuid` is returned as a normal compatibility attribute and is easier to use with
clients.

## Smoke Tests

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w "$LDAP_ADMIN_PASSWORD" \
  -b "ou=users,dc=example,dc=com" \
  "(&(objectClass=inetOrgPerson)(|(uid=admin)(mail=admin)))" \
  uuid uid mail displayName memberOf
```

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w "$LDAP_ADMIN_PASSWORD" \
  -b "ou=groups,dc=example,dc=com" \
  "(objectClass=groupOfNames)" \
  uuid cn member
```

## Known Limitations

- LDAPLite does not currently provide read-only bind users. Use admin bind only
  in trusted deployments until service-account authorization is available.
- LDAPLite does not currently terminate native LDAPS or StartTLS. Use private
  networking, VPN, or an external TLS sidecar/proxy for production traffic.
- Nextcloud UI probes may offer object classes or filters from other directory
  servers. Use the raw filters above when auto-detection does not choose
  LDAPLite's `inetOrgPerson` and `groupOfNames` schema.
