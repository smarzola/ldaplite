# Goal: Make Directory Lookup And Administration The Web UI Product

You are working in `/Users/smarzola/ldaplite`.

Your objective is to turn LDAPLite's embedded Web UI from a capability demo into
a practical directory lookup and administration console. The product is not
"show all role capabilities on one page"; the product is "find directory
entries, inspect them, and take the right action for your role." Admin,
read-only, and password-only users must get distinct workflows, not the same
mixed page with hidden or inert controls.

Keep LDAPLite's single-binary distribution model. The Web UI remains a
build-time React/shadcn/Tailwind/Vite frontend embedded in the Go binary.
Server-side authorization remains authoritative for every API and direct route.

Track this work against the UX gaps exposed after
`docs/WEB_UI_AUTHZ_REDESIGN_GOAL_PROMPT.md`: fake navigation, all features on
one page, useless static tables, no search, no pagination, no entry detail
surface, poor row actions, and noisy implementation-oriented copy.

## Repository Rules

Follow `AGENTS.md`.

Important reminders:

- Use `ast-grep` for Go structural searches and refactors.
- Use `rg` for ordinary text/file searches.
- Do not revert unrelated user changes.
- Prefer existing repo patterns in `internal/web/`, `internal/directory/`,
  `internal/store/`, and `internal/web/frontend/`.
- Use shadcn/ui components where suitable. Do not hand-roll primitives that
  shadcn already provides, such as dialogs, sheets, menus, pagination, command
  pickers, buttons, inputs, alerts, badges, tables, and skeletons.
- Use the `frontend-design` skill before changing UI structure or copy.
- Use the `shadcn` skill before adding or composing new shadcn components.
- Use the in-app Browser skill for local UI verification, screenshots, and
  visual validation before completing UI milestones.
- Preserve the single-binary runtime model: frontend assets must be built before
  Go compilation and embedded under `internal/web/static/`.
- Server-side authorization is non-negotiable. Hiding controls is not a security
  boundary.
- Password invariants from `AGENTS.md` remain non-negotiable:
  `userPassword` must not be returned in searches or stored in generic
  attributes, and password writes must go through the existing password
  processing path.
- At the end of each milestone, run verification, mark the milestone done in
  this file, commit the completed milestone, and report the commit hash before
  continuing.

## Product Principles

- Navigation must navigate. Do not render buttons, tabs, or controls that do
  nothing.
- Directory lookup is the primary workflow for admin and read-only users.
- Admin actions should be available near the entry they affect, not buried in a
  disconnected form block.
- Read-only users should be able to search, browse, inspect, and copy useful
  values without seeing write affordances.
- Password-only users should land directly on account/password self-service and
  should not see the directory console.
- Copy should name what operators do and recognize: search, users, groups, OUs,
  attributes, members, reset password, copy DN. Avoid implementation phrases
  like "role-aware operations" and "capability rail" in primary UI chrome.
- Dense operational UI is good when it is organized. Avoid marketing-style
  hero copy, large decorative panels, and cards inside cards.

## Target State

By the end, the repo should have:

- A real app shell with working navigation for `Directory`, `Users`, `Groups`,
  `OUs`, `Admin`, and `Account` as appropriate for the authenticated role.
- Password-only users land on `Account` and never see directory/admin
  navigation.
- A search-first directory view with text search, type filter, page size,
  pagination, loading, empty, and error states.
- Search/list results with useful row actions:
  - all readable users: view details and copy DN;
  - admins: edit, reset password where applicable, manage group members, and
    delete where allowed.
- Entry detail as a first-class surface, preferably a shadcn `Sheet` on desktop
  and mobile-friendly full-height sheet/dialog on narrow viewports.
- Entry detail shows summary, DN, object class/type, attributes, group members,
  computed `memberOf`, and useful operational metadata where available.
- Admin create/edit/delete/password reset/member-management flows are focused
  dialogs, sheets, or pages tied to the selected entity.
- Group member management supports adding/removing members from a searchable
  picker or clearly structured text input, with validation feedback.
- Pagination is implemented server-side or cleanly client-side for the current
  small-directory scope, with an explicit path to server-side pagination if the
  current store API requires it.
- Direct API attempts that bypass hidden controls still return `403` for
  unauthorized users.
- Browser validation proves the final UI is usable on desktop and mobile for
  admin, read-only, and password-only users.

## Current State

As of the previous redesign checkpoint:

- `internal/web/frontend/src/App.tsx` renders a single page that mixes directory
  tables, account self-service, capability status, session details, and admin
  forms.
- The top `Directory`, `Account`, and `Admin` buttons are derived from role
  labels but do not navigate between real views.
- `DirectoryTables` renders static users/groups/OUs lists from `/api/directory`.
- Tables have no search, pagination, row detail actions, copy actions, or edit
  entry affordances.
- The API has `/api/session`, `/api/directory`, `/api/users`, `/api/groups`,
  `/api/ous`, `/api/account/password`, and `/api/users/password`.
- `/api/directory` currently returns all users, groups, and OUs as grouped
  summaries.
- Admin write endpoints exist and are capability-gated, but their UI is a set
  of disconnected forms.
- Legacy Go template routes still exist for users/groups/OUs, but the embedded
  React app is the desired user-facing console.
- `internal/web/server_test.go` covers role-aware session data, directory API
  denial, write API behavior, same-origin protection, and root redirect.
- The app builds through `npm run build` / `make build` and embeds assets into
  the Go binary.

## Definition Of Done

The goal is complete only when:

1. Admin, read-only, and password-only users land in distinct workflows that
   match their capabilities.
2. Every visible primary navigation control changes view or route and has a
   clear active state.
3. Directory lookup supports search, type filtering, pagination/page size, and
   useful empty/loading/error states.
4. Result rows expose useful actions: view details, copy DN, and authorized
   admin actions.
5. Entry details expose attributes, DN, type, memberships, and `memberOf`
   clearly without returning or displaying `userPassword`.
6. Admin create/edit/delete/reset/member-management flows are reachable from
   relevant views and detail surfaces.
7. Read-only users cannot see write actions and cannot reach write APIs by
   direct request.
8. Password-only users cannot see or reach directory views or APIs, but can
   change their own password.
9. UI copy is plain, operational, and not implementation-oriented.
10. The embedded single-binary build path still works through `make build`.
11. Browser visual validation covers desktop and mobile for admin, read-only,
    and password-only roles.
12. Milestone checkboxes in this file are marked `[x]` as work completes.
13. Each completed milestone has a focused commit.
14. Final verification commands pass or any unrelated/environmental failures
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

- [x] Milestone 0: UX baseline, IA, and component plan.
- [x] Milestone 1: Search/list API contract and tests.
- [x] Milestone 2: Real app shell and role-specific landing.
- [x] Milestone 3: Searchable paginated directory results.
- [x] Milestone 4: Entry detail surface and row actions.
- [ ] Milestone 5: Focused admin workflows.
- [ ] Milestone 6: Copy, accessibility, visual validation, docs, and final
      regression.

## Milestone 0: UX Baseline, IA, And Component Plan

Problem:

- The current UI proves authz but does not support real directory work.
- The next implementation pass needs a clear information architecture before
  adding more controls.

Desired behavior:

- Produce a compact repo-local UX plan in this file's status note or a small
  companion note.
- Use `frontend-design` to define the operator audience, job-to-be-done, layout
  concept, copy principles, and one restrained visual signature.
- Use `shadcn` to inspect current project/component state and identify the
  components to add or reuse for shell, search, detail, row actions, dialogs,
  and pagination.

Acceptance criteria:

- Current UI flaws are identified by concrete component/function names.
- Target IA is specified for admin, read-only, and password-only users.
- Component choices are listed, including shadcn components needed.
- The plan explicitly removes fake navigation and mixed all-in-one workflow.
- Browser baseline screenshots or observations are recorded from the current
  app before major changes.
- Milestone status is marked done in this file and committed.

Likely files:

- `docs/DIRECTORY_ADMIN_PRODUCT_GOAL_PROMPT.md`
- `internal/web/frontend/src/App.tsx`
- `internal/web/frontend/src/components/ui/`
- `components.json`

Verification:

```bash
npm run build
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

Status note, 2026-06-23:

- Current UI flaws, by concrete code surface:
  - `App` renders directory lookup, session state, account self-service,
    capability status, and admin forms in one scrolling page.
  - `buildNavItems` produces `Directory`, `Account`, and `Admin` labels, but
    the rendered buttons do not own route/view state. Browser baseline clicked
    all three and the URL and visible content did not change.
  - `DirectoryTables` and `EntryTable` render grouped static lists from
    `/api/directory`; there is no query input, type filter, pagination, detail
    surface, or per-row `View details` / `Copy DN` action.
  - `AdminPanel` / `AdminOperations` place create, edit, reset, and delete
    forms below the lookup surface instead of near the selected entry or a
    focused admin view.
  - `CapabilityRail` and `SessionCard` make implementation state prominent in
    the primary workflow. Useful for debugging, wrong as product chrome.
  - `handlers.APIHandler.Directory` returns one grouped all-entry dump, which
    is too blunt for search, pagination, and entry-detail contracts.
- Target information architecture:
  - Admin: `Directory` default landing; `Users`, `Groups`, and `OUs` scoped
    lookup/admin views; `Admin` for create/import-style operations; `Account`
    for own password. Admin write actions also appear as contextual row/detail
    actions.
  - Read-only: `Directory` default landing; `Users`, `Groups`, and `OUs`
    scoped lookup views; `Account` for own password when allowed. No create,
    edit, reset, member-management, or delete affordances.
  - Password-only: `Account` default and only primary workflow. No directory
    shell, no directory/admin navigation, and direct directory API access must
    stay denied server-side.
- Frontend-design direction:
  - Audience: operators responsible for a small-to-medium LDAPLite directory.
  - Job: find an entry, inspect its identity and relationships, and take the
    next permitted action without reading implementation vocabulary.
  - Layout concept:
    ```text
    Desktop:
    [sidebar nav] [search toolbar + result table] [detail sheet]

    Mobile:
    [top nav] [search/filter controls] [stacked results] -> [full-height detail sheet]
    ```
  - Visual signature: a restrained DN lineage rail for selected entries,
    showing the path from RDN to base DN so the LDAP hierarchy is tangible
    without decorative clutter.
  - Token direction: keep shadcn `radix-nova`, neutral base, Geist type, and
    semantic colors; use mono text for DNs and attribute names; avoid raw color
    utilities and one-note purple/blue/cream palettes.
  - Copy principles: use operator nouns and verbs such as `Search directory`,
    `View details`, `Copy DN`, `Reset password`, `Manage members`, and
    `Delete entry`. Remove primary UI phrases such as `Directory workbench`,
    `Capability rail`, `role-aware operations`, and explanatory capability
    prose.
- shadcn component plan:
  - Reuse installed: `alert`, `badge`, `button`, `card`, `empty`, `field`,
    `input`, `label`, `separator`, `skeleton`, `table`, `tabs`, and
    `textarea`.
  - Add as needed in implementation milestones: `sidebar` for real shell
    navigation, `sheet` for entry details, `dialog` and `alert-dialog` for
    focused mutations and confirmations, `dropdown-menu` for row actions,
    `command` for searchable member picking, `pagination` for page controls,
    `select` and `toggle-group` for filters, `tooltip` for icon-only actions,
    `sonner` for mutation feedback, and `input-group` for search controls.
  - Dry run result for the proposed set: 14 new files, 1 overwrite
    (`separator.tsx`), 4 identical skips, and dependencies `sonner`,
    `next-themes`, and `cmdk`.
- Browser baseline observations:
  - Ran against `http://admin:ChangeMe123!@127.0.0.1:18080/app/` with a
    temporary local server and fresh SQLite database.
  - Visible headings: `LDAPLite directory console`, `Directory workbench`,
    `Users`, `Groups`, `Organizational units`, `Capability rail`, `Session`,
    `Account`, and `Admin operations`.
  - Visible buttons included `Directory`, `Account`, `Admin`,
    `Change password`, `Users`, `Groups`, `OUs`, `Create user`, `Save user`,
    `Reset`, and `Delete user`.
  - The page had 5 forms, 18 inputs, 4 table rows, no search input, no
    pagination text, and mixed directory/account/session/admin panels on the
    same screen.
  - Clicking `Directory`, `Account`, and `Admin` each resolved to one button
    and completed, but did not change URL or visible text.
- Commands run:
  - `npx shadcn@latest info --json`
  - `npx shadcn@latest search @shadcn -q sidebar -t ui`
  - `npx shadcn@latest docs sidebar sheet dialog dropdown-menu command pagination select toggle-group sonner alert-dialog tooltip input-group`
  - `npx shadcn@latest add sidebar sheet dialog dropdown-menu command pagination select toggle-group sonner alert-dialog tooltip input-group --dry-run`
  - Temporary local server plus in-app Browser observation for current UI.
  - Verification: `npm run build`
- Commit hash: `89b2479`.

## Milestone 1: Search/List API Contract And Tests

Problem:

- `/api/directory` currently returns one full grouped dump. That is not a
  durable contract for search, pagination, row actions, and detail views.

Desired behavior:

- Add a clear read API for directory lookup that supports:
  - query text;
  - type filter: all, users, groups, OUs;
  - page size and page token/page number;
  - total count or enough metadata for pagination;
  - stable sort order;
  - summaries suitable for result rows.
- Add an entry detail API by DN that returns a safe detail payload with
  attributes, memberships, `memberOf`, and no `userPassword`.
- Preserve `/api/directory` compatibility only if it remains useful internally;
  otherwise update callers and tests deliberately.

Acceptance criteria:

- Read-only and admin users can call search/list and detail APIs.
- Password-only users receive `403` for directory search/list/detail APIs.
- Search filters work for common fields such as DN, uid, cn, mail, ou, and
  group cn using the existing store/search behavior where possible.
- Pagination returns deterministic results and metadata.
- Detail responses do not include `userPassword`.
- API tests cover allowed roles, denied roles, query, type filter, pagination,
  detail, and password redaction.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/web/handlers/api.go`
- `internal/web/server.go`
- `internal/web/server_test.go`
- `internal/store/`
- `internal/schema/`
- `internal/models/`

Verification:

```bash
go test -v ./internal/web ./internal/store/... ./internal/schema ./internal/models
go test -v -race ./internal/web
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

Status note, 2026-06-23:

- Added `GET /api/directory/search` behind `DirectoryRead`.
  - Query params: `q`, `type`, `page`, and `pageSize`.
  - Type values normalize to `all`, `users`, `groups`, and `ous`.
  - Page defaults are `page=1` and `pageSize=25`; page size is capped at 100.
  - Results are sorted deterministically by DN and return `total` plus
    `totalPages`.
  - Search uses existing store filtering for object class/type and then a small
    in-memory operator query over DN, RDN, object class, uid, cn, mail, ou,
    description, member, and memberOf.
- Added `GET /api/directory/entry?dn=...` behind `DirectoryRead`.
  - Detail responses include DN, type, object class, display name, summary
    fields, attributes, members, memberOf, and create/update timestamps.
  - Detail lookup rejects DNs outside the configured base and returns 404 for
    missing entries.
  - `userPassword` is redacted from both persisted and computed attributes.
- Kept `/api/directory` available for the current frontend until the UI moves
  to the new search/detail contract in later milestones.
- Added HTTP tests covering:
  - read-only search by common fields;
  - type filters for users, groups, and OUs;
  - deterministic pagination metadata/results;
  - safe detail response with `memberOf`;
  - password redaction from detail JSON;
  - password-only `403` for both search and detail APIs.
- Commands run:
  - `go test -v ./internal/web`
  - `go test -v ./internal/web ./internal/store/... ./internal/schema ./internal/models`
  - `go test -v -race ./internal/web`
- Commit hash: `2af2772`.

## Milestone 2: Real App Shell And Role-Specific Landing

Problem:

- The current top buttons look like navigation but do not navigate.
- Password-only users should not be inside a directory console at all.

Desired behavior:

- Replace fake role buttons with a real app shell and navigation state.
- Use URL state or a small internal router so `Directory`, `Users`, `Groups`,
  `OUs`, `Admin`, and `Account` are real views.
- Render nav items based on server-provided roles:
  - admin: directory/search, users, groups, OUs, admin/create, account;
  - read-only: directory/search, users, groups, OUs, account;
  - password-only: account only.
- Password-only users land directly on account/password view.
- Remove primary capability rail from the main workflow; if capabilities remain
  visible, make them secondary account/session diagnostics.

Acceptance criteria:

- Every visible nav item changes the current route/view and has an active
  state.
- Password-only users do not see directory/admin navigation.
- Read-only users do not see admin/create navigation.
- Page titles and copy describe the workflow, not implementation details.
- Existing auth/session tests still pass.
- Browser validation confirms role-specific landing for admin, read-only, and
  password-only users.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/web/frontend/src/App.tsx`
- `internal/web/frontend/src/components/`
- `internal/web/frontend/src/components/ui/`
- `internal/web/server_test.go`

Verification:

```bash
npm run build
go test -v ./internal/web
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

Status note, 2026-06-23:

- Replaced the inert top button row with a real URL-backed app shell.
  - Supported views: `directory`, `users`, `groups`, `ous`, `admin`, and
    `account`.
  - Navigation writes `?view=...` with same-document relative history updates
    so it works even when Basic Auth credentials appear in the loaded URL.
  - Active navigation uses `aria-current="page"` and the page title/copy is
    derived from the current workflow.
- Rendered role-specific navigation from `/api/session` roles.
  - Admin: `Directory`, `Users`, `Groups`, `OUs`, `Admin`, `Account`.
  - Read-only: `Directory`, `Users`, `Groups`, `OUs`, `Account`.
  - Password-only: `Account` only.
- Split the previous mixed page into distinct view bodies.
  - Directory view shows grouped directory browse tables.
  - `Users`, `Groups`, and `OUs` show scoped browse views.
  - `Admin` owns the create/edit/delete/reset forms.
  - `Account` owns password self-service and secondary session details.
- Removed the primary `Capability rail` and old `Directory workbench` product
  copy. Capability badges now appear only as secondary account/session detail.
- shadcn notes:
  - Reused installed shadcn primitives: `Button`, `Card`, `Badge`, `Alert`,
    `Empty`, `Field`, `Input`, `Separator`, `Skeleton`, `Table`, `Tabs`, and
    `Textarea`.
  - Checked `npx shadcn@latest add sidebar --dry-run`; it would add sidebar
    support but prompt to overwrite `separator.tsx`. Milestone 2 stayed scoped
    to existing primitives; richer sidebar/sheet components can be added in
    the search/detail milestones if still worthwhile.
- Browser validation:
  - Admin at `127.0.0.1`: landed on `Directory`; nav showed
    `Directory`, `Users`, `Groups`, `OUs`, `Admin`, `Account`; clicking
    `Users` changed URL to `/app/?view=users`, title to `Users`, and active
    nav to `Users`.
  - Read-only at `localhost`: direct `/app/?view=admin` normalized to
    `/app/?view=directory`; nav omitted `Admin`; clicking `Groups` changed URL
    to `/app/?view=groups`, title to `Groups`, and active nav to `Groups`.
  - Password-only at `0.0.0.0`: direct `/app/?view=directory` normalized to
    `/app/?view=account`; only `Account` nav was visible; title was `Account`;
    `Change password` was visible; directory/admin nav was absent.
  - Browser console errors were empty after the relative-history fix.
- Commands run:
  - `npx shadcn@latest info --json`
  - `npx shadcn@latest docs sidebar`
  - `npx shadcn@latest add sidebar --dry-run`
  - `npx shadcn@latest add sidebar --diff internal/web/frontend/src/components/ui/separator.tsx`
  - `npm run build`
  - `go test -v ./internal/web`
  - Temporary single-binary build and in-app Browser role validation.
- Commit hash: `f5919d1`.

## Milestone 3: Searchable Paginated Directory Results

Problem:

- Static grouped tables do not help operators find entries or act on them.

Desired behavior:

- Build a search-first directory view using the Milestone 1 API.
- Include:
  - search input;
  - type filter;
  - page size control;
  - pagination controls;
  - result count/range;
  - loading skeleton;
  - empty state;
  - clear error state with retry.
- Use dense but readable result rows with type badge, name, DN, summary, and
  actions.
- Mobile should use stacked rows; desktop may use a table or data-list layout.

Acceptance criteria:

- Searching by uid/cn/mail/DN/group/OU text produces expected results.
- Type filtering scopes results.
- Pagination controls move through result pages without losing filters.
- Results include row actions for view and copy DN for readable users.
- Admin-only row actions are hidden for read-only users.
- Loading/empty/error states are visually tested.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/web/frontend/src/App.tsx`
- `internal/web/frontend/src/components/`
- `internal/web/frontend/src/components/ui/`
- `internal/web/handlers/api.go`
- `internal/web/server_test.go`

Verification:

```bash
npm run build
go test -v ./internal/web
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

Status note, 2026-06-23:

- Replaced the static grouped directory tables with a search-first results
  surface backed by `GET /api/directory/search`.
  - `Directory`, `Users`, `Groups`, and `OUs` now share the same lookup
    component, with scoped views fixing the type filter where appropriate.
  - The unscoped `Directory` view supports text search, type filtering, page
    size selection, pagination, result counts/ranges, reset, loading skeletons,
    empty state, and retryable error state.
  - Result rows show type, name, DN, summary, and row actions. Readable users
    get `View` and `Copy DN`; admins also get the contextual `Admin` action.
  - Mobile uses stacked rows while desktop uses a dense table layout.
- Added shadcn components:
  - `select`
  - `pagination`
- Browser validation with a temporary single-binary server and mock data:
  - Admin search for `alice` returned the matching user and the group where
    Alice is a member.
  - Admin search by `alice@example.com`, `search-team`, and `engineering`
    covered mail, group, and OU text.
  - Type filter `Groups` scoped the `alice` query to `search-team` and removed
    the user row.
  - Pagination moved from `Page 1 of 2` / `Showing 1-10 of 18` to
    `Page 2 of 2` / `Showing 11-18 of 18`.
  - `View` selected the Alice row, and `Copy DN` placed
    `uid=alice,ou=users,dc=example,dc=com` on the browser clipboard.
  - Read-only user `reader` could search and use `View` / `Copy DN`, while
    `Admin` navigation and row actions were absent.
  - Empty search `zz-no-results` showed `No entries found` with guidance.
  - After stopping the backend, submitting another search showed
    `Search failed` with `Retry`.
  - Mobile viewport `390x844` showed stacked DN/action rows and the desktop
    table rendered at zero size.
  - Loading state uses the shadcn `Skeleton` branch in `ResultSkeleton`; browser
    attempts to pause the search request were blocked by the in-app browser
    runtime or settled directly to the error state, so the transient skeleton
    was verified by source/build rather than a captured visual frame.
- Commands run:
  - `npx shadcn@latest docs select pagination`
  - `npx shadcn@latest add select pagination --dry-run`
  - `npx shadcn@latest add select pagination`
  - `npm run build`
  - `go test -v ./internal/web`
  - `go build -o /private/tmp/ldaplite-m3 ./cmd/ldaplite`
  - Temporary single-binary server plus in-app Browser validation.
- Commit hash: `006ecb7`.

## Milestone 4: Entry Detail Surface And Row Actions

Problem:

- Operators cannot inspect entries, attributes, memberships, or computed
  relationships from the current UI.

Desired behavior:

- Add an entry detail surface opened from result rows.
- Prefer shadcn `Sheet` for detail on desktop and a mobile-friendly sheet or
  route on narrow screens.
- Detail should show:
  - type and name;
  - full DN with copy action;
  - parent DN;
  - object classes;
  - key attributes;
  - all safe generic attributes;
  - group members;
  - user `memberOf`;
  - create/modify timestamps or other operational data where available.
- Add copy actions for DN and attribute values.

Acceptance criteria:

- Details open from search results without losing search state.
- Detail API errors and missing/deleted entries show useful messages.
- `userPassword` is never displayed.
- Read-only details include copy actions but no write actions.
- Admin details include relevant edit/reset/delete/member actions.
- Keyboard focus enters and exits the detail surface correctly.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/web/frontend/src/App.tsx`
- `internal/web/frontend/src/components/`
- `internal/web/frontend/src/components/ui/sheet.tsx`
- `internal/web/handlers/api.go`
- `internal/web/server_test.go`

Verification:

```bash
npm run build
go test -v ./internal/web
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

Status note, 2026-06-23:

- Replaced the temporary selected-entry card with a shadcn `Sheet` detail
  surface opened from search result rows.
  - The sheet fetches `GET /api/directory/entry?dn=...` when opened, so search
    state and pagination stay intact behind the detail surface.
  - Detail loading, stale/missing-entry error, and retry states are handled in
    the sheet.
  - Details show type, name, full DN, parent DN, DN lineage, object classes,
    safe attributes, members, `memberOf`, and created/modified timestamps when
    present.
  - Copy actions are available for DN, parent DN, relationship values, and
    attribute values.
  - Read-only detail sheets expose lookup/copy actions only.
  - Admin detail sheets expose contextual shortcuts for `Edit entry`,
    `Reset password` on users, `Manage members` on groups, and `Delete entry`.
- Tightened `GET /api/directory/entry` to return `404` for missing entries and
  mapped that to useful UI copy for stale/deleted result rows.
- Added shadcn component:
  - `sheet`
- Browser validation with a temporary single-binary server and mock data:
  - Admin opened Alice from search results; focus entered the sheet, URL/search
    state remained on `/app/?view=directory`, and the sheet showed DN, parent
    DN, object class, attributes, `memberOf`, and no `userPassword`.
  - Copied Alice's `telephonenumber` attribute value to the browser clipboard.
  - Closed the sheet and verified Alice search/results remained in place.
  - Admin opened `search-team`; the sheet showed both member DNs and
    `Manage members`, while user-only `Reset password` was absent.
  - Read-only user `reader` opened Alice detail with `Copy DN` and `memberOf`
    visible, with no admin nav or write actions.
  - A deleted `ghost` result row opened a detail sheet showing
    `Could not load details`, the missing-entry message, and `Retry`.
  - Mobile viewport `390x844` opened a full-width sheet with attributes and
    copy actions visible and no page horizontal overflow.
- Commands run:
  - `npx shadcn@latest docs sheet`
  - `npx shadcn@latest add sheet --dry-run`
  - `npx shadcn@latest add sheet`
  - `npm run build`
  - `go test -v ./internal/web`
  - `go build -o /private/tmp/ldaplite-m4 ./cmd/ldaplite`
  - Temporary single-binary server plus in-app Browser validation.
- Commit hash: pending.

## Milestone 5: Focused Admin Workflows

Problem:

- Admin create/edit/delete/reset/member-management forms are currently detached
  from the entries they affect and mixed into one long page.

Desired behavior:

- Move admin operations into focused workflows:
  - create user/group/OU from appropriate view-level actions;
  - edit user/group/OU from detail or row actions;
  - reset password from user detail/row action;
  - delete from detail/row action with confirmation;
  - manage group members from group detail with add/remove controls.
- Use shadcn dialogs/sheets/dropdown menus/command pickers where appropriate.
- Preserve same-origin protection and server-side capability enforcement.

Acceptance criteria:

- Admin can create, edit, delete, reset password, and manage group membership
  from contextual UI.
- Read-only users cannot see these actions and direct API calls still return
  `403`.
- Password-only users can only change their own password.
- Forms have labels, validation feedback, success feedback, and destructive
  confirmations.
- Group member editing validates member DNs and refreshes detail/search data.
- Browser validation clicks through representative admin flows.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/web/frontend/src/App.tsx`
- `internal/web/frontend/src/components/`
- `internal/web/handlers/api.go`
- `internal/directory/`
- `internal/web/server_test.go`
- `pkg/crypto/`

Verification:

```bash
npm run build
go test -v ./internal/web ./internal/directory ./pkg/crypto
go test -tags=functional -v ./tests/functional/...
```

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 6: Copy, Accessibility, Visual Validation, Docs, And Final Regression

Problem:

- A UI can pass tests and still feel confusing, clipped, inaccessible, or too
  implementation-driven.

Desired behavior:

- Rewrite primary UI copy around operator workflows.
- Ensure every control has a visible purpose and accessible label.
- Validate keyboard focus, dialogs/sheets, pagination, forms, destructive
  actions, and mobile layouts.
- Update docs to describe the Web UI workflows, role behavior, and embedded
  frontend build model.

Acceptance criteria:

- No fake controls remain.
- Primary copy is plain and task-oriented.
- Desktop and mobile visual validation covers:
  - admin search, detail, create/edit/delete/reset/member management;
  - read-only search/detail/copy with no write actions;
  - password-only account/password page;
  - unauthorized direct route/API denial.
- Screenshots show no blank app, missing CSS, broken styling, clipped text,
  overlap, role leakage, unusable mobile layout, or misleading actions.
- Keyboard focus is visible and usable for navigation, result rows, detail
  surfaces, menus, dialogs, forms, pagination, and destructive confirmations.
- Docs explain the final Web UI workflows and roles.
- Milestone status is marked done in this file and committed.

Likely files:

- `README.md`
- `docs/LDAP_AUTHORIZATION.md`
- `docs/ROADMAP.md`
- `AGENTS.md`
- `internal/web/frontend/src/`
- `internal/web/`

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
at desktop and mobile widths for:

- admin user;
- read-only user;
- password-only user.

If local listener tests fail in this environment with errors like
`listen tcp 127.0.0.1:0: bind: operation not permitted`, rerun the
listener-heavy tests with the required permissions and document the rerun.

## Final Response Required

When complete, report:

- target state achieved or not achieved;
- UX/product behavior changed;
- single-binary distribution preserved or not preserved;
- commits made, with commit hashes;
- files changed;
- exact verification commands run and results;
- browser validation performed, including roles and viewport sizes;
- screenshot paths or browser evidence;
- known residual risks or follow-up issues.
