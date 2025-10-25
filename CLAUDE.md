# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LDAPLite is a lightweight, LDAP-compliant server written in Go with SQLite backend. It provides a simple LDAP server implementation for modern environments with Docker support.

**Key Technologies:**
- Go 1.25.3
- SQLite database (modernc.org/sqlite)
- LDAP protocol via vjeantet/ldapserver
- Argon2id password hashing
- Cobra for CLI commands

## Build & Development Commands

### Code Search and Analysis

**ast-grep is available and MUST be used for:**
- Structural code search (finding functions, methods, structs, interfaces)
- Refactoring operations (renaming symbols, updating patterns)
- AST-based analysis of Go code

Examples:
```bash
# Find all function definitions
ast-grep --pattern 'func $NAME($$$) $$$' -lang go

# Find specific struct usage
ast-grep --pattern 'type $NAME struct { $$$ }' -lang go

# Find method calls
ast-grep --pattern '$OBJ.$METHOD($$$)' -lang go
```

Use ast-grep instead of grep/ripgrep when searching for Go language constructs.

### Building
```bash
make build                  # Build binary to bin/ldaplite
go build -o bin/ldaplite ./cmd/ldaplite
```

### Testing
```bash
make test                   # Run tests with race detector
make test-coverage          # Generate coverage report (coverage.html)
go test -v -race ./...      # Run tests directly
```

### Running Locally
```bash
make dev-run                # Build and run with dev settings

# Manual run:
export LDAP_BASE_DN=dc=example,dc=com
export LDAP_ADMIN_PASSWORD=your-secure-password
export LDAP_DATABASE_PATH=/tmp/ldaplite-data/ldaplite.db
./bin/ldaplite server
```

### Docker
```bash
make docker-build           # Build Docker image
make docker-run             # Start with docker-compose
make docker-stop            # Stop containers
make docker-logs            # View logs
```

### Testing LDAP Connection
```bash
# Search all entries
ldapsearch -H ldap://localhost:3389 -D "cn=admin,dc=example,dc=com" -w ChangeMe123! -b "dc=example,dc=com" "(objectClass=*)"

# Test authentication
ldapwhoami -H ldap://localhost:3389 -D "cn=admin,dc=example,dc=com" -w ChangeMe123!
```

## Architecture

### Core Components

**Server Layer** (`internal/server/ldap.go`)
- LDAP protocol handler using vjeantet/ldapserver library
- Handles: Bind, Search, Add, Delete, Modify, Compare operations
- Routes LDAP operations to store layer
- Password verification using Argon2id hasher

**Store Layer** (`internal/store/`)
- Interface-based design (`store.go` defines `Store` interface)
- SQLite implementation in `sqlite.go`, `sqlite_users.go`, `sqlite_groups.go`, `sqlite_ous.go`
- Handles all database operations with recursive group queries using SQL CTEs
- Entry, User, Group, OU CRUD operations

**Models Layer** (`internal/models/`)
- `Entry`: Base LDAP entry (DN, object classes, attributes)
- `User`: inetOrgPerson entries (uid, cn, sn, givenName, mail, passwordHash)
- `Group`: groupOfNames entries (cn, members with nesting support)
- `OrganizationalUnit`: organizationalUnit entries (ou, description)

**Schema Layer** (`internal/schema/`)
- LDAP filter parsing and evaluation
- Supports: AND/OR/NOT, equality, presence, substring filters
- Used by search operations to filter results

**Config Layer** (`pkg/config/`)
- Environment variable-based configuration
- All settings have defaults except `LDAP_BASE_DN` and `LDAP_ADMIN_PASSWORD` (required on first run)

**Crypto Layer** (`pkg/crypto/`)
- Argon2id password hashing with OWASP-recommended parameters
- Constant-time password verification
- Configurable via environment variables

### Database Schema

SQLite tables:
- `entries`: All LDAP entries (DN, parent_dn, object_class, timestamps)
- `attributes`: Multi-valued attributes (entry_id, name, value)
- `users`: User-specific data (entry_id, uid, password_hash)
- `groups`: Group-specific data (entry_id, cn)
- `group_members`: Group membership (group_id, member_id, is_nested)
- `organizational_units`: OU-specific data (entry_id, ou)

**Group Nesting:**
- Groups can contain users AND other groups
- Recursive queries use SQL CTEs for efficient traversal
- Circular reference detection (max depth: 10 by default)
- Methods: `GetGroupMembersRecursive`, `GetUserGroupsRecursive`

### Entry Point

`cmd/ldaplite/main.go` - Cobra CLI with three commands:
- `server`: Start LDAP server
- `version`: Print version info
- `healthcheck`: Health check (TODO)

### Logging

Uses structured logging (slog) with JSON or text format:
- Levels: debug, info, warn, error
- The ldapserver library's unstructured logs are suppressed
- All logs go to stderr

## Configuration

All configuration via environment variables (see `pkg/config/config.go`):

**Required:**
- `LDAP_BASE_DN`: Base DN (e.g., dc=example,dc=com)
- `LDAP_ADMIN_PASSWORD`: Admin password (first run only)

**Server:**
- `LDAP_PORT`: Default 3389
- `LDAP_BIND_ADDRESS`: Default 0.0.0.0

**Database:**
- `LDAP_DATABASE_PATH`: Default /data/ldaplite.db
- `LDAP_DATABASE_MAX_OPEN_CONNS`: Default 25
- `LDAP_DATABASE_MAX_IDLE_CONNS`: Default 5
- `LDAP_DATABASE_CONN_MAX_LIFETIME`: Default 300s

**Security:**
- Argon2id parameters (memory, iterations, parallelism, salt/key length)

## Supported LDAP Operations

**Implemented:**
- Bind (simple bind with password verification)
- Search (with basic filter support)
- Add (create entries, users, groups, OUs)
- Delete (with validation)
- Modify (update attributes)

**Object Classes:**
- `organizationalUnit`: Containers for users/groups
- `inetOrgPerson`: Users with personal info
- `groupOfNames`: Groups with member references
- `top`: Root object class

## Development Notes

### Adding New Features

**New LDAP attributes:**
1. Update model in `internal/models/`
2. Update SQLite implementation in `internal/store/sqlite_*.go`
3. Add attribute handling in `internal/server/ldap.go`

**New LDAP operations:**
1. Add handler in `internal/server/ldap.go`
2. Register in route mux (`Start()` method)
3. Implement store method in `internal/store/`

### Testing Strategy

Tests are co-located with implementation:
- `internal/models/*_test.go`: Model validation tests
- `internal/schema/*_test.go`: Filter parsing tests
- `pkg/crypto/*_test.go`: Password hashing tests
- `pkg/config/*_test.go`: Configuration tests

Integration tests can use in-memory SQLite (`:memory:`).

### Important Implementation Details

1. **DN Normalization**: DNs are case-insensitive, normalize before comparisons
2. **Password Hashing**: Always use `crypto.PasswordHasher`, never store plaintext
3. **Group Nesting**: Always check for circular references, limit recursion depth
4. **Filter Evaluation**: Basic implementation in `internal/schema/filter.go`, may need extension for complex filters
5. **Error Handling**: Return LDAP result codes (Success, NoSuchObject, InvalidCredentials, etc.)

## Current Limitations

- No TLS/SSL support (use reverse proxy)
- No SASL authentication
- Basic filter implementation (no extensible matching)
- No schema validation beyond object class requirements
- No replication or high availability
- Single-file SQLite database (suitable for small to medium deployments)
