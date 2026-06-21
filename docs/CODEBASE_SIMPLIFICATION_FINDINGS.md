# LDAPLite Codebase Simplification Findings

Date: 2026-06-21

Issue: https://github.com/smarzola/ldaplite/issues/13

## Summary

The codebase is in a healthy state for a small LDAP server: the normal Go test
suite passes, password storage invariants are well represented, and several old
roadmap items have already been addressed. The main opportunities are now less
about adding features and more about removing accidental coupling:

- split storage reads from LDAP response decoration;
- stop mutating model entries to add virtual/operational attributes;
- push LDAP scope and attribute selection into the store/query layer;
- remove duplicated CRUD/update logic across the Web UI handlers;
- replace stringly error classification with typed store/server errors;
- isolate the unsafe `goldap` workaround behind one protocol helper;
- add performance tests that prove the filter compiler and indexes actually
  cooperate.

## Verification Performed

- `go test ./...` passed when run outside the sandbox. The sandboxed run failed
  only because healthcheck tests could not bind `127.0.0.1:0`.
- `go vet ./...` passed.
- `go test -run '^$' -bench=. ./internal/store ./internal/schema` passed, but
  there are no benchmarks in those packages today.
- Structural and text searches were run over `cmd`, `internal`, `pkg`, `tests`,
  and `docs`.

## P0/P1 Opportunities

### 1. Separate stored attributes from virtual and operational attributes

Evidence:

- `internal/store/sqlite.go:237-242`, `673-674`, `772-779`, and `837-845`
  call `AddOperationalAttributes()` and `populateMemberOf()` directly inside
  generic store read methods.
- `internal/models/entry.go:185-193` mutates `Entry.Attributes` to add
  `objectclass`, `createtimestamp`, and `modifytimestamp`.
- `internal/store/sqlite.go:993-1003` mutates entries again by appending
  `memberOf`.
- `internal/server/ldap.go:731-752` has to special-case `objectClass` to avoid
  double emission.

Why this matters:

The store currently returns a hybrid object: part persisted state, part computed
LDAP response state. That makes writes riskier because `GetEntry()` returns an
entry containing attributes that should not be stored back. The current
`UpdateEntry()` protects `userPassword`, but it still rewrites whatever else is
present in `entry.Attributes`. This is easy to regress as new virtual attributes
are added.

Recommended simplification:

- Keep `models.Entry.Attributes` as persisted user attributes only.
- Add a response/projection layer, for example `server.EntryView` or
  `protocol.EntryAttributes(entry, options)`, that computes `objectClass`,
  timestamps, and `memberOf` only when an LDAP/Web caller needs them.
- Make computed attributes read-only by construction rather than by scattered
  filtering and protection lists.

Expected payoff:

- Less mutation while reading.
- Smaller write surface.
- Cleaner tests: storage tests assert persisted shape, server/protocol tests
  assert LDAP presentation shape.

### 2. Push LDAP search scope and attribute selection earlier

Evidence:

- `internal/server/ldap.go:260` calls `SearchEntries(ctx, baseDN, filterStr)`.
- `internal/server/ldap.go:266-278` applies base/one/subtree scope after the
  store has already loaded a subtree.
- `internal/server/ldap.go:283-284` applies attribute selection after every
  returned entry has had full attributes decoded and `memberOf` populated.
- `internal/store/sqlite.go:611-636` always uses a recursive subtree CTE.
- `internal/store/sqlite.go:711-717` populates `memberOf` for all SQL-matched
  entries even when the client requested `1.1`, only user attributes, or a
  non-user object class.

Why this matters:

For base and one-level searches, the store does unnecessary recursive traversal.
For attribute-light searches, it still loads all attributes and computes
membership. This is the main performance opportunity in the LDAP hot path.

Recommended simplification:

- Change the store search API to accept a query struct:

  ```go
  type SearchOptions struct {
      BaseDN string
      Scope SearchScope
      Filter *schema.Filter
      RequestedAttributes AttributeSelection
      IncludeOperational bool
  }
  ```

- Select query shape by scope: exact DN, direct children, recursive subtree.
- Only compute `memberOf` when the filter references it or the response
  selection includes it.
- Consider a store-level `SearchEntryIDs` or `SearchEntriesProjection` path for
  `typesOnly`, `1.1`, and existence-style searches.

Expected payoff:

- Less DB work for common client probes.
- Better testability for search behavior.
- Clearer division between LDAP protocol concerns and persistence concerns.

### 3. Make the filter compiler index-friendly

Evidence:

- `internal/schema/filter_compiler.go:127` uses `LOWER(e.object_class)`.
- `internal/schema/filter_compiler.go:135-136`, `154`, and `180-181` use
  `LOWER(a.name)` and `LOWER(a.value)`.
- `internal/store/migrations/002_add_indexes.up.sql` defines normal indexes on
  `attributes(name)`, `attributes(name, value)`, and `entries(object_class)`,
  not expression indexes on `LOWER(...)`.
- `internal/store/sqlite.go:320-325` comments reference partial indexes named
  `idx_attributes_uid_lookup`, `idx_attributes_cn_lookup`, and
  `idx_attributes_ou_lookup`, but the migrations in this repo do not create
  those indexes.

Why this matters:

The code says it is optimized, but the SQL likely prevents the current indexes
from doing the intended work for case-insensitive LDAP matching. That can turn
attribute equality and substring filters into repeated scans as data grows.

Recommended simplification:

- Normalize attribute names at write time and query `a.name = ?`.
- Decide whether values should be normalized in auxiliary columns, queried with
  `COLLATE NOCASE`, or backed by expression indexes such as
  `CREATE INDEX ... ON attributes(lower(name), lower(value))`.
- Add `EXPLAIN QUERY PLAN` tests for representative filters:
  `(uid=jane)`, `(objectClass=inetOrgPerson)`,
  `(&(objectClass=inetOrgPerson)(uid=jane))`, and `(cn=Jane*)`.
- Delete or update stale optimization comments so future work does not chase
  indexes that are not real.

Expected payoff:

- Performance claims become measurable.
- Fewer surprises under larger directories.
- Cleaner compiler code once casing strategy is centralized.

### 4. Replace stringly error-to-LDAP-result mapping

Evidence:

- `internal/server/ldap.go:763-775` maps write errors by substring matching
  lowercased error text.
- `internal/store/sqlite.go:367-368`, `417-418`, `486-487`, and
  `520-521` return plain formatted errors for known LDAP result classes.
- `internal/server/ldap.go:380-382` and `531-533` depend on that string mapping.

Why this matters:

The LDAP result code is part of the external compatibility contract. Today a
wording change in a store error can silently change protocol behavior from
`objectClassViolation`, `noSuchObject`, or `constraintViolation` into a generic
operations error.

Recommended simplification:

- Add sentinel or typed errors in `internal/store`, for example:
  `ErrNoSuchObject`, `ErrParentMissing`, `ErrEntryAlreadyExists`,
  `ErrConstraintViolation`, `ErrObjectClassViolation`.
- Have the server map errors with `errors.Is` / `errors.As`.
- Make tests assert result-code mapping directly for add/modify/delete.

Expected payoff:

- More reliable client compatibility.
- Less brittle logging/error copy.
- Clearer ownership of LDAP semantics.

### 5. Isolate or remove the unsafe extended-response workaround

Evidence:

- `internal/server/ldap.go:8-12` imports `reflect` and `unsafe`.
- `internal/server/ldap.go:618-627` writes an unexported `goldap` field to set
  the Who Am I response value.

Why this matters:

This may be necessary because of the dependency API, but it is the most brittle
piece of code in the server. It is also embedded in the already-large LDAP
handler file, so it is easy to miss during dependency upgrades.

Recommended simplification:

- Move this into `internal/protocol`, for example
  `protocol.NewWhoAmIResponse(authzID string)`.
- Put the unsafe reflection in a tiny file with focused tests.
- Re-check whether the current `github.com/lor00x/goldap` version or a small
  fork/PR can expose a public setter. If so, delete the unsafe path.

Expected payoff:

- One contained compatibility hack instead of a server-level pattern.
- Easier dependency upgrades.

## P2 Opportunities

### 6. Split the 1,007-line SQLite store by responsibility

Evidence:

- `internal/store/sqlite.go` is 1,007 lines and contains initialization,
  migrations, bootstrap data, CRUD, search, password lookup, group membership,
  and helper functions.

Recommended split:

- `sqlite_init.go`: `Initialize`, migrations, bootstrap.
- `sqlite_entries.go`: entry CRUD and placement validation.
- `sqlite_search.go`: search query building and row decoding.
- `sqlite_groups.go`: membership sync, `memberOf`, `IsUserInGroup`.
- `sqlite_passwords.go`: password hash lookup.
- `sqlite_rows.go`: shared row scanning/decode helpers.

This is mostly code motion, but it will make future changes much less
adversarial to review.

### 7. Deduplicate read/decode logic in store queries

Evidence:

- `GetEntry`, `SearchEntries`, `GetAllEntries`, and `GetChildren` each embed a
  JSON aggregation query and row decoding loop in
  `internal/store/sqlite.go:190-245`, `611-676`, `723-783`, and `787-848`.

Recommended simplification:

- Extract a common selected column list and scanner:
  `scanEntryWithAttributes(rows)` / `queryEntries(ctx, query, args...)`.
- Avoid JSON aggregation if simpler row grouping is easier to reason about and
  faster under `modernc.org/sqlite`; benchmark both before switching.

Expected payoff:

- Smaller store methods.
- One place to fix attribute decoding or casing behavior.

### 8. Deduplicate group membership sync

Evidence:

- Group member insert/verify code is duplicated in
  `internal/store/sqlite.go:352-370` and `470-489`.

Recommended simplification:

- Extract `syncGroupMembers(ctx, tx, groupEntryID, groupDN, members, replace bool)`.
- Validate all referenced member DNs before mutating `group_members`, then apply
  changes. This gives cleaner errors and avoids partial work before rollback.

### 9. Web UI CRUD handlers repeat the same flow and have subtle update issues

Evidence:

- `internal/web/handlers/users.go`, `groups.go`, and `ous.go` repeat list,
  form-load, edit, delete, OU-loading, extra-attribute formatting, and
  `showError` flows.
- User update parses extra attributes and assigns them into `entry.Attributes`
  (`internal/web/handlers/users.go:217-220`) but does not remove old extra
  attributes absent from the submitted form.
- Group and OU update have the same merge-only behavior in
  `internal/web/handlers/groups.go:195-198` and
  `internal/web/handlers/ous.go:162-165`.
- Group update only changes members when at least one member is submitted
  (`internal/web/handlers/groups.go:190-193`), so a user cannot clear members
  through the form even if the model/store were to allow it.

Recommended simplification:

- Extract shared helpers for loading OUs, formatting extra attributes, rendering
  forms with errors, and replacing editable extra attributes.
- Define per-resource editable/core attribute sets once, not as repeated
  string slices.
- Use a replace model for form-submitted attributes: remove prior editable
  extras, then apply submitted extras.
- Add handler tests for deleting an extra attribute, clearing optional fields,
  and clearing/replacing group members.

Expected payoff:

- Less duplicated UI code.
- Better alignment between form state and stored LDAP attributes.

### 10. LDAP operation handlers need smaller units and request-scoped context

Evidence:

- `internal/server/ldap.go` is 883 lines.
- LDAP operations use `context.Background()` at
  `internal/server/ldap.go:167`, `228`, `299`, `391`, and `425`.
- Add and modify both contain password processing and attribute mutation logic
  in-line (`internal/server/ldap.go:354-361`, `466-520`).
- `handleSearch` loads subtree results and then filters scope/attributes in the
  server (`internal/server/ldap.go:260-284`).

Recommended simplification:

- Thread the connection/server context into operation handlers so shutdown and
  disconnect cancellation reach store calls.
- Extract add/modify request parsing into pure functions returning a command
  object. Unit-test those without a network connection.
- Split RootDSE/schema/extended operations into separate files.

Expected payoff:

- Easier tests for protocol behavior.
- More predictable shutdown under slow DB operations.

### 11. Config loading should return errors instead of exiting

Evidence:

- `pkg/config/config.go:101-105` logs and calls `os.Exit(1)` when base DN is
  empty.

Why this matters:

Direct process exit makes config code harder to test and reuse. It is also
inconsistent with the rest of the codebase, which generally returns errors.

Recommended simplification:

- Add `LoadFromEnv() (*Config, error)` or `Validate() error`.
- Keep CLI exit behavior in `cmd/ldaplite/main.go`.
- Keep `Load()` only as a backwards-compatible wrapper if needed.

### 12. DN handling is too ad hoc for LDAP edge cases

Evidence:

- `internal/models/entry.go:123-131` splits parent DN with `strings.SplitN`.
- `internal/server/ldap.go:778-789` extracts UID with a simple prefix check.
- `internal/server/ldap.go:872-883` finds the first unescaped comma manually.
- `internal/store/sqlite.go:536-540` checks base containment with lowercase
  suffix matching.

Why this matters:

LDAP DNs have escaping and normalization rules. The current helpers are probably
fine for simple examples but will become a compatibility trap as clients submit
escaped commas, mixed attribute names, or non-`uid=` bind DNs.

Recommended simplification:

- Centralize DN parsing/normalization behind `internal/ldapdn`.
- Use a proven parser if one is already available in the dependency graph; if
  not, keep the local parser tiny and heavily tested.
- Store a normalized DN key or add a normalization helper used consistently by
  bind, placement, equality, and base checks.

## Testability And Performance Gaps

- There are no store/search benchmarks today; the benchmark command found no
  benchmark functions in `internal/store` or `internal/schema`.
- Add table-driven tests around search query shape and `EXPLAIN QUERY PLAN` for
  the most common filters.
- Add Web UI handler tests for form replacement semantics.
- Add cancellation tests once operation handlers accept request/server context.
- Add one integration/functional test for escaped DN components before changing
  DN parsing.

## Suggested Refactor Order

1. Introduce typed store errors and update LDAP result-code mapping.
2. Move operational/virtual attribute projection out of store read methods.
3. Add search options with scope and attribute selection, then avoid unnecessary
   recursive searches and `memberOf` computation.
4. Fix index/case-insensitive query strategy and add query-plan tests.
5. Split `sqlite.go` and `ldap.go` after behavior is better pinned down.
6. Deduplicate Web UI CRUD helpers and repair replace semantics.
7. Encapsulate the unsafe Who Am I response hack.

## Files With Highest Leverage

- `internal/store/sqlite.go`
- `internal/server/ldap.go`
- `internal/schema/filter_compiler.go`
- `internal/models/entry.go`
- `internal/web/handlers/users.go`
- `internal/web/handlers/groups.go`
- `internal/web/handlers/ous.go`
- `pkg/config/config.go`
