# SQLite Data Layer Improvement Plan

**Date:** 2025-10-25
**Status:** Planning
**Version:** 1.0

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Current Issues Analysis](#current-issues-analysis)
3. [Improvement Phases](#improvement-phases)
4. [Implementation Roadmap](#implementation-roadmap)
5. [Testing Strategy](#testing-strategy)
6. [Expected Performance Improvements](#expected-performance-improvements)
7. [References](#references)

---

## Executive Summary

This document outlines a comprehensive plan to improve the SQLite data layer in LDAPLite. The analysis identified 5 major categories of performance and deployment issues:

1. **Migration system** - External files, not embedded in binary
2. **N+1 query patterns** - Every multi-entry fetch triggers N+1 queries for attributes
3. **In-memory filtering** - LDAP filters applied after fetching all entries
4. **Inefficient LIKE patterns** - `parent_dn LIKE '%...'` forces full table scans
5. **Filter marshalling overhead** - goldap→string→parsed→evaluated (unnecessary round-trips)

**Expected overall performance improvement:** 10-100x for typical workloads.

---

## Current Issues Analysis

### Issue 1: External Migration Files

**Location:** `internal/store/sqlite.go:73`

**Problem:**
- Migrations stored in `./migrations/*.sql` external files
- Hardcoded path: `"file://./migrations"`
- Binary deployment requires migrations directory
- Runtime working directory dependency

**Impact:**
- Deployment complexity
- Docker image requires extra layers
- Potential runtime failures if working directory is wrong

### Issue 2: N+1 Query Patterns

**Locations:**
- `internal/store/sqlite.go:362-418` (SearchEntries)
- `internal/store/sqlite.go:421-473` (GetAllEntries)
- `internal/store/sqlite.go:476-529` (GetChildren)
- `internal/store/sqlite.go:162-205` (GetEntry)
- `internal/store/sqlite_users.go:40-89` (GetUserByUID)
- `internal/store/sqlite_groups.go:228-277` (GetGroupMembers)
- `internal/store/sqlite_groups.go:323-379` (GetUserGroups)
- `internal/store/sqlite_ous.go:62-116` (SearchOUs)

**Pattern:**
```go
// Query 1: Fetch entries
rows := db.Query("SELECT id, dn FROM entries WHERE ...")

// For each entry...
for rows.Next() {
    // Query N+1: Fetch attributes
    attrs := db.Query("SELECT name, value FROM attributes WHERE entry_id = ?", entry.ID)
}
```

**Impact:**
- 100 entries = 101 queries instead of 1-2 queries
- Linear scaling problem: O(N) queries for N entries
- Database round-trip overhead multiplied

**Example:**
```
Search for users under ou=users,dc=example,dc=com with 100 results:
- Current: 1 entry query + 100 attribute queries = 101 total queries
- Optimal: 1 query with JOIN or 2 queries (batch)
```

### Issue 3: In-Memory Filter Evaluation

**Location:** `internal/server/ldap.go:134-226`

**Flow:**
```
1. goldap filter → serializeFilter() [ldap.go:156]
2. store.SearchEntries(baseDN, filterStr) [ldap.go:164]
   └─> SQL: WHERE (dn = ? OR parent_dn LIKE ?) -- filter parameter IGNORED
3. Fetch ALL entries under baseDN (with N+1 pattern)
4. schema.ParseFilter(filterStr) [ldap.go:172]
5. For each entry: filter.Matches(entry) [ldap.go:195] -- IN-MEMORY
```

**Problem:**
- Filter parameter passed to `SearchEntries()` but not used in SQL
- All entries under baseDN fetched regardless of filter
- Filter evaluation happens in Go memory after fetching

**Impact:**
```
Search: (&(objectClass=inetOrgPerson)(uid=john))
Base DN: dc=example,dc=com (500 entries total)

Current behavior:
1. Fetch all 500 entries + attributes (501 queries)
2. Parse filter
3. Filter in-memory → 1 matching entry
4. Return 1 entry

Wasted: 499 entries fetched, 500 attribute queries, in-memory filtering
```

### Issue 4: Inefficient LIKE Patterns

**Locations:**
- `internal/store/sqlite.go:371`
- `internal/store/sqlite_ous.go:70`

**Pattern:**
```go
likePattern := "%" + baseDN  // e.g., "%dc=example,dc=com"
query := `SELECT ... FROM entries WHERE parent_dn LIKE ?`
```

**Problem:**
- Leading `%` wildcard prevents index usage
- Forces full table scan on `entries` table
- Index on `parent_dn` cannot be used

**Why indexes don't work:**
```sql
-- Index CAN be used (prefix match):
WHERE parent_dn LIKE 'dc=example,dc=com%'

-- Index CANNOT be used (suffix match):
WHERE parent_dn LIKE '%dc=example,dc=com'
```

**Impact:**
- O(N) full table scan for every search
- Performance degrades linearly with total entry count
- Even searches returning 1 entry scan entire table

### Issue 5: Recursive Group Query Multiplication

**Location:** `internal/store/sqlite_groups.go:280-321`

**Pattern:**
```go
func GetGroupMembersRecursive() {
    // Calls GetGroupMembers which has N+1 pattern
    directMembers := GetGroupMembers(groupDN)

    for each member {
        if member.IsGroup() {
            // Recursive call - more GetGroupMembers
            GetGroupMembersRecursive(member.DN)
        }
    }
}
```

**Impact multiplier:**
- Each recursive level calls `GetGroupMembers()`
- Each `GetGroupMembers()` = 1 + N queries (N+1 pattern)

**Example calculation:**
```
3 levels deep, 5 groups per level, 5 members per group:

Level 1: 1 call × (1 + 5) = 6 queries
Level 2: 5 calls × (1 + 5) = 30 queries
Level 3: 25 calls × (1 + 5) = 150 queries

Total: 186 queries for one recursive expansion
```

### Issue 6: Filter Round-Trip Overhead

**Location:** `internal/server/ldap.go:156, 172`

**Flow:**
1. LDAP client sends goldap.Filter message
2. `serializeFilter()` converts to string: `"(&(uid=john)(mail=*))"` [line 156]
3. String passed to `store.SearchEntries()` (unused)
4. `schema.ParseFilter()` converts string back to Filter struct [line 172]
5. `Filter.Matches()` evaluates in memory

**Problem:**
- Unnecessary serialization/deserialization
- goldap format → string → custom Filter struct
- No benefit from string representation

### Issue 7: Incomplete Filter Implementation

**Location:** `internal/schema/filter.go:209-217`

**Missing filter types:**
```go
case FilterTypeSubstring:
    // Falls through to equality match - WRONG
case FilterTypeGreaterOrEqual:
    // Falls through to equality match - WRONG
case FilterTypeLessOrEqual:
    // Falls through to equality match - WRONG
case FilterTypeApproxMatch:
    // Falls through to equality match - WRONG
```

**Impact:**
- Substring filters like `(cn=John*)` treated as equality `(cn=John*)`
- Comparison filters not supported
- Approximate matching not supported
- Silent incorrect behavior (no error, wrong results)

### Issue 8: Index Effectiveness

**Current indexes:** `migrations/002_add_indexes.up.sql`

**Effective indexes:**
- ✅ `idx_users_uid` - Used in `GetUserByUID()`
- ✅ `idx_attributes_entry_id` - Used in attribute fetching
- ✅ `idx_group_members_group_entry_id` - Used in group queries

**Ineffective/missing indexes:**
- ❌ `idx_entries_parent_dn` - Undermined by `LIKE '%...'` pattern
- ❌ `idx_attributes_name_value` - Never queried by both name AND value
- ❌ **Missing** `idx_entries_dn` - DN is UNIQUE but no explicit index
- ❌ **Missing** composite indexes for common patterns

---

## Improvement Phases

### Phase 1: Embed Migrations

**Priority:** P0 (Prerequisite for deployment improvements)
**Effort:** 2-3 hours
**Risk:** Low
**Impact:** High (deployment simplification)

#### Objectives
- Embed migration files into binary using `embed.FS`
- Remove runtime dependency on `./migrations/` directory
- Simplify Docker images and deployment

#### Implementation

**Step 1:** Create embeddings file

```go
// internal/store/migrations.go (NEW FILE)
package store

import "embed"

//go:embed migrations/*.sql
var migrationsFS embed.FS
```

**Step 2:** Update SQLite initialization

```go
// internal/store/sqlite.go (MODIFY Initialize method)
import (
    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/source/iofs"
)

func (s *SQLiteStore) Initialize(ctx context.Context) error {
    // Create source from embedded FS
    srcDriver, err := iofs.New(migrationsFS, "migrations")
    if err != nil {
        return fmt.Errorf("failed to create migration source: %w", err)
    }

    dbURL := fmt.Sprintf("sqlite://%s", s.cfg.Database.Path)
    m, err := migrate.NewWithSourceInstance("iofs", srcDriver, dbURL)
    if err != nil {
        return fmt.Errorf("failed to initialize migrations: %w", err)
    }
    defer m.Close()

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("failed to run migrations: %w", err)
    }

    return nil
}
```

#### Files Modified
- `internal/store/migrations.go` (NEW)
- `internal/store/sqlite.go` (MODIFY lines ~68-82)

#### Testing
```bash
# Build and test
go build -o bin/ldaplite ./cmd/ldaplite

# Remove migrations directory temporarily
mv migrations migrations.bak

# Run server (should work with embedded migrations)
./bin/ldaplite server

# Restore
mv migrations.bak migrations
```

#### Benefits
- ✅ Single binary deployment
- ✅ No working directory dependency
- ✅ Simpler Docker images
- ✅ No breaking changes

---

### Phase 2: Fix N+1 Query Patterns

**Priority:** P0 (Critical performance)
**Effort:** 2-3 days
**Risk:** Medium
**Impact:** Critical (50-100x performance improvement)

#### Objectives
- Eliminate N+1 pattern in all multi-entry queries
- Fetch entries with attributes in single query or batch
- Reduce query count from O(N) to O(1)

#### Solution Options

##### Option A: JSON Aggregation (Recommended)

Use SQLite 3.38+ JSON functions to aggregate attributes:

```sql
SELECT
    e.id,
    e.dn,
    e.parent_dn,
    e.object_class,
    e.created_at,
    e.updated_at,
    json_group_array(
        CASE WHEN a.name IS NOT NULL
        THEN json_object('name', a.name, 'value', a.value)
        ELSE NULL END
    ) as attributes_json
FROM entries e
LEFT JOIN attributes a ON e.id = a.entry_id
WHERE (e.dn = ? OR e.parent_dn = ?)
GROUP BY e.id
```

**Decoding function:**

```go
// internal/store/sqlite.go (ADD)

type attrPair struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

func decodeAttributesJSON(jsonStr string) (map[string][]string, error) {
    if jsonStr == "" || jsonStr == "null" || jsonStr == "[null]" {
        return make(map[string][]string), nil
    }

    var pairs []attrPair
    if err := json.Unmarshal([]byte(jsonStr), &pairs); err != nil {
        return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
    }

    attrs := make(map[string][]string)
    for _, p := range pairs {
        if p.Name == "" {
            continue // Skip null entries
        }
        name := strings.ToLower(p.Name)
        attrs[name] = append(attrs[name], p.Value)
    }
    return attrs, nil
}
```

**Requirements:**
- SQLite 3.38+ (released 2022-02-22)
- Check `modernc.org/sqlite` version supports this

**Pros:**
- ✅ Single query for entries + attributes
- ✅ No escaping issues
- ✅ Handles arbitrary attribute values
- ✅ Clean separation (SQL generates JSON, Go parses)

**Cons:**
- ⚠️ Requires SQLite 3.38+
- ⚠️ JSON parsing overhead (minimal vs N queries)
- ⚠️ More complex SQL

##### Option B: GROUP_CONCAT (Fallback)

For older SQLite versions:

```sql
SELECT
    e.id,
    e.dn,
    e.parent_dn,
    e.object_class,
    e.created_at,
    e.updated_at,
    GROUP_CONCAT(a.name || '=' || a.value, '||') as attributes_encoded
FROM entries e
LEFT JOIN attributes a ON e.id = a.entry_id
WHERE (e.dn = ? OR e.parent_dn = ?)
GROUP BY e.id
```

**Decoding function:**

```go
func decodeAttributesConcat(blob string) (map[string][]string, error) {
    attrs := make(map[string][]string)
    if blob == "" {
        return attrs, nil
    }

    // Split on delimiter
    pairs := strings.Split(blob, "||")
    for _, pair := range pairs {
        // Split name=value (only first =)
        parts := strings.SplitN(pair, "=", 2)
        if len(parts) != 2 {
            continue
        }

        name := strings.ToLower(parts[0])
        value := parts[1]
        attrs[name] = append(attrs[name], value)
    }
    return attrs, nil
}
```

**Pros:**
- ✅ Works with all SQLite versions
- ✅ Single query

**Cons:**
- ⚠️ 1MB default limit for GROUP_CONCAT
- ⚠️ Escaping needed if values contain `=` or `||`
- ⚠️ More fragile parsing

##### Option C: Batch Loading (Alternative)

Keep separate queries but batch them:

```go
// Query 1: Fetch all entries
entryIDs := []int{}
entries := []*models.Entry{}
for rows.Next() {
    var entry models.Entry
    rows.Scan(&entry.ID, &entry.DN, ...)
    entryIDs = append(entryIDs, entry.ID)
    entries = append(entries, &entry)
}

// Query 2: Fetch ALL attributes for ALL entries in one query
placeholders := strings.Repeat("?,", len(entryIDs)-1) + "?"
attrQuery := fmt.Sprintf(
    `SELECT entry_id, name, value FROM attributes WHERE entry_id IN (%s)`,
    placeholders,
)

args := make([]interface{}, len(entryIDs))
for i, id := range entryIDs {
    args[i] = id
}

attrRows := db.Query(attrQuery, args...)

// Build map: entry_id -> attributes
attrMap := make(map[int]map[string][]string)
for attrRows.Next() {
    var entryID int
    var name, value string
    attrRows.Scan(&entryID, &name, &value)

    if attrMap[entryID] == nil {
        attrMap[entryID] = make(map[string][]string)
    }
    attrMap[entryID][name] = append(attrMap[entryID][name], value)
}

// Assign attributes to entries
for _, entry := range entries {
    entry.Attributes = attrMap[entry.ID]
    if entry.Attributes == nil {
        entry.Attributes = make(map[string][]string)
    }
}
```

**Pros:**
- ✅ Works with all SQLite versions
- ✅ Simple to implement
- ✅ No JSON parsing

**Cons:**
- ⚠️ Two queries instead of one
- ⚠️ IN clause has limits (SQLite default: 999, max: 32766)
- ⚠️ More round-trips than Option A

#### Recommended Approach

**Use Option A (JSON aggregation)** as primary implementation, with fallback:

1. Check SQLite version at runtime
2. If SQLite >= 3.38: use JSON aggregation
3. Else: use Option C (batch loading)

#### Implementation Steps

**Step 1:** Add helper functions

```go
// internal/store/sqlite_helpers.go (NEW FILE)
package store

import (
    "encoding/json"
    "fmt"
    "strings"
)

type attrPair struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

// decodeAttributesJSON decodes JSON array of {name, value} pairs
func decodeAttributesJSON(jsonStr string) (map[string][]string, error) {
    if jsonStr == "" || jsonStr == "null" || jsonStr == "[null]" {
        return make(map[string][]string), nil
    }

    var pairs []attrPair
    if err := json.Unmarshal([]byte(jsonStr), &pairs); err != nil {
        return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
    }

    attrs := make(map[string][]string)
    for _, p := range pairs {
        if p.Name == "" {
            continue
        }
        name := strings.ToLower(p.Name)
        attrs[name] = append(attrs[name], p.Value)
    }
    return attrs, nil
}

// getSQLiteVersion returns SQLite version string
func (s *SQLiteStore) getSQLiteVersion(ctx context.Context) (string, error) {
    var version string
    err := s.db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
    return version, err
}

// supportsJSONFunctions checks if SQLite version supports json_group_array
func (s *SQLiteStore) supportsJSONFunctions(ctx context.Context) bool {
    version, err := s.getSQLiteVersion(ctx)
    if err != nil {
        return false
    }

    // SQLite 3.38.0+ has json_group_array
    // Simple version check (improve if needed)
    return strings.HasPrefix(version, "3.38") ||
           strings.HasPrefix(version, "3.39") ||
           strings.HasPrefix(version, "3.4") ||
           strings.HasPrefix(version, "3.5")
}
```

**Step 2:** Update SearchEntries

```go
// internal/store/sqlite.go (REPLACE SearchEntries method)

func (s *SQLiteStore) SearchEntries(ctx context.Context, baseDN string, filter string) ([]*models.Entry, error) {
    // Build query with JSON aggregation
    query := `
        SELECT
            e.id,
            e.dn,
            e.parent_dn,
            e.object_class,
            e.created_at,
            e.updated_at,
            json_group_array(
                CASE WHEN a.name IS NOT NULL
                THEN json_object('name', a.name, 'value', a.value)
                ELSE NULL END
            ) as attributes_json
        FROM entries e
        LEFT JOIN attributes a ON e.id = a.entry_id
        WHERE (e.dn = ? OR e.parent_dn = ?)
        GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
    `

    rows, err := s.db.QueryContext(ctx, query, baseDN, baseDN)
    if err != nil {
        return nil, fmt.Errorf("failed to search entries: %w", err)
    }
    defer rows.Close()

    var entries []*models.Entry
    for rows.Next() {
        entry := &models.Entry{}
        var attrsJSON string

        err := rows.Scan(
            &entry.ID,
            &entry.DN,
            &entry.ParentDN,
            &entry.ObjectClass,
            &entry.CreatedAt,
            &entry.UpdatedAt,
            &attrsJSON,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan entry: %w", err)
        }

        // Decode attributes from JSON
        entry.Attributes, err = decodeAttributesJSON(attrsJSON)
        if err != nil {
            return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
        }

        // Add objectClass to attributes
        if entry.ObjectClass != "" {
            entry.Attributes["objectclass"] = []string{entry.ObjectClass}
        }

        entries = append(entries, entry)
    }

    return entries, nil
}
```

**Step 3:** Update other query methods

Apply same pattern to:
- `GetEntry()` - lines 162-205
- `GetAllEntries()` - lines 421-473
- `GetChildren()` - lines 476-529
- `GetUserByUID()` in `sqlite_users.go` - lines 40-89
- `GetGroupMembers()` in `sqlite_groups.go` - lines 228-277
- `GetGroupByName()` in `sqlite_groups.go` - lines 40-90
- `GetUserGroups()` in `sqlite_groups.go` - lines 323-379
- `SearchOUs()` in `sqlite_ous.go` - lines 62-116

#### Files Modified
- `internal/store/sqlite_helpers.go` (NEW)
- `internal/store/sqlite.go` (MODIFY 4 methods)
- `internal/store/sqlite_users.go` (MODIFY 1 method)
- `internal/store/sqlite_groups.go` (MODIFY 3 methods)
- `internal/store/sqlite_ous.go` (MODIFY 1 method)

#### Testing

**Unit tests:**

```go
// internal/store/sqlite_test.go (ADD)

func TestDecodeAttributesJSON(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected map[string][]string
        wantErr  bool
    }{
        {
            name:     "empty",
            input:    "",
            expected: map[string][]string{},
        },
        {
            name:  "single attribute",
            input: `[{"name":"cn","value":"John Doe"}]`,
            expected: map[string][]string{
                "cn": {"John Doe"},
            },
        },
        {
            name:  "multi-valued attribute",
            input: `[{"name":"mail","value":"john@example.com"},{"name":"mail","value":"jdoe@example.com"}]`,
            expected: map[string][]string{
                "mail": {"john@example.com", "jdoe@example.com"},
            },
        },
        {
            name:     "null array",
            input:    "[null]",
            expected: map[string][]string{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := decodeAttributesJSON(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("decodeAttributesJSON() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            // Compare maps
            if !reflect.DeepEqual(got, tt.expected) {
                t.Errorf("decodeAttributesJSON() = %v, want %v", got, tt.expected)
            }
        })
    }
}

func TestSearchEntriesPerformance(t *testing.T) {
    // Setup: Create 100 test entries
    store := setupTestStore(t)
    baseDN := "dc=example,dc=com"

    // Create 100 users with 5 attributes each
    for i := 0; i < 100; i++ {
        uid := fmt.Sprintf("user%d", i)
        dn := fmt.Sprintf("uid=%s,ou=users,%s", uid, baseDN)

        entry := &models.Entry{
            DN:          dn,
            ParentDN:    fmt.Sprintf("ou=users,%s", baseDN),
            ObjectClass: "inetOrgPerson",
            Attributes: map[string][]string{
                "uid":       {uid},
                "cn":        {fmt.Sprintf("User %d", i)},
                "sn":        {fmt.Sprintf("Last%d", i)},
                "givenName": {fmt.Sprintf("First%d", i)},
                "mail":      {fmt.Sprintf("user%d@example.com", i)},
            },
        }

        err := store.CreateEntry(context.Background(), entry)
        require.NoError(t, err)
    }

    // Measure query count
    // Note: This requires instrumentation or query logging
    // For now, just test that it works

    start := time.Now()
    entries, err := store.SearchEntries(context.Background(), baseDN, "")
    duration := time.Since(start)

    require.NoError(t, err)
    assert.Len(t, entries, 100)

    // Should be fast (< 100ms for 100 entries)
    assert.Less(t, duration, 100*time.Millisecond)

    // Verify attributes loaded
    for _, entry := range entries {
        assert.NotEmpty(t, entry.Attributes, "Entry %s should have attributes", entry.DN)
    }
}
```

**Integration test:**

```bash
# Create test database with 1000 entries
go run test/populate_db.go

# Benchmark before and after
go test -bench=BenchmarkSearchEntries -benchmem

# Expected improvement:
# Before: ~100ms, 101 allocs
# After:  ~10ms, 1 alloc
```

#### Benefits
- ✅ 50-100x performance improvement
- ✅ Eliminates N+1 pattern
- ✅ Reduces query count from O(N) to O(1)
- ✅ Better scalability

---

### Phase 3: Push LDAP Filters to SQL

**Priority:** P1 (High performance impact)
**Effort:** 1 week
**Risk:** High (complex logic)
**Impact:** High (10-100x for filtered searches)

#### Objectives
- Compile LDAP filters to SQL WHERE clauses
- Push filter evaluation to database layer
- Only fetch entries matching filter criteria
- Eliminate in-memory filtering

#### Background

**Current flow:**
```
1. goldap.Filter → serializeFilter() → string
2. store.SearchEntries(baseDN, filterStr) -- filterStr IGNORED
3. Fetch ALL entries under baseDN (1000 entries)
4. schema.ParseFilter(filterStr) → Filter struct
5. For each entry: filter.Matches(entry) -- in-memory
6. Return 5 matching entries

Wasted: 995 entries fetched unnecessarily
```

**Target flow:**
```
1. goldap.Filter → serializeFilter() → string
2. schema.ParseFilter(filterStr) → Filter struct
3. schema.CompileToSQL(filter) → SQL WHERE clause
4. store.SearchEntries executes:
   SELECT ... WHERE (base DN match) AND (filter SQL)
5. Database returns only 5 matching entries
6. Return 5 entries

No wasted fetches, no in-memory filtering
```

#### Implementation Strategy

**Hybrid approach:**
- Phase 3a: Implement SQL compilation for simple filters (1-2 days)
- Phase 3b: Expand to complex filters (3-4 days)
- Phase 3c: Fallback to in-memory for unsupported filters (1 day)

#### Phase 3a: Simple Filter Compilation

**Supported filters:**
- Equality: `(uid=jdoe)`
- Present: `(mail=*)`
- AND: `(&(uid=jdoe)(objectClass=inetOrgPerson))`
- OR: `(|(uid=jdoe)(uid=jane))`
- NOT: `(!(uid=admin))`

**Implementation:**

```go
// internal/schema/filter_compiler.go (NEW FILE)
package schema

import (
    "fmt"
    "strings"
)

type FilterCompiler struct{}

// CompileToSQL converts LDAP filter to SQL WHERE clause
// Returns: (whereClause, args, error)
func (fc *FilterCompiler) CompileToSQL(filter *Filter) (string, []interface{}, error) {
    if filter == nil {
        return "", nil, fmt.Errorf("filter is nil")
    }

    switch filter.Type {
    case FilterTypeAnd:
        return fc.compileAnd(filter.SubFilters)
    case FilterTypeOr:
        return fc.compileOr(filter.SubFilters)
    case FilterTypeNot:
        return fc.compileNot(filter.SubFilters[0])
    case FilterTypeEquality:
        return fc.compileEquality(filter.Attribute, filter.Value)
    case FilterTypePresent:
        return fc.compilePresent(filter.Attribute)
    default:
        return "", nil, fmt.Errorf("unsupported filter type: %d", filter.Type)
    }
}

// CanCompileToSQL checks if filter can be compiled to SQL
func (fc *FilterCompiler) CanCompileToSQL(filter *Filter) bool {
    if filter == nil {
        return false
    }

    switch filter.Type {
    case FilterTypeEquality, FilterTypePresent:
        return true
    case FilterTypeAnd, FilterTypeOr:
        for _, sf := range filter.SubFilters {
            if !fc.CanCompileToSQL(sf) {
                return false
            }
        }
        return true
    case FilterTypeNot:
        return len(filter.SubFilters) == 1 && fc.CanCompileToSQL(filter.SubFilters[0])
    default:
        return false
    }
}

func (fc *FilterCompiler) compileEquality(attr, value string) (string, []interface{}, error) {
    attrLower := strings.ToLower(attr)

    // Special case: objectClass is in entries table
    if attrLower == "objectclass" {
        return "e.object_class = ?", []interface{}{value}, nil
    }

    // All other attributes in attributes table
    // Use EXISTS for efficiency
    clause := `EXISTS (
        SELECT 1 FROM attributes a
        WHERE a.entry_id = e.id
          AND LOWER(a.name) = LOWER(?)
          AND a.value = ?
    )`
    return clause, []interface{}{attr, value}, nil
}

func (fc *FilterCompiler) compilePresent(attr string) (string, []interface{}, error) {
    attrLower := strings.ToLower(attr)

    // Special case: objectClass
    if attrLower == "objectclass" {
        return "e.object_class IS NOT NULL AND e.object_class != ''", nil, nil
    }

    // Check attribute exists
    clause := `EXISTS (
        SELECT 1 FROM attributes a
        WHERE a.entry_id = e.id
          AND LOWER(a.name) = LOWER(?)
    )`
    return clause, []interface{}{attr}, nil
}

func (fc *FilterCompiler) compileAnd(subFilters []*Filter) (string, []interface{}, error) {
    if len(subFilters) == 0 {
        return "1=1", nil, nil // Always true
    }

    var clauses []string
    var allArgs []interface{}

    for _, sf := range subFilters {
        clause, args, err := fc.CompileToSQL(sf)
        if err != nil {
            return "", nil, err
        }
        clauses = append(clauses, "("+clause+")")
        allArgs = append(allArgs, args...)
    }

    return strings.Join(clauses, " AND "), allArgs, nil
}

func (fc *FilterCompiler) compileOr(subFilters []*Filter) (string, []interface{}, error) {
    if len(subFilters) == 0 {
        return "1=0", nil, nil // Always false
    }

    var clauses []string
    var allArgs []interface{}

    for _, sf := range subFilters {
        clause, args, err := fc.CompileToSQL(sf)
        if err != nil {
            return "", nil, err
        }
        clauses = append(clauses, "("+clause+")")
        allArgs = append(allArgs, args...)
    }

    return strings.Join(clauses, " OR "), allArgs, nil
}

func (fc *FilterCompiler) compileNot(subFilter *Filter) (string, []interface{}, error) {
    clause, args, err := fc.CompileToSQL(subFilter)
    if err != nil {
        return "", nil, err
    }

    return "NOT (" + clause + ")", args, nil
}
```

**Update SearchEntries:**

```go
// internal/store/sqlite.go (MODIFY SearchEntries)

func (s *SQLiteStore) SearchEntries(ctx context.Context, baseDN string, filterStr string) ([]*models.Entry, error) {
    // Parse filter
    filter, err := schema.ParseFilter(filterStr)
    if err != nil {
        return nil, fmt.Errorf("invalid filter: %w", err)
    }

    // Try to compile filter to SQL
    compiler := &schema.FilterCompiler{}
    var filterClause string
    var filterArgs []interface{}
    var useInMemoryFilter bool

    if compiler.CanCompileToSQL(filter) {
        filterClause, filterArgs, err = compiler.CompileToSQL(filter)
        if err != nil {
            return nil, fmt.Errorf("failed to compile filter: %w", err)
        }
        useInMemoryFilter = false
    } else {
        // Fallback: no filter in SQL, will filter in-memory
        filterClause = "1=1"
        filterArgs = nil
        useInMemoryFilter = true
    }

    // Build query with filter clause
    query := `
        SELECT
            e.id,
            e.dn,
            e.parent_dn,
            e.object_class,
            e.created_at,
            e.updated_at,
            json_group_array(
                CASE WHEN a.name IS NOT NULL
                THEN json_object('name', a.name, 'value', a.value)
                ELSE NULL END
            ) as attributes_json
        FROM entries e
        LEFT JOIN attributes a ON e.id = a.entry_id
        WHERE (e.dn = ? OR e.parent_dn = ?)
          AND (` + filterClause + `)
        GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
    `

    // Combine args: baseDN args + filter args
    args := []interface{}{baseDN, baseDN}
    args = append(args, filterArgs...)

    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to search entries: %w", err)
    }
    defer rows.Close()

    var entries []*models.Entry
    for rows.Next() {
        entry := &models.Entry{}
        var attrsJSON string

        err := rows.Scan(
            &entry.ID,
            &entry.DN,
            &entry.ParentDN,
            &entry.ObjectClass,
            &entry.CreatedAt,
            &entry.UpdatedAt,
            &attrsJSON,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan entry: %w", err)
        }

        // Decode attributes
        entry.Attributes, err = decodeAttributesJSON(attrsJSON)
        if err != nil {
            return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
        }

        // Add objectClass
        if entry.ObjectClass != "" {
            entry.Attributes["objectclass"] = []string{entry.ObjectClass}
        }

        // Apply in-memory filter if needed (fallback)
        if useInMemoryFilter {
            if !filter.Matches(entry) {
                continue
            }
        }

        entries = append(entries, entry)
    }

    return entries, nil
}
```

#### Phase 3b: Substring Filter Compilation

**Substring patterns:**
- Initial: `(cn=John*)` → `cn LIKE 'John%'`
- Final: `(cn=*Doe)` → `cn LIKE '%Doe'`
- Any: `(cn=*oh*oe)` → `cn LIKE '%oh%oe%'`

**Implementation:**

```go
// internal/schema/filter_compiler.go (ADD)

func (fc *FilterCompiler) compileSubstring(attr, initial string, any []string, final string) (string, []interface{}, error) {
    attrLower := strings.ToLower(attr)

    // objectClass doesn't support substring
    if attrLower == "objectclass" {
        return "", nil, fmt.Errorf("substring filter not supported for objectClass")
    }

    // Build LIKE pattern
    pattern := ""

    if initial != "" {
        pattern = initial
    }

    pattern += "%"

    for _, a := range any {
        pattern += a + "%"
    }

    if final != "" {
        // Remove trailing % before adding final
        pattern = strings.TrimSuffix(pattern, "%")
        pattern += final
    }

    // Escape SQL LIKE special chars
    pattern = strings.ReplaceAll(pattern, "_", "\\_")
    // Note: % is intentional, don't escape

    clause := `EXISTS (
        SELECT 1 FROM attributes a
        WHERE a.entry_id = e.id
          AND LOWER(a.name) = LOWER(?)
          AND a.value LIKE ? ESCAPE '\'
    )`

    return clause, []interface{}{attr, pattern}, nil
}
```

**Update CompileToSQL:**

```go
func (fc *FilterCompiler) CompileToSQL(filter *Filter) (string, []interface{}, error) {
    // ... existing cases ...
    case FilterTypeSubstring:
        return fc.compileSubstring(
            filter.Attribute,
            filter.SubstringInitial,
            filter.SubstringAny,
            filter.SubstringFinal,
        )
    // ...
}
```

**Update CanCompileToSQL:**

```go
func (fc *FilterCompiler) CanCompileToSQL(filter *Filter) bool {
    // ... existing cases ...
    case FilterTypeSubstring:
        // Can compile unless it's objectClass
        return strings.ToLower(filter.Attribute) != "objectclass"
    // ...
}
```

#### Files Modified
- `internal/schema/filter_compiler.go` (NEW)
- `internal/schema/filter_compiler_test.go` (NEW)
- `internal/store/sqlite.go` (MODIFY SearchEntries)
- `internal/server/ldap.go` (OPTIONAL: remove in-memory filter.Matches if all filters compiled)

#### Testing

**Unit tests:**

```go
// internal/schema/filter_compiler_test.go (NEW)

func TestCompileEquality(t *testing.T) {
    compiler := &FilterCompiler{}

    tests := []struct {
        name          string
        filter        *Filter
        expectedSQL   string
        expectedArgs  []interface{}
    }{
        {
            name: "objectClass equality",
            filter: &Filter{
                Type:      FilterTypeEquality,
                Attribute: "objectClass",
                Value:     "inetOrgPerson",
            },
            expectedSQL:  "e.object_class = ?",
            expectedArgs: []interface{}{"inetOrgPerson"},
        },
        {
            name: "attribute equality",
            filter: &Filter{
                Type:      FilterTypeEquality,
                Attribute: "uid",
                Value:     "jdoe",
            },
            expectedSQL:  "EXISTS (\n        SELECT 1 FROM attributes a \n        WHERE a.entry_id = e.id \n          AND LOWER(a.name) = LOWER(?) \n          AND a.value = ?\n    )",
            expectedArgs: []interface{}{"uid", "jdoe"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            sql, args, err := compiler.CompileToSQL(tt.filter)
            require.NoError(t, err)
            assert.Equal(t, tt.expectedSQL, sql)
            assert.Equal(t, tt.expectedArgs, args)
        })
    }
}

func TestCompileAnd(t *testing.T) {
    compiler := &FilterCompiler{}

    filter := &Filter{
        Type: FilterTypeAnd,
        SubFilters: []*Filter{
            {
                Type:      FilterTypeEquality,
                Attribute: "uid",
                Value:     "jdoe",
            },
            {
                Type:      FilterTypeEquality,
                Attribute: "objectClass",
                Value:     "inetOrgPerson",
            },
        },
    }

    sql, args, err := compiler.CompileToSQL(filter)
    require.NoError(t, err)

    // Should contain AND
    assert.Contains(t, sql, " AND ")

    // Should have 3 args: uid, jdoe, inetOrgPerson
    assert.Len(t, args, 3)
}

func TestCanCompileToSQL(t *testing.T) {
    compiler := &FilterCompiler{}

    tests := []struct {
        name     string
        filter   *Filter
        expected bool
    }{
        {
            name: "simple equality",
            filter: &Filter{
                Type:      FilterTypeEquality,
                Attribute: "uid",
                Value:     "jdoe",
            },
            expected: true,
        },
        {
            name: "present",
            filter: &Filter{
                Type:      FilterTypePresent,
                Attribute: "mail",
            },
            expected: true,
        },
        {
            name: "substring",
            filter: &Filter{
                Type:             FilterTypeSubstring,
                Attribute:        "cn",
                SubstringInitial: "John",
            },
            expected: true,
        },
        {
            name: "unsupported type",
            filter: &Filter{
                Type:      FilterTypeGreaterOrEqual,
                Attribute: "age",
                Value:     "18",
            },
            expected: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := compiler.CanCompileToSQL(tt.filter)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

**Integration test:**

```go
// internal/store/sqlite_test.go (ADD)

func TestSearchEntriesWithFilter(t *testing.T) {
    store := setupTestStore(t)
    ctx := context.Background()
    baseDN := "dc=example,dc=com"

    // Create test entries
    createTestEntry(t, store, "uid=jdoe,ou=users,"+baseDN, "inetOrgPerson", map[string][]string{
        "uid":  {"jdoe"},
        "cn":   {"John Doe"},
        "mail": {"john@example.com"},
    })
    createTestEntry(t, store, "uid=jane,ou=users,"+baseDN, "inetOrgPerson", map[string][]string{
        "uid":  {"jane"},
        "cn":   {"Jane Smith"},
        "mail": {"jane@example.com"},
    })
    createTestEntry(t, store, "cn=admins,ou=groups,"+baseDN, "groupOfNames", map[string][]string{
        "cn": {"admins"},
    })

    tests := []struct {
        name          string
        filter        string
        expectedCount int
        expectedDNs   []string
    }{
        {
            name:          "filter by objectClass",
            filter:        "(objectClass=inetOrgPerson)",
            expectedCount: 2,
            expectedDNs:   []string{"uid=jdoe,ou=users," + baseDN, "uid=jane,ou=users," + baseDN},
        },
        {
            name:          "filter by uid",
            filter:        "(uid=jdoe)",
            expectedCount: 1,
            expectedDNs:   []string{"uid=jdoe,ou=users," + baseDN},
        },
        {
            name:          "AND filter",
            filter:        "(&(objectClass=inetOrgPerson)(uid=jane))",
            expectedCount: 1,
            expectedDNs:   []string{"uid=jane,ou=users," + baseDN},
        },
        {
            name:          "OR filter",
            filter:        "(|(uid=jdoe)(cn=admins))",
            expectedCount: 2,
        },
        {
            name:          "substring filter",
            filter:        "(cn=John*)",
            expectedCount: 1,
            expectedDNs:   []string{"uid=jdoe,ou=users," + baseDN},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            entries, err := store.SearchEntries(ctx, baseDN, tt.filter)
            require.NoError(t, err)

            assert.Len(t, entries, tt.expectedCount, "Expected %d entries, got %d", tt.expectedCount, len(entries))

            if len(tt.expectedDNs) > 0 {
                var actualDNs []string
                for _, e := range entries {
                    actualDNs = append(actualDNs, e.DN)
                }
                assert.ElementsMatch(t, tt.expectedDNs, actualDNs)
            }
        })
    }
}
```

#### Benefits
- ✅ 10-100x performance for filtered searches
- ✅ Database-level filtering (indexed, optimized)
- ✅ Only fetch matching entries
- ✅ Hybrid approach: fallback to in-memory for complex filters

---

### Phase 4: Fix LIKE Pattern Index Usage

**Priority:** P0 (Critical performance)
**Effort:** 1-2 days
**Risk:** Medium
**Impact:** High (eliminates full table scans)

#### Objectives
- Eliminate leading `%` wildcard in LIKE patterns
- Enable index usage for hierarchy queries
- Use recursive CTEs for DN tree traversal

#### Problem Analysis

**Current code:**
```go
likePattern := "%" + baseDN  // e.g., "%dc=example,dc=com"
query := `SELECT ... WHERE parent_dn LIKE ?`
```

**Why it's slow:**
- SQLite cannot use index with leading `%`
- Forces full table scan of `entries` table
- Performance degrades with table size

#### Solution: Recursive CTE

Replace LIKE pattern with recursive Common Table Expression (CTE):

```sql
-- Base case: exact DN match
SELECT * FROM entries WHERE dn = ?

UNION ALL

-- Recursive case: find children using indexed parent_dn
WITH RECURSIVE subtree AS (
    -- Base: entries directly under baseDN
    SELECT id, dn, parent_dn, object_class, created_at, updated_at
    FROM entries
    WHERE parent_dn = ?

    UNION ALL

    -- Recursive: children of children
    SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
    FROM entries e
    INNER JOIN subtree s ON e.parent_dn = s.dn
)
SELECT * FROM subtree
```

#### Implementation

**Step 1:** Create helper for recursive search

```go
// internal/store/sqlite.go (ADD new method)

// searchEntriesRecursive finds all entries in DN subtree using recursive CTE
func (s *SQLiteStore) searchEntriesRecursive(ctx context.Context, baseDN string) ([]*models.Entry, error) {
    query := `
        WITH RECURSIVE subtree AS (
            -- Base case: exact DN match
            SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
            FROM entries e
            WHERE e.dn = ?

            UNION ALL

            -- Recursive case: children
            SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
            FROM entries e
            INNER JOIN subtree s ON e.parent_dn = s.dn
        )
        SELECT
            s.id, s.dn, s.parent_dn, s.object_class, s.created_at, s.updated_at,
            json_group_array(
                CASE WHEN a.name IS NOT NULL
                THEN json_object('name', a.name, 'value', a.value)
                ELSE NULL END
            ) as attributes_json
        FROM subtree s
        LEFT JOIN attributes a ON s.id = a.entry_id
        GROUP BY s.id, s.dn, s.parent_dn, s.object_class, s.created_at, s.updated_at
    `

    rows, err := s.db.QueryContext(ctx, query, baseDN)
    if err != nil {
        return nil, fmt.Errorf("failed to search entries recursively: %w", err)
    }
    defer rows.Close()

    var entries []*models.Entry
    for rows.Next() {
        entry := &models.Entry{}
        var attrsJSON string

        err := rows.Scan(
            &entry.ID,
            &entry.DN,
            &entry.ParentDN,
            &entry.ObjectClass,
            &entry.CreatedAt,
            &entry.UpdatedAt,
            &attrsJSON,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan entry: %w", err)
        }

        entry.Attributes, err = decodeAttributesJSON(attrsJSON)
        if err != nil {
            return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
        }

        if entry.ObjectClass != "" {
            entry.Attributes["objectclass"] = []string{entry.ObjectClass}
        }

        entries = append(entries, entry)
    }

    return entries, nil
}
```

**Step 2:** Update SearchEntries to use recursive CTE

```go
// internal/store/sqlite.go (REPLACE SearchEntries - combines with Phase 2 & 3 changes)

func (s *SQLiteStore) SearchEntries(ctx context.Context, baseDN string, filterStr string) ([]*models.Entry, error) {
    // Parse filter
    filter, err := schema.ParseFilter(filterStr)
    if err != nil {
        return nil, fmt.Errorf("invalid filter: %w", err)
    }

    // Compile filter to SQL
    compiler := &schema.FilterCompiler{}
    var filterClause string
    var filterArgs []interface{}
    var useInMemoryFilter bool

    if compiler.CanCompileToSQL(filter) {
        filterClause, filterArgs, err = compiler.CompileToSQL(filter)
        if err != nil {
            return nil, fmt.Errorf("failed to compile filter: %w", err)
        }
        useInMemoryFilter = false
    } else {
        filterClause = "1=1"
        filterArgs = nil
        useInMemoryFilter = true
    }

    // Build query with recursive CTE and filter
    query := `
        WITH RECURSIVE subtree AS (
            -- Base case: exact DN match
            SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
            FROM entries e
            WHERE e.dn = ?

            UNION ALL

            -- Recursive case: children (uses index on parent_dn)
            SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
            FROM entries e
            INNER JOIN subtree s ON e.parent_dn = s.dn
        )
        SELECT
            s.id, s.dn, s.parent_dn, s.object_class, s.created_at, s.updated_at,
            json_group_array(
                CASE WHEN a.name IS NOT NULL
                THEN json_object('name', a.name, 'value', a.value)
                ELSE NULL END
            ) as attributes_json
        FROM subtree s
        LEFT JOIN attributes a ON s.id = a.entry_id
        WHERE (` + filterClause + `)
        GROUP BY s.id, s.dn, s.parent_dn, s.object_class, s.created_at, s.updated_at
    `

    // Args: baseDN for CTE + filter args
    args := []interface{}{baseDN}
    args = append(args, filterArgs...)

    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to search entries: %w", err)
    }
    defer rows.Close()

    var entries []*models.Entry
    for rows.Next() {
        entry := &models.Entry{}
        var attrsJSON string

        err := rows.Scan(
            &entry.ID,
            &entry.DN,
            &entry.ParentDN,
            &entry.ObjectClass,
            &entry.CreatedAt,
            &entry.UpdatedAt,
            &attrsJSON,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan entry: %w", err)
        }

        entry.Attributes, err = decodeAttributesJSON(attrsJSON)
        if err != nil {
            return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
        }

        if entry.ObjectClass != "" {
            entry.Attributes["objectclass"] = []string{entry.ObjectClass}
        }

        // Fallback: in-memory filter if needed
        if useInMemoryFilter && !filter.Matches(entry) {
            continue
        }

        entries = append(entries, entry)
    }

    return entries, nil
}
```

**Step 3:** Update SearchOUs similarly

```go
// internal/store/sqlite_ous.go (MODIFY SearchOUs method)

func (s *SQLiteStore) SearchOUs(ctx context.Context, baseDN string) ([]*models.OrganizationalUnit, error) {
    query := `
        WITH RECURSIVE subtree AS (
            SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
            FROM entries e
            WHERE e.dn = ? AND e.object_class = ?

            UNION ALL

            SELECT e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
            FROM entries e
            INNER JOIN subtree s ON e.parent_dn = s.dn
            WHERE e.object_class = ?
        )
        SELECT
            s.id, s.dn, s.parent_dn, s.object_class, s.created_at, s.updated_at,
            o.ou,
            json_group_array(
                CASE WHEN a.name IS NOT NULL
                THEN json_object('name', a.name, 'value', a.value)
                ELSE NULL END
            ) as attributes_json
        FROM subtree s
        INNER JOIN organizational_units o ON s.id = o.entry_id
        LEFT JOIN attributes a ON s.id = a.entry_id
        GROUP BY s.id
    `

    objectClass := string(models.ObjectClassOrganizationalUnit)
    rows, err := s.db.QueryContext(ctx, query, baseDN, objectClass, objectClass)
    // ... rest similar to SearchEntries
}
```

#### Files Modified
- `internal/store/sqlite.go` (MODIFY SearchEntries, GetChildren)
- `internal/store/sqlite_ous.go` (MODIFY SearchOUs)

#### Testing

```go
// internal/store/sqlite_test.go (ADD)

func TestSearchEntriesRecursivePerformance(t *testing.T) {
    store := setupTestStore(t)
    ctx := context.Background()
    baseDN := "dc=example,dc=com"

    // Create deep hierarchy: 5 levels, 10 entries per level = 50 total
    createDeepHierarchy(t, store, baseDN, 5, 10)

    // Measure performance
    start := time.Now()
    entries, err := store.SearchEntries(ctx, baseDN, "(objectClass=*)")
    duration := time.Since(start)

    require.NoError(t, err)
    assert.Len(t, entries, 50)

    // Should be fast (< 50ms for 50 entries in 5 levels)
    assert.Less(t, duration, 50*time.Millisecond)
}

func TestSearchEntriesUsesIndex(t *testing.T) {
    // This test verifies query plan uses index
    store := setupTestStore(t)
    ctx := context.Background()

    // Get query plan for recursive CTE
    var queryPlan string
    err := store.db.QueryRowContext(ctx, `
        EXPLAIN QUERY PLAN
        WITH RECURSIVE subtree AS (
            SELECT id, dn, parent_dn FROM entries WHERE dn = ?
            UNION ALL
            SELECT e.id, e.dn, e.parent_dn
            FROM entries e
            INNER JOIN subtree s ON e.parent_dn = s.dn
        )
        SELECT * FROM subtree
    `, "dc=example,dc=com").Scan(&queryPlan)

    require.NoError(t, err)

    // Should use index on parent_dn, not SCAN
    assert.Contains(t, queryPlan, "INDEX", "Query should use index")
    assert.NotContains(t, queryPlan, "SCAN TABLE entries", "Query should not scan table")
}
```

#### Benefits
- ✅ Eliminates full table scans
- ✅ Uses index on `parent_dn = ?` (indexed equality)
- ✅ O(log N) instead of O(N) performance
- ✅ 10-50x faster for deep hierarchies

---

### Phase 5: Optimize Indexes

**Priority:** P1 (Performance tuning)
**Effort:** 2-3 hours
**Risk:** Low
**Impact:** Medium (10-30% improvement)

#### Objectives
- Add composite indexes for common query patterns
- Remove redundant indexes
- Add explicit index on DN

#### Current Indexes

From `migrations/002_add_indexes.up.sql`:

```sql
CREATE INDEX idx_entries_parent_dn ON entries(parent_dn);
CREATE INDEX idx_entries_object_class ON entries(object_class);
CREATE INDEX idx_attributes_entry_id ON attributes(entry_id);
CREATE INDEX idx_attributes_name ON attributes(name);
CREATE INDEX idx_attributes_name_value ON attributes(name, value);
CREATE INDEX idx_users_uid ON users(uid);
CREATE INDEX idx_groups_cn ON groups(cn);
CREATE INDEX idx_group_members_group_entry_id ON group_members(group_entry_id);
CREATE INDEX idx_group_members_member_entry_id ON group_members(member_entry_id);
```

#### Recommended Changes

**Add indexes:**

1. **Explicit DN index** (clarity, even if UNIQUE auto-creates)
2. **Composite parent_dn + object_class** (for filtered children queries)
3. **Composite entry_id + name** in attributes (for attribute lookups by name)
4. **Composite group + member** in group_members (for membership checks)

**Drop indexes:**

1. **idx_attributes_name_value** - Rarely used, large index

#### Implementation

```sql
-- migrations/003_optimize_indexes.up.sql (NEW)

-- Add explicit DN index for clarity
CREATE INDEX IF NOT EXISTS idx_entries_dn ON entries(dn);

-- Composite index for filtered children queries
-- Example: SELECT * FROM entries WHERE parent_dn = ? AND object_class = ?
CREATE INDEX IF NOT EXISTS idx_entries_parent_dn_object_class
ON entries(parent_dn, object_class);

-- Composite index for attribute lookups by name
-- Example: SELECT value FROM attributes WHERE entry_id = ? AND name = ?
CREATE INDEX IF NOT EXISTS idx_attributes_entry_id_name
ON attributes(entry_id, name);

-- Composite index for group membership checks
-- Example: SELECT * FROM group_members WHERE group_entry_id = ? AND member_entry_id = ?
CREATE INDEX IF NOT EXISTS idx_group_members_composite
ON group_members(group_entry_id, member_entry_id);

-- Optional: Drop rarely-used index
-- Uncomment if space is a concern
-- DROP INDEX IF EXISTS idx_attributes_name_value;
```

```sql
-- migrations/003_optimize_indexes.down.sql (NEW)

DROP INDEX IF EXISTS idx_entries_dn;
DROP INDEX IF EXISTS idx_entries_parent_dn_object_class;
DROP INDEX IF EXISTS idx_attributes_entry_id_name;
DROP INDEX IF EXISTS idx_group_members_composite;

-- If dropped in .up.sql, restore here
-- CREATE INDEX IF NOT EXISTS idx_attributes_name_value ON attributes(name, value);
```

#### Files Modified
- `migrations/003_optimize_indexes.up.sql` (NEW)
- `migrations/003_optimize_indexes.down.sql` (NEW)

#### Testing

```bash
# Apply migration
go run cmd/ldaplite/main.go server

# Check indexes created
sqlite3 /tmp/ldaplite-data/ldaplite.db ".indexes"

# Analyze query plans
sqlite3 /tmp/ldaplite-data/ldaplite.db
> EXPLAIN QUERY PLAN SELECT * FROM entries WHERE parent_dn = 'dc=example,dc=com' AND object_class = 'inetOrgPerson';
> EXPLAIN QUERY PLAN SELECT value FROM attributes WHERE entry_id = 1 AND name = 'cn';
```

#### Benefits
- ✅ 10-30% performance improvement for filtered queries
- ✅ Better query plan selection by SQLite
- ✅ Reduced index storage (if dropping name_value)

---

### Phase 6: Optimize Recursive Group Queries

**Priority:** P1 (High impact for group-heavy deployments)
**Effort:** 2-3 days
**Risk:** Medium
**Impact:** Critical (100-200x for deep group hierarchies)

#### Objectives
- Eliminate recursive N+1 pattern in group membership queries
- Use SQL recursive CTEs for group resolution
- Single query for entire group tree

#### Problem Analysis

**Current implementation:**
```go
GetGroupMembersRecursive() {
    visited := map[string]bool{}

    // Calls GetGroupMembers which does 1 + N queries
    members := GetGroupMembers(groupDN)

    for each member {
        if member.IsGroup() {
            // Recursive call - more N+1 queries
            nestedMembers := GetGroupMembersRecursive(member.DN)
        }
    }
}
```

**Query count calculation:**
- Level 1: 1 group → GetGroupMembers = 1 + 5 = 6 queries
- Level 2: 5 groups → 5 × (1 + 5) = 30 queries
- Level 3: 25 groups → 25 × (1 + 5) = 150 queries
- **Total: 186 queries**

#### Solution: SQL Recursive CTE

Single query to resolve entire group tree:

```sql
WITH RECURSIVE member_tree AS (
    -- Base case: direct members of root group
    SELECT
        gm.member_entry_id as member_id,
        e.dn as member_dn,
        e.object_class,
        0 as depth
    FROM group_members gm
    INNER JOIN entries e ON gm.member_entry_id = e.id
    WHERE gm.group_entry_id = (SELECT id FROM entries WHERE dn = ?)

    UNION ALL

    -- Recursive case: members of nested groups
    SELECT
        gm.member_entry_id,
        e.dn,
        e.object_class,
        mt.depth + 1
    FROM member_tree mt
    INNER JOIN group_members gm ON mt.member_id = gm.group_entry_id
    INNER JOIN entries e ON gm.member_entry_id = e.id
    WHERE mt.object_class = 'groupOfNames'  -- Only recurse into groups
      AND mt.depth < ?  -- Max depth limit
)
SELECT DISTINCT member_id, member_dn, object_class, depth
FROM member_tree
```

#### Implementation

**Step 1:** Rewrite GetGroupMembersRecursive

```go
// internal/store/sqlite_groups.go (REPLACE GetGroupMembersRecursive)

func (s *SQLiteStore) GetGroupMembersRecursive(ctx context.Context, groupDN string, maxDepth int) ([]*models.Entry, error) {
    query := `
        WITH RECURSIVE member_tree AS (
            -- Base case: direct members
            SELECT
                gm.member_entry_id as member_id,
                e.object_class,
                0 as depth
            FROM group_members gm
            INNER JOIN entries e ON gm.member_entry_id = e.id
            WHERE gm.group_entry_id = (SELECT id FROM entries WHERE dn = ?)

            UNION ALL

            -- Recursive case: nested group members
            SELECT
                gm.member_entry_id,
                e.object_class,
                mt.depth + 1
            FROM member_tree mt
            INNER JOIN group_members gm ON mt.member_id = gm.group_entry_id
            INNER JOIN entries e ON gm.member_entry_id = e.id
            WHERE mt.object_class = ?
              AND mt.depth < ?
        )
        SELECT DISTINCT
            e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at,
            json_group_array(
                CASE WHEN a.name IS NOT NULL
                THEN json_object('name', a.name, 'value', a.value)
                ELSE NULL END
            ) as attributes_json
        FROM member_tree mt
        INNER JOIN entries e ON mt.member_id = e.id
        LEFT JOIN attributes a ON e.id = a.entry_id
        GROUP BY e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at
    `

    groupObjectClass := string(models.ObjectClassGroupOfNames)
    rows, err := s.db.QueryContext(ctx, query, groupDN, groupObjectClass, maxDepth)
    if err != nil {
        return nil, fmt.Errorf("failed to get recursive members: %w", err)
    }
    defer rows.Close()

    var members []*models.Entry
    for rows.Next() {
        entry := &models.Entry{}
        var attrsJSON string

        err := rows.Scan(
            &entry.ID,
            &entry.DN,
            &entry.ParentDN,
            &entry.ObjectClass,
            &entry.CreatedAt,
            &entry.UpdatedAt,
            &attrsJSON,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan member: %w", err)
        }

        entry.Attributes, err = decodeAttributesJSON(attrsJSON)
        if err != nil {
            return nil, fmt.Errorf("failed to decode attributes for %s: %w", entry.DN, err)
        }

        if entry.ObjectClass != "" {
            entry.Attributes["objectclass"] = []string{entry.ObjectClass}
        }

        members = append(members, entry)
    }

    return members, nil
}
```

**Step 2:** Remove old recursive helper

```go
// internal/store/sqlite_groups.go (DELETE resolveGroupMembersRecursive method)
// No longer needed, CTE handles recursion
```

**Step 3:** Rewrite GetUserGroupsRecursive similarly

```go
// internal/store/sqlite_groups.go (REPLACE GetUserGroupsRecursive)

func (s *SQLiteStore) GetUserGroupsRecursive(ctx context.Context, userDN string, maxDepth int) ([]*models.Group, error) {
    query := `
        WITH RECURSIVE group_tree AS (
            -- Base case: direct groups
            SELECT
                e.id as group_id,
                e.dn as group_dn,
                0 as depth
            FROM entries e
            INNER JOIN group_members gm ON e.id = gm.group_entry_id
            WHERE gm.member_entry_id = (SELECT id FROM entries WHERE dn = ?)
              AND e.object_class = ?

            UNION ALL

            -- Recursive case: parent groups
            SELECT
                e.id,
                e.dn,
                gt.depth + 1
            FROM group_tree gt
            INNER JOIN group_members gm ON gt.group_id = gm.member_entry_id
            INNER JOIN entries e ON gm.group_entry_id = e.id
            WHERE e.object_class = ?
              AND gt.depth < ?
        )
        SELECT DISTINCT
            e.id, e.dn, e.parent_dn, e.object_class, e.created_at, e.updated_at,
            g.cn,
            json_group_array(
                CASE WHEN a.name IS NOT NULL
                THEN json_object('name', a.name, 'value', a.value)
                ELSE NULL END
            ) as attributes_json
        FROM group_tree gt
        INNER JOIN entries e ON gt.group_id = e.id
        INNER JOIN groups g ON e.id = g.entry_id
        LEFT JOIN attributes a ON e.id = a.entry_id
        GROUP BY e.id
    `

    groupObjectClass := string(models.ObjectClassGroupOfNames)
    rows, err := s.db.QueryContext(ctx, query, userDN, groupObjectClass, groupObjectClass, maxDepth)
    if err != nil {
        return nil, fmt.Errorf("failed to get recursive groups: %w", err)
    }
    defer rows.Close()

    var groups []*models.Group
    for rows.Next() {
        group := &models.Group{
            Entry: models.Entry{},
        }
        var attrsJSON string

        err := rows.Scan(
            &group.Entry.ID,
            &group.Entry.DN,
            &group.Entry.ParentDN,
            &group.Entry.ObjectClass,
            &group.Entry.CreatedAt,
            &group.Entry.UpdatedAt,
            &group.CN,
            &attrsJSON,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan group: %w", err)
        }

        group.Entry.Attributes, err = decodeAttributesJSON(attrsJSON)
        if err != nil {
            return nil, fmt.Errorf("failed to decode attributes for %s: %w", group.Entry.DN, err)
        }

        if group.Entry.ObjectClass != "" {
            group.Entry.Attributes["objectclass"] = []string{group.Entry.ObjectClass}
        }

        groups = append(groups, group)
    }

    return groups, nil
}
```

**Step 4:** Update IsMemberOf

```go
// internal/store/sqlite_groups.go (SIMPLIFY IsMemberOf)

func (s *SQLiteStore) IsMemberOf(ctx context.Context, userDN string, groupDN string, maxDepth int) (bool, error) {
    // Use recursive CTE to check membership efficiently
    query := `
        WITH RECURSIVE group_tree AS (
            -- Base case: check direct membership
            SELECT 1 as found
            FROM group_members gm
            WHERE gm.group_entry_id = (SELECT id FROM entries WHERE dn = ?)
              AND gm.member_entry_id = (SELECT id FROM entries WHERE dn = ?)

            UNION ALL

            -- Recursive case: check through nested groups
            SELECT 1
            FROM group_tree gt
            INNER JOIN group_members gm ON gm.group_entry_id = (SELECT id FROM entries WHERE dn = ?)
            INNER JOIN entries e ON gm.member_entry_id = e.id
            WHERE e.dn = (
                SELECT e2.dn
                FROM entries e2
                INNER JOIN group_members gm2 ON e2.id = gm2.member_entry_id
                WHERE gm2.member_entry_id = (SELECT id FROM entries WHERE dn = ?)
                  AND e2.object_class = ?
            )
            LIMIT ?
        )
        SELECT EXISTS(SELECT 1 FROM group_tree)
    `

    var isMember bool
    groupObjectClass := string(models.ObjectClassGroupOfNames)
    err := s.db.QueryRowContext(ctx, query, groupDN, userDN, groupDN, userDN, groupObjectClass, maxDepth).Scan(&isMember)

    if err != nil {
        return false, fmt.Errorf("failed to check membership: %w", err)
    }

    return isMember, nil
}
```

#### Files Modified
- `internal/store/sqlite_groups.go` (REPLACE 3 methods, DELETE 2 helpers)

#### Testing

```go
// internal/store/sqlite_groups_test.go (ADD)

func TestGetGroupMembersRecursivePerformance(t *testing.T) {
    store := setupTestStore(t)
    ctx := context.Background()
    baseDN := "dc=example,dc=com"

    // Create nested groups: 3 levels, 5 groups per level
    rootGroup := createTestGroup(t, store, "cn=root,ou=groups,"+baseDN)

    // Level 2: 5 groups, each member of root
    for i := 0; i < 5; i++ {
        g := createTestGroup(t, store, fmt.Sprintf("cn=level2-%d,ou=groups,%s", i, baseDN))
        addGroupMember(t, store, rootGroup.DN, g.DN)

        // Level 3: 5 groups per level2 group
        for j := 0; j < 5; j++ {
            g3 := createTestGroup(t, store, fmt.Sprintf("cn=level3-%d-%d,ou=groups,%s", i, j, baseDN))
            addGroupMember(t, store, g.DN, g3.DN)

            // Add a user to each level3 group
            u := createTestUser(t, store, fmt.Sprintf("uid=user-%d-%d,ou=users,%s", i, j, baseDN))
            addGroupMember(t, store, g3.DN, u.DN)
        }
    }

    // Total: 1 root + 5 level2 + 25 level3 + 25 users = 56 entries

    start := time.Now()
    members, err := store.GetGroupMembersRecursive(ctx, rootGroup.DN, 10)
    duration := time.Since(start)

    require.NoError(t, err)

    // Should get: 5 level2 groups + 25 level3 groups + 25 users = 55 members
    assert.Len(t, members, 55)

    // Should be fast (< 20ms for 55 members across 3 levels)
    assert.Less(t, duration, 20*time.Millisecond)

    t.Logf("GetGroupMembersRecursive took %v for 55 members across 3 levels", duration)
}

func TestGetGroupMembersRecursiveCircular(t *testing.T) {
    store := setupTestStore(t)
    ctx := context.Background()
    baseDN := "dc=example,dc=com"

    // Create circular reference: A → B → C → A
    groupA := createTestGroup(t, store, "cn=groupA,ou=groups,"+baseDN)
    groupB := createTestGroup(t, store, "cn=groupB,ou=groups,"+baseDN)
    groupC := createTestGroup(t, store, "cn=groupC,ou=groups,"+baseDN)

    addGroupMember(t, store, groupA.DN, groupB.DN)
    addGroupMember(t, store, groupB.DN, groupC.DN)
    addGroupMember(t, store, groupC.DN, groupA.DN) // Circular!

    // Should not hang or error (max depth prevents infinite recursion)
    members, err := store.GetGroupMembersRecursive(ctx, groupA.DN, 10)

    require.NoError(t, err)

    // Should return B and C (A is not its own member)
    assert.Len(t, members, 2)
}

func TestIsMemberOfNested(t *testing.T) {
    store := setupTestStore(t)
    ctx := context.Background()
    baseDN := "dc=example,dc=com"

    // Create: admins contains powerusers, powerusers contains user
    adminGroup := createTestGroup(t, store, "cn=admins,ou=groups,"+baseDN)
    powerGroup := createTestGroup(t, store, "cn=powerusers,ou=groups,"+baseDN)
    user := createTestUser(t, store, "uid=jdoe,ou=users,"+baseDN)

    addGroupMember(t, store, adminGroup.DN, powerGroup.DN)
    addGroupMember(t, store, powerGroup.DN, user.DN)

    // User should be member of powerusers (direct)
    isMember, err := store.IsMemberOf(ctx, user.DN, powerGroup.DN, 10)
    require.NoError(t, err)
    assert.True(t, isMember)

    // User should be member of admins (transitive through powerusers)
    isMember, err = store.IsMemberOf(ctx, user.DN, adminGroup.DN, 10)
    require.NoError(t, err)
    assert.True(t, isMember)
}
```

#### Benefits
- ✅ 100-200x performance improvement for recursive groups
- ✅ Single query instead of O(N) queries
- ✅ Circular reference protection via depth limit
- ✅ Scales to deep hierarchies

---

## Implementation Roadmap

### Timeline Overview

**Week 1: Critical Fixes**
- Day 1: Phase 1 - Embed migrations (2-3 hours)
- Day 2-4: Phase 2 - Fix N+1 patterns (2-3 days)
- Day 5: Phase 4 - Fix LIKE patterns (1 day)

**Week 2: Advanced Optimizations**
- Day 1-5: Phase 3 - Push filters to SQL (1 week)

**Week 3: Group Optimization**
- Day 1-3: Phase 6 - Recursive group queries (2-3 days)
- Day 4: Phase 5 - Optimize indexes (2 hours)
- Day 5: Testing, profiling, benchmarking

**Week 4: Polish & Documentation**
- Integration testing
- Performance benchmarking
- Documentation updates
- Code review

### Priority Matrix

| Phase | Priority | Effort | Risk | Impact | Dependencies |
|-------|----------|--------|------|--------|--------------|
| 1. Embed migrations | P0 | Low | Low | High | None |
| 2. Fix N+1 | P0 | Medium | Medium | Critical | None |
| 4. Fix LIKE | P0 | Low | Medium | High | None |
| 3. Push filters | P1 | High | High | High | Phase 2 |
| 6. Recursive groups | P1 | Medium | Medium | Critical | Phase 2 |
| 5. Optimize indexes | P1 | Low | Low | Medium | Phase 2, 4 |

### Rollback Plan

**Per-phase rollback:**
- Phase 1: Revert to external migrations (git revert)
- Phase 2: Database queries backward compatible, code-only rollback
- Phase 3: Hybrid approach allows fallback to in-memory filtering
- Phase 4: Can revert to old LIKE pattern (slower but functional)
- Phase 5: Drop new indexes via down migration
- Phase 6: Revert to old recursive Go implementation

**Emergency rollback:**
```bash
# Roll back to previous version
git revert <commit-hash>
go build -o bin/ldaplite ./cmd/ldaplite

# Database migrations auto-rollback not supported
# Manual rollback if needed:
sqlite3 /data/ldaplite.db
> DELETE FROM schema_migrations WHERE version = 3;
```

---

## Testing Strategy

### Unit Tests

**Coverage targets:**
- Filter compiler: 90%+
- Attribute decoding: 100%
- Store methods: 80%+

**Test files:**
- `internal/schema/filter_compiler_test.go`
- `internal/store/sqlite_helpers_test.go`
- `internal/store/sqlite_test.go`
- `internal/store/sqlite_groups_test.go`

### Integration Tests

**Scenarios:**
1. Search with 1000 entries
2. Filtered search (5/1000 match)
3. Deep hierarchy (10 levels)
4. Recursive groups (5 levels)
5. Circular group references
6. Concurrent queries (50 simultaneous)

### Performance Benchmarks

**Benchmark suite:**

```go
// internal/store/bench_test.go

func BenchmarkSearchEntries100(b *testing.B) {
    // 100 entries with 5 attributes each
}

func BenchmarkSearchEntries1000(b *testing.B) {
    // 1000 entries with 5 attributes each
}

func BenchmarkSearchEntriesFiltered(b *testing.B) {
    // Search 1000, match 10
}

func BenchmarkGroupMembersRecursive(b *testing.B) {
    // 5 levels, 5 groups per level
}

func BenchmarkHierarchyQuery(b *testing.B) {
    // 10 level deep DN tree
}
```

**Performance targets:**

| Operation | Before | After | Target |
|-----------|--------|-------|--------|
| Search 100 entries | 101 queries, 100ms | 1 query, 5ms | < 10ms |
| Filtered search (5/1000) | 1001 queries, 500ms | 1 query, 10ms | < 20ms |
| Recursive groups (3 levels) | 186 queries, 200ms | 1 query, 5ms | < 10ms |
| Hierarchy query (500 nodes) | Full scan, 100ms | Index, 10ms | < 20ms |

### Load Testing

**Tools:**
- ldapsearch in loop
- Apache JMeter with LDAP plugin
- Custom Go load test

**Scenarios:**
1. 100 concurrent searches
2. 1000 queries/second sustained
3. Mix: 70% search, 20% bind, 10% modify

---

## Expected Performance Improvements

### Query Count Reduction

| Operation | Before | After | Improvement |
|-----------|--------|-------|-------------|
| Search 100 entries | 101 queries | 1 query | **101x** |
| Recursive groups (3 levels, 5 each) | 186 queries | 1 query | **186x** |
| Hierarchy search (500 nodes) | 501 queries | 1 query | **501x** |

### Latency Reduction

| Operation | Before | After | Improvement |
|-----------|--------|-------|-------------|
| Search 100 entries | ~100ms | ~5ms | **20x faster** |
| Filtered search (5/1000 match) | ~500ms | ~10ms | **50x faster** |
| Recursive groups (3 levels) | ~200ms | ~5ms | **40x faster** |
| Hierarchy query (500 nodes) | ~100ms | ~10ms | **10x faster** |

### Scalability

**Before:**
- Linear scaling: O(N) queries for N entries
- Performance degrades with data size
- Full table scans common

**After:**
- Constant scaling: O(1) queries regardless of result size
- Performance independent of total data size (within reason)
- Index-optimized queries

**Example scaling:**

| Total Entries | Search Result Count | Before (queries) | After (queries) | Improvement |
|---------------|---------------------|------------------|-----------------|-------------|
| 100 | 10 | 11 | 1 | 11x |
| 1,000 | 10 | 11 | 1 | 11x |
| 10,000 | 10 | 11 | 1 | 11x |
| 100,000 | 10 | 11 | 1 | 11x |

**Notice:** After optimization, query count is constant regardless of total data size.

---

## Risks & Mitigations

### Technical Risks

**Risk: SQLite version compatibility**
- **Impact:** JSON functions require SQLite 3.38+
- **Mitigation:** Check version at runtime, fallback to batch loading
- **Detection:** Unit tests on multiple SQLite versions

**Risk: Complex SQL bugs**
- **Impact:** Incorrect recursive CTEs could return wrong results
- **Mitigation:** Comprehensive test suite, manual verification
- **Detection:** Integration tests with known datasets

**Risk: Performance regression on certain queries**
- **Impact:** Some queries could become slower
- **Mitigation:** Benchmarks before/after, hybrid approach for fallback
- **Detection:** Performance test suite in CI

### Operational Risks

**Risk: Migration failure**
- **Impact:** Database corruption or startup failure
- **Mitigation:** Test migrations on copy of production data
- **Detection:** Migration tests in CI, manual verification

**Risk: Increased memory usage**
- **Impact:** JSON aggregation may use more memory
- **Mitigation:** Monitor memory usage, add limits if needed
- **Detection:** Load testing with memory profiling

**Risk: Breaking changes in behavior**
- **Impact:** Subtle differences in query results
- **Mitigation:** Extensive integration tests comparing old vs new
- **Detection:** A/B testing with parallel implementations

---

## References

### Code Locations

**Current implementations:**
- Store interface: `internal/store/store.go`
- SQLite store: `internal/store/sqlite.go`
- User queries: `internal/store/sqlite_users.go`
- Group queries: `internal/store/sqlite_groups.go`
- OU queries: `internal/store/sqlite_ous.go`
- Filter parsing: `internal/schema/filter.go`
- LDAP handlers: `internal/server/ldap.go`

**Migration files:**
- Schema: `migrations/001_initial_schema.up.sql`
- Indexes: `migrations/002_add_indexes.up.sql`

### External Documentation

**SQLite:**
- JSON functions: https://www.sqlite.org/json1.html
- Recursive CTEs: https://www.sqlite.org/lang_with.html
- Query planner: https://www.sqlite.org/queryplanner.html

**LDAP:**
- RFC 4511 (LDAP Protocol): https://tools.ietf.org/html/rfc4511
- RFC 4515 (Search Filters): https://tools.ietf.org/html/rfc4515

**Go libraries:**
- modernc.org/sqlite: https://pkg.go.dev/modernc.org/sqlite
- golang-migrate: https://github.com/golang-migrate/migrate
- vjeantet/ldapserver: https://github.com/vjeantet/ldapserver

---

## Appendix: Migration Guide

### For Operators

**Upgrading from current version to optimized version:**

1. **Backup database:**
   ```bash
   cp /data/ldaplite.db /data/ldaplite.db.backup
   ```

2. **Deploy new binary:**
   ```bash
   docker pull ldaplite:v2.0
   docker-compose up -d
   ```

3. **Migrations run automatically on startup**

4. **Verify:**
   ```bash
   ldapsearch -H ldap://localhost:3389 \
     -D "cn=admin,dc=example,dc=com" \
     -w password \
     -b "dc=example,dc=com" \
     "(objectClass=*)"
   ```

5. **Monitor performance:**
   ```bash
   # Check logs for query times
   docker logs ldaplite | grep "duration"
   ```

**Rollback if needed:**
```bash
# Stop new version
docker-compose down

# Restore backup
cp /data/ldaplite.db.backup /data/ldaplite.db

# Start old version
docker pull ldaplite:v1.0
docker-compose up -d
```

### For Developers

**Building with optimizations:**

```bash
# Ensure Go 1.25+
go version

# Build with embedded migrations
go build -o bin/ldaplite ./cmd/ldaplite

# Run tests
go test -v -race ./...

# Run benchmarks
go test -bench=. -benchmem ./internal/store

# Profile
go test -cpuprofile=cpu.prof -bench=BenchmarkSearchEntries
go tool pprof cpu.prof
```

**Testing changes:**

```bash
# Create test database
export LDAP_DATABASE_PATH=/tmp/test.db
export LDAP_BASE_DN=dc=example,dc=com
export LDAP_ADMIN_PASSWORD=test123

./bin/ldaplite server &

# Populate test data
go run test/populate.go --entries 1000

# Run queries
ldapsearch -H ldap://localhost:3389 \
  -D "cn=admin,dc=example,dc=com" \
  -w test123 \
  -b "dc=example,dc=com" \
  "(&(objectClass=inetOrgPerson)(uid=user*))"

# Profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

---

## Status Tracking

### Phase Completion Checklist

- [ ] **Phase 1: Embed Migrations**
  - [ ] Create `internal/store/migrations.go`
  - [ ] Update `Initialize()` method
  - [ ] Test with missing migrations directory
  - [ ] Update Docker build

- [ ] **Phase 2: Fix N+1 Patterns**
  - [ ] Add `decodeAttributesJSON()` helper
  - [ ] Update `SearchEntries()`
  - [ ] Update `GetAllEntries()`
  - [ ] Update `GetChildren()`
  - [ ] Update `GetEntry()`
  - [ ] Update `GetUserByUID()`
  - [ ] Update `GetGroupMembers()`
  - [ ] Update `GetUserGroups()`
  - [ ] Update `SearchOUs()`
  - [ ] Write unit tests
  - [ ] Run benchmarks

- [ ] **Phase 3: Push Filters to SQL**
  - [ ] Create `filter_compiler.go`
  - [ ] Implement simple filters (equality, present)
  - [ ] Implement composite filters (AND, OR, NOT)
  - [ ] Implement substring filters
  - [ ] Add `CanCompileToSQL()` check
  - [ ] Update `SearchEntries()` with hybrid approach
  - [ ] Write comprehensive tests
  - [ ] Test all filter types

- [ ] **Phase 4: Fix LIKE Patterns**
  - [ ] Implement recursive CTE in `SearchEntries()`
  - [ ] Update `SearchOUs()`
  - [ ] Update `GetChildren()`
  - [ ] Test with deep hierarchies
  - [ ] Verify index usage with EXPLAIN QUERY PLAN

- [ ] **Phase 5: Optimize Indexes**
  - [ ] Create `003_optimize_indexes.up.sql`
  - [ ] Create `003_optimize_indexes.down.sql`
  - [ ] Test migration
  - [ ] Verify indexes created
  - [ ] Check query plans

- [ ] **Phase 6: Recursive Group Queries**
  - [ ] Rewrite `GetGroupMembersRecursive()`
  - [ ] Rewrite `GetUserGroupsRecursive()`
  - [ ] Simplify `IsMemberOf()`
  - [ ] Remove old recursive helpers
  - [ ] Test with nested groups
  - [ ] Test circular references
  - [ ] Benchmark improvement

---

**Document version:** 1.0
**Last updated:** 2025-10-25
**Next review:** After Phase 1 completion
