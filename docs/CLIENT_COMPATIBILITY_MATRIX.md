# LDAP Client Compatibility Matrix

This matrix tracks LDAPLite against real LDAP-consuming applications and the
product gaps identified from the lldap comparison.

Status labels:

- **Works now**: LDAPLite has the protocol/schema behavior in current source or
  tests.
- **Likely works**: LDAPLite has the required primitives, but this exact client
  has not been tested end-to-end.
- **Gap**: LDAPLite needs code, docs, or a tested deployment pattern.
- **Out of scope**: The expectation should not be implemented unless product
  direction changes.

## Current LDAPLite Baseline

Confirmed current strengths:

- Simple bind, search, add, modify, delete, RootDSE, schema discovery, and Who
  Am I are implemented.
- Search supports base, one-level, and subtree scopes; requested attributes;
  `1.1`, `*`, `+`; `typesOnly`; common equality, presence, substring, boolean,
  and timestamp filters.
- Users are `inetOrgPerson`; groups are `groupOfNames` with `member` DNs.
- `memberOf` is computed, read-only, and supports nested group membership.
- Functional tests exercise AD-like bind/search/member/memberOf/password/result
  code behavior through `github.com/go-ldap/ldap/v3`.
- Audit-grade structured logs, optional OpenTelemetry tracing, and
  Prometheus-compatible metrics are implemented and documented.

Confirmed current gaps:

- No generated stable `entryUUID` or `uuid` attributes yet.
- LDAP Compare currently returns false and needs real assertion handling.
- LDAP write authorization is coarse: any authenticated non-anonymous user can
  write.
- No native TLS/LDAPS/StartTLS; deployment guidance currently relies on an
  external TLS terminator.
- No LDIF/CSV/bootstrap import/export path yet.
- No tested per-client integration recipes yet.

## Consumer Matrix

| Consumer | Expected LDAP Pattern | LDAPLite Status | Next LDAPLite Work |
| --- | --- | --- | --- |
| Pocket ID | Periodic LDAP sync with bind DN, search base, user filter such as `(objectClass=person)`, group filter `(objectClass=groupOfNames)`, stable user/group unique attributes, `uid`, `mail`, `givenName`, `sn`, group `member`, and group name mapping. Examples use `ldaps://` and `uuid`. | **Partial / gap.** LDAPLite supports bind/search, `inetOrgPerson`, `groupOfNames`, `member`, `uid`, `mail`, `givenName`, and `sn`. Missing generated `uuid`/`entryUUID`; `objectClass=person` may not match because LDAPLite stores the primary class as `inetOrgPerson`; LDAPS needs a sidecar/proxy or native support. | Milestone 1 stable IDs; Milestone 2 Pocket ID functional test and recipe. Decide whether to make `(objectClass=person)` match `inetOrgPerson` inheritance or document `(objectClass=inetOrgPerson)`. |
| Authelia | Service bind user searches users and groups. Configurable `base_dn`, `additional_users_dn`, `users_filter`, `additional_groups_dn`, `groups_filter`, group search mode, `memberOf`, `cn`, `uid`, `mail`, `givenName`, `sn`, and TLS/StartTLS settings. | **Likely works for simple custom LDAP.** LDAPLite supports bind/search, `memberOf`, group `member`, and common user attributes. Missing read-only service-account authorization and tested recipe. Native StartTLS/LDAPS is missing. | Add Authelia recipe using `implementation: custom`, `ou=users`, `ou=groups`, `(objectClass=inetOrgPerson)`, and either `memberOf` or group filter mode. Add read-only bind strategy. |
| Dex LDAP connector | Service account bind, user search that combines a filter with username attribute, then bind as found user. Group search supports `userMatchers` such as user `DN` to group `member`; supports recursive group lookup using `recursionGroupAttr` for nested group schemas. | **Likely works for direct groups.** LDAPLite supports service bind via any user, user bind verification, user search by `uid`/`mail`/`userPrincipalName`, group `member` as DN, and nested `memberOf`. Dex recursive `groupSearch` using `recursionGroupAttr: member` needs a recipe/test to confirm expected behavior against group DNs. | Add Dex recipe and client-shaped functional test. Prefer `userAttr: DN`, `groupAttr: member`, `nameAttr: cn`, and `idAttr: DN` until stable IDs exist. |
| Gitea / Forgejo | LDAP via BindDN or simple auth. Needs host/port/TLS, optional bind DN, user search base/filter, username attribute, first name `givenName`, surname `sn`, required email `mail`, optional admin filter, and optional group membership verification using group base, group member attribute, and user attribute such as DN or `uid`. | **Likely works.** LDAPLite supports simple bind, BindDN search, `uid`, `givenName`, `sn`, `mail`, `member`, and `memberOf` filters. Missing read-only bind guidance and tested recipe. | Add Gitea/Forgejo recipe using `uid=%[1]s` or `(|(uid=%[1]s)(mail=%[1]s))`, group member attribute `member`, user attribute `dn`, and admin filter with `memberOf`. |
| Grafana | Bind DN is normally a read-only user. User lookup uses `search_filter` such as `(uid=%s)` and search base DNs. Attributes include `memberOf`, `mail`/`email`, display/name attributes. Group role mapping can use `memberOf`; POSIX fallback can search groups by `memberUid`. TLS/LDAPS and StartTLS are first-class config choices. | **Likely works for `memberOf` mapping.** LDAPLite supports bind, user search by `uid`/`cn`/`mail`, `memberOf`, group DNs, and common attributes. Missing read-only service account and LDAPS/StartTLS path. | Add Grafana recipe using `member_of = "memberOf"`, `email = "mail"`, `search_filter = "(uid=%s)"`, and group DN mappings. Document that AD recursive matching rule is out of scope. |
| Nextcloud | LDAP app uses read-only directory access. Needs host/port or `ldaps://`, user DN/bind user, base DN, user filters such as `inetOrgPerson` plus optional `memberOf`, login attributes such as `uid` and `mail`, group filters, group display name `cn`, group member association, and stable LDAP UUID/DN mapping. | **Partial / gap.** LDAPLite supports read/search patterns, `inetOrgPerson`, `memberOf`, `uid`, `mail`, group `cn`, and group `member`. Missing generated stable UUID attributes, read-only service account, and tested LDAPS guidance. Nextcloud nested group filters using AD matching rule are out of scope. | Add Nextcloud recipe after stable IDs. Test user listing, login filter by `uid`/`mail`, group filter, and `memberOf` gating. |
| Vaultwarden | Current Vaultwarden project documentation is centered on HTTP reverse-proxy deployment. Native LDAP authentication is not a standard first-party LDAP consumer path in the checked docs. | **Not a direct LDAP target.** LDAPLite may still serve the upstream IdP or auth proxy in front of Vaultwarden, but Vaultwarden itself should not drive LDAP protocol milestones unless a maintained LDAP integration is selected. | Treat as an indirect recipe later: LDAPLite -> Authelia/Pocket ID/other OIDC or forward-auth component -> Vaultwarden. Do not block core LDAP milestones on Vaultwarden-native LDAP. |

## Cross-Client Requirements

These requirements recur across multiple clients and should drive the milestone
order.

### Stable Identity Attributes

Clients that synchronize users and groups need durable unique identifiers.

Implementation questions:

- Should LDAPLite expose both `entryUUID` and `uuid`?
- Should `uuid` be a stored alias or computed alias of `entryUUID`?
- Should stable IDs be returned under `*`, `+`, or only when explicitly
  requested?
- Should IDs exist on all entries or only users/groups?

Initial product recommendation:

- Store one generated UUID per entry.
- Expose `entryUUID` as the canonical server-managed attribute.
- Expose `uuid` as a compatibility alias for clients such as Pocket ID.
- Protect both from Add/Modify input.

### Object Class Matching

Some clients default to `(objectClass=person)` while LDAPLite stores users as
`inetOrgPerson`.

Implementation questions:

- Should `(objectClass=person)` match `inetOrgPerson` by schema inheritance?
- Should LDAPLite preserve and return multi-valued objectClass hierarchies
  instead of only the primary class?

Initial product recommendation:

- Add compatibility tests before changing objectClass semantics.
- Prefer schema-correct matching for inherited object classes if it can be done
  without destabilizing storage.
- Until then, recipes should use `(objectClass=inetOrgPerson)`.

### Service Bind Authorization

Most clients recommend a read-only bind user for searches.

Current risk:

- LDAPLite currently allows any authenticated non-anonymous LDAP user to write.

Initial product recommendation:

- Add a minimal read-only role/group before encouraging app bind users.
- Keep Web UI admin authorization separate from LDAP write authorization unless
  a service layer refactor intentionally unifies them.

### TLS/LDAPS

Many clients can use plain LDAP for local/private deployments, but their docs
often show `ldaps://`, StartTLS, or TLS verification settings.

Initial product recommendation:

- Keep native TLS out of the first compatibility pass.
- Add a tested LDAPS sidecar/proxy recipe.
- Revisit native LDAPS/StartTLS only if the sidecar path cannot satisfy common
  clients.

### Integration Recipe Shape

Each integration recipe should include:

- LDAPLite server assumptions and required environment variables.
- Bind DN and password guidance.
- User base DN and user filter.
- Group base DN and group filter.
- Attribute mapping.
- Group membership mapping.
- TLS/LDAPS note.
- Smoke-test commands using `ldapsearch` or the client.
- Known unsupported features.

## Source References

- Pocket ID LDAP Integration: https://pocket-id.org/docs/configuration/ldap
- Authelia LDAP configuration: https://www.authelia.com/configuration/first-factor/ldap/
- Dex LDAP connector: https://dexidp.io/docs/connectors/ldap/
- Gitea authentication LDAP docs: https://docs.gitea.com/administration/authentication
- Grafana LDAP authentication: https://grafana.com/docs/grafana/latest/setup-grafana/configure-access/configure-authentication/ldap/
- Nextcloud LDAP user backend: https://docs.nextcloud.com/server/latest/admin_manual/configuration_user/user_auth_ldap.html
- Vaultwarden proxy documentation: https://github.com/dani-garcia/vaultwarden/wiki/Proxy-examples

## Repo Evidence

Relevant local files:

- `README.md`
- `docs/ROADMAP.md`
- `docs/TELEMETRY.md`
- `tests/functional/ad_compat_test.go`
- `internal/server/search.go`
- `internal/server/write.go`
- `internal/server/ldap.go`
- `internal/server/discovery.go`
- `internal/store/sqlite_membership.go`
