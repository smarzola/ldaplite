# LDAPLite

A lightweight, RFC-compliant LDAP server written in Go with SQLite backend. Built for simplicity and modern workflows.

## Why LDAPLite?

**Simplicity over complexity.** Most directory systems are complex beasts requiring extensive setup, configuration files, and operational expertise. LDAPLite takes a different approach:

- **Just a binary** - Download and run. No complex installation, no external dependencies.
- **Opinionated** - Sensible defaults that work out of the box. Configure only what you need.
- **SQLite storage** - Single-file database. Easy backups, no complex datastores required.
- **Modern tooling** - Docker-ready, structured logging (JSON), built with Go.

Perfect for homelabs, development environments, and single-instance deployments where you need LDAP without the operational overhead.

**Also:** This project serves as an experiment in building complex, performant software with AI assistance (Claude Code) for educational purposes. The entire codebase, from the recursive SQL CTEs to RFC-compliant timestamp handling, was developed collaboratively with an LLM.

## Features

### LDAP Protocol Support

- **RFC-Compliant**: Implements core LDAP v3 operations
  - Bind with simple authentication
  - Search with SQL-optimized filters
  - Add, Modify, Delete operations
  - Compare operations (basic)
  - RootDSE and Schema queries

- **Object Classes** (RFC 2256, RFC 2798):
  - `organizationalUnit` - Container entries
  - `inetOrgPerson` - User entries with email, phone, display name
  - `groupOfNames` - Groups with nested group support
  - `top` - Root of object class hierarchy

- **Operational Attributes** (RFC 4512, RFC 4517):
  - `createTimestamp` - Entry creation time (LDAP Generalized Time format)
  - `modifyTimestamp` - Last modification time
  - `objectClass` - Structural object class
  - Searchable with `>=` and `<=` operators

### Advanced Features

- **Nested Groups**: Groups can contain users and other groups with circular reference detection
- **SQL Filter Compilation**: LDAP filters compiled to indexed SQL queries for performance
- **Hybrid Filtering**: Falls back to in-memory filtering for complex queries
- **Argon2id Password Hashing**: OWASP-recommended parameters (64MB memory, 3 iterations)
- **Recursive Hierarchy Traversal**: Efficient SQL CTEs for searching deep directory trees
- **Structured Logging**: JSON or text format with configurable levels

### Storage & Deployment

- **Single Go Binary**: ~10MB static binary, no runtime dependencies
- **SQLite Backend**: Single-file database, ideal for backups and migrations
- **Docker Support**: Distroless image, non-root user, health checks
- **Simple Configuration**: Environment variables only, no config files required
- **Direct Protocol Implementation**: Uses goldap for ASN.1 BER encoding, no high-level framework overhead
- **Reverse Proxy Friendly**: No TLS support by design - meant to run behind nginx/traefik

## Quick Start

### Option 1: Download Binary

```bash
# Download latest release
curl -LO https://github.com/smarzola/ldaplite/releases/latest/download/ldaplite-linux-amd64.tar.gz
tar -xzf ldaplite-linux-amd64.tar.gz
chmod +x ldaplite-linux-amd64

# Set required environment variables
export LDAP_BASE_DN="dc=example,dc=com"
export LDAP_ADMIN_PASSWORD="YourSecurePassword123!"

# Run
./ldaplite-linux-amd64 server
```

### Option 2: Docker

```bash
docker run -d \
  --name ldaplite \
  -p 3389:3389 \
  -e LDAP_BASE_DN=dc=example,dc=com \
  -e LDAP_ADMIN_PASSWORD=YourSecurePassword \
  -v ldap_data:/data \
  ghcr.io/smarzola/ldaplite:latest
```

Or use Docker Compose:

```yaml
version: '3.8'
services:
  ldaplite:
    image: ghcr.io/smarzola/ldaplite:latest
    ports:
      - "3389:3389"
    environment:
      LDAP_BASE_DN: dc=example,dc=com
      LDAP_ADMIN_PASSWORD: ${LDAP_ADMIN_PASSWORD}
    volumes:
      - ldap_data:/data
    restart: unless-stopped

volumes:
  ldap_data:
```

### Option 3: Build from Source

```bash
# Prerequisites: Go 1.23+
git clone https://github.com/smarzola/ldaplite.git
cd ldaplite

# Build
make build

# Run
export LDAP_BASE_DN=dc=example,dc=com
export LDAP_ADMIN_PASSWORD=SecurePassword123!
./bin/ldaplite server
```

## What Gets Created on First Run

LDAPLite automatically initializes your directory with:

```
dc=example,dc=com (base DN)
├── ou=users
│   └── uid=admin (with your LDAP_ADMIN_PASSWORD)
└── ou=groups
```

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

### Security Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_ALLOW_ANONYMOUS_BIND` | `false` | Allow anonymous bind (not recommended) |
| `LDAP_ARGON2_MEMORY` | `65536` | Argon2 memory cost in KB (64MB) |
| `LDAP_ARGON2_ITERATIONS` | `3` | Argon2 time cost (iterations) |
| `LDAP_ARGON2_PARALLELISM` | `2` | Argon2 parallelism factor |
| `LDAP_ARGON2_SALT_LENGTH` | `16` | Salt length in bytes |
| `LDAP_ARGON2_KEY_LENGTH` | `32` | Derived key length in bytes |

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

## LDAP Filters

LDAPLite supports comprehensive LDAP filter syntax:

### Basic Filters

```
(objectClass=inetOrgPerson)          # All users
(uid=john)                            # Exact match
(cn=John*)                            # Starts with
(mail=*@example.com)                  # Ends with
(displayName=*Doe*)                   # Contains
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
- **attributes** - Multi-valued attributes storage
- **users** - User-specific data (uid, password hash)
- **groups** - Group data with recursive membership
- **organizational_units** - OU-specific data

### Performance Optimizations

- **Indexed Hierarchy**: Uses recursive CTEs with indexed `parent_dn` lookups
- **SQL Filter Compilation**: Converts LDAP filters to indexed SQL WHERE clauses
- **Hybrid Approach**: Falls back to in-memory filtering for unsupported filters
- **Connection Pooling**: Configurable connection limits for concurrent operations

### Password Security

- **Argon2id hashing** with OWASP-recommended parameters
- **Constant-time verification** to prevent timing attacks
- **Configurable cost parameters** for future-proofing

## Roadmap

### Planned Features

- **SCIM 2.0 Support** - Modern API for user/group provisioning alongside LDAP
  - RESTful HTTP interface (RFC 7643, RFC 7644)
  - JSON payloads for easier integration
  - Compatible with modern IdP systems

- **Minimal Web UI** - Simple web interface for directory management
  - Browse and search entries
  - User and group management
  - View operational statistics
  - No external dependencies, embedded in binary

### Future Considerations

- Enhanced ACLs for granular permissions
- Import/export tools (LDIF, CSV)
- TLS/LDAPS support (currently recommend reverse proxy)

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
# Run all tests with race detection
make test

# Run with coverage
make test-coverage

# View coverage in browser
open coverage.html
```

### Code Structure

```
ldaplite/
├── cmd/ldaplite/           # Main entry point
├── internal/
│   ├── server/             # LDAP protocol handler
│   ├── store/              # SQLite storage layer
│   │   └── migrations/     # Embedded SQL migrations
│   ├── models/             # Domain models (User, Group, OU)
│   └── schema/             # Filter parsing & compilation
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

## Acknowledgments

Built with:
- [vjeantet/ldapserver](https://github.com/vjeantet/ldapserver) - LDAP protocol implementation
- [modernc.org/sqlite](https://modernc.org/sqlite) - Pure Go SQLite driver
- [Claude Code](https://claude.com/claude-code) - AI pair programming assistant

---

**Current Version**: v0.3.1
**Status**: Beta - Suitable for development, testing, and homelab use
