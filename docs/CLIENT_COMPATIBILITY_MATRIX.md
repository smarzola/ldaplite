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
- Stable generated `entryUUID` and `uuid` attributes are available on entries;
  `entryUUID` is operational and `uuid` is a normal compatibility alias.
- Functional tests exercise AD-like bind/search/member/memberOf/password/result
  code behavior through `github.com/go-ldap/ldap/v3`.
- Pocket ID has a tested functional compatibility path and integration recipe.
- Integration recipes exist for Authelia, Dex, Gitea/Forgejo, Grafana, and
  Nextcloud.
- LDAP Compare returns meaningful compareTrue, compareFalse, and noSuchObject
  results for safe attributes.
- Audit-grade structured logs, optional OpenTelemetry tracing, and
  Prometheus-compatible metrics are implemented and documented.

Confirmed current gaps:

- LDAP write authorization is coarse: any authenticated non-anonymous user can
  write.
- No native TLS/LDAPS/StartTLS; deployment guidance currently relies on an
  external TLS terminator.
- No LDIF/CSV/bootstrap import/export path yet.

## Consumer Matrix

| Consumer | Expected LDAP Pattern | LDAPLite Status | Next LDAPLite Work |
| --- | --- | --- | --- |
| Pocket ID | Periodic LDAP sync with bind DN, search base, user filter such as `(objectClass=person)`, group filter `(objectClass=groupOfNames)`, stable user/group unique attributes, `uid`, `mail`, `givenName`, `sn`, group `member`, and group name mapping. Examples use `ldaps://` and `uuid`. | **Works with LDAPLite-specific settings.** LDAPLite has functional coverage for Pocket ID-shaped reads using `inetOrgPerson`, `groupOfNames`, `member`, `memberOf`, `uid`, `mail`, `givenName`, `sn`, and generated `uuid`. See [Pocket ID recipe](integrations/pocket-id.md). | Remaining gaps are operational: use `(objectClass=inetOrgPerson)` instead of Pocket ID's example `(objectClass=person)`, use `cn` for group names, use admin bind until read-only service accounts exist, and place LDAPLite behind private networking or TLS sidecar/proxy until native LDAPS/StartTLS exists. |
| Authelia | Service bind user searches users and groups. Configurable `base_dn`, `additional_users_dn`, `users_filter`, `additional_groups_dn`, `groups_filter`, group search mode, `memberOf`, `cn`, `uid`, `mail`, `givenName`, `sn`, and TLS/StartTLS settings. | **Likely works with documented settings.** LDAPLite supports bind/search, `memberOf`, group `member`, and common user attributes. See [Authelia recipe](integrations/authelia.md). | Remaining gaps: read-only service-account authorization and native StartTLS/LDAPS. Keep AD recursive matching-rule examples out of LDAPLite recipes. |
| Dex LDAP connector | Service account bind, user search that combines a filter with username attribute, then bind as found user. Group search supports `userMatchers` such as user `DN` to group `member`; supports recursive group lookup using `recursionGroupAttr` for nested group schemas. | **Likely works for direct groups with documented settings.** LDAPLite supports service bind via any user, user bind verification, user search by `uid`/`mail`/`userPrincipalName`, group `member` as DN, generated `uuid`, and nested `memberOf`. See [Dex recipe](integrations/dex.md). | Add client-shaped functional test if Dex becomes a release gate. Remaining operational gaps: read-only bind and TLS sidecar/native LDAPS path. |
| Gitea / Forgejo | LDAP via BindDN or simple auth. Needs host/port/TLS, optional bind DN, user search base/filter, username attribute, first name `givenName`, surname `sn`, required email `mail`, optional admin filter, and optional group membership verification using group base, group member attribute, and user attribute such as DN or `uid`. | **Likely works with documented settings.** LDAPLite supports simple bind, BindDN search, `uid`, `givenName`, `sn`, `mail`, `member`, and `memberOf` filters. See [Gitea/Forgejo recipe](integrations/gitea-forgejo.md). | Remaining gaps: read-only bind and TLS sidecar/native LDAPS path. Kerberos, SASL, and SPNEGO/SSPI are out of scope. |
| Grafana | Bind DN is normally a read-only user. User lookup uses `search_filter` such as `(uid=%s)` and search base DNs. Attributes include `memberOf`, `mail`/`email`, display/name attributes. Group role mapping can use `memberOf`; POSIX fallback can search groups by `memberUid`. TLS/LDAPS and StartTLS are first-class config choices. | **Likely works for `memberOf` mapping with documented settings.** LDAPLite supports bind, user search by `uid`/`cn`/`mail`, `memberOf`, group DNs, and common attributes. See [Grafana recipe](integrations/grafana.md). | Remaining gaps: read-only service account and TLS sidecar/native LDAPS path. Do not use AD recursive matching-rule examples with LDAPLite. |
| Nextcloud | LDAP app uses read-only directory access. Needs host/port or `ldaps://`, user DN/bind user, base DN, user filters such as `inetOrgPerson` plus optional `memberOf`, login attributes such as `uid` and `mail`, group filters, group display name `cn`, group member association, and stable LDAP UUID/DN mapping. | **Likely works with documented settings.** LDAPLite supports read/search patterns, `inetOrgPerson`, `memberOf`, `uid`, `mail`, group `cn`, group `member`, and stable generated UUID attributes. See [Nextcloud recipe](integrations/nextcloud.md). | Remaining gaps: read-only service account and TLS sidecar/native LDAPS path. Use raw filters when Nextcloud auto-detection picks schema assumptions from other directory servers. |
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

Implemented direction:

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
