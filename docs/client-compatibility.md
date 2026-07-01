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
- Stable generated `entryUUID` attributes are available on entries as
  operational server-managed identifiers.
- Functional tests exercise AD-like bind/search/member/memberOf/password/result
  code behavior through `github.com/go-ldap/ldap/v3`.
- Pocket ID has a tested functional compatibility path and integration recipe.
- Integration recipes exist for Authelia, Dex, Gitea/Forgejo, Grafana, and
  Nextcloud.
- LDAP Compare returns meaningful compareTrue, compareFalse, and noSuchObject
  results for safe attributes.
- Authenticated non-admin users can bind/search/compare and change their own
  password, while arbitrary Add/Modify/Delete return insufficientAccessRights.
- Members of `cn=ldaplite.readonly,ou=groups,<baseDN>` remain the recommended
  explicit app bind users for read-only integrations.
- LDAPS and StartTLS are covered by native TLS support and the tested sidecar deployment path remains available.
- Audit-grade structured logs, optional OpenTelemetry tracing, and
  Prometheus-compatible metrics are implemented and documented.

Confirmed current gaps:

- LDIF import/export command design is documented; implementation is pending.

## Consumer Matrix

| Consumer | Expected LDAP Pattern | LDAPLite Status | Next LDAPLite Work |
| --- | --- | --- | --- |
| Pocket ID | Periodic LDAP sync with bind DN, search base, user filter such as `(objectClass=person)`, group filter `(objectClass=groupOfNames)`, stable user/group unique attributes, `uid`, `mail`, `givenName`, `sn`, group `member`, and group name mapping. Examples use `ldaps://` and `entryUUID`. | **Works with LDAPLite-specific settings.** LDAPLite has functional coverage for Pocket ID-shaped reads using `inetOrgPerson`, `groupOfNames`, `member`, `memberOf`, `uid`, `mail`, `givenName`, `sn`, generated `entryUUID`, and native LDAPS/StartTLS. See [Pocket ID recipe](integrations/pocket-id.md). | Remaining schema gap: use `(objectClass=inetOrgPerson)` instead of Pocket ID's example `(objectClass=person)` and use `cn` for group names. |
| Authelia | Service bind user searches users and groups. Configurable `base_dn`, `additional_users_dn`, `users_filter`, `additional_groups_dn`, `groups_filter`, group search mode, `memberOf`, `cn`, `uid`, `mail`, `givenName`, `sn`, and TLS/StartTLS settings. | **Likely works with documented settings.** LDAPLite supports bind/search, `memberOf`, group `member`, common user attributes, native LDAPS, and StartTLS. See [Authelia recipe](integrations/authelia.md). | Keep AD recursive matching-rule examples out of LDAPLite recipes. |
| Dex LDAP connector | Service account bind, user search that combines a filter with username attribute, then bind as found user. Group search supports `userMatchers` such as user `DN` to group `member`; supports recursive group lookup using `recursionGroupAttr` for nested group schemas. | **Likely works for direct groups with documented settings.** LDAPLite supports service bind via read-only app users, user bind verification, user search by `uid`/`mail`/`userPrincipalName`, group `member` as DN, generated `entryUUID`, nested `memberOf`, and native LDAPS/StartTLS. See [Dex recipe](integrations/dex.md). | Add client-shaped functional test if Dex becomes a release gate. |
| Gitea / Forgejo | LDAP via BindDN or simple auth. Needs host/port/TLS, optional bind DN, user search base/filter, username attribute, first name `givenName`, surname `sn`, required email `mail`, optional admin filter, and optional group membership verification using group base, group member attribute, and user attribute such as DN or `uid`. | **Likely works with documented settings.** LDAPLite supports simple bind, BindDN search, read-only app bind users, `uid`, `givenName`, `sn`, `mail`, `member`, `memberOf` filters, native LDAPS, and StartTLS. See [Gitea/Forgejo recipe](integrations/gitea-forgejo.md). | Kerberos, SASL, and SPNEGO/SSPI are out of scope. |
| Grafana | Bind DN is normally a read-only user. User lookup uses `search_filter` such as `(uid=%s)` and search base DNs. Attributes include `memberOf`, `mail`/`email`, display/name attributes. Group role mapping can use `memberOf`; POSIX fallback can search groups by `memberUid`. TLS/LDAPS and StartTLS are first-class config choices. | **Likely works for `memberOf` mapping with documented settings.** LDAPLite supports bind, read-only app bind users, user search by `uid`/`cn`/`mail`, `memberOf`, group DNs, common attributes, native LDAPS, and StartTLS. See [Grafana recipe](integrations/grafana.md). | Do not use AD recursive matching-rule examples with LDAPLite. |
| Nextcloud | LDAP app uses read-only directory access. Needs host/port or `ldaps://`, user DN/bind user, base DN, user filters such as `inetOrgPerson` plus optional `memberOf`, login attributes such as `uid` and `mail`, group filters, group display name `cn`, group member association, and stable LDAP ID/DN mapping. | **Likely works with documented settings.** LDAPLite supports read-only app bind users, read/search patterns, `inetOrgPerson`, `memberOf`, `uid`, `mail`, group `cn`, group `member`, stable generated `entryUUID` attributes, and native LDAPS/StartTLS. See [Nextcloud recipe](integrations/nextcloud.md). | Use raw filters when Nextcloud auto-detection picks schema assumptions from other directory servers. |
| Vaultwarden | Current Vaultwarden project documentation is centered on HTTP reverse-proxy deployment. Native LDAP authentication is not a standard first-party LDAP consumer path in the checked docs. | **Not a direct LDAP target.** LDAPLite may still serve the upstream IdP or auth proxy in front of Vaultwarden, but Vaultwarden itself should not drive LDAP protocol milestones unless a maintained LDAP integration is selected. | Treat as an indirect recipe later: LDAPLite -> Authelia/Pocket ID/other OIDC or forward-auth component -> Vaultwarden. Do not block core LDAP milestones on Vaultwarden-native LDAP. |

## Cross-Client Requirements

These requirements recur across multiple clients and should drive the milestone
order.

### Stable Identity Attributes

Clients that synchronize users and groups need durable unique identifiers.

Implementation questions:

- Should stable IDs be returned under `*`, `+`, or only when explicitly
  requested?
- Should IDs exist on all entries or only users/groups?

Implemented direction:

- Store one generated UUID per entry.
- Expose `entryUUID` as the canonical server-managed attribute.
- Protect it from Add/Modify input.

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

Implemented direction:

- LDAPLite is least-privilege by default: authenticated non-admin users can
  bind, search, compare, and change their own password, but arbitrary Add,
  Modify, and Delete return insufficientAccessRights.
- App bind users should still be added to
  `cn=ldaplite.readonly,ou=groups,<baseDN>` to make their read-only purpose
  explicit.
- Directory write administration requires
  `cn=ldaplite.admin,ou=groups,<baseDN>`.

### TLS/LDAPS

Many clients can use plain LDAP for local/private deployments, but their docs
often show `ldaps://`, StartTLS, or TLS verification settings.

Implemented direction:

- Keep native TLS certificate rotation and mutual TLS out of the first compatibility pass.
- Use the tested [LDAPS TLS sidecar guide](deployment/ldaps-tls-sidecar.md).
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
- `docs/roadmap.md`
- `docs/telemetry.md`
- `tests/functional/ad_compat_test.go`
- `internal/server/search.go`
- `internal/server/write.go`
- `internal/server/ldap.go`
- `internal/server/discovery.go`
- `internal/store/sqlite_membership.go`
