# Goal: Close Client Compatibility And Product Adoption Gaps

You are working in `/Users/smarzola/ldaplite`.

Your objective is to move LDAPLite from "LDAP server with solid core
compatibility" to "easy to adopt as a self-hosted directory for real LDAP
consumers." Focus on the concrete gaps found by comparing LDAPLite with lldap
and by reviewing expectations from LDAP-consuming apps such as Pocket ID,
Authelia, Dex, Gitea/Forgejo, Grafana, Nextcloud, and Vaultwarden.

## Repository Rules

Follow `AGENTS.md`.

Important reminders:

- Use `ast-grep` for Go structural searches.
- Use `rg` for ordinary text/file searches.
- Keep changes scoped to the current milestone.
- Do not revert unrelated user changes.
- Prefer black-box LDAP compatibility tests for client-visible behavior.
- When a test exposes a real client compatibility gap, fix LDAPLite rather than
  weakening the test.
- Preserve the completed observability work. Audit logs, metrics, tracing, and
  sensitive-data redaction must not regress.
- At every checkpoint, update this file's milestone checklist and commit the
  completed checkpoint before moving on.

## Target State

LDAPLite should be credible as a drop-in lightweight LDAP directory for common
self-hosted identity consumers.

The target state is:

- Users and groups expose stable generated identifiers such as `entryUUID` and
  `uuid`, suitable for clients that need durable sync keys.
- Pocket ID has a tested, documented integration path.
- A small set of high-value client recipes exists under `docs/integrations/`.
- The functional compatibility suite includes client-consumer expectations, not
  only generic LDAP behavior.
- LDAP Compare returns meaningful results instead of always returning false.
- Operators can create read-only or otherwise constrained service accounts for
  application bind users, or the remaining ACL limitation is explicitly tracked
  with a concrete design.
- The TLS/LDAPS story is operationally clear: either native support exists, or
  a tested reverse-proxy/sidecar recipe exists.
- Bootstrap/import/export has a practical path for initial directory setup.
- Existing security, password handling, `memberOf`, telemetry, Web UI, CI, and
  release behavior do not regress.

## Current Strengths To Preserve

Do not accidentally remove or weaken these advantages:

- Simple binary, SQLite-backed deployment.
- Core LDAP v3 operations: Bind, Search, Add, Modify, Delete, RootDSE, schema,
  and Who Am I.
- Black-box functional compatibility tests through a real LDAP client library.
- `groupOfNames` groups with referential integrity.
- Computed, read-only `memberOf`, including nested groups.
- Hidden `userPassword` search behavior and isolated password storage.
- Argon2id password processing and constant-time verification.
- Audit-grade logs, optional OpenTelemetry tracing, Prometheus-compatible
  metrics, and bounded telemetry labels.
- Existing CI and tag-driven release workflow shape.

## Definition Of Done

This goal is complete only when:

1. Stable user/group identifiers are implemented, migrated, tested, and
   documented.
2. Pocket ID compatibility has a functional test path and a documented recipe.
3. At least four additional integration recipes exist and have config examples
   aligned with LDAPLite's actual schema.
4. LDAP Compare is implemented and covered by tests.
5. Service-account/read-only bind behavior is either implemented and tested or
   captured in a concrete design document with explicit non-goals and risks.
6. TLS/LDAPS deployment guidance is tested or native support is implemented and
   tested.
7. Bootstrap/import/export has at least one practical implemented path or a
   committed design with command-level acceptance criteria.
8. The roadmap and README are updated to reflect the new completed work and
   remaining intentional limits.
9. Regression gates pass:
   ```bash
   npm ci
   npm run build:css
   go test -v -race ./...
   make test-functional
   ```
10. Telemetry regression checks pass for changed telemetry-adjacent code:
    ```bash
    go test ./internal/audit ./internal/telemetry ./internal/server ./internal/web/...
    ```
11. Each milestone has a focused commit, and this checklist is updated before
    or in the same commit.

## Milestone Checklist

Update this checklist as the goal loop progresses. When a milestone is
complete:

1. Run the milestone's verification commands.
2. Run any listed regression commands.
3. Update the milestone status from `[ ]` to `[x]`.
4. Commit the milestone with a focused commit message.
5. Record the commit hash in the goal-loop status before continuing.

- [x] Milestone 0: Baseline audit and compatibility matrix.
- [x] Milestone 1: Stable generated IDs for users and groups.
- [x] Milestone 2: Pocket ID compatibility test and integration recipe.
- [x] Milestone 3: Integration recipe pack for common LDAP consumers.
- [x] Milestone 4: Correct LDAP Compare behavior.
- [x] Milestone 5: Service-account/read-only bind strategy.
- [x] Milestone 6: TLS/LDAPS deployment path.
- [ ] Milestone 7: Bootstrap/import/export path.
- [ ] Milestone 8: Final docs, roadmap update, full regression, and release
  readiness summary.

## Milestone 0: Baseline Audit And Compatibility Matrix

Problem:

- The product priorities come from a comparison against lldap and app docs, but
  the repo needs a durable compatibility matrix that can guide future work.

Desired behavior:

- Create a concise matrix that lists common LDAP consumers and the exact LDAP
  expectations LDAPLite must satisfy.
- Include Pocket ID, Authelia, Dex, Gitea/Forgejo, Grafana, Nextcloud, and
  Vaultwarden.
- Capture expected attributes, bind pattern, user filter, group filter, group
  membership mode, TLS assumptions, and known LDAPLite status.
- Mark observability as completed, not a remaining gap.

Acceptance criteria:

- A new document exists under `docs/`, for example
  `docs/CLIENT_COMPATIBILITY_MATRIX.md`.
- Each client entry separates confirmed behavior from assumptions.
- The matrix links to integration recipe files once they exist.
- No code behavior changes are made in this milestone unless needed to run
  discovery tests.
- Checklist is updated and committed.

Verification:

```bash
go test ./...
```

## Milestone 1: Stable Generated IDs For Users And Groups

Problem:

- Clients such as Pocket ID often expect durable unique IDs for users and
  groups. Pocket ID examples use `uuid`; many LDAP clients also understand
  `entryUUID`.
- LDAPLite can store arbitrary attributes, but stable generated IDs should not
  depend on manual operator input.

Desired behavior:

- New entries receive stable generated UUID attributes where appropriate.
- Existing entries are backfilled by migration.
- `entryUUID` is treated as server-managed and read-only.
- `uuid` is available as a compatibility alias if that is the chosen product
  direction.
- IDs remain stable across Modify operations and restarts.
- IDs are returned in search results when requested, and optionally under `*`
  if that matches the schema decision.

Acceptance criteria:

- Migrations add durable IDs without breaking existing databases.
- Add, Modify, Search, and schema discovery behavior are tested.
- Functional tests assert stable IDs for user and group searches.
- Attempts to client-modify server-managed ID attributes return an appropriate
  LDAP result code.
- README and schema docs explain the attributes and client usage.
- Checklist is updated and committed.

Likely files:

- `internal/store/migrations/`
- `internal/store/sqlite_entries.go`
- `internal/server/write.go`
- `internal/server/search.go`
- `internal/server/discovery.go`
- `internal/models/`
- `tests/functional/`
- `README.md`

Verification:

```bash
go test ./internal/models ./internal/store ./internal/server
make test-functional
```

Regression focus:

- `userPassword` must remain hidden.
- `objectClass`, timestamps, and `memberOf` protection must remain intact.
- Existing migrations must still apply from an empty database.

## Milestone 2: Pocket ID Compatibility Test And Integration Recipe

Problem:

- Pocket ID is a representative modern LDAP consumer. It expects a bind user,
  user/group search bases, user and group filters, stable unique attributes,
  and group membership attributes.

Desired behavior:

- LDAPLite has a documented Pocket ID recipe using the actual initialized
  directory layout.
- Functional tests cover the LDAP queries and attributes Pocket ID needs.
- Any unsupported Pocket ID expectation is explicit and justified.

Acceptance criteria:

- Add `docs/integrations/pocket-id.md`.
- Include exact LDAPLite environment variables, bind DN, user base DN, group
  base DN, filters, attribute mapping, group membership mapping, and TLS notes.
- Add a functional test that simulates Pocket ID's LDAP reads:
  - bind as an application/service user or admin until service accounts exist;
  - search users by configured filter;
  - search groups by configured filter;
  - read stable user/group IDs;
  - resolve group `member` values to user DNs;
  - optionally confirm `memberOf` works for users.
- Checklist is updated and committed.

Likely files:

- `docs/integrations/pocket-id.md`
- `tests/functional/`
- `README.md`

Verification:

```bash
make test-functional
go test ./internal/server ./internal/store
```

Regression focus:

- Attribute casing must remain client-compatible.
- Group `member` and computed `memberOf` must both continue to work.

## Milestone 3: Integration Recipe Pack For Common LDAP Consumers

Problem:

- lldap has a strong adoption advantage because it ships many integration
  examples. LDAPLite needs practical recipes, not just generic LDAP examples.

Desired behavior:

- Add focused integration recipes for at least four additional clients.
- Prefer clients whose LDAP behavior is simple and common in self-hosted
  deployments.

Recommended first set:

- Authelia
- Dex
- Gitea or Forgejo
- Grafana
- Nextcloud
- Vaultwarden

Acceptance criteria:

- At least four new recipe files exist under `docs/integrations/`.
- Each recipe includes:
  - LDAP URL and TLS/LDAPS note;
  - bind DN guidance;
  - user search base and filter;
  - group search base and filter;
  - attribute mapping;
  - group membership mapping;
  - known limitations;
  - a minimal LDAP smoke-test command.
- Recipes do not promise unsupported AD, SASL, Kerberos, paging, sorting, or
  schema-extension behavior.
- README links to the integration directory.
- Checklist is updated and committed.

Verification:

```bash
go test ./...
```

Regression focus:

- Documentation must match the actual default tree:
  `ou=users,<baseDN>` and `ou=groups,<baseDN>`.
- Do not create config examples that require unsupported LDAP controls.

## Milestone 4: Correct LDAP Compare Behavior

Problem:

- LDAP Compare currently exists but must return meaningful true/false/no-such
  object behavior for real clients.

Desired behavior:

- Compare checks the target entry and requested attribute value.
- Compare supports ordinary attributes, `objectClass`, and safe operational
  attributes where appropriate.
- Compare never exposes `userPassword` hashes or compares against raw stored
  password hashes in a way that leaks information.

Acceptance criteria:

- Compare returns compareTrue for matching ordinary attributes.
- Compare returns compareFalse for non-matching ordinary attributes.
- Compare returns noSuchObject for missing entries.
- Compare handles missing attributes predictably.
- Password compare semantics are explicitly chosen and tested. Prefer refusing
  or returning compareFalse for `userPassword` unless there is a strong LDAP
  compatibility reason to verify supplied plaintext through the password hasher.
- Functional tests cover the behavior through `github.com/go-ldap/ldap/v3`.
- Checklist is updated and committed.

Likely files:

- `internal/server/ldap.go`
- `internal/store/`
- `tests/functional/`

Verification:

```bash
go test ./internal/server ./internal/store
make test-functional
```

Regression focus:

- Bind behavior must remain unchanged.
- `userPassword` must not appear in Search responses.

## Milestone 5: Service-Account/Read-Only Bind Strategy

Problem:

- LDAPLite currently has coarse LDAP write authorization. App bind users should
  not need broad write access.

Desired behavior:

- Choose and implement a minimal authorization model, or create a precise
  design if implementation scope is too large for this goal loop.
- Prefer a pragmatic first step: an LDAPLite-managed read-only group or role
  that can bind and search but cannot Add/Modify/Delete.

Acceptance criteria for implementation:

- There is a documented way to create an application bind user with read-only
  LDAP access.
- Add/Modify/Delete reject read-only users with `insufficientAccessRights`.
- Admin or authorized writer behavior remains available.
- Web UI admin authorization remains separate and unchanged unless explicitly
  redesigned.
- Functional tests cover read-only bind search success and write rejection.
- Checklist is updated and committed.

Acceptance criteria for design-only fallback:

- A committed design document specifies:
  - role/group model;
  - default admin behavior;
  - migration path;
  - LDAP result codes;
  - Web UI implications;
  - tests required for implementation.
- The roadmap marks the implementation as a follow-up with concrete acceptance
  criteria.
- Checklist is updated and committed.

Likely files:

- `internal/server/ldap.go`
- `internal/server/write.go`
- `internal/store/`
- `internal/web/middleware/auth.go`
- `docs/`
- `tests/functional/`

Verification:

```bash
go test ./internal/server ./internal/store ./internal/web/...
make test-functional
```

Regression focus:

- Existing admin user must still be able to manage the directory.
- Anonymous bind behavior must remain consistent with configuration.

## Milestone 6: TLS/LDAPS Deployment Path

Problem:

- Many LDAP consumers document `ldaps://` or StartTLS. LDAPLite intentionally
  has no in-server TLS today, but operators need a tested path.

Desired behavior:

- Decide whether this milestone implements native LDAPS/StartTLS or documents a
  tested reverse-proxy/sidecar path.
- Keep the product simple. Do not add broad TLS complexity unless the current
  reverse-proxy stance cannot satisfy common clients.

Acceptance criteria for reverse-proxy/sidecar path:

- Add a tested recipe under `docs/integrations/` or `docs/deployment/`.
- Include a minimal local TLS wrapper example, certificate expectations, port
  mapping, and client URL examples.
- Include smoke-test commands using `ldapsearch` or an equivalent client.
- Update client recipes to point to this TLS guidance.
- Checklist is updated and committed.

Acceptance criteria for native implementation:

- Add configuration for LDAPS and/or StartTLS.
- Add tests for enabled and disabled TLS behavior.
- Preserve plain LDAP defaults.
- Document certificate configuration and security caveats.
- Checklist is updated and committed.

Verification:

```bash
go test ./...
make test-functional
```

Regression focus:

- Plain LDAP deployments must keep working.
- Telemetry and healthcheck behavior must remain clear with TLS enabled or with
  a sidecar.

## Milestone 7: Bootstrap/Import/Export Path

Problem:

- Operators need a practical way to seed users, groups, and app bind users
  without hand-clicking the Web UI or writing ad hoc LDAP commands.

Desired behavior:

- Implement one practical path first. Prefer LDIF import/export if it aligns
  best with LDAP operators; prefer CSV if that is simpler and clearly useful.
- Keep the command safe and repeatable.

Acceptance criteria for implementation:

- Add CLI command(s) or documented scripts for import/export.
- Imported users preserve password security rules.
- Imported groups enforce member referential integrity.
- Export does not leak `userPassword` hashes unless an explicit, safe admin
  backup mode is designed.
- Tests cover successful import/export and invalid input.
- Checklist is updated and committed.

Acceptance criteria for design-only fallback:

- A committed design document defines commands, formats, validation, password
  handling, and tests.
- Roadmap links to the design.
- Checklist is updated and committed.

Likely files:

- `cmd/ldaplite/`
- `internal/store/`
- `internal/models/`
- `docs/`

Verification:

```bash
go test ./cmd/ldaplite ./internal/store ./internal/models
make test-functional
```

Regression focus:

- No backup/export mode should accidentally expose credentials.
- Imported entries must obey base DN and parent DN rules.

## Milestone 8: Final Docs, Roadmap Update, Full Regression, And Release Readiness

Problem:

- Product work is only useful if the repo reflects the new supported state and
  remaining limits accurately.

Desired behavior:

- README, roadmap, integration docs, and compatibility matrix all agree.
- Completed milestones are marked done.
- Remaining limitations are explicit and not hidden.

Acceptance criteria:

- README links to the compatibility matrix and integration recipes.
- `docs/ROADMAP.md` reflects completed work and remaining follow-ups.
- The compatibility matrix marks tested versus untested clients honestly.
- `AGENTS.md` is updated only if agent operating guidance changed.
- Full regression commands pass:
  ```bash
  npm ci
  npm run build:css
  go test -v -race ./...
  make test-functional
  ```
- Final commit includes checklist updates and docs.
- Final goal-loop summary lists:
  - commits created;
  - files changed;
  - client compatibility now covered;
  - remaining known gaps;
  - exact commands run.

## Regression Policy

Run targeted tests after each milestone and full regression before final
completion. If a full regression command is too slow or fails for environmental
reasons, document the exact command, failure, and why it is not a product
regression.

Never skip these checks silently:

- password storage and `userPassword` search hiding;
- `member` and computed `memberOf` behavior;
- bind/search/write authorization;
- telemetry redaction and bounded labels;
- migrations from an empty database;
- functional compatibility suite.

## Commit Policy

Each checkpoint must end with a focused commit. Do not batch unrelated
milestones into one commit.

Recommended commit message shape:

```text
feat: add stable ldap entry ids
docs: add pocket id integration recipe
test: cover pocket id ldap expectations
fix: implement ldap compare
docs: document ldaps sidecar deployment
```

Before each commit:

1. Check `git status --short`.
2. Review the diff.
3. Run the milestone verification commands.
4. Update this file's checklist.
5. Commit only the intended files.

After each commit:

1. Record the commit hash in the goal-loop status.
2. Continue to the next milestone only if the checkpoint is coherent.

## Open Decisions

If the agent cannot make a reasonable choice from repo context, stop and ask
for alignment before implementing. In particular, ask before:

- choosing whether `uuid` should be a stored alias, a computed alias, or not
  supported in favor of `entryUUID`;
- implementing native TLS/StartTLS instead of documenting a sidecar/proxy path;
- changing LDAP write authorization semantics for existing authenticated users;
- exposing any password material in import/export;
- adding a new public API such as REST or SCIM inside this goal loop.
