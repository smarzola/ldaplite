# Grafana LDAP Integration

This recipe configures Grafana LDAP authentication against LDAPLite using
`memberOf` for group role mapping.

Reference: https://grafana.com/docs/grafana/latest/setup-grafana/configure-access/configure-authentication/ldap/

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
| Group base | `ou=groups,dc=example,dc=com` |
| Bind DN | `uid=appbind,ou=users,dc=example,dc=com` |

Create the bind user and add it to
`cn=ldaplite.readonly,ou=groups,dc=example,dc=com`.

## ldap.toml

```toml
[[servers]]
host = "ldaplite"
port = 3389
use_ssl = false
start_tls = false
ssl_skip_verify = false
bind_dn = "uid=appbind,ou=users,dc=example,dc=com"
bind_password = "app-bind-change-me"
search_filter = "(&(objectClass=inetOrgPerson)(uid=%s))"
search_base_dns = ["ou=users,dc=example,dc=com"]

[servers.attributes]
name = "displayName"
surname = "sn"
username = "uid"
member_of = "memberOf"
email = "mail"

[[servers.group_mappings]]
group_dn = "cn=grafana_admins,ou=groups,dc=example,dc=com"
org_role = "Admin"
grafana_admin = true

[[servers.group_mappings]]
group_dn = "cn=grafana_editors,ou=groups,dc=example,dc=com"
org_role = "Editor"

[[servers.group_mappings]]
group_dn = "*"
org_role = "Viewer"
```

Create the groups referenced by `group_dn` under
`ou=groups,dc=example,dc=com` and add users by DN.

## Smoke Tests

Verify user attributes:

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD" \
  -b "ou=users,dc=example,dc=com" \
  "(&(objectClass=inetOrgPerson)(uid=admin))" \
  uid mail displayName sn memberOf
```

Verify the mapped group exists:

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD" \
  -b "ou=groups,dc=example,dc=com" \
  "(cn=grafana_admins)" \
  cn member
```

## Known Limitations

- Read-only app bind users must be members of
  `cn=ldaplite.readonly,ou=groups,dc=example,dc=com`.
- Use native LDAPS/StartTLS or the [LDAPS TLS sidecar guide](../deployment/ldaps-tls-sidecar.md)
  for production traffic.
- Do not use Active Directory recursive matching-rule examples with LDAPLite.
  LDAPLite exposes computed nested `memberOf`, but it does not implement the AD
  `1.2.840.113556.1.4.1941` matching rule.
