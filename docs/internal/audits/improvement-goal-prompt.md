# LDAPLite Improvement Goal Prompt

> Status note, 2026-06-21: this prompt has been substantially executed in the current worktree. GitHub issue `#2` was split into focused issues `#7` through `#11` and closed; Dependabot PR `#5` was merged. Keep this file as the original improvement brief and use `docs/ROADMAP.md` for current roadmap state.

Use this prompt in a goal loop to harden LDAPLite and turn the current review findings into focused, verified improvements.

## Goal

Improve LDAPLite's security, LDAP interoperability, release hygiene, and roadmap clarity without changing the project's core simplicity: a lightweight Go LDAP server with SQLite storage and an optional embedded Web UI.

Work in small, reviewable increments. For each completed item, add or update focused tests, run the relevant test suite, and keep README/docs aligned with actual behavior.

## Current Context

- Repository: `smarzola/ldaplite`
- Main branch currently includes embedded migrations, SQL-compiled search filters, operational timestamps, Web UI CRUD, direct `memberOf`, and generated Web UI CSS.
- GitHub issue `#2 Future development` asked about Web UI, REST API, and templates. Web UI now exists; REST/SCIM, templates, OpenTelemetry, password flows, and compatibility tests are split into focused follow-up issues.
- Dependabot PR `#5`, which bumped `picomatch` from `2.3.1` to `2.3.2`, has been merged.
- `go test ./...` passes after the local attribute-casing fix.

## Priority Order

1. LDAP authentication and authorization enforcement.
2. Web UI write safety.
3. Directory referential integrity and LDAP result semantics.
4. Attribute casing/interoperability completion.
5. Dependency, CI, release, and docs hygiene.
6. Roadmap issue cleanup and strategic feature planning.

## P0: LDAP Operations Must Require Bind

Problem:

- `LDAP_ALLOW_ANONYMOUS_BIND=false` rejects anonymous bind requests, but unbound clients can still call `Search`, `Add`, `Modify`, and `Delete`.
- `protocol.Connection.dispatch` sends operations directly to handlers, and handlers do not check `conn.GetBoundDN()`.

Desired behavior:

- RootDSE and schema searches may remain anonymously readable unless configuration says otherwise.
- Normal search behavior should be explicit: authenticated-only by default, optional anonymous read if configured.
- Add, Modify, Delete, and password changes must require an authenticated bind.
- Return proper LDAP result codes such as `insufficientAccessRights`, `strongerAuthRequired`, or another appropriate code supported by the current library.

Acceptance criteria:

- Add tests showing unbound Add/Modify/Delete are rejected.
- Add tests showing unbound normal Search is rejected by default.
- Add tests showing RootDSE/schema behavior remains intentional.
- README documents anonymous/bind behavior accurately.

Likely files:

- `internal/protocol/connection.go`
- `internal/server/ldap.go`
- `pkg/config/config.go`
- tests under `internal/server` or protocol-level integration tests

## P1: Web UI CSRF And GET Mutation

Problem:

- Web UI delete actions mutate state via GET query URLs like `/users/delete?dn=...`.
- Create/edit forms are POSTs but have no CSRF token or Origin validation.
- Basic Auth credentials can be automatically sent by browsers, which makes cross-site form/action abuse possible.

Desired behavior:

- Deletes use POST or DELETE, never GET.
- Mutating routes validate a CSRF token or strict same-origin policy.
- Error handling remains user-friendly in the Web UI.

Acceptance criteria:

- Delete buttons submit POST forms or equivalent safe requests.
- Mutating handlers reject missing/invalid CSRF token or invalid Origin.
- Tests cover rejected CSRF attempts for user, group, and OU mutations.
- Templates still render correctly.

Likely files:

- `internal/web/server.go`
- `internal/web/handlers/users.go`
- `internal/web/handlers/groups.go`
- `internal/web/handlers/ous.go`
- `internal/web/templates/*.html`
- `internal/web/middleware`

## P1: Referential Integrity For Parent DNs And Group Members

Problem:

- `CreateEntry` derives `ParentDN` but does not verify the parent exists or is under the configured base DN.
- Group `member` attributes can reference nonexistent DNs. The `INSERT ... SELECT ... WHERE dn = ?` sync can affect zero rows without returning an error, leaving `member` attributes and computed `memberOf` out of sync.

Desired behavior:

- Entries cannot be created outside the configured base DN.
- Non-root entries require an existing parent.
- Group members must reference existing entries, unless a deliberate deferred-reference feature is implemented and documented.
- Sync failures return LDAP constraint/noSuchObject-style errors instead of logging only.

Acceptance criteria:

- Tests for orphan entry creation rejection.
- Tests for out-of-base DN rejection.
- Tests for nonexistent group member rejection on create and update.
- Tests prove `member` attributes and `group_members` stay consistent.

Likely files:

- `internal/store/sqlite.go`
- `internal/server/ldap.go`
- `internal/web/handlers/groups.go`
- store tests

## P1: Nested Groups And `memberOf` Semantics

Problem:

- README advertises nested group support and circular reference detection.
- Current `IsUserInGroup` is direct-membership only.
- `populateMemberOf` emits direct groups only.
- Web UI admin authorization checks direct membership only, so nested admin groups do not work.

Desired behavior:

- Either document group support as direct-only, or implement recursive group semantics.
- If implementing recursive behavior, use SQLite recursive CTEs with cycle/depth protection.
- Decide whether `memberOf` should include direct groups only or transitive groups; document the decision.

Acceptance criteria:

- Tests for nested groups.
- Tests for cycle handling.
- Tests for Web UI admin access through nested admin group, if supported.
- README and schema docs match implemented semantics.

Likely files:

- `internal/store/sqlite.go`
- `internal/web/middleware/auth.go`
- `README.md`
- `internal/store/sqlite_search_test.go`

## P2: LDAP Search Response Attribute Selection

Problem:

- Search responses currently return all attributes regardless of the requested attribute selection.
- LDAP clients often rely on requested attributes, operational attribute selection, and `1.1` no-attribute behavior.

Desired behavior:

- Honor requested attribute names case-insensitively.
- Implement `1.1` no-attribute behavior.
- Decide support for `*` and `+` conventions if feasible.
- Ensure operational attributes like `memberOf`, `createTimestamp`, and `modifyTimestamp` are returned according to LDAP expectations and project policy.

Acceptance criteria:

- Tests for requesting a single attribute.
- Tests for requesting mixed-case attribute names.
- Tests for `1.1`.
- Tests for operational attribute behavior.

Likely files:

- `internal/server/ldap.go`
- `internal/protocol`
- server/integration tests

## P2: Attribute Casing Interoperability

Problem:

- LDAP matching is case-insensitive, but some client software expects exact presentation casing such as `memberOf`, not `memberof`.
- A response-boundary canonical casing helper now exists for known attributes, but custom attributes are still lowercased when stored through `Entry.SetAttribute` and `Entry.AddAttribute`.

Desired behavior:

- Keep internal matching case-insensitive.
- Emit canonical casing for known LDAP attributes on the wire.
- Decide whether custom attribute original casing should be preserved for response presentation.

Acceptance criteria:

- Add/keep tests for canonical known attributes: `objectClass`, `memberOf`, `createTimestamp`, `modifyTimestamp`, `givenName`, `displayName`, `telephoneNumber`, `userPassword`, RootDSE attributes.
- Add integration or BER-level tests proving search responses emit canonical names.
- If preserving custom casing, extend the model/storage format without breaking case-insensitive lookup.

Likely files:

- `internal/protocol/response.go`
- `internal/protocol/response_test.go`
- `internal/models/entry.go`
- `internal/store/sqlite.go`

## P2: Duplicate `objectClass` Emission

Problem:

- Entries add `objectclass` as an operational/internal attribute, while search handling also adds `objectClass` explicitly.
- This can produce duplicate objectClass attributes with different casing unless skipped.

Desired behavior:

- Emit exactly one `objectClass` attribute in search responses.
- Keep internal filters working for `(objectClass=*)` and equality filters.

Acceptance criteria:

- Test response construction or integration search to prove a single `objectClass` is emitted.

Likely files:

- `internal/server/ldap.go`
- `internal/protocol`

## P2: Healthcheck Is A False Positive

Problem:

- Docker uses `ldaplite healthcheck`, but the command currently always prints success.

Desired behavior:

- Healthcheck verifies meaningful readiness:
  - database can be opened,
  - migrations are usable or DB has expected schema,
  - LDAP listener is reachable when checking a running service, or define separate `healthcheck` vs `check-config` semantics.

Acceptance criteria:

- Failing DB path/config returns non-zero.
- Docker healthcheck accurately reflects service health.
- Tests cover success/failure where feasible.

Likely files:

- `cmd/ldaplite/main.go`
- `Dockerfile`
- `pkg/config`

## P2: Dependency And PR Hygiene

Problem:

- Dependabot PR `#5` updates `picomatch` from `2.3.1` to `2.3.2` for security fixes and has passing checks.
- The project still locally resolves `picomatch@2.3.1` through Tailwind tooling until the PR is merged.

Desired behavior:

- Merge or refresh PR `#5`.
- Consider Dependabot auto-merge for low-risk security patch bumps after CI success.

Acceptance criteria:

- `package-lock.json` resolves `picomatch@2.3.2`.
- `npm ci`, CSS build, and Go tests pass.

Likely files:

- `package-lock.json`
- `.github/dependabot.yml` if added

## P2: Go Version And CI/Release Drift

Problem:

- `go.mod` says `go 1.25.3`.
- Test CI uses Go `1.25`.
- Docker uses `golang:1.25-alpine`.
- Release binary build uses Go `1.23`.
- README says Go `1.23+`; QUICKSTART says Go `1.22+`.

Desired behavior:

- Pick one supported Go version policy.
- Align `go.mod`, CI, release workflow, Dockerfile, README, and QUICKSTART.

Acceptance criteria:

- No conflicting Go version claims remain.
- Release workflow uses the same major/minor as CI unless intentionally documented.
- `go test ./...` and release build still pass.

Likely files:

- `go.mod`
- `.github/workflows/test.yml`
- `.github/workflows/release.yml`
- `Dockerfile`
- `README.md`
- `QUICKSTART.md`

## P3: Generated CSS Developer Experience

Problem:

- `make test` now builds CSS first, but direct `go test ./...` fails on a fresh checkout if `internal/web/static/output.css` does not exist.

Desired behavior:

- Choose one:
  - commit generated CSS,
  - provide a test fallback,
  - restructure embedding,
  - or consistently document/enforce `make test`.

Acceptance criteria:

- Fresh checkout test path is reliable and documented.
- CI and local docs use the same recommended test command.

Likely files:

- `Makefile`
- `README.md`
- `internal/web/server.go`
- `.gitignore`

## P3: Stale Planning Docs

Problem:

- `docs/internal/design-history/sqlite-improvement-plan.md` and `docs/internal/design-history/timestamp-attributes-plan.md` describe many items as future/current problems even though parts are already implemented.

Desired behavior:

- Convert old plans into status documents or archive them.
- Clearly mark completed, superseded, and remaining work.

Acceptance criteria:

- No stale line references or false “current issue” claims remain.
- Remaining work becomes actionable issues or TODO sections.

Likely files:

- `docs/internal/design-history/sqlite-improvement-plan.md`
- `docs/internal/design-history/timestamp-attributes-plan.md`
- new `docs/ROADMAP.md` if helpful

## P3: GitHub Issue Roadmap Cleanup

Problem:

- Issue `#2 Future development` asks whether Web UI, REST API, and templates are planned.
- Owner reply promised Web UI, SCIM API, and OpenTelemetry.
- Web UI now exists, but the issue remains open and broad.

Desired behavior:

- Update issue `#2` with current status.
- Close the completed Web UI part or split into separate roadmap issues:
  - SCIM/REST API,
  - user/group templates,
  - OpenTelemetry metrics/tracing,
  - Web UI password reset/change flow if missing.

Acceptance criteria:

- GitHub issues reflect actual project state and next work.
- README roadmap, if added, matches issues.

## Strategic Feature Direction

After the security and correctness fixes above, consider these larger improvements:

- Extract a service layer shared by LDAP handlers, Web UI handlers, and future SCIM/REST API so validation and authorization are not duplicated.
- Add user/group templates in the Web UI as structured presets over the existing attributes system.
- Add SCIM only after DN/member/ref integrity and auth boundaries are settled.
- Add OpenTelemetry after healthcheck/readiness semantics are real, so metrics reflect meaningful states.
- Add compatibility tests against common LDAP clients or libraries that are known to be casing-sensitive.

## Suggested Loop Instruction

For each loop:

1. Pick the highest-priority unfinished item.
2. Read the relevant code and tests before editing.
3. Implement the smallest complete fix.
4. Add tests that fail before the fix and pass after.
5. Run targeted tests, then `go test ./...`.
6. Update docs if behavior changed.
7. Summarize:
   - files changed,
   - behavior changed,
   - tests run,
   - remaining risks,
   - next recommended item.
