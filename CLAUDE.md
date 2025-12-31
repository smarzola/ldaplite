# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LDAPLite is a lightweight, LDAP-compliant server written in Go with SQLite backend. It provides a simple LDAP server implementation for modern environments with Docker support.

**Key Technologies:**
- Go 1.25.3
- SQLite database (modernc.org/sqlite)
- LDAP protocol via lor00x/goldap (low-level ASN.1 BER encoding/decoding)
- Custom TCP server implementation (no high-level LDAP server framework)
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
- Custom LDAP protocol implementation using raw TCP and goldap for ASN.1 BER encoding
- Handles: Bind, Search, Add, Delete, Modify, Compare operations
- **Routes all operations to generic store.Entry methods** - no type-specific code
- Bind operation: Uses `GetUserPasswordHash(uid)` to retrieve password hash securely
- Automatic password processing in Add/Modify operations (hashes plain text, validates pre-hashed)
- Password verification using Argon2id hasher for Bind operations
- Connection management: Per-connection goroutines with graceful shutdown

**Protocol Layer** (`internal/protocol/`)
- `transport.go`: BER message reading/writing using goldap
- `connection.go`: TCP connection lifecycle and request dispatching
- `response.go`: Helper functions for constructing LDAP responses
- Direct use of goldap message types (BindRequest, SearchRequest, etc.)
- No dependency on high-level LDAP server frameworks

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

**Design Philosophy: Optimized Single-Source Architecture (Phase 3)**

LDAPLite uses an **optimized storage strategy** that eliminates redundancy while maintaining performance:
- **Generic attributes table**: Single source of truth for ALL LDAP attributes (flexible, schema-free)
- **Specialized tables**: Store ONLY essential non-attribute data (security-sensitive passwords, referential integrity)
- **Composite indexes**: Replace redundant columns with optimized filtered indexes for fast lookups

#### Core Tables

**`entries` table** - Primary table for all LDAP entries:
- `id`: Primary key
- `dn`: Distinguished Name (unique, indexed)
- `parent_dn`: Parent DN for hierarchy (indexed for recursive queries)
- `object_class`: Primary structural object class (indexed for type filtering)
- `created_at`, `updated_at`: Timestamps for operational attributes

**`attributes` table** - Generic EAV (Entity-Attribute-Value) storage:
- Stores ALL attributes for entries (multi-valued support)
- **Exception: Security-sensitive attributes excluded** (e.g., `userPassword`)
- Indexed on `(entry_id, name, value)` for fast lookups
- Source of truth returned in LDAP search operations

#### Specialized Tables (Security + Referential Integrity Only)

**`users` table** - Security-sensitive data ONLY:
- `entry_id`: Foreign key to entries table
- `password_hash`: Password storage with LDAP scheme prefix
  - **SECURITY: Stored ONLY here, never in attributes table**
  - Accessed only during authentication (bind operations via JOIN)
  - Never exposed in LDAP search results
  - **No uid column** - looked up via JOIN with attributes table

**`groups` table** - Referential integrity marker:
- `entry_id`: Foreign key to entries table (enables group_members FK)
  - **No cn column** - all attributes in attributes table

**`group_members` table** - Group membership (many-to-many):
- Supports direct membership and nested groups
- Enables efficient recursive queries with SQL CTEs
- Circular reference detection (max depth: 10)
- Powers the `memberOf` operational attribute for user entries (RFC2307bis)

**`organizational_units` table** - Referential integrity marker:
- `entry_id`: Foreign key to entries table
  - **No ou column** - all attributes in attributes table

#### Phase 3 Optimization: Zero Redundancy

**Phase 3 eliminated ALL redundant columns** (`uid`, `cn`, `ou`) from specialized tables.

**Performance maintained via composite indexes:**
- `idx_attributes_uid_lookup ON attributes(name, value) WHERE name = 'uid'`
- `idx_attributes_cn_lookup ON attributes(name, value) WHERE name = 'cn'`
- `idx_attributes_ou_lookup ON attributes(name, value) WHERE name = 'ou'`

These filtered indexes provide equivalent performance to dedicated columns while:
- ✅ **Zero storage redundancy** (single source of truth)
- ✅ **No consistency issues** (data only in one place)
- ✅ **Same query speed** (composite index on (name, value) with WHERE filter)

**Result:** Specialized tables contain ONLY data that cannot be attributes:
- Security-sensitive data (passwords)
- Foreign key relationships (group_members)

#### Security Architecture

**Password Storage (RFC 3112 Compliant):**
- Format: `{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$salt$hash`
- Storage: `users.password_hash` column ONLY
- **Never stored in `attributes` table** (migration 003 removes if present)
- **Never returned in LDAP search operations**
- Access: Via dedicated `GetUserPasswordHash(uid)` method (bind operations only)
- Processing: `ProcessPassword()` auto-hashes plain text, validates pre-hashed passwords

**Query Patterns (Phase 3):**
- Searches: Use `attributes` table exclusively (single source of truth, no passwords)
- Bind (auth): JOIN `attributes` (for uid lookup) with `users` (for password_hash)
  - Uses `idx_attributes_uid_lookup` for fast uid→entry_id resolution
  - Then retrieves password_hash from users table
- Updates: Write ONLY to `attributes` table (except passwords → users.password_hash)

#### Group Nesting

- Groups can contain users AND other groups (via `group_members` table)
- Recursive queries use SQL CTEs for efficient traversal
- Circular reference detection (max depth: 10 by default)
- Methods: `GetGroupMembersRecursive`, `GetUserGroupsRecursive`

#### Indexes (see `002_add_indexes.up.sql` and `004_remove_redundant_columns.up.sql`)

**Performance-critical indexes:**
- `entries(dn)`: Unique, for direct lookups
- `entries(parent_dn)`: For hierarchical queries
- `entries(object_class)`: For type filtering
- `attributes(entry_id, name, value)`: For EAV searches
- **Phase 3 composite indexes** (replace dedicated column indexes):
  - `idx_attributes_uid_lookup ON (name, value) WHERE name = 'uid'`: Fast login lookups
  - `idx_attributes_cn_lookup ON (name, value) WHERE name = 'cn'`: Fast cn-based searches
  - `idx_attributes_ou_lookup ON (name, value) WHERE name = 'ou'`: Fast OU searches
- `users(entry_id)`, `groups(entry_id)`, `organizational_units(entry_id)`: FK lookups
- `group_members(group_entry_id, member_entry_id)`: Group membership queries

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
- `memberOf`: Groups the user belongs to (RFC2307bis, inetOrgPerson only)

These attributes are automatically added to all entries by the server and cannot be modified by clients. They conform to RFC 4512 (LDAP Directory Information Models). Format: `YYYYMMDDHHMMSSz` (e.g., `20251025143045Z`)

**memberOf Attribute (RFC2307bis):**
- Automatically computed for `inetOrgPerson` entries from the `group_members` table
- Contains DN of each `groupOfNames` the user is a member of
- Multi-valued: one value per group membership
- Populated by `populateMemberOf()` in `internal/store/sqlite.go`
- Defined in schema with OID `1.2.840.113556.1.2.102` (Microsoft standard, widely adopted)
- Marked as `NO-USER-MODIFICATION` and `directoryOperation` (read-only operational attribute)

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

2. **Optimized Single-Source Architecture (Phase 3)**:
   - **Single source of truth**: ALL attributes stored in `attributes` table (EAV pattern)
   - **Zero redundancy**: Specialized tables contain ONLY non-attribute data
   - **Query patterns**:
     - All searches use `attributes` table exclusively
     - Authentication uses JOIN: `attributes` (uid lookup) → `users` (password_hash)
     - Fast lookups via composite indexes on attributes table
   - **Specialized tables reduced to essentials**:
     - `users`: entry_id + password_hash (security-sensitive)
     - `groups`, `organizational_units`: entry_id only (referential integrity)
   - **Performance**: Composite filtered indexes provide same speed as dedicated columns

3. **Password Security (Critical)**:
   - **Storage Location**: `users.password_hash` column ONLY (never in `attributes` table)
   - **Format**: LDAP RFC 3112 compliant - `{ARGON2ID}$argon2id$v=19$m=65536,t=3,p=2$salt$hash`
   - **Processing**: `ProcessPassword()` in `internal/server/ldap.go` line 306, 422, 473:
     - Plain text → automatically hashed with Argon2id
     - Pre-hashed with `{ARGON2ID}` → validated and accepted as-is
     - Unsupported schemes (e.g., `{SSHA}`) → rejected with ConstraintViolation
   - **Access**: Via `GetUserPasswordHash(uid)` for bind operations only
   - **Never Exposed**: Password hashes never returned in LDAP search results
   - **Migration**: Migration 003 removes any legacy passwords from attributes table

4. **Entry Creation & Updates (Phase 3)**:
   - Use `CreateEntry` for all entry types (users, groups, OUs)
   - Automatically detects objectClass and validates accordingly
   - Writes ALL attributes to `attributes` table (except userPassword → users.password_hash)
   - **Phase 3**: Specialized tables store ONLY essential data:
     - `users`: entry_id + password_hash (no uid column)
     - `groups`: entry_id (no cn column)
     - `organizational_units`: entry_id (no ou column)
   - `CreateEntry` skips `userPassword` when writing to attributes (line 272 in sqlite.go)
   - `UpdateEntry` skips `userPassword` from attributes, updates `users.password_hash` (line 372, 382-388)
   - No type-specific Create/Update/Delete methods - everything through generic Entry API

5. **Authentication (Bind Operations - Phase 3)**:
   - Extract `uid` from DN (e.g., `uid=john,ou=users,dc=example,dc=com` → `john`)
   - Fetch password hash via `GetUserPasswordHash(uid)`:
     - JOINs `attributes` table (uid lookup using `idx_attributes_uid_lookup`)
     - With `users` table (password_hash retrieval)
     - Query: `SELECT u.password_hash FROM users u INNER JOIN attributes a ON u.entry_id = a.entry_id WHERE a.name = 'uid' AND a.value = ?`
   - Verify password using `Verify()` method (constant-time comparison)
   - Security isolation maintained (password never in search results)

6. **Group Nesting**: Always check for circular references, limit recursion depth

7. **Filter Evaluation**: Hybrid SQL + in-memory filtering in `internal/schema/filter.go`
   - Simple filters compiled to SQL for performance
   - Complex filters fall back to in-memory evaluation
   - Timestamp comparisons supported for operational attributes

8. **Error Handling**: Return proper LDAP result codes:
   - Success (0), NoSuchObject (32), InvalidCredentials (49)
   - ConstraintViolation (19) for password scheme errors
   - ObjectClassViolation (65), UnwillingToPerform (53)

## Current Limitations

- No TLS/SSL support (use reverse proxy)
- No SASL authentication
- Basic filter implementation (no extensible matching)
- No schema validation beyond object class requirements
- No replication or high availability
- Single-file SQLite database (suitable for small to medium deployments)
