# Goal: Improve LDAPLite memberOf Search Performance

Use this prompt in a goal loop to improve `memberOf` and narrow LDAP search
performance with benchmark evidence.

## Goal

Make LDAPLite's common search paths scale more predictably for small-to-medium
directories while keeping `memberOf` optional, computed, and read-only.

The first target is to reduce unnecessary work in:

- exact indexed attribute searches such as `(uid=user-000000)`;
- `memberOf=<groupDN>` filters;
- broad `memberOf` projection allocation pressure.

Do not persist `memberOf` as a generic LDAP attribute. It should remain derived
from `group_members`.

## Current Benchmark Command

Run the checked-in scale benchmark matrix with:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test -run '^$' -bench='BenchmarkMemberOf' -benchmem -benchtime=1x ./internal/store
```

Use `-benchtime=1x` for the full matrix because the 10k-entry fixtures are
explicit scale probes and should not be rebuilt repeatedly during Go benchmark
calibration.

## Current Baseline

Baseline from 2026-06-22 on Apple M1:

```text
BenchmarkMemberOfSkipProjection/users=1000               2.61 ms/op, 13 KB/op, 205 allocs/op
BenchmarkMemberOfSkipProjection/users=10000             46.6 ms/op, 13 KB/op, 198 allocs/op
BenchmarkMemberOfSingleResultProjection/users=1000       3.22 ms/op, 27 KB/op, 324 allocs/op
BenchmarkMemberOfSingleResultProjection/users=10000     48.1 ms/op, 27 KB/op, 335 allocs/op
BenchmarkMemberOfAllUsersProjection/users=1000          15.1 ms/op, 3.4 MB/op, 70k allocs/op
BenchmarkMemberOfAllUsersProjection/users=10000          129 ms/op, 25.7 MB/op, 516k allocs/op
BenchmarkMemberOfDirectFilter/users=1000                23.8 ms/op, 4.5 MB/op, 83k allocs/op
BenchmarkMemberOfDirectFilter/users=10000                218 ms/op, 26.9 MB/op, 547k allocs/op
BenchmarkMemberOfNestedProjection/depth=50              0.82 ms/op, 31 KB/op, 655 allocs/op
BenchmarkMemberOfNestedFilter/depth=50                  27.1 ms/op, 4.8 MB/op, 92k allocs/op
```

Interpretation:

- Single-result `memberOf` projection adds little over the current narrow-search
  baseline.
- Exact narrow searches are still too sensitive to directory size.
- `memberOf` filters are the expensive path because they currently require
  computed attribute population before in-memory matching.
- Broad projections are allocation-heavy and should be treated separately from
  narrow lookup work.

## Latest Result

After the first optimization pass on 2026-06-22, the same command produced:

```text
BenchmarkMemberOfSkipProjection/users=1000              0.40 ms/op, 12 KB/op, 257 allocs/op
BenchmarkMemberOfSkipProjection/users=10000             0.46 ms/op, 12 KB/op, 257 allocs/op
BenchmarkMemberOfSingleResultProjection/users=1000      0.73 ms/op, 27 KB/op, 405 allocs/op
BenchmarkMemberOfSingleResultProjection/users=10000     0.72 ms/op, 26 KB/op, 395 allocs/op
BenchmarkMemberOfAllUsersProjection/users=1000          15.1 ms/op, 3.2 MB/op, 70k allocs/op
BenchmarkMemberOfAllUsersProjection/users=10000          128 ms/op, 25.2 MB/op, 516k allocs/op
BenchmarkMemberOfDirectFilter/users=1000                0.85 ms/op, 79 KB/op, 2.8k allocs/op
BenchmarkMemberOfDirectFilter/users=10000               1.91 ms/op, 79 KB/op, 2.8k allocs/op
BenchmarkMemberOfNestedProjection/depth=50              0.83 ms/op, 32 KB/op, 657 allocs/op
BenchmarkMemberOfNestedFilter/depth=50                  1.01 ms/op, 79 KB/op, 2.9k allocs/op
```

Interpretation:

- Exact indexed attribute searches now avoid loading broad candidate sets before
  finding the matching entry.
- `memberOf=<groupDN>` filters now anchor from the target group and recursively
  walk `group_members` to users, preserving the optional projection contract.
- Broad projection is still dominated by JSON attribute hydration, but the 10k
  path is slightly lower in both memory and allocation count.
- The next meaningful broad-projection loop should challenge the JSON aggregate
  scan itself rather than shaving more map/slice setup around it.

## Priority Work

### 1. Optimize Exact Attribute Searches

Problem:

- `(uid=user-000000)` against 10k users is still tens of milliseconds even when
  `IncludeMemberOf` is false.
- This suggests the query path is scanning or aggregating too broadly before
  narrowing to the exact entry.

Desired behavior:

- Equality filters on indexed attributes such as `uid`, `cn`, `mail`, and
  `objectClass` should reduce candidate entries before attribute aggregation.
- Search semantics must remain case-insensitive where LDAP expects them.

Acceptance criteria:

- Add or update query-plan tests proving the narrow path uses the intended
  indexes.
- Benchmark shows `BenchmarkMemberOfSkipProjection/users=10000` and
  `BenchmarkMemberOfSingleResultProjection/users=10000` materially improve.
- Existing store, server, and functional tests pass.

Likely files:

- `internal/store/sqlite_search.go`
- `internal/schema/filter_compiler.go`
- `internal/store/sqlite_search_test.go`
- `internal/store/sqlite_memberof_benchmark_test.go`

### 2. Add A SQL Fast Path For memberOf Equality Filters

Problem:

- `(memberOf=<groupDN>)` currently computes memberOf for broad candidates and
  then filters in memory.
- For direct equality filters, the store can anchor from the target group and
  traverse `group_members` to find member user entries.

Desired behavior:

- Detect simple `memberOf=<groupDN>` filters, and ideally conjunctions like
  `(&(objectClass=inetOrgPerson)(memberOf=<groupDN>))`.
- Resolve the target group DN case-insensitively.
- Use recursive SQL to walk nested group membership from that group to user
  entries, with cycle protection.
- Preserve the response projection contract: filtering by `memberOf` must not
  force `memberOf` into returned entries unless `IncludeMemberOf` is true.

Acceptance criteria:

- Store tests cover direct group membership, nested group membership, cycles,
  missing group DN, and `IncludeMemberOf: false` response behavior.
- Functional LDAP tests still pass for `memberOf` filters.
- `BenchmarkMemberOfDirectFilter/users=10000` and nested filter benchmarks
  materially improve.

Likely files:

- `internal/schema/filter.go`
- `internal/schema/filter_compiler.go`
- `internal/store/sqlite_search.go`
- `internal/store/sqlite_membership.go`
- `internal/store/sqlite_memberof_benchmark_test.go`

### 3. Reduce Broad Projection Allocation Pressure

Problem:

- Broad projection allocates heavily: the 10k-user projection path currently
  allocates about 25 MB and more than 500k objects.

Desired behavior:

- Keep the batched `memberOf` query, but reduce intermediate slices/maps/string
  churn where practical.
- Avoid abstractions that hide LDAP semantics or make correctness harder to
  audit.

Acceptance criteria:

- `BenchmarkMemberOfAllUsersProjection/users=10000` shows lower allocation
  count and memory per op.
- No regression in direct/nested membership correctness.

Likely files:

- `internal/store/sqlite_membership.go`
- `internal/models/entry.go`

## CI Policy

Do not make these benchmarks a required PR gate yet.

Rationale:

- GitHub shared runners are noisy for timing comparisons.
- The full matrix includes explicit 10k-entry scale probes.
- Normal `go test ./...` already compiles benchmark code, which catches API and
  fixture drift.

If trend tracking becomes important, add a separate non-blocking manual or
scheduled workflow that uploads benchmark artifacts instead of failing PRs on
timing variance.

## Verification

For every optimization attempt, run:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/store ./internal/schema ./internal/server
GOCACHE=/private/tmp/ldaplite-gocache go test -run '^$' -bench='BenchmarkMemberOf' -benchmem -benchtime=1x ./internal/store
```

Before finishing a larger change, also run:

```bash
GOCACHE=/private/tmp/ldaplite-gocache make test
GOCACHE=/private/tmp/ldaplite-gocache go test -count=1 -tags=functional -v ./tests/functional/...
```

## Definition Of Done

- The benchmark command still runs successfully.
- Baseline and after numbers are recorded in the final response or a follow-up
  doc update.
- Search semantics remain correct for direct and nested `memberOf`.
- `memberOf` remains computed from `group_members`, not persisted in generic
  attributes.
- The final implementation is pushed.
