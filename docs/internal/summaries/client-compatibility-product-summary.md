# Client Compatibility Product Summary

This summarizes the client-compatibility goal-loop work on branch
`client-compatibility-goal-prompt`.

## Commits

- `7d3e027` - `docs: add client compatibility goal loop`
- `09d1921` - `feat: add stable ldap entry ids`
- `66fd3f7` - `test: cover pocket id ldap expectations`
- `d0509ca` - `docs: add ldap client integration recipes`
- `f3e453f` - `feat: implement ldap compare semantics`
- `c8b056d` - `feat: add read-only ldap service accounts`
- `351b191` - `test: document ldaps tls sidecar path`
- `2205b0b` - `docs: design ldif import export path`

## Files Changed By Area

- Goal and product tracking:
  - `docs/internal/prompts/client-compatibility-product-goal-prompt.md`
  - `docs/CLIENT_COMPATIBILITY_MATRIX.md`
  - `docs/internal/summaries/client-compatibility-product-summary.md`
  - `docs/ROADMAP.md`
  - `README.md`
- Stable IDs:
  - SQLite stable ID migrations
  - `internal/store/sqlite_entries.go`
  - `internal/server/search.go`
  - `internal/server/write.go`
  - `internal/server/discovery.go`
  - `internal/server/search_attributes_test.go`
  - `internal/server/write_test.go`
  - `internal/store/stable_ids_test.go`
  - `tests/functional/ad_compat_test.go`
- Client recipes:
  - `docs/integrations/README.md`
  - `docs/integrations/pocket-id.md`
  - `docs/integrations/authelia.md`
  - `docs/integrations/dex.md`
  - `docs/integrations/gitea-forgejo.md`
  - `docs/integrations/grafana.md`
  - `docs/integrations/nextcloud.md`
- Compare and authorization:
  - `internal/server/ldap.go`
  - `internal/server/write.go`
  - `internal/server/authz_test.go`
  - `internal/server/compare_test.go`
  - `docs/LDAP_AUTHORIZATION.md`
- TLS and bootstrap:
  - `docs/deployment/ldaps-tls-sidecar.md`
  - `tests/functional/tls_sidecar_compat_test.go`
  - `docs/internal/design-history/import-export-design.md`

## Client Compatibility Now Covered

- Pocket ID has a functional sync-read test and a recipe using LDAPLite's real
  `inetOrgPerson`, `groupOfNames`, `entryUUID`, `member`, and `memberOf` behavior.
- Authelia, Dex, Gitea/Forgejo, Grafana, and Nextcloud have LDAPLite-specific
  recipes with bind DN, bases, filters, attribute mapping, group mapping, TLS
  notes, and smoke tests.
- Read-only app bind users are available through
  `cn=ldaplite.readonly,ou=groups,<baseDN>`.
- LDAPS clients have a tested TLS sidecar deployment path.

## Remaining Follow-Ups

- Native LDAPS/StartTLS is still intentionally not implemented.
- Object class inheritance matching is not implemented; recipes use
  `(objectClass=inetOrgPerson)` rather than relying on `(objectClass=person)`.
- LDIF import/export commands are designed but not implemented.
- Additional client-shaped functional tests can be added if Authelia, Dex,
  Gitea/Forgejo, Grafana, or Nextcloud become release gates.
