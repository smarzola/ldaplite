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
- **Routes all operations to generic store.Entry methods** - no type-specific code
- Bind operation: Uses `SearchEntries` with filter `(uid=xxx)` to find user and verify password
- Automatic password processing in Add/Modify operations (hashes plain text, validates pre-hashed)
- Password verification using Argon2id hasher for Bind operations

**Store Layer** (`internal/store/`)
- Interface-based design (`store.go` defines `Store` interface)
- SQLite implementation in `sqlite.go` - single file, simple and focused
- **Pure entry-based API** - all operations work with generic `Entry` objects
- Interface methods:
  - `CreateEntry`, `GetEntry`, `UpdateEntry`, `DeleteEntry` - CRUD operations
  - `SearchEntries` - filter-based search across all entry types
  - `EntryExists` - validation helper
  - `GetAllEntries`, `GetChildren` - utility methods
- `CreateEntry` automatically detects objectClass and handles Users, Groups, OUs with proper validation

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
- LDAP RFC 3112 compliant password scheme format: `{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$salt$hash`
- `ProcessPassword()` method handles both plain text (auto-hashed) and pre-hashed passwords
- Rejects unsupported password schemes (currently only {ARGON2ID} supported)
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

**Operational Attributes (Automatic):**
- `createTimestamp`: Entry creation time in LDAP Generalized Time format (RFC 4517)
- `modifyTimestamp`: Last modification time in LDAP Generalized Time format
- `objectClass`: Structural object class

These attributes are automatically added to all entries by the server and cannot be modified by clients. They conform to RFC 4512 (LDAP Directory Information Models). Format: `YYYYMMDDHHMMSSz` (e.g., `20251025143045Z`)

## Development Notes

### Adding New Features

**New LDAP attributes:**
1. Update model in `internal/models/` (e.g., add validation in User/Group/OU struct)
2. Attributes are stored automatically via the generic `Entry.Attributes` map - no store changes needed
3. Add any special handling in `internal/server/ldap.go` if required (like userPassword processing)

**New LDAP operations:**
1. Add handler in `internal/server/ldap.go`
2. Register in route mux (`Start()` method)
3. Use existing `CreateEntry`, `GetEntry`, `UpdateEntry`, `DeleteEntry`, or `SearchEntries` store methods

**New object classes:**
1. Add model struct in `internal/models/` (embed `*Entry`)
2. Add validation method (e.g., `ValidateXxx()`)
3. Update `CreateEntry` in `sqlite.go` to detect and validate the new objectClass
4. Add type-specific table/columns if needed (similar to users/groups/organizational_units tables)

### Testing Strategy

Tests are co-located with implementation:
- `internal/models/*_test.go`: Model validation tests
- `internal/schema/*_test.go`: Filter parsing tests
- `pkg/crypto/*_test.go`: Password hashing tests
- `pkg/config/*_test.go`: Configuration tests

Integration tests can use in-memory SQLite (`:memory:`).

### Important Implementation Details

1. **DN Normalization**: DNs are case-insensitive, normalize before comparisons
2. **Password Handling**:
   - All passwords stored with LDAP scheme prefix: `{ARGON2ID}$argon2id$...`
   - LDAP Add/Modify operations automatically process `userPassword` attribute via `ProcessPassword()`
   - Plain text passwords are automatically hashed with scheme prefix
   - Pre-hashed passwords with `{ARGON2ID}` prefix are validated and accepted as-is
   - Passwords with unsupported schemes (e.g., `{SSHA}`) are rejected with ConstraintViolation
   - Never store plaintext passwords
3. **Entry Creation**:
   - Use `CreateEntry` for all entry types (users, groups, OUs)
   - It automatically detects objectClass and validates/stores accordingly
   - No type-specific Create/Update/Delete methods exist - everything goes through generic Entry methods
   - Server uses `SearchEntries` for lookups instead of type-specific Get methods (e.g., Bind uses `(uid=xxx)` filter)
4. **Group Nesting**: Always check for circular references, limit recursion depth
5. **Filter Evaluation**: Basic implementation in `internal/schema/filter.go`, may need extension for complex filters
6. **Error Handling**: Return LDAP result codes (Success, NoSuchObject, InvalidCredentials, ConstraintViolation, etc.)

## Current Limitations

- No TLS/SSL support (use reverse proxy)
- No SASL authentication
- Basic filter implementation (no extensible matching)
- No schema validation beyond object class requirements
- No replication or high availability
- Single-file SQLite database (suitable for small to medium deployments)
