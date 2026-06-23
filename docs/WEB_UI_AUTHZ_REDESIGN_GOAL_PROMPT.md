# Goal: Redesign LDAPLite Web UI And Authorization

You are working in `/Users/smarzola/ldaplite`.

Your objective is to replace LDAPLite's current template/DaisyUI admin surface
with a modern, capability-aware embedded Web UI built with shadcn/ui, while
fixing the directory authorization model to follow least privilege. This is
allowed to be a breaking change: do not preserve the current behavior where
authenticated LDAP users can write unless they are placed in a read-only group.

Keep LDAPLite's distribution model intact. The released product must remain a
single Go binary with embedded Web UI assets. Node, Vite, shadcn/ui, Tailwind,
and related frontend tools are build-time dependencies only; operators must not
need a separate frontend server or Node runtime to run LDAPLite.

Track this work against the existing roadmap themes in `docs/ROADMAP.md`:
shared LDAP/Web UI authorization, Web UI password flows, and Web UI templates.

## Repository Rules

Follow `AGENTS.md`.

Important reminders:

- Use `ast-grep` for Go structural searches.
- Use `rg` for ordinary text/file searches.
- Do not revert unrelated user changes.
- This goal intentionally allows breaking authorization semantics.
- Do not add a separate CI or release workflow unless the existing
  `.github/workflows/test.yml` and `.github/workflows/release.yml` cannot
  support the change.
- Preserve the single-binary runtime model: frontend assets must be built before
  Go compilation and embedded into the Go binary.
- Keep `tests/functional/` black-box with a real LDAPLite server subprocess and
  a real LDAP client library.
- Password handling invariants from `AGENTS.md` are non-negotiable:
  `userPassword` must not be stored as a generic attribute, searches must not
  return it, and password writes must go through `ProcessPassword()`.
- Use the installed `frontend-design` skill for the visual/product design pass.
- Use the installed `shadcn` skill for shadcn/ui setup, component selection,
  component docs, and component composition.
- Use the in-app Browser skill for local UI verification, screenshots, and
  visual validation before completing UI milestones.
- At the end of each milestone, run verification, mark the milestone done in
  this file, add a status note, commit the completed milestone, and report the
  commit hash before continuing.

## Skill Requirements

Before changing UI code:

1. Read `frontend-design` and make a compact design plan for LDAPLite's admin
   surface: audience, job-to-be-done, palette, typography, layout concept, and
   one justified signature interaction or visual detail.
2. Read `shadcn` and run the project-aware shadcn commands from inside this
   repository. Use `npx shadcn@latest info`, `search`, `docs`, and `add
   --dry-run` before adding components.
3. Do not hand-roll components when shadcn/ui has a suitable component. Compose
   from shadcn components and use semantic tokens rather than raw color utility
   classes.
4. Read the Browser skill before browser validation. Use the in-app Browser
   against the local LDAPLite Web UI, capture screenshots, and inspect the UI at
   desktop and mobile widths.

## Target State

LDAPLite should have a modern embedded Web UI and a single authorization model
shared by LDAP operations, Web UI API/actions, and future HTTP provisioning
surfaces.

By the end, the repo should have:

- Default authenticated directory users can bind, search, compare, and change
  their own password, but cannot create/update/delete arbitrary entries.
- Members of `cn=ldaplite.admin,ou=groups,<baseDN>` can administer users,
  groups, OUs, attributes, memberships, and password reset flows.
- Read-only users can log into the Web UI and inspect directory data without
  seeing or reaching write actions.
- Password-only users can access only account/password functionality unless
  they also have broader roles.
- Authorization is enforced server-side through a shared capability layer, not
  only by hiding Web UI controls.
- The Web UI is implemented with shadcn/ui over a build-time frontend toolchain
  and embedded into the Go binary.
- The UI has role-aware navigation, clear empty/loading/error states, accessible
  forms, and polished responsive layouts.
- Browser-based visual validation covers admin, read-only, and password-only
  experiences on desktop and mobile.

## Current State

LDAPLite currently has two mismatched authorization models:

- LDAP write authorization is inverted from least privilege. Authenticated
  non-anonymous users can write unless they are members of
  `cn=ldaplite.readonly,ou=groups,<baseDN>`.
- Web UI authorization requires `cn=ldaplite.admin,ou=groups,<baseDN>` for the
  protected UI, so normal authenticated users can write through LDAP but cannot
  do anything in the UI.
- `docs/LDAP_AUTHORIZATION.md` documents the current coarse read-only service
  account behavior and explicitly says Web UI authorization is separate.
- `internal/server/ldap.go` contains `canWrite`, which checks only the reserved
  read-only group.
- `internal/web/middleware/auth.go` authenticates HTTP Basic credentials and
  separately checks admin group membership.
- `internal/web/server.go` serves Go templates and embedded CSS from
  `internal/web/templates` and `internal/web/static/output.css`.
- `package.json` currently builds Tailwind/DaisyUI CSS only.
- Existing Web UI tests cover authentication, same-origin protection, and
  representative handler behavior, but there is no browser visual test gate.

## Authorization Model

Implement a small capability model instead of a full LDAP ACL engine.

Required capabilities:

- `directory.read`: bind/search/compare normal directory entries.
- `directory.write`: create/update/delete directory entries.
- `directory.manageGroups`: create/update/delete groups and group membership.
- `password.changeSelf`: change the authenticated user's own password.
- `password.resetAny`: reset another user's password.
- `ui.read`: access read-only Web UI views.
- `ui.admin`: access full administrative Web UI actions.

Required built-in groups:

- `cn=ldaplite.admin,ou=groups,<baseDN>` grants all capabilities.
- `cn=ldaplite.readonly,ou=groups,<baseDN>` grants explicit read-only service
  account semantics. Because read-only is now the default for authenticated
  non-admin users, this group is mostly documentary and useful for integrations.
- `cn=ldaplite.password,ou=groups,<baseDN>` grants password-only UI access if
  the implementation chooses to restrict self-service password changes by group.

Default policy:

- Unbound users can read only RootDSE and schema discovery.
- Anonymous normal search remains controlled by `LDAP_ALLOW_ANONYMOUS_BIND`.
- Authenticated non-anonymous users receive `directory.read`, `ui.read`, and
  `password.changeSelf`.
- Only admin/write-authorized users receive `directory.write`.
- Only admin/password-reset-authorized users receive `password.resetAny`.
- LDAP Add, Modify, and Delete must return LDAP `insufficientAccessRights`
  (`50`) when the actor lacks the required capability.

## Frontend Architecture

The exact layout can change during implementation, but keep this shape unless
repo inspection reveals a better local fit:

- Put frontend source under `internal/web/frontend/`.
- Build static frontend assets into an embedded directory under
  `internal/web/static/`.
- Replace or retire the old Go template routes once equivalent React/shadcn
  surfaces exist.
- Keep API handlers in Go under `internal/web/` or a clearly named subpackage.
- Use same-origin protections and server-side authorization for mutating API
  calls.
- Keep `make build` as the single command that builds frontend assets and then
  builds `bin/ldaplite`.
- Keep release artifacts as Go binaries and Docker images produced by the
  existing tag-driven release workflow.

Do not introduce a runtime dependency on a separate Node process, reverse proxy,
or external asset server.

## UX Scope

The redesigned UI should include:

- Sign-in/authenticated shell using the existing LDAP credentials path unless a
  better local pattern is implemented.
- Directory overview with users, groups, and OUs.
- Search/filter controls that are useful for small-to-medium directories.
- User list, user detail, create/edit/delete flows for authorized users.
- Group list, group detail, member editing, and nested membership visibility
  where practical.
- OU list and create/delete/edit flows for authorized users.
- Account page for self-service password change.
- Admin password reset flow for authorized users.
- Role-aware navigation and action controls.
- Empty states, error states, loading states, validation feedback, and success
  feedback.
- Keyboard-visible focus, accessible labels, dialogs with titles, and usable
  mobile layouts.

## Definition Of Done

The goal is complete only when:

1. LDAP write authorization is least-privilege by default.
2. LDAP Add, Modify, and Delete deny non-admin/non-write-authorized users with
   LDAP `insufficientAccessRights` (`50`).
3. Existing admin users can still administer the directory through LDAP and the
   Web UI.
4. A shared authorization/capability layer is used by LDAP and Web UI code.
5. Web UI read-only, password-only, and admin experiences are distinct and
   enforced server-side.
6. The Web UI is built with shadcn/ui and embedded into the Go binary.
7. `make build` produces a working single binary containing the built UI.
8. `make test`, `make test-functional`, and relevant frontend checks pass.
9. Browser visual validation is performed with the in-app Browser at desktop
   and mobile widths for admin, read-only, and password-only flows.
10. Browser screenshots show no blank app, broken layout, illegible text,
    overlapping controls, unreachable primary actions, or role leakage.
11. Documentation explains the new breaking authorization model and UI role
    behavior.
12. Existing integration docs that mention read-only service accounts are
    updated if their language becomes misleading.
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

- [x] Milestone 0: Baseline audit and design direction.
- [ ] Milestone 1: Shared capability authorization layer.
- [ ] Milestone 2: Breaking least-privilege LDAP write policy.
- [ ] Milestone 3: Embedded shadcn frontend build foundation.
- [ ] Milestone 4: Role-aware Web UI API and shell.
- [ ] Milestone 5: Directory management and password flows.
- [ ] Milestone 6: Browser visual validation, documentation, and final
      regression.

## Milestone 0: Baseline Audit And Design Direction

Problem:

- The current Web UI and LDAP authorization model disagree about who can write.
- The implementation will touch security-sensitive behavior, build tooling, and
  user-facing UI. The next agent needs a precise baseline before changing it.

Desired behavior:

- Produce a short implementation note in this file's status section or a small
  companion note that captures the current authz paths, Web UI routes, frontend
  build path, and chosen UI design direction.
- Use `frontend-design` to choose a UI direction tailored to LDAPLite as an
  operational directory tool, not a generic SaaS landing page.
- Use `shadcn` to inspect project state and select initial components.

Acceptance criteria:

- Current LDAP/Web UI authorization paths are identified by symbol and file.
- Current build/embed flow is identified.
- Chosen frontend architecture preserves single-binary runtime distribution.
- Design direction includes palette, typography, layout, and one justified
  signature interaction/detail.
- Initial shadcn component candidates are listed with the docs consulted.
- Milestone status is marked done in this file and committed.

Likely files:

- `docs/WEB_UI_AUTHZ_REDESIGN_GOAL_PROMPT.md`
- `AGENTS.md`
- `docs/ROADMAP.md`
- `docs/LDAP_AUTHORIZATION.md`
- `internal/server/ldap.go`
- `internal/web/`
- `package.json`
- `Makefile`

Verification:

```bash
rg -n "canWrite|RequireAuth|ldaplite.admin|ldaplite.readonly|Web UI|build:css" internal docs package.json Makefile
npx shadcn@latest info
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

Status note, 2026-06-23:

- Commands run:
  - `rg -n "canWrite|RequireAuth|ldaplite.admin|ldaplite.readonly|Web UI|build:css" internal docs package.json Makefile`
  - `npx shadcn@latest info`
  - `npx shadcn@latest docs sidebar table form dialog sheet alert empty skeleton badge button input select dropdown-menu tabs sonner`
  - `npx shadcn@latest search @shadcn -q "sidebar"`
- Current LDAP authorization path:
  - `internal/server/ldap.go` has `canWrite`, which denies unbound and
    anonymous writes, then checks only `cn=ldaplite.readonly,ou=groups,<baseDN>`
    through `store.IsUserInGroup`; authenticated non-read-only users can write.
  - `internal/server/write.go` calls `canWrite` from Add, Delete, and Modify and
    returns LDAP `insufficientAccessRights` when it is false.
  - `internal/server/authz_test.go` currently expects authenticated writes to be
    allowed and read-only group writes to be denied.
- Current Web UI authorization path:
  - `internal/web/middleware/auth.go` authenticates HTTP Basic credentials
    against LDAP user passwords, finds `ldaplite.admin`, and requires nested
    membership through `store.IsUserInGroup`.
  - `internal/web/server.go` applies `RequireAuth` to list routes and
    `RequireAuth` plus `RequireSameOrigin` to mutating routes.
  - Web UI tests cover admin success, missing/invalid credentials, non-admin
    denial, malformed Basic auth, and same-origin rejection.
- Current build/embed flow:
  - `package.json` only defines `build:css` and `watch:css` around Tailwind v4
    CLI output to `internal/web/static/output.css`.
  - `Makefile` target `build-css` runs `npm run build:css`; target `build`
    depends on `build-css` before compiling `bin/ldaplite`.
  - `internal/web/server.go` embeds `templates/*.html` and
    `static/output.css`, so the current runtime is already single-binary after
    build.
- shadcn discovery:
  - `npx shadcn@latest info` reports framework `Manual`, Tailwind `v4`,
    CSS file `internal/web/static/input.css`, no TypeScript, no import alias,
    no `components.json`, and no installed components.
  - Docs consulted for `sidebar`, `table`, `dialog`, `sheet`, `alert`, `empty`,
    `skeleton`, `badge`, `button`, `input`, `select`, `dropdown-menu`, `tabs`,
    and `sonner`; the CLI reported no documentation links for `form`.
  - Registry search found `@shadcn/sidebar`, sidebar blocks, and
    `@shadcn/dashboard-01` as useful references for the admin shell.
- Initial component candidates:
  - Shell/navigation: `sidebar`, `dropdown-menu`, `tabs`, `sheet`.
  - Directory data: `table`, `badge`, `skeleton`, `empty`.
  - Forms/actions: `button`, `input`, `select`, `dialog`, `alert`.
  - Feedback: `sonner`, `alert`, `skeleton`.
- Design direction:
  - Subject: a compact operational directory console for people administering a
    small-to-medium LDAP directory.
  - Audience: operators and app owners who need to inspect trust boundaries,
    memberships, and write authority quickly.
  - Job: make access state visible before mutation; the UI should answer "who
    can do what here?" before it asks for edits.
  - Palette: `directory-ink #16201f`, `console-paper #f6f7f2`,
    `schema-mint #7dd3b0`, `replica-blue #5b7cfa`, `warning-amber #d99a2b`,
    `deny-red #bf4d4d`.
  - Type: use a restrained humanist sans for body/UI, a narrow utility face for
    DN/filter/code-like strings, and a modest display treatment only for section
    headers.
  - Layout: persistent left sidebar with role/capability badges, dense table
    center pane, detail/edit drawer on the right, account/password surface as a
    separate low-distraction workspace.
  - Signature detail: a "capability rail" beside each detail view showing read,
    write, group-management, self-password, and reset-password state as compact
    badges sourced from server capabilities. This makes authorization visible
    without turning the app into a decorative dashboard.
  - Self-critique: avoid a marketing hero, gradient-heavy landing page, or
    overly cardy SaaS dashboard; this should feel like a careful directory
    workbench.
- Commit hash: pending checkpoint commit.

## Milestone 1: Shared Capability Authorization Layer

Problem:

- LDAP and Web UI authorization are currently separate, making it easy for one
  surface to allow behavior that another surface denies.
- Future REST/SCIM work will repeat the same problem unless authorization is
  centralized first.

Desired behavior:

- Add a shared internal capability package or service used by LDAP and Web UI
  code.
- Resolve effective capabilities from the authenticated actor DN and nested
  group membership.
- Keep the capability model intentionally small and auditable.

Acceptance criteria:

- Shared code can answer whether an actor has each required capability.
- Admin membership grants full capabilities.
- Authenticated non-admin users receive read and self-password capabilities by
  default.
- Read-only service account group semantics remain understandable under the new
  default-read model.
- Unit tests cover admin, normal authenticated user, explicit read-only user,
  password-only user if implemented, anonymous user, unbound user, nested group
  membership, and membership lookup errors.
- No password, password hash, authorization header, or `userPassword` value is
  logged.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/authz/`
- `internal/server/authz_test.go`
- `internal/server/ldap.go`
- `internal/web/middleware/auth.go`
- `internal/store/store.go`
- `internal/store/sqlite_membership.go`
- `docs/LDAP_AUTHORIZATION.md`

Verification:

```bash
go test -v ./internal/authz ./internal/server ./internal/web/...
go test -v ./internal/store/...
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 2: Breaking Least-Privilege LDAP Write Policy

Problem:

- Current LDAP write behavior is backwards for least privilege: authenticated
  users can write unless explicitly downgraded.
- This surprises operators and conflicts with the intended Web UI roles.

Desired behavior:

- Invert LDAP write authorization so non-admin/non-write-authorized users cannot
  Add, Modify, or Delete.
- Preserve authenticated read/search/compare behavior.
- Preserve public RootDSE and schema discovery.
- Preserve configured anonymous search behavior.
- Return meaningful LDAP result codes, especially `insufficientAccessRights`
  for write denial.

Acceptance criteria:

- `canWrite` or its replacement grants write only through explicit capability.
- Functional tests prove:
  - admin can add/modify/delete;
  - normal authenticated user can search/compare but cannot add/modify/delete;
  - read-only service account can search/compare but cannot add/modify/delete;
  - self password change works only through the intended path;
  - unauthorized write attempts return LDAP result code `50`.
- Existing client compatibility expectations are updated where the old
  read-only-service-account explanation is now incomplete.
- Documentation clearly calls this a breaking authorization change.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/server/ldap.go`
- `internal/server/write.go`
- `internal/server/authz_test.go`
- `tests/functional/ad_compat_test.go`
- `docs/LDAP_AUTHORIZATION.md`
- `docs/CLIENT_COMPATIBILITY_MATRIX.md`
- `docs/integrations/*.md`
- `docs/ROADMAP.md`

Verification:

```bash
go test -v ./internal/server
go test -tags=functional -v ./tests/functional/...
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 3: Embedded shadcn Frontend Build Foundation

Problem:

- The current Web UI is server-rendered Go templates with DaisyUI classes and a
  Tailwind-only CSS build.
- shadcn/ui expects source components in the frontend project, but LDAPLite must
  still ship as a single Go binary.

Desired behavior:

- Add a frontend project layout that supports React, Vite, Tailwind, shadcn/ui,
  and TypeScript if selected.
- Build static assets into a Go-embedded location.
- Keep `make build` and CI/release flows aligned with the single-binary model.

Acceptance criteria:

- `npx shadcn@latest info` works from the project context after initialization.
- shadcn components are added through the CLI after consulting docs and
  dry-runs.
- `npm run build` or the chosen script builds the Web UI assets.
- `make build` builds frontend assets and then `bin/ldaplite`.
- The Go server serves the built frontend from embedded assets.
- The old Go template/DaisyUI path is removed or isolated so there is no
  confusing dual UI.
- CI commands in `.github/workflows/test.yml` and `release.yml` still represent
  the required build/test gate.
- Milestone status is marked done in this file and committed.

Likely files:

- `package.json`
- `package-lock.json`
- `components.json`
- `internal/web/frontend/`
- `internal/web/static/`
- `internal/web/server.go`
- `internal/web/server_test.go`
- `Makefile`
- `.github/workflows/test.yml`
- `.github/workflows/release.yml`

Verification:

```bash
npm ci
npx shadcn@latest info
npm run build
make build
go test -v ./internal/web/...
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 4: Role-Aware Web UI API And Shell

Problem:

- The current Web UI authorizes all protected routes as admin-only.
- A role-aware UI needs an API that returns the current actor and capabilities,
  and server-side enforcement for each action.

Desired behavior:

- Add Web API endpoints for current session/capabilities and directory reads.
- Add a shadcn-based app shell with role-aware navigation.
- Read-only users can view directory data but cannot reach mutating endpoints.
- Password-only users see only account/password functionality unless they also
  have broader capabilities.
- Admin users see full administrative navigation and actions.

Acceptance criteria:

- API endpoints enforce authorization through the shared capability layer.
- UI navigation and action visibility are derived from server-provided
  capabilities.
- Direct HTTP calls to unauthorized write endpoints are denied even when hidden
  UI controls are bypassed.
- Web UI auth, authorization denial, and write attempts continue to be audited.
- Unit/handler tests cover admin, read-only, password-only, and normal user
  access to representative endpoints.
- The shell has accessible navigation, loading states, error states, and empty
  states.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/web/server.go`
- `internal/web/middleware/auth.go`
- `internal/web/handlers/`
- `internal/web/frontend/src/`
- `internal/audit/`
- `internal/web/middleware/audit.go`
- `internal/web/server_test.go`
- `internal/web/middleware/auth_test.go`

Verification:

```bash
npm run build
go test -v ./internal/web/...
go test -v ./internal/server
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 5: Directory Management And Password Flows

Problem:

- The redesigned UI must be feature-rich enough to replace the existing CRUD
  templates and add the missing password-oriented workflows.
- Password and group-membership changes are security-sensitive and must not
  bypass existing model/store invariants.

Desired behavior:

- Implement user, group, OU, membership, attribute, self-password-change, and
  admin-password-reset workflows through the shared API and shadcn UI.
- Keep forms accessible, validated, and explicit about destructive operations.
- Use shadcn form, dialog, alert, table, badge, tabs/sidebar, dropdown, toast,
  skeleton, and empty-state components where appropriate.

Acceptance criteria:

- Admin can create, edit, and delete users, groups, and OUs.
- Admin can edit group membership with validation that members exist.
- Admin can reset another user's password without exposing hashes.
- Authenticated users can change their own password through the allowed path.
- Read-only users cannot mutate directory data through UI or direct API calls.
- Password-only users cannot access directory administration.
- LDAP searches still do not return `userPassword`.
- Tests cover successful and denied password flows, group membership edits, and
  protected attribute behavior.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/web/handlers/`
- `internal/web/frontend/src/`
- `internal/server/write.go`
- `internal/models/`
- `internal/store/`
- `pkg/crypto/`
- `internal/web/handlers/*_test.go`
- `tests/functional/`

Verification:

```bash
npm run build
go test -v ./internal/web/... ./internal/server ./internal/store/... ./pkg/crypto/...
go test -tags=functional -v ./tests/functional/...
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 6: Browser Visual Validation, Documentation, And Final Regression

Problem:

- A modern UI rewrite can compile and still be visually broken, inaccessible,
  or role-confusing.
- Operators need documentation for the breaking authorization model and the new
  Web UI behavior.

Desired behavior:

- Run LDAPLite locally with built embedded assets.
- Use the in-app Browser skill to validate real UI behavior and screenshots.
- Validate admin, read-only, and password-only experiences at desktop and mobile
  widths.
- Update docs and roadmap material to match the new authorization model and UI.

Acceptance criteria:

- Browser validation covers:
  - sign-in/authenticated landing route;
  - admin directory management flow;
  - read-only user directory view with no effective write actions;
  - password-only/self-service password flow;
  - unauthorized direct navigation/API denial behavior;
  - mobile layout;
  - desktop layout.
- Screenshots show no blank app, missing CSS, broken component styling,
  illegible text, overlapping controls, role leakage, or unusable mobile layout.
- Keyboard focus is visible for primary navigation and form controls.
- Reduced-motion and loading/error states are acceptable where applicable.
- Docs explain:
  - breaking default-write removal;
  - built-in capability groups;
  - Web UI role behavior;
  - single-binary distribution and frontend build behavior.
- Final regression passes or environment-specific failures are documented with
  exact evidence.
- Milestone status is marked done in this file and committed.

Likely files:

- `docs/LDAP_AUTHORIZATION.md`
- `docs/ROADMAP.md`
- `docs/CLIENT_COMPATIBILITY_MATRIX.md`
- `docs/integrations/*.md`
- `README.md`
- `AGENTS.md`
- `internal/web/frontend/`
- `internal/web/`
- `.github/workflows/test.yml`
- `.github/workflows/release.yml`

Verification:

```bash
npm ci
npm run build
make build
go test -v -race ./...
go test -tags=functional -v ./tests/functional/...
go vet ./...
test -z "$(gofmt -l .)"
```

Browser validation:

```text
1. Start LDAPLite locally from the built binary with a temporary SQLite database.
2. Open the embedded Web UI in the in-app Browser.
3. Capture desktop screenshots for admin, read-only, and password-only users.
4. Resize to a mobile viewport and capture the same role surfaces.
5. Click through representative create/edit/delete/password flows.
6. Confirm hidden controls are also denied by direct URL/API attempts.
7. Record screenshot filenames or browser-observed evidence in this milestone's
   status note.
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Final Verification

Before the goal is complete, run:

```bash
npm ci
npm run build
make build
go test -v -race ./...
go test -tags=functional -v ./tests/functional/...
go vet ./...
test -z "$(gofmt -l .)"
```

Also perform final in-app Browser validation against the built embedded Web UI
at desktop and mobile widths for admin, read-only, and password-only users.

If local listener tests fail in this environment with errors like
`listen tcp 127.0.0.1:0: bind: operation not permitted`, rerun the listener-heavy
tests with the required permissions and document the rerun.

## Final Response Required

When complete, report:

- target state achieved or not achieved;
- breaking authorization behavior changed;
- single-binary distribution preserved or not preserved;
- commits made, with commit hashes;
- files changed;
- exact verification commands run and results;
- browser validation performed, including roles and viewport sizes;
- known residual risks or follow-up issues.
