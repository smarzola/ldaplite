# Changelog

## Unreleased

### Provisioning

- Added a SCIM-compatible HTTP provisioning API for user and group discovery,
  listing, lookup, creation, replacement, and deletion.
- Added SCIM discovery endpoints, SCIM error responses, pagination, stable
  `entryUUID` resource IDs, and a documented filter subset.
- Reused LDAPLite's shared directory service for SCIM writes so password
  handling, group member validation, and referential integrity match LDAP and
  Web UI behavior.
- Documented endpoint paths, authentication, field mappings, examples, and
  current SCIM limits.

## v0.14.0 - 2026-06-24

### Web UI

- Rebuilt the embedded Web UI as a React/shadcn directory lookup and
  administration console.
- Added role-specific workflows for admin, read-only, and password-only users.
- Added searchable, filterable, paginated directory results with detail sheets,
  copyable DNs/attributes, and contextual row actions.
- Added focused admin flows for creating entries, editing attributes, resetting
  passwords, managing group members, and deleting entries.
- Added server-side Web UI APIs and authorization checks for directory search,
  entry details, write operations, same-origin mutation protection, and
  account-only password self-service.
- Updated Web UI documentation and goal-loop milestone history for the new
  product workflow.

## v0.13.0 - 2026-06-23

### Native LDAP TLS

- Added native implicit LDAPS support on the configured LDAP listener with
  operator-provided PEM certificate and key files.
- Added StartTLS extended operation support and RootDSE advertisement when
  enabled.
- Added functional coverage for native `ldaps://`, StartTLS, and the existing
  TLS sidecar compatibility path.
- Updated client compatibility docs and integration recipes to describe native
  LDAPS/StartTLS as the preferred in-server encryption path.

## v0.12.1 - 2026-06-23

### LDAP Standards Alignment

- Removed the non-standard LDAP compatibility alias; `entryUUID` is now the
  only stable server-managed entry identifier.
- Added a migration to delete persisted alias attributes from databases upgraded
  from v0.12.0.
- Updated client integration recipes and compatibility docs to configure
  `entryUUID` for stable user and group IDs.

## v0.12.0 - 2026-06-23

### LDAP Client Compatibility

- Added stable `entryUUID` values for clients that key synced users and groups by immutable IDs.
- Added LDAP Compare operation support, including compare result codes and functional coverage for client-visible behavior.
- Added Pocket ID compatibility coverage for user and group synchronization expectations.
- Added read-only LDAP service account authorization using `cn=ldaplite.readonly,ou=groups,<baseDN>`.
- Added documented LDAPS sidecar deployment guidance and TLS compatibility coverage.

### Documentation

- Added a client compatibility matrix, client-readiness summary, and integration recipes for Pocket ID, Authelia, Dex, Gitea/Forgejo, Grafana, and Nextcloud.
- Added an LDIF import/export design document for the next provisioning milestone.

## v0.11.0 - 2026-06-22

### Telemetry

- Added audit-grade LDAP and Web UI logging with stable event names, request and connection correlation, and sensitive-value redaction.
- Added OpenTelemetry metrics for LDAP operations, Web UI requests, store calls, authentication outcomes, and audit events.
- Added optional Prometheus metrics exposure.
- Added optional OpenTelemetry tracing with OTLP HTTP export and spans across LDAP, HTTP, and store paths.

### Documentation

- Added telemetry configuration documentation and the completed telemetry goal prompt with milestone history.

## v0.10.0 - 2026-06-22

### Protocol

- Replaced `github.com/lor00x/goldap` request decoding and response encoding with repo-owned LDAP BER handling.
- Added internal LDAP message types for bind, search, add, modify, delete, compare, abandon, unbind, and extended operations.
- Added BER fixtures and malformed-message coverage for representative LDAP request/response paths.
- Removed the `goldap` module dependency from `go.mod` and `go.sum`.

### Documentation

- Added a protocol inventory and goldap replacement goal prompt documenting the migration path and completed checklist.

## v0.9.0 - 2026-06-22

### Performance

- Added store benchmarks for `memberOf` projection, `memberOf` filters, and narrow indexed searches at 1k/10k-entry scale.
- Added indexed exact attribute lookup paths for searches such as `(uid=user-000000)`.
- Added a SQL fast path for direct and nested `memberOf=<groupDN>` filters with cycle protection.
- Made `memberOf` projection optional across search and Web UI read paths so callers that do not need operational attributes avoid the extra work.
- Batched group member resolution and reused entry IDs for password updates to reduce storage-layer work.

### LDAP Interoperability And Correctness

- Projected `memberOf` as a computed, read-only operational attribute derived from `group_members` instead of generic stored attributes.
- Kept server-managed attributes such as `objectClass`, `createTimestamp`, `modifyTimestamp`, and `memberOf` out of generic attribute storage.
- Added case-insensitive DN lookup and uniqueness enforcement.
- Matched group membership DNs case-insensitively.
- Required group entries to contain valid existing members.
- Escaped serialized LDAP filter values and SQL wildcard characters in substring filters.
- Accepted non-canonical BER boolean `TRUE` values for LDAP `typesOnly` searches.
- Returned typed validation and placement errors for better LDAP result-code mapping.

### Internal Design And Testability

- Split the LDAP server handlers into discovery, search, and write modules.
- Split the SQLite store into focused initialization, entry, search, auth, helper, and membership modules.
- Centralized DN helpers, web form helpers, generic attribute writes, web delete handling, and redirect-message encoding.
- Added store context-cancellation coverage, query-plan coverage, escaped-DN functional coverage, and fail-fast functional server startup checks.
- Refreshed the codebase simplification audit documentation and memberOf performance goal prompt.

### Documentation

- Reframed the README around self-hosted identity and performance-focused small-directory operation.
- Removed the stale LLM-assisted-coding experiment positioning.
- Added README benchmark results and instructions for running the benchmark matrix locally.

### Dependencies

- Updated the vulnerable `github.com/Azure/go-ntlmssp` dependency.

## v0.8.2 - 2026-06-21

This release fixes Windows compatibility issues found by the expanded cross-platform CI matrix.

### Compatibility

- Fixed SQLite migrations with Windows drive-letter database paths.
- Ran Go CI commands under Bash on Windows to avoid PowerShell argument parsing issues.
- Added Git attributes to keep Go source files LF-normalized on Windows checkouts.

## v0.8.1 - 2026-06-21

This release expands native platform compatibility for CI and release artifacts.

### Compatibility

- Added Linux, macOS, and Windows CI coverage for unit, race, and functional compatibility tests.
- Added release binaries for macOS and Windows on `amd64` and `arm64`.
- Fixed the functional test server launcher to use a `.exe` binary on Windows.
- Documented the full native binary archive matrix in the README.

## v0.8.0 - 2026-06-21

This release adds AD-like compatibility verification and CI coverage for the functional test suite.

### Compatibility

- Added a black-box AD-like functional compatibility suite using `github.com/go-ldap/ldap/v3`.
- Covered simple bind, subtree search, AD-facing attributes, group membership searches, password modification, deletion, hidden password attributes, operational timestamps, and LDAP result codes.
- Returned LDAP object class violation for entry validation failures instead of a generic operations error.

### Operations And Release Hygiene

- Added a GitHub Actions CI pipeline that runs unit tests and AD-like functional compatibility tests.
- Added `make test-functional`.
- Added the MIT license.
- Documented compatibility scope and testing commands in the README.

## v0.7.0 - 2026-06-21

This release is a security and interoperability hardening release.

### Breaking Changes

- LDAP clients must bind before normal directory searches and all write operations. RootDSE and schema discovery remain intentionally readable before bind.
- Anonymous bind, when enabled, allows search only; Add, Modify, and Delete require an authenticated user DN.
- The embedded Web UI is disabled by default. Enable it with `LDAP_WEB_UI_ENABLED=true`.
- Web UI delete actions are POST-only, and mutating Web UI requests require same-origin `Origin` or `Referer` validation.
- The Docker healthcheck now fails if the database/schema/base DN are invalid or if the LDAP listener is unreachable.

### Security

- Enforced bind state for LDAP Search, Add, Modify, and Delete.
- Cleared prior bind state on failed bind attempts and unbind.
- Added same-origin protection for Basic Auth Web UI mutation routes.
- Prevented group member references to nonexistent entries.
- Rejected entry creation outside the configured base DN or below missing parent DNs.

### LDAP Interoperability

- Added canonical LDAP response casing for known attributes such as `objectClass`, `memberOf`, `createTimestamp`, `modifyTimestamp`, `givenName`, `displayName`, and `telephoneNumber`.
- Honored requested search attributes, including `1.1`, `*`, and `+`.
- Ensured search responses emit a single canonical `objectClass` attribute.
- Added transitive nested group membership and transitive `memberOf` with cycle protection.

### Operations And Release Hygiene

- Updated `picomatch` to `2.3.2`.
- Aligned Go version claims across CI, release builds, Docker, README, and QUICKSTART.
- Added `docs/ROADMAP.md` and split the broad future-development issue into focused roadmap issues.
- Marked stale planning docs as historical.
