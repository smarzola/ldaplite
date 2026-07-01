# Goal: Add LDIF Import And Export Commands

You are working in `/Users/smarzola/projects/ldaplite`.

Your objective is to add practical LDIF import and export commands for
LDAPLite. Keep the implementation focused on bootstrap, backup inspection, and
GitOps-style directory setup while preserving LDAPLite's SQLite-backed storage
model, password security invariants, group referential integrity, and existing
directory validation behavior.

Track this work against the Product Roadmap item in `docs/ROADMAP.md` and the
accepted design in `docs/IMPORT_EXPORT_DESIGN.md`.

## Repository Rules

Follow `AGENTS.md`.

Important reminders:

- Inspect current source, tests, and docs directly before editing.
- Keep changes scoped to the current milestone.
- Do not revert unrelated user changes.
- Do not add a second CI or release workflow unless `.github/workflows/test.yml`
  and `.github/workflows/release.yml` truly cannot support the change.
- Reuse LDAPLite's existing store validation, directory service behavior,
  password processing, and referential-integrity rules.
- Do not write raw SQL from import/export code to bypass existing invariants. If
  a narrow store or directory API extension is needed, add it deliberately and
  test it.
- Password handling invariants from `AGENTS.md` are non-negotiable:
  `userPassword` must not be stored as a generic attribute, searches and exports
  must not leak password hashes, and password writes must go through
  `ProcessPassword()`.
- Operational and server-managed attributes such as `objectClass`,
  `entryUUID`, `createTimestamp`, `modifyTimestamp`, and computed `memberOf`
  remain server-managed.
- Keep import/export independent of the LDAP listener and Web UI.
- Keep `tests/functional/` black-box with a real LDAPLite server subprocess and
  a real LDAP client library.
- At the end of each milestone, run verification, mark the milestone done in
  this file, add a status note, commit the completed milestone, and report the
  commit hash before continuing.

## Scope Controls

Implement practical LDAPLite LDIF import/export, not full OpenLDAP `slapadd`
compatibility.

In scope:

- `ldaplite import ldif --file <path> [--dry-run] [--replace-existing]
  [--allow-generated-passwords]`.
- `ldaplite export ldif [--file <path-or->] [--include-operational]
  [--include-password-placeholders]`.
- LDIF records with `dn`, attributes, multi-value attributes, folded lines,
  comments, blank record separators, and base64-encoded values.
- Import of supported LDAPLite entries: base DN, organizational units,
  `inetOrgPerson` users, `groupOfNames` groups, and read-only bind users/groups.
- Full-batch validation before writes for base-DN containment, parent
  existence, supported structural object classes, group member existence,
  protected attributes, and password handling.
- Safe export that is importable by LDAPLite and ordered parent-before-child.
- Unit, command, and functional coverage for the user-visible behavior.
- Operator documentation and roadmap cleanup after the commands are complete.

Out of scope unless the user explicitly expands this goal:

- Full LDIF change records for `changetype: modify`, `delete`, `modrdn`, or
  `moddn`.
- LDAP controls, schema extension import, server-specific operational imports,
  replication, incremental sync, and conflict-free merge semantics.
- Importing arbitrary third-party password hashes.
- Exporting raw password hashes, even behind an opt-in flag.
- Web UI import/export screens.

## Target State

LDAPLite should let operators seed and inspect a directory without running LDAP
client write commands by hand.

By the end, the repo should have:

- Cobra command groups for `import ldif` and `export ldif` wired from
  `cmd/ldaplite/main.go`.
- An internal LDIF parser/writer package that is tested independently from the
  CLI.
- An import planning and validation layer that parses the whole file, validates
  the whole batch, and orders writes parent-before-child.
- Import writes that reuse existing model, directory, password, and store
  invariants instead of bypassing them.
- A dry-run mode that performs all parse and validation work without modifying
  the database.
- A replace-existing mode with documented and tested semantics, or a deliberate
  design update if implementation proves that the flag should be deferred.
- Generated-password handling that prints generated values once to stdout and
  never stores them outside the password hash, or a deliberate design update if
  the flag should be deferred.
- Safe LDIF export that omits `userPassword` and computed `memberOf` by default.
- Documentation that describes command usage, security behavior, limitations,
  and example import/export flows.

## Current State

LDAPLite has most of the internal building blocks:

- `docs/IMPORT_EXPORT_DESIGN.md` defines the intended command shape, flags,
  validation rules, password behavior, and required tests.
- `docs/ROADMAP.md` lists LDIF import/export as an open Product Roadmap item.
- `cmd/ldaplite/main.go` wires Cobra commands for server, version, and
  healthcheck; import/export commands do not exist yet.
- `internal/directory.Service` supports user, group, OU, membership, and
  password mutation flows with shared validation.
- `internal/store` validates entry placement, parent existence, group members,
  and password storage rules during create/update operations.
- `internal/models.Entry.ToLDIF` exists, but it is too small for production
  export because it does not define safe ordering, LDIF escaping/base64 rules,
  password redaction, or operational-attribute policy.

What is missing:

- No `internal/ldif` parser or writer exists.
- No import planning or dry-run validation layer exists.
- No CLI command tests cover LDIF import/export.
- No functional test proves an imported database can serve real LDAP binds and
  searches.
- No user-facing command documentation exists beyond the design document.

Known constraints:

- Direct `go test ./...` requires embedded Web UI assets to already exist.
  Prefer `make test` on a fresh checkout.
- The current store interface does not expose a whole-import transaction. The
  implementation must fail before writes for validation errors and must document
  any remaining partial-write risk for storage errors unless a tested batch
  transaction API is added.
- `groupOfNames` requires at least one `member`, and member references must
  resolve to existing entries or entries in the same import batch.

## Definition Of Done

The goal is complete only when:

1. `ldaplite import ldif --file <path> --dry-run` parses and validates an LDIF
   file without writing to the configured SQLite database.
2. `ldaplite import ldif --file <path>` imports base-compatible OUs, users, and
   groups in parent-before-child order.
3. Import rejects entries outside `LDAP_BASE_DN`, entries with missing parents,
   unsupported structural object classes, missing group members, malformed
   LDIF, unsupported LDIF change records, and client-supplied protected
   attributes with clear errors that include the failing DN when available.
4. Import accepts plaintext `userPassword` only through existing password
   processing and never stores it in the generic attributes table.
5. Replace-existing behavior is either implemented with tests or intentionally
   removed/deferred in both code and docs.
6. Generated-password behavior is either implemented with tests or
   intentionally removed/deferred in both code and docs.
7. `ldaplite export ldif --file -` emits safe, importable LDIF ordered
   parent-before-child.
8. Export omits `userPassword` and computed `memberOf` by default and never
   emits raw password hashes.
9. Export flags for operational attributes and password placeholders behave as
   documented and are covered by tests.
10. Command failures exit non-zero and print concise, operator-actionable
    messages.
11. Unit, command, and functional tests cover parser, import validation, import
    writes, export safety, password behavior, and real LDAP server use of an
    imported database.
12. `docs/IMPORT_EXPORT_DESIGN.md`, `docs/ROADMAP.md`, `README.md` or another
    appropriate operator doc, and `CHANGELOG.md` are updated.
13. Milestone checkboxes in this file are marked `[x]` as work completes.
14. Each completed milestone has a focused commit.
15. Final verification commands pass or any unrelated failures are documented
    with evidence.

## Milestone Checklist

When a milestone is complete:

1. Run the milestone's verification commands.
2. Update this checklist by changing the milestone from `[ ]` to `[x]`.
3. Add a short status note under that milestone with the date, exact commands
   run, and commit hash if available.
4. Commit the code, tests, docs, and checklist/status update with a focused
   commit message.
5. Report the commit hash in the goal-loop status before starting the next
   milestone.

- [x] Milestone 0: Baseline Contract And Fixtures
- [x] Milestone 1: LDIF Parser And Writer Primitives
- [ ] Milestone 2: Import Planning And Dry-Run Validation
- [ ] Milestone 3: Import Write Path And CLI Command
- [ ] Milestone 4: Export Path And CLI Command
- [ ] Milestone 5: Optional Import/Export Flags
- [ ] Milestone 6: Functional Coverage, Docs, And Final Regression

## Milestone 0: Baseline Contract And Fixtures

Problem:

- The design document defines the command behavior, but implementation needs
  concrete fixtures and a test contract before parser and command code grow.

Desired behavior:

- The repo has minimal, reusable LDIF fixtures and test expectations for valid
  and invalid imports.
- The implementation path is confirmed against the current command, directory,
  model, store, and password APIs.

Acceptance criteria:

- Add or identify fixtures for a valid base/user/group import, malformed LDIF,
  outside-base DN, missing parent, missing group member, protected attributes,
  and password handling.
- Document any discovered API gap in this file before implementing around it.
- Do not add production behavior in this milestone unless it is needed for test
  scaffolding.
- Milestone status is marked done in this file and committed.

Likely files:

- `docs/LDIF_IMPORT_EXPORT_GOAL_PROMPT.md`
- `internal/ldif/testdata/*.ldif`
- `cmd/ldaplite/*_test.go`
- `internal/directory/*`
- `internal/store/*`
- `pkg/crypto/*`

Verification:

```bash
go test ./internal/directory ./internal/store ./pkg/crypto
```

Status notes:

- 2026-07-01: Added baseline LDIF fixtures under `internal/ldif/testdata/` for
  valid bootstrap import, base64 values, malformed input, outside-base DN,
  missing parent, missing group member, protected attributes, unsupported
  password scheme, and unsupported changetype. Current API gaps to resolve in
  later milestones: `internal/directory.Service` create methods derive DNs from
  parent/RDN fields rather than accepting full LDIF DNs, and `store.Store` does
  not expose a whole-import transaction boundary. Verification command:
  `go test ./internal/directory ./internal/store ./pkg/crypto`; the first
  sandboxed run failed because Go could not create `~/Library/Caches/go-build`,
  then the same command passed outside the sandbox.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 1: LDIF Parser And Writer Primitives

Problem:

- Command code must not parse LDIF with ad hoc string splitting. LDIF syntax has
  record separators, comments, folded lines, base64 values, and repeated
  attributes that need one tested implementation.

Desired behavior:

- `internal/ldif` can parse practical LDAPLite LDIF records into structured
  data and write structured records back to safe LDIF text.

Acceptance criteria:

- Parser supports comments, blank-line record separators, folded lines,
  `attr: value`, `attr:: base64`, repeated attributes, and line-numbered error
  reporting.
- Parser rejects malformed records, missing `dn`, duplicate `dn`, unsupported
  URL values, and unsupported `changetype` records with clear errors.
- Writer emits deterministic records with correct escaping/base64 handling for
  values that require it.
- Unit tests cover parser success, parser failure, writer output, and round-trip
  safety for representative LDAPLite records.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/ldif/`
- `internal/ldif/testdata/`

Verification:

```bash
go test ./internal/ldif
```

Status notes:

- 2026-07-01: Added `internal/ldif` parser and writer primitives with ordered
  records, multi-value attributes, folded-line handling, base64 value decoding
  and encoding, line-numbered parse errors, unsupported URL value rejection,
  and unsupported changetype rejection. Verification command:
  `go test ./internal/ldif`; initial runs caught an unused test import and a
  line-number-sensitive round-trip assertion, then the command passed.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 2: Import Planning And Dry-Run Validation

Problem:

- Import must validate the full batch before it writes anything, otherwise
  common operator mistakes can leave a partially seeded directory.

Desired behavior:

- A planning layer turns parsed LDIF records into validated LDAPLite operations
  without mutating the database.

Acceptance criteria:

- Validate every DN is under `LDAP_BASE_DN`.
- Validate every non-base entry has a parent already in the database or in the
  import batch.
- Validate each record has exactly one supported structural object class:
  `organizationalUnit`, `inetOrgPerson`, `groupOfNames`, or `top` for the base
  DN only.
- Reject client-supplied `entryUUID`, `createTimestamp`, `modifyTimestamp`,
  `memberOf`, and other protected/server-managed attributes.
- Validate group `member` values exist in the database or batch.
- Validate user records satisfy existing `inetOrgPerson` requirements.
- Validate password values through the existing password processor without
  storing or returning password material from dry-run.
- Sort planned operations parent-before-child.
- Tests prove dry-run validation leaves the database unchanged.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/ldif/`
- `internal/directory/`
- `internal/models/`
- `internal/store/`
- `pkg/crypto/`

Verification:

```bash
go test ./internal/ldif ./internal/directory ./internal/models ./internal/store ./pkg/crypto
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 3: Import Write Path And CLI Command

Problem:

- Operators need a real `ldaplite import ldif` command that uses the validated
  plan and writes through LDAPLite's normal invariants.

Desired behavior:

- `ldaplite import ldif --file <path>` initializes the configured store, applies
  a validated plan, and reports a concise summary. `--dry-run` performs the
  same parse and validation work without writes.

Acceptance criteria:

- Wire `import ldif` into Cobra from `cmd/ldaplite/main.go`.
- Read configuration from the existing environment/config path used by the
  server.
- Require `--file`; report missing or unreadable files clearly.
- Initialize SQLite store and migrations before validation and import.
- Apply writes parent-before-child through existing model/directory/store paths.
- Preserve password invariants during real writes.
- Print a clear summary of records parsed, records imported, and dry-run status.
- Return non-zero for parse, validation, config, and storage failures.
- Command tests cover dry-run, successful import, invalid input, and no password
  leakage into generic attributes.
- Milestone status is marked done in this file and committed.

Likely files:

- `cmd/ldaplite/main.go`
- `cmd/ldaplite/*import*.go`
- `cmd/ldaplite/*_test.go`
- `internal/ldif/`
- `internal/directory/`
- `internal/store/`
- `pkg/config/`

Verification:

```bash
npm run build:css
go test ./cmd/ldaplite ./internal/ldif ./internal/directory ./internal/store ./pkg/config ./pkg/crypto
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 4: Export Path And CLI Command

Problem:

- LDAPLite needs a safe inspection/export path that can produce practical LDIF
  without leaking password material or computed attributes.

Desired behavior:

- `ldaplite export ldif --file -` emits deterministic, importable LDIF for the
  configured database.

Acceptance criteria:

- Wire `export ldif` into Cobra from `cmd/ldaplite/main.go`.
- Read configuration from the existing environment/config path used by the
  server.
- Default `--file` to `-` for stdout and write atomically enough for normal file
  destinations.
- Export base DN, OUs, users, and groups parent-before-child.
- Export `objectClass`, normal attributes, and group `member` values.
- Omit `userPassword` and computed `memberOf` by default.
- Never export raw password hashes.
- Command tests cover stdout export, file export, ordering, importability, and
  redaction behavior.
- Milestone status is marked done in this file and committed.

Likely files:

- `cmd/ldaplite/main.go`
- `cmd/ldaplite/*export*.go`
- `cmd/ldaplite/*_test.go`
- `internal/ldif/`
- `internal/store/`
- `internal/models/`

Verification:

```bash
npm run build:css
go test ./cmd/ldaplite ./internal/ldif ./internal/store ./internal/models
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 5: Optional Import/Export Flags

Problem:

- The design includes replace-existing imports, generated passwords, safe
  operational export, and password placeholders. These flags need explicit
  semantics and tests before they are exposed to operators.

Desired behavior:

- Optional flags either work as documented or are intentionally deferred with
  code and docs kept in sync.

Acceptance criteria:

- `--replace-existing` replaces an existing entry by DN only after full
  validation succeeds, preserves password and relationship invariants, and has
  clear behavior for children and group membership references.
- `--allow-generated-passwords` generates missing user passwords only when the
  flag is set, prints generated passwords once to stdout, and never persists
  plaintext.
- `--include-operational` exports only safe operational attributes such as
  `entryUUID`, `createTimestamp`, and `modifyTimestamp`; it does not export
  computed `memberOf` unless the design is explicitly changed and tested.
- `--include-password-placeholders` emits `userPassword: {REDACTED}` for user
  entries without exposing hashes or plaintext.
- If any flag is deferred, remove or mark it unsupported in CLI help,
  `docs/IMPORT_EXPORT_DESIGN.md`, and operator docs.
- Tests cover every shipped flag and unsupported/deferred flag behavior.
- Milestone status is marked done in this file and committed.

Likely files:

- `cmd/ldaplite/*import*.go`
- `cmd/ldaplite/*export*.go`
- `internal/ldif/`
- `internal/directory/`
- `internal/store/`
- `docs/IMPORT_EXPORT_DESIGN.md`

Verification:

```bash
npm run build:css
go test ./cmd/ldaplite ./internal/ldif ./internal/directory ./internal/store ./pkg/crypto
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 6: Functional Coverage, Docs, And Final Regression

Problem:

- LDIF import/export is an operator-facing workflow. Unit tests alone do not
  prove that an imported database works through the real LDAP server.

Desired behavior:

- Functional tests prove imported data can be served by LDAPLite, and docs
  describe the shipped behavior accurately.

Acceptance criteria:

- Add functional coverage that imports an LDIF fixture into a temporary
  database, starts a real LDAPLite server subprocess, binds as an imported user,
  binds as an imported read-only app user, confirms search succeeds, and
  confirms unauthorized writes return `insufficientAccessRights`.
- Add an export/import round-trip test where practical.
- Update `docs/IMPORT_EXPORT_DESIGN.md` from design contract to implemented
  behavior.
- Update `docs/ROADMAP.md` to move LDIF import/export out of open roadmap
  status.
- Update `README.md` or another operator-facing doc with concise command
  examples and security notes.
- Update `CHANGELOG.md` with the feature entry for the intended release.
- Run full local verification.
- Milestone status is marked done in this file and committed.

Likely files:

- `tests/functional/`
- `docs/IMPORT_EXPORT_DESIGN.md`
- `docs/ROADMAP.md`
- `README.md`
- `CHANGELOG.md`
- `cmd/ldaplite/`
- `internal/ldif/`

Verification:

```bash
npm ci
npm run build:css
go test -v -race ./...
make test-functional
go vet ./...
test -z "$(gofmt -l .)"
go build -v ./...
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Final Verification

Before the goal is complete, run:

```bash
npm ci
npm run build:css
go test -v -race ./...
make test-functional
go vet ./...
test -z "$(gofmt -l .)"
go build -v ./...
```

Also run at least one manual smoke sequence against a temporary database:

```bash
ldaplite import ldif --file ./path/to/fixture.ldif --dry-run
ldaplite import ldif --file ./path/to/fixture.ldif
ldaplite export ldif --file -
```

## Final Response Required

When complete, report:

- target state achieved or not achieved;
- commits made, including milestone commit hashes;
- files changed;
- exact verification commands run and results;
- any design flags implemented, deferred, or intentionally removed;
- known residual risks or follow-up issues.
