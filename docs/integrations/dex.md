# Dex LDAP Integration

This recipe configures Dex's LDAP connector against LDAPLite.

Reference: https://dexidp.io/docs/connectors/ldap/

## LDAPLite Assumptions

```bash
LDAP_BASE_DN=dc=example,dc=com
LDAP_ADMIN_PASSWORD=change-me
LDAP_PORT=3389
```

Default directory layout:

| Purpose | DN |
| --- | --- |
| User base | `ou=users,dc=example,dc=com` |
| Group base | `ou=groups,dc=example,dc=com` |
| Bind DN | `uid=admin,ou=users,dc=example,dc=com` |

Use the admin bind DN only until LDAPLite has read-only service accounts. Dex
uses the bind DN to search for users and groups, then binds as the found user to
verify the login password.

## Dex Connector

```yaml
connectors:
  - type: ldap
    id: ldaplite
    name: LDAPLite
    config:
      host: ldaplite:3389
      insecureNoSSL: true
      bindDN: uid=admin,ou=users,dc=example,dc=com
      bindPW: change-me
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
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w "$LDAP_ADMIN_PASSWORD" \
  -b "ou=users,dc=example,dc=com" \
  "(&(objectClass=inetOrgPerson)(uid=admin))" \
  uuid uid mail displayName
```

Verify group lookup by user DN:

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w "$LDAP_ADMIN_PASSWORD" \
  -b "ou=groups,dc=example,dc=com" \
  "(&(objectClass=groupOfNames)(member=uid=admin,ou=users,dc=example,dc=com))" \
  cn member
```

## Known Limitations

- Dex strongly recommends TLS for LDAP because it binds with the user's plain
  password. LDAPLite does not currently terminate native LDAPS or StartTLS; use
  a trusted private network only for development and a TLS sidecar/proxy for
  production.
- LDAPLite does not currently provide read-only bind users. Use admin bind only
  in trusted deployments until service-account authorization is available.
- Dex recursive group search options should not be configured to depend on AD
  matching rules. LDAPLite's direct `member` group search and computed nested
  `memberOf` behavior are separate features.
