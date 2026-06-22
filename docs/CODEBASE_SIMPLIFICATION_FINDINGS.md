# LDAPLite Codebase Simplification Tracker

Date: 2026-06-22

Issue: https://github.com/smarzola/ldaplite/issues/13

## Updated Goal

This document refreshes the original adversarial codebase audit into a current
closure tracker. The immediate goal is reached when the audit states, without
stale line-number evidence, what has been implemented, what still matters, and
what acceptance criteria should close or split issue #13.

The remaining implementation goal is narrower than the original audit:

> Finish the residual cleanup by consolidating remaining Web UI handler
> repetition where it reduces code without hiding behavior.

## Current Status

The broad P0/P1 audit is mostly implemented. Treat this file as the current
tracker and the old findings as historical context only. Source inspection on
2026-06-22 confirms:

- LDAP operation handling is split across `internal/server/ldap.go`,
  `internal/server/search.go`, and `internal/server/write.go`.
- SQLite storage is split across focused files in `internal/store/`.
- `AddOperationalAttributes`, `GetAllEntries`, and `GetChildren` are no longer
  present.
- Search calls use `SearchOptions` / `SearchEntriesWithOptions`.
- Typed store errors and `entryWriteResultCode` handle known LDAP result
  classes without substring matching.
- The Who Am I extended-response workaround is isolated in
  `internal/protocol/extended_response.go`.
- Store benchmarks and query-plan tests exist in
  `internal/store/sqlite_search_test.go`.
- The black-box LDAP compatibility suite lives in `tests/functional/`.
- Release and CI workflows now run the functional suite.

The audit has therefore moved from "large design corrections" to "close the
remaining sharp edges and prove they stay closed."

## Completed Work

The following themes from the original audit have been implemented or reduced
to residual follow-up work:

- Store/server decomposition: the old large files are now split by
  responsibility.
- Typed error mapping: store errors map to LDAP result codes through typed
  error handling.
- DN behavior: DN parsing and placement checks have dedicated coverage,
  including escaped DN cases.
- Search options: scope and LDAP search options are represented explicitly.
- `memberOf` cost control: `memberOf` projection can be skipped when unused.
- Case-insensitive query support: expression indexes and query-plan tests now
  protect common search paths.
- Security-sensitive attributes: passwords stay out of generic attributes, and
  server-managed attributes are protected.
- Web UI replacement semantics: editable extra attributes use replace-style
  behavior instead of stale merge-only updates.
- Functional compatibility: the suite starts a real server subprocess and uses
  a real LDAP client library.
- LDAP filter serialization: filter values are escaped before serialization.
- Obsolete store reads: broad helper reads that encouraged over-fetching have
  been removed.
- Server lifecycle: startup uses signal-aware context handling.
- Virtual attribute boundary: `memberOf` is projected through explicit computed
  attributes instead of being injected into persisted generic attributes.
- LDAP response projection: attribute selection, `1.1`, `typesOnly`, selected
  attributes, and operational attributes are covered at the functional LDAP
  boundary without adding a store-level projection API; BER boolean decoding
  now accepts non-canonical true values emitted by real clients.
- Cancellation coverage: store read, search, and write paths are covered for
  already-canceled contexts without sleep-based timing.
- Dependency health: the moderate `github.com/Azure/go-ntlmssp` alert is
  addressed by selecting the patched `v0.1.1` module version.

## Remaining Opportunities

### 1. Consolidate remaining Web UI handler repetition

Current state:

- Web UI handlers already have shared helpers for some form behavior.
- Users, groups, and OUs still repeat enough list/form/load/update flow that
  future validation changes can drift.

Recommendation:

- Extract only the repeated mechanics: OU loading, editable extra-attribute
  replacement, form error rendering, and common redirect/error handling.
- Leave resource-specific validation and model construction visible.

Acceptance criteria:

- Handler code shrinks without hiding security-sensitive or LDAP-specific
  behavior behind generic reflection-style helpers.
- Tests cover create, edit, delete, validation error, and extra-attribute
  removal paths for each resource type touched.

## Out Of Scope

The original audit should not be used to smuggle in broader LDAP product scope.
These remain intentional limits unless the roadmap changes:

- TLS/LDAPS termination inside the server.
- SASL, Kerberos, or GSSAPI.
- Full Active Directory schema semantics.
- Global Catalog, DirSync, paging controls, server-side sorting controls, and
  AD recursive matching rule support.
- Replication or high availability.

## Verification To Keep Using

For implementation follow-ups, keep the verification bar high:

```bash
GOCACHE=/private/tmp/ldaplite-gocache make test
GOCACHE=/private/tmp/ldaplite-gocache go test -count=1 -tags=functional -v ./tests/functional/...
```

For search/storage changes, also consider:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test -run '^$' -bench=. ./internal/store ./internal/schema
```

## Closure Criteria For Issue #13

Issue #13 can be closed, or split into smaller follow-up issues, when:

- the remaining opportunities above are implemented or intentionally deferred;
- the full Go test suite passes;
- the functional LDAP suite passes;
- no stale audit evidence or obsolete line-number claims remain in this file;
- dependency health has been checked; and
- all resulting commits are pushed.
