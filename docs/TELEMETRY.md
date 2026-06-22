# LDAPLite Telemetry

LDAPLite emits structured audit logs by default and can optionally expose
OpenTelemetry metrics, Prometheus-compatible metrics, and OpenTelemetry traces.

Telemetry must not change LDAP or Web UI behavior. Credentials, password
hashes, authorization headers, and `userPassword` values must not be logged,
recorded as metric labels, or attached to spans.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_LOG_LEVEL` | `info` | Audit/debug log level: `debug`, `info`, `warn`, or `error` |
| `LDAP_LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `LDAP_TELEMETRY_ENABLED` | `false` | Enable OpenTelemetry tracing setup |
| `LDAP_OTEL_SERVICE_NAME` | `ldaplite` | OpenTelemetry service name |
| `LDAP_OTEL_EXPORTER_OTLP_ENDPOINT` | empty | OTLP HTTP trace endpoint, for example `http://collector:4318/v1/traces` |
| `LDAP_METRICS_ENABLED` | `false` | Enable the Prometheus-compatible metrics endpoint |
| `LDAP_METRICS_BIND_ADDRESS` | `0.0.0.0` | Metrics HTTP bind address |
| `LDAP_METRICS_PORT` | `9090` | Metrics HTTP port |
| `LDAP_METRICS_PATH` | `/metrics` | Metrics scrape path |

Metrics can be enabled without an OTLP collector:

```bash
export LDAP_METRICS_ENABLED=true
export LDAP_METRICS_BIND_ADDRESS=127.0.0.1
export LDAP_METRICS_PORT=9090
export LDAP_METRICS_PATH=/metrics
```

Tracing export requires telemetry and an OTLP HTTP endpoint:

```bash
export LDAP_TELEMETRY_ENABLED=true
export LDAP_OTEL_SERVICE_NAME=ldaplite
export LDAP_OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4318/v1/traces
```

## Audit Logs

Audit logs are structured `slog` events written to stderr. In JSON mode each
event includes stable fields where applicable.

Common fields:

| Field | Meaning |
|-------|---------|
| `event` | Stable event name |
| `component` | `ldap` or `web` |
| `operation` | LDAP operation or Web write operation |
| `request_id` | HTTP request ID or LDAP connection/message ID pair |
| `connection_id` | LDAP connection ID |
| `message_id` | LDAP message ID |
| `remote_addr` | Remote network address |
| `actor_dn` | Authenticated LDAP/Web actor DN when known |
| `actor_uid` | Web UI username attempted during failed authentication |
| `target_dn` | Target entry DN for write-like operations |
| `base_dn` | LDAP search base DN |
| `route` | Normalized Web UI route |
| `method` | HTTP method |
| `result_code` | LDAP result code |
| `status` | HTTP status |
| `duration_ms` | Operation duration in milliseconds |
| `error` | Error string where useful |

LDAP event names:

- `ldap.operation`
- `ldap.read_error`
- `ldap.handler_error`

Web event names:

- `http.request`
- `web.auth_required`
- `web.auth_failed`
- `web.authorization_denied`
- `web.same_origin_denied`
- `web.write`

## Metrics

When `LDAP_METRICS_ENABLED=true`, LDAPLite starts a separate HTTP server for
Prometheus-compatible metrics. Labels are intentionally bounded.

| Prometheus metric | Labels | Meaning |
|-------------------|--------|---------|
| `ldaplite_ldap_operations_total` | `operation`, `result_code` | LDAP operations completed |
| `ldaplite_ldap_operation_duration_milliseconds_*` | `operation`, `result_code` | LDAP operation duration histogram |
| `ldaplite_ldap_connections_accepted_total` | none | Accepted LDAP connections |
| `ldaplite_ldap_connections_active` | none | Active LDAP connections |
| `ldaplite_ldap_read_errors_total` | none | LDAP transport read errors |
| `ldaplite_ldap_handler_errors_total` | `operation` | LDAP handler errors |
| `ldaplite_http_requests_total` | `method`, `route`, `status` | Web UI HTTP requests |
| `ldaplite_http_request_duration_milliseconds_*` | `method`, `route`, `status` | Web UI request duration histogram |
| `ldaplite_web_writes_total` | `operation`, `resource`, `status` | Web UI write actions |
| `ldaplite_db_connections_open` | none | Open SQLite connections |
| `ldaplite_db_connections_in_use` | none | In-use SQLite connections |
| `ldaplite_db_connections_idle` | none | Idle SQLite connections |

Routes are normalized before they become metric labels. Raw query strings, DNs,
filters, credentials, and attribute values are not metric labels.

## Tracing

Tracing is initialized when `LDAP_TELEMETRY_ENABLED=true`. If
`LDAP_OTEL_EXPORTER_OTLP_ENDPOINT` is set, spans are exported through the OTLP
HTTP exporter.

Span names:

- `ldap.bind`
- `ldap.search`
- `ldap.add`
- `ldap.modify`
- `ldap.delete`
- `ldap.compare`
- `ldap.extended`
- `ldap.unbind`
- `http.request`
- `store.GetEntryWithOptions`
- `store.CreateEntry`
- `store.UpdateEntry`
- `store.DeleteEntry`
- `store.EntryExists`
- `store.SearchEntriesWithOptions`
- `store.GetUserPasswordHash`
- `store.GetUserPasswordHashByDN`

Span attributes stay bounded:

- LDAP spans use `ldap.operation` and `ldap.result_code`.
- HTTP spans use `http.request.method`, `http.route`, and `http.response.status_code`.
- Store spans use `store.method`.

DNs, LDAP filters, HTTP authorization headers, passwords, password hashes, and
raw attribute values are intentionally omitted from span attributes.
