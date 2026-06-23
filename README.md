# LDAPLite

A lightweight LDAP v3 server written in Go with a SQLite backend. Built for
self-hosted identity, development environments, and small-to-medium deployments
that need predictable LDAP behavior without running a full directory stack.

## Why LDAPLite?

**Simplicity over complexity.** Most directory systems are complex beasts requiring extensive setup, configuration files, and operational expertise. LDAPLite takes a different approach:

- **Just a binary** - Download and run. No complex installation, no external dependencies.
- **Opinionated** - Sensible defaults that work out of the box. Configure only what you need.
- **SQLite storage** - Single-file database. Easy backups, no complex datastores required.
- **Self-hosting friendly** - Docker-ready, structured logging, environment-variable configuration, and an embedded Web UI when you want one.
- **Performance-minded** - Indexed search paths, computed attributes, and benchmark coverage for the LDAP operations most likely to matter in daily use.

Self-hosting is getting interesting again as AI-assisted tooling makes it easier
for small teams and individuals to operate useful infrastructure. LDAPLite is
for that world: a small directory server you can understand, back up, run in a
container, and connect to ordinary LDAP clients without adopting enterprise
directory operations as a hobby.

## Features

### LDAP Protocol Support

- **RFC-Compliant**: Implements core LDAP v3 operations
  - Bind with simple authentication
  - Search with SQL-optimized filters
  - Add, Modify, Delete operations
  - Compare operations with true/false/no-such-object result semantics
  - RootDSE and Schema queries

- **Object Classes** (RFC 2256, RFC 2798):
  - `organizationalUnit` - Container entries
  - `inetOrgPerson` - User entries with email, phone, display name, and `memberOf` attribute
  - `groupOfNames` - Groups with nested group support
  - `top` - Root of object class hierarchy

- **Operational Attributes** (RFC 4512, RFC 4517, RFC2307bis-style compatibility):
  - `createTimestamp` - Entry creation time (LDAP Generalized Time format)
  - `modifyTimestamp` - Last modification time
  - `entryUUID` - Stable server-generated entry identifier (RFC 4530-style)
  - `objectClass` - Structural object class
  - `memberOf` - Groups the user belongs to (computed, read-only)
  - Searchable with `>=` and `<=` operators for timestamps

### Advanced Features

- **Nested Groups**: Groups can contain users and other groups with circular reference detection
- **memberOf Attribute**: Users can request a computed, read-only `memberOf` attribute with DNs of all groups they belong to
- **SQL Filter Compilation**: LDAP filters compiled to indexed SQL queries for performance
- **Fast memberOf Filters**: Direct and nested `memberOf=<groupDN>` filters use recursive SQL over membership indexes
- **Hybrid Filtering**: Falls back to in-memory filtering for complex queries
- **Argon2id Password Hashing**: OWASP-recommended parameters (64MB memory, 3 iterations)
- **Recursive Hierarchy Traversal**: Efficient SQL CTEs for searching deep directory trees
- **Structured Logging**: JSON or text format with configurable levels
- **Telemetry**: Audit-grade structured logs plus optional OpenTelemetry metrics/tracing and Prometheus-compatible scraping
- **Read-only service accounts**: Members of `cn=ldaplite.readonly,ou=groups,<baseDN>` can bind/search/compare but cannot write

### Storage & Deployment

- **Single Go Binary**: ~10MB static binary, no runtime dependencies
- **SQLite Backend**: Single-file database, ideal for backups and migrations
- **Docker Support**: Distroless image, non-root user, health checks
- **Simple Configuration**: Environment variables only, no config files required
- **Direct Protocol Implementation**: Repo-owned LDAP BER encoding/decoding, no high-level framework overhead
- **Reverse Proxy Friendly**: No TLS support by design - meant to run behind nginx/traefik

### Web UI

- **Embedded Web Interface**: Simple, modern web UI for directory management
  - HTTP Basic authentication with admin group authorization
  - Browse and manage users, groups, and organizational units
  - Full CRUD operations (create, read, update, delete)
  - Dark/light theme toggle (wireframe/black themes)
  - Responsive design with Tailwind CSS and DaisyUI
  - No external dependencies, embedded in binary

## Quick Start

### Option 1: Download Binary

```bash
# Download latest Linux AMD64 release
curl -LO https://github.com/smarzola/ldaplite/releases/latest/download/ldaplite-linux-amd64.tar.gz
tar -xzf ldaplite-linux-amd64.tar.gz
chmod +x ldaplite-linux-amd64

# Set required environment variables
export LDAP_BASE_DN="dc=example,dc=com"
export LDAP_ADMIN_PASSWORD="YourSecurePassword123!"

# Optional: Enable Web UI
export LDAP_WEB_UI_ENABLED=true
export LDAP_WEB_UI_PORT=8080

# Run
./ldaplite-linux-amd64 server

# Access Web UI at http://localhost:8080 (login with admin:YourSecurePassword123!)
```

Release archives are available for:

| Platform | Archive |
|----------|---------|
| Linux AMD64 | `ldaplite-linux-amd64.tar.gz` |
| Linux ARM64 | `ldaplite-linux-arm64.tar.gz` |
| macOS Intel | `ldaplite-darwin-amd64.tar.gz` |
| macOS Apple Silicon | `ldaplite-darwin-arm64.tar.gz` |
| Windows AMD64 | `ldaplite-windows-amd64.zip` |
| Windows ARM64 | `ldaplite-windows-arm64.zip` |

### Option 2: Docker

```bash
docker run -d \
  --name ldaplite \
  -p 3389:3389 \
  -p 8080:8080 \
  -e LDAP_BASE_DN=dc=example,dc=com \
  -e LDAP_ADMIN_PASSWORD=YourSecurePassword \
  -e LDAP_WEB_UI_ENABLED=true \
  -v ldap_data:/data \
  ghcr.io/smarzola/ldaplite:latest

# Access Web UI at http://localhost:8080 (login with admin:YourSecurePassword)
```

Or use Docker Compose:

```yaml
version: '3.8'
services:
  ldaplite:
    image: ghcr.io/smarzola/ldaplite:latest
    ports:
      - "3389:3389"
      - "8080:8080"
    environment:
      LDAP_BASE_DN: dc=example,dc=com
      LDAP_ADMIN_PASSWORD: ${LDAP_ADMIN_PASSWORD}
      LDAP_WEB_UI_ENABLED: "true"
    volumes:
      - ldap_data:/data
    restart: unless-stopped

volumes:
  ldap_data:
```

### Option 3: Build from Source

```bash
# Prerequisites: Go 1.25+
git clone https://github.com/smarzola/ldaplite.git
cd ldaplite

# Build
make build

# Run with Web UI enabled
export LDAP_BASE_DN=dc=example,dc=com
export LDAP_ADMIN_PASSWORD=SecurePassword123!
export LDAP_WEB_UI_ENABLED=true
./bin/ldaplite server

# Access Web UI at http://localhost:8080
```

## What Gets Created on First Run

LDAPLite automatically initializes your directory with:

```
dc=example,dc=com (base DN)
├── ou=users
│   └── uid=admin (with your LDAP_ADMIN_PASSWORD)
└── ou=groups
    └── cn=ldaplite.admin (admin group, contains uid=admin)
```

The admin user is automatically added to the `ldaplite.admin` group, which grants access to the Web UI.

## Testing Your Connection

```bash
# Test authentication
ldapwhoami -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w YourSecurePassword

# Search all entries
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w YourSecurePassword \
  -b "dc=example,dc=com" \
  "(objectClass=*)"

# Search with timestamp filter
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w YourSecurePassword \
  -b "dc=example,dc=com" \
  "(modifyTimestamp>=20240101000000Z)"
```

## Configuration

All configuration via environment variables. No config files needed.

### Required Variables

| Variable | Description |
|----------|-------------|
| `LDAP_BASE_DN` | Base DN for your directory (e.g., `dc=example,dc=com`) |
| `LDAP_ADMIN_PASSWORD` | Admin user password (required on first run only) |

### Server Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_PORT` | `3389` | LDAP server port |
| `LDAP_BIND_ADDRESS` | `0.0.0.0` | Network interface to bind to |
| `LDAP_READ_TIMEOUT` | `30` | Read timeout in seconds |
| `LDAP_WRITE_TIMEOUT` | `30` | Write timeout in seconds |

### Database Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_DATABASE_PATH` | `/data/ldaplite.db` | SQLite database file path |
| `LDAP_DATABASE_MAX_OPEN_CONNS` | `25` | Maximum open database connections |
| `LDAP_DATABASE_MAX_IDLE_CONNS` | `5` | Maximum idle database connections |
| `LDAP_DATABASE_CONN_MAX_LIFETIME` | `300` | Connection max lifetime in seconds |

### Logging Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LDAP_LOG_FORMAT` | `json` | Log format: `json` or `text` |

### Telemetry Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_TELEMETRY_ENABLED` | `false` | Enable OpenTelemetry tracing setup |
| `LDAP_OTEL_SERVICE_NAME` | `ldaplite` | OpenTelemetry service name |
| `LDAP_OTEL_EXPORTER_OTLP_ENDPOINT` | empty | OTLP HTTP trace endpoint |
| `LDAP_METRICS_ENABLED` | `false` | Enable Prometheus-compatible metrics endpoint |
| `LDAP_METRICS_BIND_ADDRESS` | `0.0.0.0` | Metrics HTTP bind address |
| `LDAP_METRICS_PORT` | `9090` | Metrics HTTP port |
| `LDAP_METRICS_PATH` | `/metrics` | Metrics scrape path |

See [Telemetry](docs/TELEMETRY.md) for audit fields, metric names, tracing behavior, and sensitive-data handling.

### Web UI Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_WEB_UI_ENABLED` | `false` | Enable the embedded web UI |
| `LDAP_WEB_UI_PORT` | `8080` | Web UI HTTP port |
| `LDAP_WEB_UI_BIND_ADDRESS` | `0.0.0.0` | Web UI bind address |

**Note**: Web UI requires authentication using admin user credentials (HTTP Basic Auth). Only members of the `cn=ldaplite.admin,ou=groups` group can access the web interface. Mutating Web UI requests require a same-origin `Origin` or `Referer` header, and delete actions use POST requests. The older `LDAP_WEBUI_*` spelling is still accepted as a compatibility alias.

### Security Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_ALLOW_ANONYMOUS_BIND` | `false` | Allow anonymous bind (not recommended) |
| `LDAP_ARGON2_MEMORY` | `65536` | Argon2 memory cost in KB (64MB) |
| `LDAP_ARGON2_ITERATIONS` | `3` | Argon2 time cost (iterations) |
| `LDAP_ARGON2_PARALLELISM` | `2` | Argon2 parallelism factor |
| `LDAP_ARGON2_SALT_LENGTH` | `16` | Salt length in bytes |
| `LDAP_ARGON2_KEY_LENGTH` | `32` | Derived key length in bytes |

LDAPLite requires a successful bind before normal directory searches and all write operations. RootDSE and schema searches are intentionally readable before bind so clients can discover server capabilities. When `LDAP_ALLOW_ANONYMOUS_BIND=true`, anonymous clients must still perform an anonymous bind first, and anonymous sessions are limited to search access; Add, Modify, and Delete require an authenticated user DN.

**Note**: Argon2id parameters follow [OWASP recommendations](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#argon2id) for secure password hashing.

## Usage Examples

### Adding a User

```bash
# Create user.ldif
cat > user.ldif <<EOF
dn: uid=john,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: john
cn: John Doe
sn: Doe
givenName: John
mail: john@example.com
displayName: John Doe
userPassword: password123
EOF

# Add to directory
ldapadd -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w YourPassword \
  -f user.ldif
```

### Creating a Group

```bash
cat > group.ldif <<EOF
dn: cn=developers,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: developers
member: uid=john,ou=users,dc=example,dc=com
member: uid=jane,ou=users,dc=example,dc=com
EOF

ldapadd -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w YourPassword \
  -f group.ldif
```

### Nested Groups

```bash
# Create parent group that includes another group
cat > parent-group.ldif <<EOF
dn: cn=engineering,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: engineering
member: cn=developers,ou=groups,dc=example,dc=com
member: cn=devops,ou=groups,dc=example,dc=com
EOF

ldapadd -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w YourPassword \
  -f parent-group.ldif
```

### Querying User Group Memberships (memberOf)

LDAPLite computes the optional `memberOf` attribute for user entries as RFC2307bis-style client compatibility. Membership is transitive through nested groups, with cycle protection to avoid infinite traversal:

```bash
# Search for a user - memberOf is automatically included
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w YourPassword \
  -b "dc=example,dc=com" \
  "(uid=john)"

# Example output:
# dn: uid=john,ou=users,dc=example,dc=com
# objectClass: inetOrgPerson
# uid: john
# cn: John Doe
# sn: Doe
# mail: john@example.com
# entryUUID: 1d84d1af-89ef-4cc2-98fb-f868b84f10e1
# memberOf: cn=developers,ou=groups,dc=example,dc=com
# memberOf: cn=ldaplite.admin,ou=groups,dc=example,dc=com

# Find all users in a specific group using memberOf
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w YourPassword \
  -b "ou=users,dc=example,dc=com" \
  "(memberOf=cn=developers,ou=groups,dc=example,dc=com)"
```

Every entry receives a stable generated `entryUUID`. It is server-managed and cannot be set or modified by LDAP clients.

Search result attribute selection is honored case-insensitively. Requesting `1.1` returns no attributes, `*` returns user attributes, and `+` returns operational attributes such as `entryUUID`, `memberOf`, `createTimestamp`, and `modifyTimestamp`. Explicitly requested operational attributes are returned by name. When no attribute list is supplied, LDAPLite returns both user and operational attributes for compatibility with common clients.

LDAPLite emits canonical presentation casing for known LDAP attributes such as `objectClass`, `entryUUID`, `memberOf`, `createTimestamp`, `modifyTimestamp`, `givenName`, `displayName`, and `telephoneNumber`. Custom attributes remain case-insensitive internally and are currently presented using the normalized stored name.

## LDAP Filters

LDAPLite supports comprehensive LDAP filter syntax:

### Basic Filters

```
(objectClass=inetOrgPerson)          # All users
(uid=john)                            # Exact match
(cn=John*)                            # Starts with
(mail=*@example.com)                  # Ends with
(displayName=*Doe*)                   # Contains
(memberOf=cn=developers,ou=groups,dc=example,dc=com)  # Users in group
```

### Logical Operators

```
(&(objectClass=inetOrgPerson)(mail=*))              # AND
(|(uid=john)(uid=jane))                              # OR
(!(objectClass=organizationalUnit))                  # NOT
```

### Timestamp Queries

```
(modifyTimestamp>=20240101000000Z)    # Modified after date
(createTimestamp<=20241231235959Z)    # Created before date
(&(objectClass=inetOrgPerson)(modifyTimestamp>=20240601000000Z))
```

### Complex Queries

```
(&
  (objectClass=inetOrgPerson)
  (|
    (mail=*@example.com)
    (mail=*@company.com)
  )
  (!(uid=guest*))
)
```

## Architecture Highlights

### Database Schema

- **entries** - All LDAP entries with timestamps and hierarchy
- **attributes** - Multi-valued attributes storage (EAV pattern)
- **users** - User-specific data (password hash only, security isolation)
- **groups** - Group entry markers for referential integrity
- **group_members** - Group membership junction table (powers `memberOf` attribute)
- **organizational_units** - OU entry markers

### Performance Optimizations

- **Indexed Hierarchy**: Uses recursive CTEs with indexed `parent_dn` lookups
- **Indexed Attribute Equality**: Exact searches such as `(uid=john)` use indexed attribute lookups before loading full entries
- **SQL Filter Compilation**: Converts LDAP filters to indexed SQL WHERE clauses where possible
- **memberOf Fast Path**: `memberOf=<groupDN>` filters anchor from the group and recursively walk `group_members`
- **Optional Operational Projection**: Computed attributes such as `memberOf` are skipped when clients do not request them
- **Hybrid Approach**: Falls back to in-memory filtering for unsupported filters
- **Connection Pooling**: Configurable connection limits for concurrent operations

### Benchmarks

LDAPLite includes store-level benchmarks for common `memberOf` and narrow search
paths. Current results from an Apple M1 development machine, using the checked-in
10k-entry scale probes:

| Benchmark | Result |
|-----------|--------|
| Exact lookup, no `memberOf` projection, 10k users | `0.46 ms/op`, `12 KB/op`, `257 allocs/op` |
| Exact lookup with one-result `memberOf` projection, 10k users | `0.72 ms/op`, `26 KB/op`, `395 allocs/op` |
| All-users `memberOf` projection, 10k users | `128 ms/op`, `25.2 MB/op`, `516k allocs/op` |
| Direct `memberOf=<groupDN>` filter, 10k users | `1.91 ms/op`, `79 KB/op`, `2.8k allocs/op` |
| Nested `memberOf=<groupDN>` filter, depth 50 | `1.01 ms/op`, `79 KB/op`, `2.9k allocs/op` |

Run the benchmark matrix locally with:

```bash
GOCACHE=/private/tmp/ldaplite-gocache go test \
  -run '^$' \
  -bench='BenchmarkMemberOf' \
  -benchmem \
  -benchtime=1x \
  ./internal/store
```

`-benchtime=1x` is intentional for this matrix: the large fixtures are explicit
scale probes, and rebuilding them repeatedly during benchmark calibration adds
noise. These benchmarks are not a CI timing gate; normal `go test` still
compiles them, while local runs are better for comparing before/after changes.

### Password Security

- **Argon2id hashing** with OWASP-recommended parameters
- **Constant-time verification** to prevent timing attacks
- **Configurable cost parameters** for future-proofing

## Integration Guides

See [docs/CLIENT_COMPATIBILITY_MATRIX.md](docs/CLIENT_COMPATIBILITY_MATRIX.md) for the LDAP client compatibility matrix.
See [docs/integrations/](docs/integrations/) for LDAP consumer recipes.
See [docs/LDAP_AUTHORIZATION.md](docs/LDAP_AUTHORIZATION.md) for read-only app bind users.
See [docs/deployment/ldaps-tls-sidecar.md](docs/deployment/ldaps-tls-sidecar.md) for LDAPS sidecar deployment.

- [Authelia](docs/integrations/authelia.md)
- [Dex](docs/integrations/dex.md)
- [Gitea and Forgejo](docs/integrations/gitea-forgejo.md)
- [Grafana](docs/integrations/grafana.md)
- [Nextcloud](docs/integrations/nextcloud.md)
- [Pocket ID](docs/integrations/pocket-id.md)

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md) for current project status and planned work.
See [docs/CLIENT_COMPATIBILITY_PRODUCT_SUMMARY.md](docs/CLIENT_COMPATIBILITY_PRODUCT_SUMMARY.md) for the latest client-compatibility goal summary.

- **SCIM 2.0 Support** - Modern API for user/group provisioning alongside LDAP
  - RESTful HTTP interface (RFC 7643, RFC 7644)
  - JSON payloads for easier integration
  - Compatible with modern IdP systems
- Enhanced ACLs for granular permissions
- LDIF import/export commands from [docs/IMPORT_EXPORT_DESIGN.md](docs/IMPORT_EXPORT_DESIGN.md)
- Native TLS/LDAPS support (sidecar deployment is documented)

## Limitations

Current limitations (by design or priority):

- **No TLS/SSL** - Use reverse proxy (Nginx, Traefik) for encryption
- **No SASL** - Simple bind only (username/password)
- **No Replication** - Single-instance only
- **No Complex ACLs** - Admin has full access, users can bind
- **No Schema Extension** - Fixed object classes (sufficient for most use cases)
- **SQLite Concurrency** - Suitable for small-to-medium deployments

These are intentional trade-offs for simplicity. For large enterprise deployments, consider OpenLDAP or 389 Directory Server.

## Development

### Running Tests

```bash
# Run all tests with race detection (also builds embedded Web UI CSS)
make test

# Run AD-like functional compatibility tests
make test-functional

# Run with coverage (also builds embedded Web UI CSS)
make test-coverage

# View coverage in browser
open coverage.html
```

Note: direct `go test ./...` requires `internal/web/static/output.css` to exist because the Web UI embeds it at compile time. Prefer `make test` on a fresh checkout so CSS is generated first.

### Compatibility Testing

LDAPLite includes a black-box functional compatibility suite that starts the real `ldaplite server` binary on a random local port, uses a temporary SQLite database, and drives LDAP operations through `github.com/go-ldap/ldap/v3`.

The suite covers an Active Directory-like first milestone for common LDAP clients: simple bind, subtree search, AD-facing attributes such as `sAMAccountName` and `userPrincipalName`, group `member` searches, password modification, deletion, hidden `userPassword`, operational timestamps, and LDAP result codes for invalid credentials, missing objects, password scheme violations, and object class violations.

This is not full Active Directory compatibility. LDAPLite still intentionally excludes Kerberos, SASL/GSSAPI, LDAPS/TLS termination, Global Catalog, DirSync, paging controls, server-side sorting controls, the AD recursive matching rule, and complete Microsoft schema behavior.

CI runs both the normal Go test suite and the AD-like functional compatibility suite for pull requests and pushes to `main`.

### Code Structure

```
ldaplite/
├── cmd/ldaplite/           # Main entry point
├── internal/
│   ├── server/             # LDAP protocol handler
│   ├── store/              # SQLite storage layer
│   │   └── migrations/     # Embedded SQL migrations
│   ├── models/             # Domain models (User, Group, OU)
│   ├── schema/             # Filter parsing & compilation
│   └── web/                # Web UI server & handlers
│       ├── handlers/       # HTTP request handlers
│       ├── middleware/     # Authentication middleware
│       ├── templates/      # HTML templates (embedded)
│       └── static/         # CSS assets (embedded)
├── pkg/
│   ├── config/             # Configuration management
│   └── crypto/             # Password hashing
└── Makefile                # Build & test automation
```

### Contributing

Contributions welcome! Please ensure:
- Tests pass: `make test`
- Code is formatted: `go fmt ./...`
- Commits are clear and focused

## License

MIT License - See [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/smarzola/ldaplite/issues)
- **Discussions**: [GitHub Discussions](https://github.com/smarzola/ldaplite/discussions)
