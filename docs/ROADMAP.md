# LDAPLite Roadmap

Last updated: 2026-06-23

This roadmap tracks current project direction after the security and interoperability review. Historical plans in this directory are useful design references, but this file is the current status source.

## Completed

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

## Near-Term Hardening

- Add compatibility tests against casing-sensitive LDAP clients (#11).
- Revisit original presentation casing for custom attributes if compatibility tests show real client impact; current behavior stores and emits custom attributes using normalized names.
- Add richer LDAP result mapping for store constraint errors.
- Expand healthcheck modes if deployments need separate database, listener, and full LDAP bind/search readiness checks.

## Product Roadmap

- SCIM 2.0 or REST provisioning API on top of the shared directory service (#7).
- User and group templates in the Web UI as structured presets over the attribute system (#8).
- Implement LDIF import/export commands from [docs/IMPORT_EXPORT_DESIGN.md](IMPORT_EXPORT_DESIGN.md).

## GitHub Issue Cleanup

Issue `#2 Future development` was closed on 2026-06-21 after the completed Web UI portion was separated from remaining roadmap work. Follow-up issues:

- #7 SCIM 2.0 / REST provisioning API.
- #8 User and group templates.
- #11 LDAP client compatibility test matrix.
