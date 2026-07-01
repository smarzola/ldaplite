# Goal: Add SCIM Provisioning API

You are working in `/Users/smarzola/projects/ldaplite`.

Your objective is to add a practical SCIM 2.0-compatible provisioning API for
LDAPLite users and groups. Keep the first implementation intentionally narrow:
support the common identity-provider provisioning path while preserving
LDAPLite's small deployment model, SQLite-backed storage, LDAP compatibility,
password security, and shared directory validation.

Track this work against GitHub issue `#7 Add SCIM 2.0 / REST provisioning API`
and the Product Roadmap item in `docs/ROADMAP.md`.

## Repository Rules

Follow `AGENTS.md`.

Important reminders:

- Inspect current source, tests, and docs directly before editing.
- Keep changes scoped to the current milestone.
- Do not revert unrelated user changes.
- Do not add a second CI or release workflow unless `.github/workflows/test.yml`
  and `.github/workflows/release.yml` truly cannot support the change.
- Reuse LDAPLite's existing store validation, authorization, password handling,
  audit logging, and directory service behavior.
- Do not write directly to SQLite from SCIM handlers when an existing
  `internal/directory.Service` operation can express the behavior.
- Password handling invariants from `AGENTS.md` are non-negotiable:
  `userPassword` must not be stored as a generic attribute, searches and SCIM
  responses must not return password material, and password writes must go
  through `ProcessPassword()`.
- Operational and server-managed attributes such as `objectClass`,
  `entryUUID`, `createTimestamp`, `modifyTimestamp`, and computed `memberOf`
  remain server-managed.
- Keep `tests/functional/` black-box with a real LDAPLite server subprocess and
  a real LDAP client library.
- At the end of each milestone, run verification, mark the milestone done in
  this file, add a status note, commit the completed milestone, and report the
  commit hash before continuing.

## Scope Controls

Implement SCIM-compatible provisioning, not full enterprise SCIM breadth.

In scope:

- SCIM 2.0 discovery endpoints for service provider config, schemas, and
  resource types.
- User and group list, get, create, replace/update, and delete flows.
- `entryUUID` as the stable SCIM resource `id`.
- Basic pagination with `startIndex` and `count`.
- A small supported filter subset for common IdP provisioning:
  `userName eq "..."`, `id eq "..."`, `displayName eq "..."`, and group
  `displayName eq "..."` where practical.
- HTTP Basic authentication using the existing LDAPLite user credentials path.
- Read routes guarded by `directory.read`.
- Write routes guarded by `directory.write`.
- Audit logs for mutating SCIM routes.
- Documentation of compatibility expectations, security model, and current
  limits.

Out of scope unless the user explicitly expands this goal:

- OAuth bearer-token management.
- SCIM PATCH support.
- Bulk operations.
- Enterprise User schema.
- Multi-valued email type semantics beyond a practical primary email mapping.
- Full SCIM filter grammar.
- SCIM ETags/version preconditions.
- Schema extension system.
- Dedicated HTTP listener separate from the existing embedded HTTP server.

## Target State

LDAPLite should expose a provisioning API that common SCIM-capable identity
systems can use to create, update, disable/delete, list, and inspect users and
groups.

By the end, the repo should have:

- `/scim/v2/ServiceProviderConfig`, `/scim/v2/Schemas`, and
  `/scim/v2/ResourceTypes` endpoints with stable SCIM JSON responses.
- `/scim/v2/Users` and `/scim/v2/Groups` collection endpoints.
- `/scim/v2/Users/{id}` and `/scim/v2/Groups/{id}` resource endpoints where
  `{id}` is LDAPLite's stable `entryUUID`.
- SCIM user responses mapped from `inetOrgPerson` entries without password
  leaks.
- SCIM group responses mapped from `groupOfNames` entries, including members by
  stable SCIM resource id where possible.
- SCIM writes translated into `directory.UserInput` and
  `directory.GroupInput` operations so LDAP, Web UI, and SCIM enforce the same
  validation and referential-integrity rules.
- Clear SCIM error responses with appropriate HTTP status codes for auth,
  validation, not-found, conflict, and unsupported-operation cases.
- Tests that prove non-admin users can read when authorized but cannot write,
  admin users can provision users/groups, protected attributes remain
  protected, and password material is never returned.
- Operator documentation explaining endpoint paths, auth, supported SCIM
  fields, unsupported SCIM features, and example requests.

## Current State

LDAPLite already has most of the internal prerequisites:

- `internal/directory.Service` supports user, group, OU, password-reset, and
  self-service password operations.
- `internal/web/server.go` mounts HTTP routes and protects them with capability
  middleware.
- `internal/authz` exposes `directory.read` and `directory.write`
  capabilities.
- `internal/store` can look up and search entries, including computed
  `memberOf` and stable `entryUUID`.
- The Web UI JSON API already exercises the shared directory service for user,
  group, OU, attribute, membership, and password mutations.
- The roadmap lists SCIM 2.0 or REST provisioning as an open product item.
- GitHub issue `#7` asks for an HTTP API surface for user and group
  provisioning that reuses store validation and authorization rules.

What is missing:

- No SCIM response/request model exists.
- No SCIM routes are mounted.
- No lookup helper maps `entryUUID` resource IDs back to DNs.
- No SCIM filter or pagination parser exists.
- No SCIM compatibility documentation exists.
- No SCIM-specific tests cover JSON shape, authz, password safety, or
  provisioning behavior.

Known constraints:

- The existing HTTP server is currently tied to the Web UI configuration.
  Unless the repo design changes during implementation, mount SCIM on that
  existing HTTP surface and document the operational behavior.
- Group creation in the directory service requires at least one member because
  `groupOfNames` requires `member`.
- The current directory service update methods replace extra attributes; SCIM
  PUT behavior should be explicit and tested.
- DELETE should remove entries rather than implement SCIM soft-disable unless a
  tested LDAP attribute mapping for `active` is added.

## SCIM Mapping

Use this first-pass mapping unless repo inspection reveals a better local fit:

User:

- SCIM `id` maps to LDAP `entryUUID`.
- SCIM `userName` maps to LDAP `uid`.
- SCIM `name.givenName` maps to LDAP `givenName`.
- SCIM `name.familyName` maps to LDAP `sn`.
- SCIM `displayName` maps to LDAP `cn`.
- SCIM `emails[0].value` maps to LDAP `mail`.
- SCIM `password` is accepted only on create/update and is never returned.
- SCIM `active` is accepted only if the implementation documents a real LDAP
  attribute mapping; otherwise reject unsupported active-state changes with a
  clear SCIM error.
- SCIM `externalId` may map to a generic LDAP attribute only if documented and
  covered by tests.
- SCIM `meta.resourceType`, `meta.created`, `meta.lastModified`, and
  `meta.location` derive from LDAPLite entry metadata.

Group:

- SCIM `id` maps to LDAP `entryUUID`.
- SCIM `displayName` maps to LDAP `cn`.
- SCIM `members[].value` accepts SCIM user or group ids and resolves them to
  LDAP DNs before calling the directory service.
- SCIM `members[].display` may be derived from LDAP `cn` or `uid`.
- SCIM `meta.resourceType`, `meta.created`, `meta.lastModified`, and
  `meta.location` derive from LDAPLite entry metadata.

General:

- Do not expose raw LDAP `userPassword`.
- Do not expose password hashes, Argon2 strings, or generated password values.
- Preserve LDAPLite's canonical `entryUUID` behavior.
- Keep unknown SCIM extension fields out of persisted LDAP attributes unless
  this file is updated with an explicit mapping and tests.

## Definition Of Done

The goal is complete only when:

1. SCIM discovery endpoints return stable, documented JSON responses.
2. SCIM users can be listed, fetched by `entryUUID`, created, replaced/updated,
   and deleted through authenticated HTTP requests.
3. SCIM groups can be listed, fetched by `entryUUID`, created,
   replaced/updated, and deleted through authenticated HTTP requests.
4. SCIM writes reuse `internal/directory.Service` or deliberately extend it
   where needed, without duplicating store validation in handlers.
5. SCIM routes enforce `directory.read` for reads and `directory.write` for
   writes.
6. Non-admin/non-write-authorized users cannot create, update, or delete SCIM
   resources.
7. Password material is accepted only on write paths and never appears in SCIM
   responses, logs, or generic attributes.
8. Protected LDAP attributes cannot be set through SCIM.
9. Group member references resolve through stable SCIM ids and still preserve
   LDAPLite's group member referential integrity.
10. Unsupported SCIM features fail with clear SCIM error responses instead of
    silent partial behavior.
11. Documentation explains endpoint paths, authentication, supported fields,
    examples, and intentional limits.
12. `docs/ROADMAP.md`, `README.md`, and `CHANGELOG.md` reflect the completed
    provisioning API when the implementation is complete.
13. Milestone checkboxes in this file are marked `[x]` as work completes.
14. Each completed milestone has a focused commit.
15. Final verification commands pass or any unrelated/environmental failures
    are documented with evidence.

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

- [x] Milestone 0: Baseline contract and test harness.
- [x] Milestone 1: SCIM discovery, shared response types, and error handling.
- [x] Milestone 2: User read/list/filter endpoints.
- [x] Milestone 3: User provisioning writes.
- [x] Milestone 4: Group read/list/filter endpoints.
- [x] Milestone 5: Group provisioning writes.
- [x] Milestone 6: Documentation, roadmap, regression, and release readiness.

## Milestone 0: Baseline Contract And Test Harness

Problem:

- SCIM has broad RFC surface area. LDAPLite needs a deliberately bounded
  contract before implementation so future agents do not overbuild or invent
  incompatible behavior midstream.

Desired behavior:

- Create the package and test scaffolding for SCIM without changing product
  behavior.
- Capture the supported endpoint, field, filter, pagination, auth, and error
  contract in tests or docs close to the implementation.

Acceptance criteria:

- A new SCIM package skeleton exists, likely under `internal/scim/`.
- Handler tests can construct the existing Web server or a SCIM-specific
  handler with the existing SQLite test store.
- The milestone does not expose partially working public routes unless they
  return a deliberate not-implemented response behind tests.
- Checklist is updated and committed.

Likely files:

- `internal/scim/`
- `internal/web/server.go`
- `internal/web/server_test.go`
- `docs/internal/prompts/scim-provisioning-api-goal-prompt.md`

Verification:

```bash
go test ./internal/scim ./internal/web
```

Status note, 2026-07-01:

- Commands run:
  `/opt/homebrew/opt/go@1.25/bin/gofmt -w internal/scim/contract.go internal/scim/handler.go internal/scim/handler_test.go`
  and
  `GOCACHE=/private/tmp/ldaplite-go-cache GOMODCACHE=/private/tmp/ldaplite-go-mod /opt/homebrew/opt/go@1.25/bin/go test ./internal/scim ./internal/web`.
- Result: passed. The first sandboxed test attempt could not resolve
  `proxy.golang.org` while populating the temporary module cache; the command
  passed after rerunning with network access.
- Commit: `b995bbb`.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 1: SCIM Discovery, Shared Response Types, And Error Handling

Problem:

- SCIM clients first probe discovery endpoints and expect consistent SCIM JSON
  and error shapes. Those foundations should be stable before resource
  handlers are added.

Desired behavior:

- Mount `/scim/v2/ServiceProviderConfig`, `/scim/v2/Schemas`, and
  `/scim/v2/ResourceTypes`.
- Add SCIM response helpers for content type, list responses, resource
  metadata, and error responses.
- Discovery responses should accurately advertise supported and unsupported
  features. For example, advertise no PATCH and no bulk support unless those
  are actually implemented.
- Discovery and read routes require authentication and `directory.read`.

Acceptance criteria:

- Discovery endpoints return JSON with SCIM-compatible `schemas` fields.
- Unsupported methods return appropriate HTTP status and SCIM error responses.
- Unauthenticated requests receive the existing HTTP Basic challenge behavior.
- Password-only users without `directory.read` cannot read SCIM discovery if
  the route is protected consistently with directory reads.
- Checklist is updated and committed.

Likely files:

- `internal/scim/*.go`
- `internal/scim/*_test.go`
- `internal/web/server.go`
- `internal/web/server_test.go`

Verification:

```bash
go test ./internal/scim ./internal/web
```

Status note, 2026-07-01:

- Commands run:
  `/opt/homebrew/opt/go@1.25/bin/gofmt -w internal/scim/contract.go internal/scim/handler.go internal/scim/handler_test.go internal/web/server.go internal/web/server_test.go`
  and
  `GOCACHE=/private/tmp/ldaplite-go-cache GOMODCACHE=/private/tmp/ldaplite-go-mod /opt/homebrew/opt/go@1.25/bin/go test ./internal/scim ./internal/web`.
- Result: passed.
- Commit: `bb96cfc`.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 2: User Read/List/Filter Endpoints

Problem:

- SCIM clients need to discover and reconcile users before provisioning writes.
  User reads must use stable ids and must not leak password material.

Desired behavior:

- Implement `GET /scim/v2/Users`.
- Implement `GET /scim/v2/Users/{id}` where `{id}` is `entryUUID`.
- Implement pagination with `startIndex` and `count`.
- Implement the first supported filter subset for users:
  `id eq "..."`, `userName eq "..."`, and `displayName eq "..."`.
- Map LDAP `inetOrgPerson` entries into SCIM user resources.
- Include `meta.created`, `meta.lastModified`, and `meta.location` when the
  source data is available.

Acceptance criteria:

- User list returns a SCIM `ListResponse`.
- User lookup by `entryUUID` returns exactly one user or a SCIM not-found
  error.
- Unsupported filters return a clear SCIM error.
- Search results never contain plaintext passwords, password hashes, or
  `userPassword`.
- Normal authenticated directory users can read user resources if they have
  `directory.read`.
- Password-only users without `directory.read` are denied.
- Checklist is updated and committed.

Likely files:

- `internal/scim/*.go`
- `internal/scim/*_test.go`
- `internal/web/server.go`
- `internal/web/server_test.go`
- `internal/store/store.go` only if a minimal helper abstraction is needed

Verification:

```bash
go test ./internal/scim ./internal/web ./internal/store
```

Status note, 2026-07-01:

- Commands run:
  `/opt/homebrew/opt/go@1.25/bin/gofmt -w internal/scim/handler.go internal/scim/handler_test.go internal/web/server.go internal/web/server_test.go`
  and
  `GOCACHE=/private/tmp/ldaplite-go-cache GOMODCACHE=/private/tmp/ldaplite-go-mod /opt/homebrew/opt/go@1.25/bin/go test ./internal/scim ./internal/web ./internal/store`.
- Result: passed.
- Commit: `df8a729`.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 3: User Provisioning Writes

Problem:

- User provisioning is the primary value of the SCIM API. The implementation
  must avoid a parallel validation path and preserve password security.

Desired behavior:

- Implement `POST /scim/v2/Users`.
- Implement `PUT /scim/v2/Users/{id}` for full replacement of supported user
  fields.
- Implement `DELETE /scim/v2/Users/{id}` as deletion unless this file is
  updated with a tested active-state mapping.
- Translate SCIM user requests into `directory.UserInput`.
- Resolve updates and deletes from SCIM `entryUUID` to LDAP DN.
- Accept `password` on create and update but never return it.
- Reject unsupported mutable fields with SCIM error responses.

Acceptance criteria:

- Admin/write-authorized users can create a user and then bind with the
  provided password through existing password verification helpers.
- Admin/write-authorized users can update supported user fields.
- Admin/write-authorized users can delete a user by SCIM id.
- Non-admin users with only `directory.read` cannot create, update, or delete
  users.
- Attempts to set protected attributes, server-managed attributes, or password
  material through generic attribute paths fail safely.
- SCIM write responses include stable `id`, `userName`, mapped names, email,
  and metadata, without password material.
- LDAP user behavior covered by existing tests still passes.
- Checklist is updated and committed.

Likely files:

- `internal/scim/*.go`
- `internal/scim/*_test.go`
- `internal/directory/service.go` only if existing inputs need a narrow,
  tested extension
- `internal/web/server.go`
- `internal/web/server_test.go`
- `internal/web/handlers/api_write.go` only for shared helper extraction if
  useful

Verification:

```bash
go test ./internal/scim ./internal/web ./internal/directory ./internal/store ./pkg/crypto
```

Status note, 2026-07-01:

- Commands run:
  `/opt/homebrew/opt/go@1.25/bin/gofmt -w internal/scim/handler.go internal/scim/handler_test.go internal/web/server.go internal/web/server_test.go`
  and
  `GOCACHE=/private/tmp/ldaplite-go-cache GOMODCACHE=/private/tmp/ldaplite-go-mod /opt/homebrew/opt/go@1.25/bin/go test ./internal/scim ./internal/web ./internal/directory ./internal/store ./pkg/crypto`.
- Result: passed. The first sandboxed test attempt could not resolve
  `proxy.golang.org` while downloading `github.com/stretchr/testify`; the
  command passed after rerunning with network access.
- Commit: `7b6722a`.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 4: Group Read/List/Filter Endpoints

Problem:

- SCIM clients often reconcile group membership after user sync. Group reads
  need to expose stable ids for both groups and members.

Desired behavior:

- Implement `GET /scim/v2/Groups`.
- Implement `GET /scim/v2/Groups/{id}` where `{id}` is `entryUUID`.
- Implement pagination with `startIndex` and `count`.
- Implement the first supported filter subset for groups:
  `id eq "..."` and `displayName eq "..."`.
- Map LDAP `groupOfNames` entries into SCIM group resources.
- Resolve LDAP `member` DNs into SCIM member ids when the referenced entry has
  an `entryUUID`.

Acceptance criteria:

- Group list returns a SCIM `ListResponse`.
- Group lookup by `entryUUID` returns exactly one group or a SCIM not-found
  error.
- Group member values are stable SCIM ids, not raw passwords or sensitive
  attributes.
- Missing or inconsistent member references, if encountered in legacy data, are
  handled predictably and documented in tests.
- Unsupported filters return a clear SCIM error.
- Read authorization matches user SCIM read behavior.
- Checklist is updated and committed.

Likely files:

- `internal/scim/*.go`
- `internal/scim/*_test.go`
- `internal/web/server.go`
- `internal/web/server_test.go`
- `internal/store/sqlite_search_test.go` only if lookup behavior needs store
  coverage

Verification:

```bash
go test ./internal/scim ./internal/web ./internal/store
```

Status note, 2026-07-01:

- Commands run:
  `/opt/homebrew/opt/go@1.25/bin/gofmt -w internal/scim/handler.go internal/scim/handler_test.go internal/web/server.go internal/web/server_test.go`
  and
  `GOCACHE=/private/tmp/ldaplite-go-cache GOMODCACHE=/private/tmp/ldaplite-go-mod /opt/homebrew/opt/go@1.25/bin/go test ./internal/scim ./internal/web ./internal/store`.
- Result: passed.
- Commit: `ddd83d6`.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 5: Group Provisioning Writes

Problem:

- Group provisioning completes the useful SCIM lifecycle. The implementation
  must preserve LDAPLite's `groupOfNames` referential integrity and member
  constraints.

Desired behavior:

- Implement `POST /scim/v2/Groups`.
- Implement `PUT /scim/v2/Groups/{id}` for full replacement of supported group
  fields and members.
- Implement `DELETE /scim/v2/Groups/{id}`.
- Translate SCIM group requests into `directory.GroupInput`.
- Resolve SCIM member ids to LDAP DNs before calling the directory service.
- Keep LDAPLite's requirement that group members point to existing entries.

Acceptance criteria:

- Admin/write-authorized users can create, update, and delete groups through
  SCIM.
- SCIM group writes reject missing member ids, unknown member ids, and empty
  member sets unless the directory model is deliberately changed with tests.
- Non-admin users with only `directory.read` cannot create, update, or delete
  groups.
- Group membership changes are visible through existing LDAP search behavior
  and computed `memberOf` behavior.
- Existing nested group and referential-integrity tests still pass.
- Checklist is updated and committed.

Likely files:

- `internal/scim/*.go`
- `internal/scim/*_test.go`
- `internal/directory/service.go` only if existing inputs need a narrow,
  tested extension
- `internal/web/server.go`
- `internal/web/server_test.go`
- `internal/store/sqlite_referential_test.go` only if shared validation changes

Verification:

```bash
go test ./internal/scim ./internal/web ./internal/directory ./internal/store
```

Status note, 2026-07-01:

- Commands run:
  `/opt/homebrew/opt/go@1.25/bin/gofmt -w internal/scim/handler.go internal/scim/handler_test.go internal/web/server.go internal/web/server_test.go`
  and
  `GOCACHE=/private/tmp/ldaplite-go-cache GOMODCACHE=/private/tmp/ldaplite-go-mod /opt/homebrew/opt/go@1.25/bin/go test ./internal/scim ./internal/web ./internal/directory ./internal/store`.
- Result: passed.
- Commit: `f08c838`.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 6: Documentation, Roadmap, Regression, And Release Readiness

Problem:

- Operators and future agents need exact documentation for what SCIM supports,
  how to secure it, and what remains intentionally unsupported.

Desired behavior:

- Add SCIM provisioning documentation with endpoint examples, authentication
  model, supported fields, unsupported features, and security notes.
- Update `README.md`, `docs/ROADMAP.md`, and `CHANGELOG.md`.
- Include example curl requests for discovery, user create/list/update/delete,
  and group create/list/update/delete.
- Make clear that SCIM is available on the existing embedded HTTP surface.
- Confirm CI commands still match the existing workflow shape.

Acceptance criteria:

- Documentation names the supported SCIM field mapping and limits.
- Documentation warns that passwords are write-only and never exported.
- Roadmap moves SCIM/provisioning from planned to completed or narrows any
  remaining follow-up item.
- Changelog includes a release-note-ready entry.
- Full regression passes or any unrelated/environmental failure is documented
  with exact output.
- Checklist is updated and committed.

Likely files:

- `docs/SCIM.md` or `docs/provisioning/scim.md`
- `README.md`
- `docs/ROADMAP.md`
- `CHANGELOG.md`
- `docs/internal/prompts/scim-provisioning-api-goal-prompt.md`

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

Status note, 2026-07-01:

- Commands run:
  `npm ci`;
  `npm run build:css`;
  `PATH=/opt/homebrew/opt/go@1.25/bin:$PATH GOCACHE=/private/tmp/ldaplite-go-cache GOMODCACHE=/private/tmp/ldaplite-go-mod go test -v -race ./...`;
  `PATH=/opt/homebrew/opt/go@1.25/bin:$PATH GOCACHE=/private/tmp/ldaplite-go-cache GOMODCACHE=/private/tmp/ldaplite-go-mod make test-functional`;
  `PATH=/opt/homebrew/opt/go@1.25/bin:$PATH GOCACHE=/private/tmp/ldaplite-go-cache GOMODCACHE=/private/tmp/ldaplite-go-mod go vet ./...`;
  `PATH=/opt/homebrew/opt/go@1.25/bin:$PATH gofmt -l .`;
  and
  `PATH=/opt/homebrew/opt/go@1.25/bin:$PATH GOCACHE=/private/tmp/ldaplite-go-cache GOMODCACHE=/private/tmp/ldaplite-go-mod go build -v ./...`.
- Result: passed. `make test-functional` first failed in the sandbox because
  `proxy.golang.org` could not be resolved while downloading
  `github.com/go-ldap/ldap/v3`; the same command passed after rerunning with
  network access. The full `go test -v -race ./...` regression was also run
  outside the sandbox because earlier sandbox attempts hit module-fetch and
  loopback-listener restrictions.
- Commit: `3b150fa`.

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

If a command fails because of environment restrictions rather than product
behavior, capture the exact failure and run the narrowest equivalent command
that can still validate the changed code.

## Final Response Required

When complete, report:

- target state achieved or not achieved;
- commits made, with hashes and milestone names;
- files changed by area;
- exact verification commands run and results;
- known residual risks or follow-up issues;
- any SCIM features deliberately left unsupported.
