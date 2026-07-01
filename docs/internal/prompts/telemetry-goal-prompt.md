# Goal: Add Audit-Grade Telemetry To LDAPLite

You are working in `/Users/smarzola/ldaplite`.

Your objective is to implement production-grade telemetry for LDAPLite: stable
audit logging plus optional OpenTelemetry metrics/tracing with Prometheus
compatibility.

Track this work against GitHub issue
[#9](https://github.com/smarzola/ldaplite/issues/9).

## Repository Rules

Follow `AGENTS.md`.

Important reminders:

- Use `ast-grep` for Go structural searches.
- Use `rg` for ordinary text/file searches.
- Keep changes scoped to telemetry, audit logging, configuration, tests, and
  documentation.
- Do not revert unrelated user changes.
- Do not log credentials, password hashes, authorization headers, or
  `userPassword` values.
- Prefer centralized observation points over scattered one-off log/counter
  calls.
- At the end of each milestone, commit the completed milestone and mark that
  milestone as done in this file before moving to the next milestone.

## Target State

LDAPLite should give operators enough telemetry to answer:

- who attempted an LDAP or Web UI operation;
- what operation was attempted;
- which DN, base DN, route, or resource was targeted;
- where the request came from;
- what result code or status was returned;
- how long the operation took;
- whether errors are LDAP protocol failures, authorization failures, store
  failures, validation failures, or transport failures.

The telemetry system should remain optional and should not change LDAP or Web UI
behavior when disabled.

## Current State

LDAPLite already uses `log/slog` with JSON or text output controlled by:

- `LDAP_LOG_LEVEL`
- `LDAP_LOG_FORMAT`

Existing logs are useful for debugging, but they are not yet an audit-grade
event stream:

- LDAP handlers log some request and success/failure paths with inconsistent
  fields.
- LDAP Add/Modify/Delete successes are logged, but many rejected or failed
  paths are debug-only or lack stable result fields.
- Bind failures are logged without a stable audit event shape.
- Web UI authentication and admin authorization failures are logged.
- Web UI create/update/delete successes and most validation failures are not
  logged as audit events.
- There are no request IDs, connection IDs, actor fields, operation result
  fields, duration fields, counters, histograms, gauges, or traces.

## Definition Of Done

The goal is complete only when:

1. Audit logs have stable event names and stable fields for LDAP and Web UI
   security-relevant events.
2. LDAP Bind, Search, Add, Modify, Delete, Compare, Extended, Unbind,
   unsupported-operation, read-error, and handler-error paths are observed.
3. Web UI authentication, authorization, same-origin rejection,
   create/update/delete, validation failure, and store failure paths are
   observed.
4. Audit events include actor/bound DN where available, remote address,
   operation type, target DN/base DN/route, result code/status, and duration
   where applicable.
5. Correlation identifiers exist for LDAP connections/operations and HTTP
   requests.
6. OpenTelemetry metrics are optional and configurable.
7. LDAP operation counters and latency histograms are exported by operation and
   result code.
8. Web UI request counters and latency histograms are exported by method,
   normalized route, and status.
9. Active LDAP connections, accepted LDAP connections, read errors, handler
   errors, and database pool stats are exported where practical.
10. Prometheus-compatible scraping is possible directly or through the chosen
    OpenTelemetry setup.
11. Tracing exists around request handling and store calls where it adds useful
    diagnostic value.
12. Tests cover representative audit events, secret redaction, metric labels,
    and disabled-by-default behavior.
13. Documentation covers telemetry configuration, audit field conventions,
    exported metrics, tracing behavior, and sensitive-data handling.
14. Each completed milestone has its own commit, and this file's milestone
    checklist is updated before the commit or in the same commit.

## Candidate Configuration

Exact names can change during implementation, but keep the shape simple:

```text
LDAP_TELEMETRY_ENABLED=false
LDAP_OTEL_SERVICE_NAME=ldaplite
LDAP_OTEL_EXPORTER_OTLP_ENDPOINT=
LDAP_METRICS_ENABLED=false
LDAP_METRICS_BIND_ADDRESS=0.0.0.0
LDAP_METRICS_PORT=9090
LDAP_METRICS_PATH=/metrics
```

Do not require operators to configure OTLP just to scrape local Prometheus
metrics.

## Audit Event Field Guidance

Use stable field names that can be parsed by log aggregation tools:

```text
event
component
operation
request_id
connection_id
message_id
remote_addr
actor_dn
target_dn
base_dn
route
method
result_code
status
duration_ms
error
```

Use fields only when they apply. Keep high-cardinality values out of metrics,
but they are acceptable in audit logs where operationally necessary.

## Milestone Checklist

Update this checklist as the goal loop progresses. When a milestone is complete:

1. Run the milestone's verification commands.
2. Update the milestone status from `[ ]` to `[x]`.
3. Commit the milestone with a focused commit message.
4. Mention the commit hash in the goal-loop status before continuing.

- [x] Milestone 1: Audit event model and LDAP observation.
- [x] Milestone 2: Web UI audit logging and HTTP request correlation.
- [x] Milestone 3: OpenTelemetry metrics foundation and Prometheus scrape path.
- [x] Milestone 4: LDAP and Web UI metrics coverage.
- [x] Milestone 5: Tracing and store-call spans.
- [x] Milestone 6: Documentation, final tests, and issue-ready summary.

## Milestone 1: Audit Event Model And LDAP Observation

Problem:

- LDAP logs are spread across handlers and use ad hoc message strings.
- Operators cannot reliably query by operation, result code, actor, target DN,
  connection, message, or latency.

Desired behavior:

- Add a small internal telemetry/audit package or equivalent local abstraction.
- Define stable LDAP audit event helpers.
- Add connection IDs and operation/request IDs for LDAP handling.
- Observe Bind, Search, Add, Modify, Delete, Compare, Extended, Unbind,
  unsupported operation, read error, and handler error paths.
- Preserve current LDAP behavior and result codes.

Acceptance criteria:

- LDAP audit events use stable event names and fields.
- Successful and failed binds are audit-logged without passwords or hashes.
- Search audit logs include base DN, scope, result code, result count where
  available, and duration.
- Write audit logs include actor DN, target DN, result code, and duration.
- Tests prove representative LDAP audit fields and secret redaction.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/protocol/connection.go`
- `internal/server/ldap.go`
- `internal/server/search.go`
- `internal/server/write.go`
- `internal/server/discovery.go`
- new `internal/telemetry/` or `internal/audit/`

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/protocol ./internal/server
```

## Milestone 2: Web UI Audit Logging And HTTP Request Correlation

Problem:

- Web UI auth failures are logged, but create/update/delete successes and many
  rejection paths are not audit events.
- HTTP requests do not have request IDs or route/status/duration fields.

Desired behavior:

- Add HTTP request IDs.
- Add middleware that records normalized route, method, status, remote address,
  duration, and authenticated user DN when available.
- Add audit events for Web UI authentication failure, admin authorization
  failure, same-origin rejection, create/update/delete success, validation
  failure, and store failure.

Acceptance criteria:

- Web UI write actions create audit events with actor DN and target DN.
- Same-origin rejection and admin authorization rejection are audit-logged.
- HTTP audit logs use normalized routes, not raw unbounded URLs.
- Tests cover representative Web UI audit logs.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/web/server.go`
- `internal/web/middleware/auth.go`
- `internal/web/middleware/same_origin.go`
- `internal/web/handlers/common.go`
- `internal/web/handlers/users.go`
- `internal/web/handlers/groups.go`
- `internal/web/handlers/ous.go`
- new `internal/telemetry/` or `internal/audit/`

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/web/...
```

## Milestone 3: OpenTelemetry Metrics Foundation And Prometheus Scrape Path

Problem:

- There is no metrics implementation.
- Operators need optional metrics without being forced into a full collector
  setup.

Desired behavior:

- Add telemetry config to `pkg/config`.
- Initialize OpenTelemetry metrics only when enabled.
- Expose a Prometheus-compatible `/metrics` endpoint when metrics are enabled.
- Keep telemetry disabled or minimal by default.
- Ensure graceful shutdown of telemetry exporters/servers.

Acceptance criteria:

- Default config does not start a metrics listener.
- Enabling metrics starts a scrape endpoint on the configured bind address,
  port, and path.
- Telemetry initialization failure returns a clear startup error.
- Config tests cover defaults and env overrides.
- Milestone status is marked done in this file and committed.

Likely files:

- `pkg/config/config.go`
- `pkg/config/config_test.go`
- `cmd/ldaplite/main.go`
- new `internal/telemetry/`
- `go.mod`
- `go.sum`

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./pkg/config ./cmd/ldaplite
```

## Milestone 4: LDAP And Web UI Metrics Coverage

Problem:

- Once metrics exist, LDAP and HTTP handlers need useful counters and latency
  histograms with bounded labels.

Desired behavior:

- Add LDAP operation counters by operation and result code.
- Add LDAP operation duration histograms by operation and result code.
- Add Web UI request counters by method, normalized route, and status.
- Add Web UI request duration histograms by method, normalized route, and
  status.
- Add active LDAP connection gauge, accepted connection counter, read error
  counter, handler error counter, and database pool stats where practical.

Acceptance criteria:

- Metrics use bounded label sets.
- Tests cover representative metric names and labels.
- Metrics are no-ops when telemetry is disabled.
- Existing LDAP/Web UI behavior is unchanged.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/protocol/connection.go`
- `internal/server/ldap.go`
- `internal/server/search.go`
- `internal/server/write.go`
- `internal/web/server.go`
- `internal/store/sqlite.go`
- `internal/telemetry/`

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/protocol ./internal/server ./internal/web/... ./internal/store
```

## Milestone 5: Tracing And Store-Call Spans

Problem:

- Logs and metrics show what happened, but not enough request flow context for
  slower store-backed operations.

Desired behavior:

- Add OpenTelemetry tracing only when configured.
- Create spans around LDAP operation handling and Web UI request handling.
- Add spans around store calls where they help diagnose latency.
- Add span attributes with bounded values: operation, result code/status,
  route, and store method.
- Avoid high-cardinality or sensitive span attributes such as passwords,
  attribute values, or raw authorization headers.

Acceptance criteria:

- Tracing is optional and disabled/minimal by default.
- Representative spans are emitted in tests or via an in-memory exporter.
- Store-call tracing does not change store APIs unnecessarily.
- Milestone status is marked done in this file and committed.

Likely files:

- `internal/server/`
- `internal/web/`
- `internal/store/`
- `internal/telemetry/`

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test ./internal/server ./internal/web/... ./internal/store
```

## Milestone 6: Documentation, Final Tests, And Issue-Ready Summary

Problem:

- Operators need clear telemetry setup docs.
- The final implementation needs full regression verification.

Desired behavior:

- Document audit-log event names and fields.
- Document telemetry environment variables.
- Document Prometheus scrape setup.
- Document OTLP/collector setup if implemented.
- Document sensitive-data guarantees and known limits.
- Update roadmap or issue references only if they become stale.

Acceptance criteria:

- Docs describe configuration, metrics, tracing, audit fields, and redaction.
- `make test` passes.
- Functional tests pass.
- The final response or issue comment summarizes completed milestones, commit
  hashes, tests run, and any intentionally deferred telemetry work.
- Milestone status is marked done in this file and committed.

Likely files:

- `README.md`
- `docs/roadmap.md`
- new `docs/telemetry.md` if the content is too large for README
- `docs/internal/prompts/telemetry-goal-prompt.md`

Verification:

```bash
GOCACHE=/private/tmp/ldaplite-gocache make test
GOCACHE=/private/tmp/ldaplite-gocache go test -count=1 -tags=functional -v ./tests/functional/...
```

## Suggested Goal Loop

For each loop:

1. Read `AGENTS.md`, this file, and issue `#9`.
2. Pick the first incomplete milestone.
3. Inspect the relevant code and tests before editing.
4. Implement the smallest complete milestone slice.
5. Add focused tests that prove the telemetry behavior.
6. Run the milestone verification command.
7. Update this file to mark the milestone done.
8. Commit the milestone with a focused message.
9. Report the commit hash and exact verification command output summary.
10. Continue to the next milestone only after the commit is complete.

Do not batch multiple milestones into one commit unless the user explicitly
requests it.
