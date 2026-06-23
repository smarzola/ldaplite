# LDAPLite Integration Guides

These recipes map common LDAP-consuming applications to LDAPLite's actual
schema and current operational limits.

Available guides:

- [Authelia](authelia.md)
- [Dex](dex.md)
- [Gitea and Forgejo](gitea-forgejo.md)
- [Grafana](grafana.md)
- [Nextcloud](nextcloud.md)
- [Pocket ID](pocket-id.md)

Shared assumptions:

- Users live under `ou=users,<baseDN>` and use `inetOrgPerson`.
- Groups live under `ou=groups,<baseDN>` and use `groupOfNames`.
- Group membership is DN-based through `member`.
- User reverse membership is exposed as computed `memberOf`.
- Stable IDs are exposed as `entryUUID` and `uuid`.
- Native LDAPS/StartTLS is not implemented yet; use private networking or a
  TLS sidecar/proxy where encryption is required.
- Read-only service accounts are not implemented yet; recipes use admin bind
  until that milestone lands.
