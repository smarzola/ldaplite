# Dex LDAP Integration

This recipe configures Dex's LDAP connector against LDAPLite.

Reference: https://dexidp.io/docs/connectors/ldap/

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
| User base | `ou=users,dc=example,dc=com` |
| Group base | `ou=groups,dc=example,dc=com` |
| Bind DN | `uid=appbind,ou=users,dc=example,dc=com` |

Create the bind user and add it to
`cn=ldaplite.readonly,ou=groups,dc=example,dc=com`. Dex uses the bind DN to
search for users and groups, then binds as the found user to verify the login
password.

## Dex Connector

```yaml
connectors:
  - type: ldap
    id: ldaplite
    name: LDAPLite
    config:
      host: ldaplite:3389
      insecureNoSSL: true
      bindDN: uid=appbind,ou=users,dc=example,dc=com
      bindPW: app-bind-change-me
      usernamePrompt: Username
      userSearch:
        baseDN: ou=users,dc=example,dc=com
        filter: "(objectClass=inetOrgPerson)"
        username: uid
        idAttr: uuid
        emailAttr: mail
        nameAttr: displayName
        preferredUsernameAttr: uid
      groupSearch:
        baseDN: ou=groups,dc=example,dc=com
        filter: "(objectClass=groupOfNames)"
        userMatchers:
          - userAttr: DN
            groupAttr: member
        nameAttr: cn
```

`DN` is case-sensitive in Dex configuration. Use `userAttr: DN` with LDAPLite
because `groupOfNames.member` stores user DNs.

## Smoke Tests

Verify user lookup:

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD" \
  -b "ou=users,dc=example,dc=com" \
  "(&(objectClass=inetOrgPerson)(uid=admin))" \
  uuid uid mail displayName
```

Verify group lookup by user DN:

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD" \
  -b "ou=groups,dc=example,dc=com" \
  "(&(objectClass=groupOfNames)(member=uid=admin,ou=users,dc=example,dc=com))" \
  cn member
```

## Known Limitations

- Dex strongly recommends TLS for LDAP because it binds with the user's plain
  password. LDAPLite does not currently terminate native LDAPS or StartTLS; use
  a trusted private network only for development and the [LDAPS TLS sidecar guide](../deployment/ldaps-tls-sidecar.md) for
  production.
- Read-only app bind users must be members of
  `cn=ldaplite.readonly,ou=groups,dc=example,dc=com`.
- Dex recursive group search options should not be configured to depend on AD
  matching rules. LDAPLite's direct `member` group search and computed nested
  `memberOf` behavior are separate features.
