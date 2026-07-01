# Goal: Prepare Documentation Architecture For 1.0.0

You are working in `/Users/smarzola/projects/ldaplite`.

Your objective is to reorganize LDAPLite documentation so public `docs/` files
are operator-facing and release-ready, while implementation prompts, design
history, audits, and goal-loop working files live under `docs/internal/`.
Treat this as a 1.0 preparation goal, not as the actual `v1.0.0` release cut.
Do not tag or publish `v1.0.0` unless the user explicitly asks after this work
lands.

## Repository Rules

Follow `AGENTS.md`.

Important reminders:

- Inspect current source, tests, docs, and workflows directly before editing.
- Keep repository `AGENTS.md` project-specific and short; do not copy personal
  Codex behavior rules into it.
- Keep broad product documentation in `README.md` or public `docs/`.
- Keep internal implementation prompts and historical planning material out of
  the public documentation path.
- Do not rely on stale line numbers in docs or comments. Prefer current source,
  symbol names, tests, and direct document inspection.
- Do not revert unrelated user changes.
- Do not add or duplicate CI/release workflows unless the existing workflows
  cannot support the change.
- Use the existing release model: source default versions live in
  `cmd/ldaplite/main.go`, `Makefile`, `package.json`, and `package-lock.json`,
  but release artifacts are tag-driven.
- At the end of each milestone, run verification, mark the milestone done in
  this file, add a status note, commit the completed milestone, and report the
  commit hash before continuing.

## Scope Controls

In scope:

- Move goal-loop prompts, implementation plans, historical audits, and product
  summaries into `docs/internal/`.
- Rename public documentation to a consistent lower-case kebab-case style.
- Refresh public documentation so README links lead to durable user-facing
  material, not internal summaries.
- Update stale claims in public docs, especially around shipped SCIM, LDIF,
  telemetry, TLS, compatibility, and roadmap status.
- Add a small public docs index if it makes navigation clearer.
- Update `AGENTS.md` only for project-specific docs conventions and source-map
  changes.
- Add or update `CHANGELOG.md` with the documentation architecture change.
- Add final release-readiness notes for a later `v1.0.0` cut.

Out of scope unless the user explicitly expands this goal:

- Cutting, tagging, or publishing `v1.0.0`.
- New LDAP, SCIM, Web UI, storage, authz, telemetry, or release automation
  features.
- Rewriting every integration recipe beyond fixing links, stale claims, and
  naming consistency.
- Creating a docs website, static-site generator, or second documentation build
  pipeline.
- Removing useful historical design material just because it is no longer
  public-facing.

## Target State

By the end, LDAPLite documentation should have a clear audience boundary:

- Root files:
  - `README.md`: product overview, quick start, supported surfaces, and links
    to stable public docs.
  - `QUICKSTART.md`: either refreshed as the shortest runnable guide or folded
    into README and removed.
  - `CHANGELOG.md`: release history.
  - `AGENTS.md`: short project-specific agent guide.
- Public `docs/`:
  - `docs/README.md` or equivalent index for operator docs.
  - `docs/roadmap.md`.
  - `docs/authorization.md`.
  - `docs/scim.md`.
  - `docs/import-export.md`.
  - `docs/telemetry.md`.
  - `docs/client-compatibility.md`.
  - `docs/deployment/ldaps-tls-sidecar.md`.
  - `docs/integrations/*.md`.
- Internal `docs/internal/`:
  - `docs/internal/prompts/*` for goal-loop prompts.
  - `docs/internal/design-history/*` for implementation plans and protocol
    inventories.
  - `docs/internal/audits/*` for audit and simplification trackers.
  - `docs/internal/summaries/*` for completed goal summaries.

Public docs should no longer tell users to read goal summaries or pending
implementation designs as the source of product truth. Internal docs should
remain durable enough for future agents to understand why major decisions were
made.

## Current State

Current public `docs/` mixes several document classes:

- Operator-facing docs:
  - `docs/scim.md`
  - `docs/telemetry.md`
  - `docs/authorization.md`
  - `docs/client-compatibility.md`
  - `docs/deployment/ldaps-tls-sidecar.md`
  - `docs/integrations/*.md`
- Goal-loop prompts:
  - `docs/*_GOAL_PROMPT.md`
- Historical plans and design records:
  - `docs/internal/design-history/import-export-design.md`
  - `docs/internal/design-history/goldap-protocol-inventory.md`
  - `docs/internal/design-history/sqlite-improvement-plan.md`
  - `docs/internal/design-history/timestamp-attributes-plan.md`
- Audit and summary records:
  - `docs/internal/audits/codebase-simplification-findings.md`
  - `docs/internal/summaries/client-compatibility-product-summary.md`
  - `docs/internal/audits/improvement-goal-prompt.md`

Known stale public-doc issues:

- `README.md` links to `docs/internal/summaries/client-compatibility-product-summary.md` as a
  current roadmap/supporting doc.
- `README.md` still points roadmap readers at LDIF import/export as planned
  work, even though `v0.16.0` shipped LDIF import/export.
- `docs/roadmap.md` still references issue `#8` and issue `#11` as follow-up
  work even though both are closed in GitHub.
- `docs/client-compatibility.md` still says LDIF import/export
  implementation is pending.
- `QUICKSTART.md` has older Docker Compose and bind-DN examples that must be
  checked against current product behavior before it remains public.
- `CLAUDE.md` duplicates or conflicts with `AGENTS.md` and should not remain a
  second stale agent guide unless it is reduced to a pointer.

Known constraints:

- Moving files will break relative links unless they are updated deliberately.
- Direct `go test ./...` requires embedded Web UI assets to already exist.
  Prefer `make test` on a fresh checkout if code tests are needed.
- This is primarily documentation work, but link and packaging references are
  release-sensitive because README is the first release surface.

## Definition Of Done

The goal is complete only when:

1. Public `docs/` contains only user-facing, operator-facing, deployment,
   integration, compatibility, roadmap, and reference documentation.
2. Goal-loop prompts are under `docs/internal/prompts/`.
3. Historical implementation plans and protocol/design records are under
   `docs/internal/design-history/`.
4. Audits, closure trackers, and completed product summaries are under
   `docs/internal/audits/` or `docs/internal/summaries/`.
5. Public documentation filenames use a consistent lower-case kebab-case style
   unless an existing external convention requires otherwise.
6. README links point only to current public docs or intentionally stable root
   files.
7. All moved-file links are updated, including links inside internal docs.
8. Public docs no longer describe shipped features as pending.
9. `docs/roadmap.md` reflects post-`v0.16.0` reality and does not list closed
   issues as active follow-ups.
10. `QUICKSTART.md` is either refreshed and linked correctly or removed after
    its useful content is folded into README/public docs.
11. `CLAUDE.md` is removed, or reduced to a short pointer to `AGENTS.md`, if it
    remains useful for compatibility with Claude Code.
12. `AGENTS.md` source-map and docs guidance match the new layout without
    becoming generic or personal.
13. `CHANGELOG.md` records the docs IA cleanup.
14. A link audit finds no references to removed or moved paths.
15. Milestone checkboxes in this file are marked `[x]` as work completes.
16. Each completed milestone has a focused commit.
17. Final verification passes, or unrelated failures are documented with
    evidence.

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

- [x] Milestone 0: Inventory, Classification, And Link Map
- [x] Milestone 1: Create Internal Documentation Layout And Move Historical Files
- [x] Milestone 2: Normalize Public Documentation Names And Links
- [x] Milestone 3: Refresh Public Product Truth
- [ ] Milestone 4: Root Documentation And Agent Guide Cleanup
- [ ] Milestone 5: Final Link Audit, Regression Checks, And 1.0 Release Notes

## Milestone 0: Inventory, Classification, And Link Map

Problem:

- The repo has enough documentation that moving files without a link map will
  create broken references and hide stale public claims.

Desired behavior:

- Before moving files, establish the exact current documentation inventory,
  classify each file by audience, and identify all path references that must be
  updated.

Acceptance criteria:

- Produce a short classification note in this file under status notes or in a
  temporary working note that is folded into the next commit.
- Classify every root and `docs/` markdown file as public, internal prompt,
  internal design history, internal audit, internal summary, or removal
  candidate.
- Identify README and public docs links that must change.
- Do not move files in this milestone unless needed to create this prompt's
  parent directory.
- Mark this milestone done and commit the prompt/status update.

Likely files:

- `docs/internal/prompts/docs-ia-1-0-goal-prompt.md`
- `README.md`
- `QUICKSTART.md`
- `CLAUDE.md`
- `AGENTS.md`
- `docs/**/*.md`

Verification:

```bash
find . -maxdepth 3 -name '*.md' -type f | sort
find docs -maxdepth 4 -type f | sort
```

Status notes:

- 2026-07-02: Completed the initial inventory and classification pass.
  Verification commands run:
  `find . -maxdepth 3 -name '*.md' -type f | sort` and
  `find docs -maxdepth 4 -type f | sort`. The root-level inventory command also
  reported vendored `node_modules` markdown because `node_modules/` is present
  in this checkout; the project documentation classification below is scoped to
  root repository docs and `docs/`.

  Classification:
  - Public root docs: `README.md`, `CHANGELOG.md`.
  - Public root doc to refresh or fold into README: `QUICKSTART.md`.
  - Project agent guide: `AGENTS.md`.
  - Removal or pointer candidate: `CLAUDE.md`.
  - Public operator docs:
    `docs/client-compatibility.md`,
    `docs/authorization.md`, `docs/roadmap.md`, `docs/scim.md`,
    `docs/telemetry.md`, `docs/deployment/ldaps-tls-sidecar.md`, and
    `docs/integrations/*.md`.
  - Internal prompts: `docs/internal/prompts/ad-compat-functional-goal-prompt.md`,
    `docs/internal/prompts/client-compatibility-product-goal-prompt.md`,
    `docs/internal/prompts/directory-admin-product-goal-prompt.md`,
    `docs/internal/prompts/goldap-replacement-goal-prompt.md`,
    `docs/internal/prompts/ldif-import-export-goal-prompt.md`,
    `docs/internal/prompts/memberof-performance-goal-prompt.md`,
    `docs/internal/prompts/scim-provisioning-api-goal-prompt.md`,
    `docs/internal/prompts/telemetry-goal-prompt.md`,
    `docs/internal/prompts/web-ui-authz-redesign-goal-prompt.md`, and this prompt.
  - Internal design history: `docs/internal/design-history/goldap-protocol-inventory.md`,
    `docs/internal/design-history/import-export-design.md`, `docs/internal/design-history/sqlite-improvement-plan.md`, and
    `docs/internal/design-history/timestamp-attributes-plan.md`.
  - Internal audits: `docs/internal/audits/codebase-simplification-findings.md` and
    `docs/internal/audits/improvement-goal-prompt.md`.
  - Internal summaries: `docs/internal/summaries/client-compatibility-product-summary.md`.

  Link map to handle in later milestones:
  - `README.md` links to uppercase public docs paths and internal summary/design
    files.
  - Public and internal docs link to `docs/roadmap.md`,
    `docs/telemetry.md`, `docs/authorization.md`,
    `docs/client-compatibility.md`, and
    `docs/internal/design-history/import-export-design.md`.
  - Integration docs use relative links to `../deployment/ldaps-tls-sidecar.md`;
    those should remain valid.
  - `AGENTS.md` names `docs/roadmap.md` in the source map and should be updated
    after the public docs rename.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 1: Create Internal Documentation Layout And Move Historical Files

Problem:

- Internal prompts and historical records currently sit beside user-facing
  docs, which makes the public documentation surface look unfinished.

Desired behavior:

- Internal documentation lives under `docs/internal/` with stable subfolders
  that future agents can use without guessing.

Acceptance criteria:

- Create `docs/internal/prompts/`, `docs/internal/design-history/`,
  `docs/internal/audits/`, and `docs/internal/summaries/`.
- Move all `*_GOAL_PROMPT.md` files into `docs/internal/prompts/`, including
  this prompt if the chosen path changes during implementation.
- Move implementation plans and protocol inventories into
  `docs/internal/design-history/`.
- Move audit/closure tracker material into `docs/internal/audits/`.
- Move completed product summaries into `docs/internal/summaries/`.
- Keep file content changes minimal in this milestone except for path/link
  updates required by the move.
- Mark this milestone done, add a status note, and commit.

Likely files:

- `docs/*_GOAL_PROMPT.md`
- `docs/internal/design-history/import-export-design.md`
- `docs/internal/design-history/goldap-protocol-inventory.md`
- `docs/internal/design-history/sqlite-improvement-plan.md`
- `docs/internal/design-history/timestamp-attributes-plan.md`
- `docs/internal/audits/codebase-simplification-findings.md`
- `docs/internal/summaries/client-compatibility-product-summary.md`
- `docs/internal/**`

Verification:

```bash
find docs -maxdepth 4 -type f | sort
test -z "$(find docs -maxdepth 1 -type f -name '*GOAL_PROMPT.md' -print)"
```

Status notes:

- 2026-07-02: Created the internal documentation layout and moved historical
  implementation material out of public `docs/`. Verification commands run:
  `find docs -maxdepth 4 -type f | sort` and
  `test -z "$(find docs -maxdepth 1 -type f -name '*GOAL_PROMPT.md' -print)"`.
  Also ran a moved-path search with `rg` and updated moved-file references in
  README and internal docs so they point at `docs/internal/prompts/`,
  `docs/internal/design-history/`, `docs/internal/audits/`, or
  `docs/internal/summaries/`.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 2: Normalize Public Documentation Names And Links

Problem:

- Public docs currently use mixed uppercase names. Moving to a stable public
  docs surface is easier if filenames are predictable and URLs are readable.

Desired behavior:

- User-facing docs use lower-case kebab-case names, and all local links resolve
  to the new paths.

Acceptance criteria:

- Rename:
  - `docs/roadmap.md` to `docs/roadmap.md`.
  - `docs/scim.md` to `docs/scim.md`.
  - `docs/telemetry.md` to `docs/telemetry.md`.
  - `docs/authorization.md` to `docs/authorization.md`.
  - `docs/client-compatibility.md` to
    `docs/client-compatibility.md`.
- Decide whether `docs/import-export.md` should be created from the public
  portions of `docs/internal/design-history/import-export-design.md` or whether
  README's LDIF section is enough. Prefer a public `docs/import-export.md` for
  1.0 readiness.
- Update links in README, public docs, internal docs, and `AGENTS.md`.
- Add `docs/README.md` if it improves navigation and avoids overloading
  README.
- Mark this milestone done, add a status note, and commit.

Likely files:

- `README.md`
- `AGENTS.md`
- `docs/*.md`
- `docs/deployment/*.md`
- `docs/integrations/*.md`
- `docs/internal/**/*.md`

Verification:

```bash
find docs -maxdepth 2 -type f | sort
grep -R "docs/[A-Z_][A-Z_]*\\.md" -n README.md docs AGENTS.md || true
```

Status notes:

- 2026-07-02: Renamed public operator docs to lower-case kebab-case, added
  `docs/import-export.md`, added `docs/README.md`, and updated README,
  `AGENTS.md`, public docs, and internal docs to point at the new public paths.
  Verification commands run:
  `find docs -maxdepth 2 -type f | sort` and
  `grep -R "docs/[A-Z_][A-Z_]*\\.md" -n README.md docs AGENTS.md || true`.
  The grep output only reported conventional `docs/README.md` references,
  which are intentional and allowed by the target layout.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 3: Refresh Public Product Truth

Problem:

- Several public docs still describe completed work as pending or point users
  at internal planning summaries.

Desired behavior:

- Public docs describe the current released product accurately after `v0.16.0`
  and are credible as the basis for a later `v1.0.0` release.

Acceptance criteria:

- Update `docs/roadmap.md` to remove closed issue references from active
  follow-ups and state the next real product bets.
- Update `docs/client-compatibility.md` so LDIF import/export is no longer
  listed as pending.
- Ensure `docs/scim.md`, `docs/import-export.md`, `docs/telemetry.md`, and
  `docs/authorization.md` contain operator-facing descriptions, not milestone
  history.
- Remove README links to internal summaries and internal designs.
- Keep intentional limitations visible and current.
- Mark this milestone done, add a status note, and commit.

Likely files:

- `README.md`
- `docs/README.md`
- `docs/roadmap.md`
- `docs/client-compatibility.md`
- `docs/scim.md`
- `docs/import-export.md`
- `docs/telemetry.md`
- `docs/authorization.md`

Verification:

```bash
grep -R "implementation is pending\\|planned work\\|PRODUCT_SUMMARY\\|GOAL_PROMPT\\|IMPORT_EXPORT_DESIGN" -n README.md docs/*.md docs/integrations docs/deployment || true
```

Status notes:

- 2026-07-02: Refreshed public product truth after `v0.16.0`. Verified with
  GitHub that there are no open issues and that issue `#8` and issue `#11` are
  closed. Updated `docs/roadmap.md` to remove stale active follow-up issue
  references and frame future work as candidates. Updated
  `docs/client-compatibility.md` so LDIF import/export is no longer listed as
  pending. Updated README roadmap links to public docs. Verification command
  run:
  `grep -R "implementation is pending\\|planned work\\|PRODUCT_SUMMARY\\|GOAL_PROMPT\\|IMPORT_EXPORT_DESIGN" -n README.md docs/*.md docs/integrations docs/deployment || true`;
  it returned no stale public-doc matches.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 4: Root Documentation And Agent Guide Cleanup

Problem:

- Root-level docs are part of the first impression for users and agents.
  Stale root docs make the repository look pre-1.0 even when the product
  surface is stronger.

Desired behavior:

- Root docs are current, concise, and non-duplicative.

Acceptance criteria:

- Decide whether `QUICKSTART.md` remains. If it remains, update it for current
  Docker, binary, Web UI, admin DN, and LDIF examples. If it is removed, fold
  any useful unique content into README or public docs.
- Remove `CLAUDE.md`, or reduce it to a short pointer to `AGENTS.md` if keeping
  it is necessary for external tooling compatibility.
- Update `AGENTS.md` source map to point at the new public docs and internal
  prompt location.
- Keep `AGENTS.md` short and project-specific.
- Mark this milestone done, add a status note, and commit.

Likely files:

- `README.md`
- `QUICKSTART.md`
- `CLAUDE.md`
- `AGENTS.md`
- `docs/README.md`

Verification:

```bash
find . -maxdepth 2 -name '*.md' -type f | sort
grep -R "CLAUDE.md\\|CLIENT_COMPATIBILITY_PRODUCT_SUMMARY\\|docs/roadmap.md\\|docs/scim.md" -n README.md AGENTS.md QUICKSTART.md docs || true
```

Status notes:

- Pending.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Milestone 5: Final Link Audit, Regression Checks, And 1.0 Release Notes

Problem:

- Documentation moves can silently break links, and a 1.0 preparation branch
  needs a clear handoff that distinguishes completed docs work from the actual
  release cut.

Desired behavior:

- The branch is ready for PR review, with verified links, updated changelog,
  and a short note describing what remains before `v1.0.0`.

Acceptance criteria:

- Add a `CHANGELOG.md` entry under `Unreleased` for the documentation
  architecture cleanup.
- Run a local link/path audit and fix all broken moved-path references.
- Run documentation-sensitive tests or the full repo test command if any
  non-doc files changed in a way that could affect build/test behavior.
- Confirm no public docs link to `docs/internal/` unless explicitly explaining
  contributor/internal material.
- Confirm no root README link points at moved uppercase docs paths.
- Confirm no open GitHub issue references in public roadmap are stale; use
  GitHub state when available.
- Do not tag `v1.0.0`; instead write final notes saying the docs IA branch is
  ready as a prerequisite.
- Mark this milestone done, add a status note, and commit.

Likely files:

- `CHANGELOG.md`
- `README.md`
- `AGENTS.md`
- `docs/**/*.md`

Verification:

```bash
find . -maxdepth 4 -name '*.md' -type f | sort
grep -R "docs/[A-Z_][A-Z_]*\\.md\\|GOAL_PROMPT\\|PRODUCT_SUMMARY\\|IMPORT_EXPORT_DESIGN" -n README.md docs/*.md docs/deployment docs/integrations AGENTS.md || true
make test
make test-functional
```

If only markdown files changed and the user wants a faster docs-only pass, it is
acceptable to replace the full test commands with a documented rationale and a
complete link/path audit. Run full tests if `AGENTS.md`, packaging references,
embedded assets, commands, release metadata, or code changed.

Status notes:

- Pending.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Final Response Requirements

When the goal is complete, report:

- The final documentation layout.
- The files moved, renamed, removed, and materially rewritten.
- The verification commands run and their results.
- The milestone commits made, with hashes.
- Any public documentation links intentionally left pointing to internal docs.
- Whether the branch is ready for PR review.
- Whether the repo appears ready for a separate `v1.0.0` release branch/tag,
  and any residual risks before cutting that release.
