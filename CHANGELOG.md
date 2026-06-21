# Changelog

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
