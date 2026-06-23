# Authelia LDAP Integration

This recipe configures Authelia's LDAP authentication backend against LDAPLite
using Authelia's `custom` LDAP implementation.

Reference: https://www.authelia.com/configuration/first-factor/ldap/

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
| Search base | `dc=example,dc=com` |
| User base | `ou=users,dc=example,dc=com` |
| Group base | `ou=groups,dc=example,dc=com` |
| Bind DN | `uid=appbind,ou=users,dc=example,dc=com` |

Create the bind user and add it to
`cn=ldaplite.readonly,ou=groups,dc=example,dc=com`.

## Authelia Configuration

```yaml
authentication_backend:
  ldap:
    address: ldap://ldaplite:3389
    implementation: custom
    timeout: 5s
    start_tls: false
    base_dn: dc=example,dc=com
    additional_users_dn: ou=users
    users_filter: "(&({username_attribute}={input})(objectClass=inetOrgPerson))"
    additional_groups_dn: ou=groups
    groups_filter: "(&(member={dn})(objectClass=groupOfNames))"
    group_search_mode: filter
    permit_referrals: false
    permit_unauthenticated_bind: false
    user: uid=appbind,ou=users,dc=example,dc=com
    password: app-bind-change-me
    attributes:
      username: uid
      display_name: displayName
      family_name: sn
      given_name: givenName
      mail: mail
      member_of: memberOf
      group_name: cn
```

Authelia replaces `{username_attribute}`, `{input}`, and `{dn}` inside filters.
LDAPLite supports the resulting `uid`, `objectClass`, and `member` filters.

## Group Authorization

Create groups under `ou=groups,dc=example,dc=com` and add users by DN:

```ldif
dn: cn=authelia_admins,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: authelia_admins
member: uid=admin,ou=users,dc=example,dc=com
```

Use the group DN in Authelia access-control rules:

```yaml
access_control:
  rules:
    - domain: admin.example.com
      policy: two_factor
      subject:
        - group:cn=authelia_admins,ou=groups,dc=example,dc=com
```

## Smoke Test

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD" \
  -b "ou=users,dc=example,dc=com" \
  "(&(uid=admin)(objectClass=inetOrgPerson))" \
  uid mail givenName sn memberOf
```

## Known Limitations

- Read-only app bind users must be members of
  `cn=ldaplite.readonly,ou=groups,dc=example,dc=com`.
- LDAPLite does not currently terminate native LDAPS or StartTLS. Use private
  networking, VPN, or the [LDAPS TLS sidecar guide](../deployment/ldaps-tls-sidecar.md) for production traffic.
- Do not use Authelia's AD recursive matching-rule examples with LDAPLite.
  LDAPLite computes nested `memberOf`, but it does not implement the AD
  `1.2.840.113556.1.4.1941` matching rule.
