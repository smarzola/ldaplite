# Goal: Replace goldap With LDAPLite-Owned BER And LDAP Protocol Handling

Use this prompt in a goal loop to remove LDAPLite's dependency on
`github.com/lor00x/goldap` and replace it with repo-owned LDAP message types,
BER decoding, and BER encoding.

## Goal

Replace `goldap` without regressing LDAP interoperability, security-sensitive
behavior, or release quality.

The end state is:

- no production or test imports of `github.com/lor00x/goldap/message`;
- no `github.com/lor00x/goldap` requirement in `go.mod` or `go.sum`;
- no protocol implementation that depends on reflection or `unsafe`;
- LDAP request decoding and response encoding are implemented in
  `internal/protocol` or a clearly named child package;
- black-box functional compatibility tests still pass through a real LDAP
  client library.

## Repository Rules

Follow `AGENTS.md`.

Important reminders:

- Use `ast-grep` for Go structural searches and refactors.
- Use `rg` for ordinary text and file searches.
- Keep changes scoped and reviewable.
- Do not revert unrelated user changes.
- Preserve the existing CI/release workflow shape.
- Prefer black-box functional tests for compatibility behavior.
- If tests expose a real protocol compatibility gap, fix LDAPLite rather than
  weakening the test.

## Working Loop

Work in small, committed steps.

For every step below:

1. Complete only the scoped work for that step.
2. Add or update focused tests before relying on manual verification.
3. Run the verification commands listed for the step.
4. Update this file by changing the step checkbox from `[ ]` to `[x]` and add a
   short status note under the step with the date, commit hash if available, and
   exact commands run.
5. Commit the code, tests, and status-note update before starting the next step.

Do not batch multiple unchecked steps into one large commit unless a later step
cannot compile without the earlier step in the same commit. If that happens,
state the reason in the status note.

## Definition Of Done

The goal is complete only when:

1. All milestone checkboxes in this file are marked `[x]`.
2. `rg "github.com/lor00x/goldap|goldap" .` finds no dependency references
   except historical changelog or explanatory docs intentionally left in place.
3. `go.mod` and `go.sum` no longer include `github.com/lor00x/goldap`.
4. The protocol layer has tests for normal messages, malformed BER, size/length
   handling, filters, and response encoding.
5. The AD-like functional suite passes:
   ```bash
   make test-functional
   ```
6. The normal test suite passes:
   ```bash
   make test
   ```
7. The final response summarizes:
   - commits made;
   - files changed;
   - goldap imports removed;
   - protocol tests added;
   - exact verification commands run.

## Current Protocol Boundary

Start by inspecting these files:

- `internal/protocol/transport.go`
- `internal/protocol/connection.go`
- `internal/protocol/response.go`
- `internal/protocol/extended_response.go`
- `internal/server/ldap.go`
- `internal/server/search.go`
- `internal/server/write.go`
- tests under `internal/protocol/`, `internal/server/`, and
  `tests/functional/`

Known starting problems:

- `goldap/message` leaks into protocol, server handlers, and tests.
- `internal/protocol/extended_response.go` uses `unsafe` because `goldap` does
  not expose an ExtendedResponse responseValue setter.
- `ReadLDAPMessage` already does custom BER framing around `goldap`.
- The existing functional suite is the main compatibility gate because it uses
  `github.com/go-ldap/ldap/v3` against a real LDAPLite subprocess.

## Milestones

### [ ] 1. Baseline And Protocol Inventory

Map every current `goldap` use and record the exact protocol surface LDAPLite
needs to preserve.

Required work:

- Search all production and test imports of `github.com/lor00x/goldap`.
- List request operations currently decoded:
  - Bind
  - Search
  - Add
  - Modify
  - Delete
  - Compare
  - Extended
  - Unbind
- List response operations currently encoded.
- Identify all filter forms currently expected by server/search tests and
  functional tests.
- Add or update protocol fixture tests that capture current behavior before
  replacing internals.

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/protocol ./internal/server
GOCACHE=/private/tmp/ldaplite-gocache go test -tags=functional -v ./tests/functional/...
```

Status note:

- Pending.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

### [ ] 2. Introduce LDAPLite-Owned Message Types

Create internal protocol types so server code no longer depends directly on
`goldap/message`.

Required work:

- Add repo-owned types for:
  - LDAP message ID;
  - result codes;
  - controls if currently needed;
  - bind request/response;
  - search request, result entry, and result done;
  - add, modify, delete, compare, extended, and unbind operations;
  - LDAP filters used by the current server.
- Keep the types small and purpose-built for LDAPLite.
- Avoid exporting protocol internals outside the packages that need them.
- Add conversion adapters from `goldap/message` to the new types.
- Keep `goldap` behind the adapter for now.

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/protocol
```

Status note:

- Pending.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

### [ ] 3. Move Server Handlers Off goldap Types

Change the server and protocol connection dispatch path to use LDAPLite-owned
types.

Required work:

- Update `protocol.OperationHandlers` to receive internal message/request
  types rather than `*message.LDAPMessage`.
- Update `internal/server` handlers to consume internal protocol types.
- Update response helpers to return internal response types.
- Keep the adapter layer as the only place that imports `goldap/message`.
- Update internal tests to construct LDAPLite-owned messages directly.

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/protocol ./internal/server
GOCACHE=/private/tmp/ldaplite-gocache go test -tags=functional -v ./tests/functional/...
```

Status note:

- Pending.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

### [ ] 4. Implement Minimal BER Encoder For Responses

Implement LDAP response encoding without `goldap`.

Required work:

- Add a BER writer for LDAPLite's needed subset:
  - sequence;
  - set;
  - integer;
  - enumerated;
  - boolean;
  - octet string;
  - null;
  - application-specific tags;
  - context-specific tags;
  - definite lengths only.
- Encode all current server responses with the new writer.
- Remove the `unsafe` ExtendedResponse responseValue hack.
- Add byte-level tests for representative responses:
  - bind success and invalid credentials;
  - search result entry;
  - search result done;
  - add/modify/delete/compare responses;
  - WhoAmI extended response.
- Keep request decoding on the adapter temporarily if needed.

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/protocol ./internal/server
GOCACHE=/private/tmp/ldaplite-gocache go test -tags=functional -v ./tests/functional/...
```

Status note:

- Pending.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

### [ ] 5. Implement BER Reader And LDAP Request Decoder

Decode LDAP requests without `goldap`.

Required work:

- Add a BER reader for LDAPLite's needed subset.
- Enforce sane limits for:
  - malformed lengths;
  - unsupported indefinite lengths;
  - truncated messages;
  - oversized messages;
  - unexpected tags;
  - invalid message structure.
- Preserve compatibility with clients that send non-canonical boolean true
  values where LDAPLite already tolerates them.
- Decode current request operations:
  - Bind
  - Search
  - Add
  - Modify
  - Delete
  - Compare
  - Extended
  - Unbind
- Decode current filter forms:
  - equality;
  - present;
  - and/or/not;
  - greater-or-equal;
  - less-or-equal;
  - approximate;
  - substrings.
- Add malformed-packet and partial-read tests.

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/protocol ./internal/server
GOCACHE=/private/tmp/ldaplite-gocache go test -tags=functional -v ./tests/functional/...
```

Status note:

- Pending.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

### [ ] 6. Remove goldap Dependency

Delete the temporary adapter and remove `goldap` from the module.

Required work:

- Remove all imports of `github.com/lor00x/goldap/message`.
- Remove `github.com/lor00x/goldap` from `go.mod`.
- Run `go mod tidy`.
- Confirm `go.sum` no longer includes `github.com/lor00x/goldap`.
- Search for leftover dependency references and remove any stale code comments.

Verification:

```bash
rg "github.com/lor00x/goldap|goldap" .
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/protocol ./internal/server
GOCACHE=/private/tmp/ldaplite-gocache go test -tags=functional -v ./tests/functional/...
```

Status note:

- Pending.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

### [ ] 7. Full Regression And Cleanup

Run the full repository verification and clean up the final implementation.

Required work:

- Run the normal build/test entrypoints.
- Review public docs and comments for stale protocol claims.
- Keep BER implementation comments focused on non-obvious LDAP/BER rules.
- Avoid broad product documentation unless behavior changed.
- Confirm no unrelated generated or dependency churn remains.

Verification:

```bash
npm ci
npm run build:css
make test
make test-functional
go vet ./...
gofmt -l .
```

Status note:

- Pending.

Commit requirement:

- Commit after marking this milestone done and adding the status note.

## Implementation Guidance

### Preferred Package Shape

Keep protocol ownership close to the existing boundary. A reasonable shape is:

```text
internal/protocol/
  ber/
    reader.go
    writer.go
    tags.go
    reader_test.go
    writer_test.go
  ldapmsg/
    message.go
    requests.go
    responses.go
    filters.go
  transport.go
  connection.go
  response.go
```

Adjust names if the existing code points to a simpler layout.

### BER Policy

Support only what LDAPLite needs:

- BER definite lengths.
- Short and long-form lengths.
- Primitive and constructed tags required by LDAP v3 messages.
- Application-specific LDAP operation tags.
- Context-specific tags used by authentication, filters, substrings, and
  extended operations.

Reject or explicitly do not implement:

- indefinite lengths;
- arbitrary ASN.1 types not used by LDAPLite;
- BER features that only matter for unsupported LDAP controls or SASL flows;
- silent best-effort decoding of malformed structure.

### Compatibility Policy

The replacement must preserve externally observable behavior unless a current
behavior is clearly invalid and the fix is covered by tests.

Keep these compatibility gates central:

```bash
make test-functional
```

and, when iterating quickly:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test -tags=functional -v ./tests/functional/...
```

The functional suite must remain black-box: start a real `ldaplite server`
subprocess, use a temporary SQLite database, and drive LDAP operations through
`github.com/go-ldap/ldap/v3`.

### Optimization Policy

Do not optimize before correctness is proven.

After the dependency is removed, add benchmarks only if protocol performance is
shown to matter. Candidate benchmark targets:

- request decode for bind and search;
- response encode for search entries;
- large multi-entry search response writes;
- allocation count for filter decoding.

Potential optimizations after correctness:

- reduce per-attribute allocations in search response encoding;
- stream large search result entries directly to a writer;
- reuse small buffers inside a connection where safe;
- avoid interface-heavy intermediate packet trees.

### Stop Conditions

Stop and document the blocker instead of forcing a risky change if:

- a required LDAP operation's BER shape is unclear;
- a functional test failure indicates an existing behavior decision, not a
  simple protocol bug;
- full replacement would require implementing unsupported LDAP features such as
  SASL, TLS/LDAPS termination, paging controls, or full schema semantics.

If stopping, update the relevant milestone status note with:

- what was completed;
- what failed;
- exact command output or error summary;
- the smallest proposed next step.
