# LDAPLite Roadmap

Last updated: 2026-07-02

This roadmap tracks current project direction after the security,
interoperability, SCIM, and LDIF import/export work through `v0.16.0`.
Historical prompts, audits, and design records live under `docs/internal/`; this
file is the public status source.

## Current Baseline

- Embedded Web UI for users, groups, and organizational units.
- Embedded SQLite migrations.
- SQL-compiled LDAP filters with in-memory fallback for unsupported cases.
- Operational attributes: `objectClass`, `createTimestamp`, `modifyTimestamp`, and computed `memberOf`.
- Stable generated entry identifiers: operational `entryUUID`.
- Pocket ID LDAP sync compatibility recipe and functional coverage.
- LDAP integration recipes for Authelia, Dex, Gitea/Forgejo, Grafana, and Nextcloud.
- LDAP Compare returns meaningful true, false, and no-such-object results for safe attributes.
- Read-only LDAP service accounts through `cn=ldaplite.readonly,ou=groups,<baseDN>`.
- Tested LDAPS deployment path using a TLS-terminating TCP sidecar.
- Native LDAPS and StartTLS with operator-provided certificate/key files.
- Canonical LDAP attribute casing for known response attributes.
- Bind enforcement for normal searches and write operations.
- Web UI same-origin protection for mutating routes.
- Role-aware React/shadcn Web UI with admin, read-only, and account-only
  password surfaces.
- Shared directory service for Web UI user, group, OU, membership, attribute,
  and password mutation flows.
- Web UI password reset and self-service password change flows.
- Referential integrity for parent DNs and group members.
- Recursive nested group membership with cycle/depth protection.
- LDAP search response attribute selection, including `1.1`, `*`, and `+`.
- Meaningful database and LDAP listener healthcheck used by the Docker image.
- Audit-grade LDAP/Web UI logging, optional OpenTelemetry metrics/tracing, and Prometheus-compatible scraping (#9).
- SCIM-compatible user and group provisioning API on the embedded HTTP server (#7).
- LDIF import/export commands for bootstrap, safe inspection, generated
  passwords, and replace-existing workflows.
- Public operator documentation split from internal prompts and design history.

## 1.0 Readiness

LDAPLite is close to a practical 1.0 baseline for small-to-medium self-hosted
directory deployments. Before cutting `v1.0.0`, keep the public docs, release
notes, and examples aligned with the shipped behavior.

Recommended final checks:

- Verify README, quick start, operator docs, and integration recipes against the
  current command set and release artifacts.
- Confirm no public docs describe shipped features as pending.
- Run the normal local validation suite and release workflow after the version
  bump.

## Near-Term Hardening

- Add richer LDAP result mapping for store constraint errors where a known
  storage or validation class can map to a more precise LDAP result code.
- Expand healthcheck modes if deployments need separate database, listener, and
  full LDAP bind/search readiness checks.
- Revisit original presentation casing for custom attributes only if
  compatibility tests show real client impact; current behavior stores and
  emits custom attributes using normalized names.

## Product Candidates

These are candidates for future issues, not active commitments:

- Backup and restore commands for consistent SQLite backup, validation, and
  recovery workflows.
- User and group templates in the Web UI as structured presets over the
  attribute system.
- Additional client-shaped functional gates for Authelia, Dex, Gitea/Forgejo,
  Grafana, or Nextcloud if one becomes release-critical.
- More granular authorization roles if deployments need permissions beyond
  admin, read-only app bind users, and password-only users.

## Issue Tracking

There are no open GitHub roadmap issues as of 2026-07-02. Historical follow-up
issues `#8` and `#11` are closed and should not be treated as active roadmap
commitments.
