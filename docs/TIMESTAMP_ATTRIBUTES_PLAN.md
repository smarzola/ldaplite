# LDAP Operational Timestamp Attributes Implementation Plan

**Date:** 2025-10-25
**Status:** Planning
**Version:** 1.0

---

## Overview

Add standard LDAP operational attributes `createTimestamp` and `modifyTimestamp` to all entries. These are read-only attributes automatically maintained by the server that indicate when an entry was created and last modified.

---

## LDAP Standards Reference

### RFC 4512 - Section 3.3.13 (Generalized Time)

**Attribute Definitions:**
- `createTimestamp`: Single-valued attribute recording entry creation time
- `modifyTimestamp`: Single-valued attribute recording last modification time

**Format:** Generalized Time (RFC 4517)
- Format: `YYYYMMDDHHMMSSz` (UTC timezone, 'Z' suffix required)
- Example: `20250125143045Z` (January 25, 2025 at 14:30:45 UTC)
- Precision: Seconds (subseconds optional but not typically used)

**Characteristics:**
- Operational attributes (not stored in user attributes, computed from database fields)
- Read-only (cannot be modified by users via LDAP operations)
- Always present on all entries
- Single-valued (not multi-valued)
- Case-insensitive attribute names

---

## Current State Analysis

### Database Schema
**Table: `entries`**
```sql
CREATE TABLE entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dn TEXT UNIQUE NOT NULL,
    parent_dn TEXT,
    object_class TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

✅ **Good news:** Database already has `created_at` and `updated_at` columns!

### Current Code Locations

**Entry Model:** `internal/models/entry.go`
```go
type Entry struct {
    ID          int64
    DN          string
    ParentDN    string
    ObjectClass string
    Attributes  map[string][]string
    CreatedAt   time.Time  // ✅ Already exists
    UpdatedAt   time.Time  // ✅ Already exists
}
```

**Where Attributes Are Added:**
After decoding entries from database, attributes are added:
- `internal/store/sqlite.go:469` - SearchEntries()
- `internal/store/sqlite.go:537` - GetAllEntries()
- `internal/store/sqlite.go:599` - GetChildren()
- `internal/store/sqlite.go:212` - GetEntry()
- `internal/store/sqlite_users.go` - User operations
- `internal/store/sqlite_groups.go` - Group operations
- `internal/store/sqlite_ous.go` - OU operations

**Pattern:**
```go
// Current pattern after decoding attributes
if entry.ObjectClass != "" {
    entry.Attributes["objectclass"] = []string{entry.ObjectClass}
}
```

**New pattern (proposed):**
```go
// Add operational attributes
entry.AddOperationalAttributes()
```

---

## Implementation Plan

### Phase 1: Add Timestamp Formatting Helper (30 minutes)

**File:** `internal/models/entry.go` (NEW FUNCTIONS)

**Add helper function:**
```go
// FormatLDAPTimestamp formats a time.Time into LDAP Generalized Time format
// Format: YYYYMMDDHHMMSSz (UTC)
// Example: 20250125143045Z
func FormatLDAPTimestamp(t time.Time) string {
    return t.UTC().Format("20060102150405Z")
}

// AddOperationalAttributes adds LDAP operational attributes to the entry
// These are computed from the Entry's fields and are read-only
func (e *Entry) AddOperationalAttributes() {
    // Add objectClass (structural attribute)
    if e.ObjectClass != "" {
        e.Attributes["objectclass"] = []string{e.ObjectClass}
    }

    // Add operational timestamp attributes
    e.Attributes["createtimestamp"] = []string{FormatLDAPTimestamp(e.CreatedAt)}
    e.Attributes["modifytimestamp"] = []string{FormatLDAPTimestamp(e.UpdatedAt)}
}
```

**Why this approach:**
- Centralized logic in one place
- Easy to add more operational attributes later
- Consistent formatting across all entry types
- Clear separation of operational vs. user attributes

---

### Phase 2: Update Store Methods (1 hour)

Replace all instances of manual objectClass addition with the new helper method.

#### File: `internal/store/sqlite.go`

**Location 1: GetEntry() - Line ~212**
```go
// OLD:
// (no operational attributes added here currently)

// NEW:
entry.AddOperationalAttributes()
```

**Location 2: SearchEntries() - Line ~469**
```go
// OLD:
if entry.ObjectClass != "" {
    entry.Attributes["objectclass"] = []string{entry.ObjectClass}
}

// NEW:
entry.AddOperationalAttributes()
```

**Location 3: GetAllEntries() - Line ~537**
```go
// OLD:
if entry.ObjectClass != "" {
    entry.Attributes["objectclass"] = []string{entry.ObjectClass}
}

// NEW:
entry.AddOperationalAttributes()
```

**Location 4: GetChildren() - Line ~599**
```go
// OLD:
if entry.ObjectClass != "" {
    entry.Attributes["objectclass"] = []string{entry.ObjectClass}
}

// NEW:
entry.AddOperationalAttributes()
```

#### File: `internal/store/sqlite_users.go`

**Search pattern:** Find all locations where entries are constructed and add:
```go
entry.AddOperationalAttributes()
```

Likely locations:
- `GetUser()`
- `GetUserByUID()`
- `SearchUsers()`

#### File: `internal/store/sqlite_groups.go`

Similar updates for:
- `GetGroup()`
- `GetGroupByName()`
- `GetGroupMembers()`
- `GetGroupMembersRecursive()`
- `SearchGroups()`

#### File: `internal/store/sqlite_ous.go`

Similar updates for:
- `GetOU()`
- `SearchOUs()`

---

### Phase 3: Protect Timestamps from Modification (30 minutes)

Add validation to prevent users from setting these operational attributes.

#### File: `internal/server/ldap.go` (MODIFY handleModify)

**Add validation in modify operation:**
```go
// List of protected operational attributes
var protectedAttributes = []string{
    "createtimestamp",
    "modifytimestamp",
    "objectclass", // structural attribute, cannot be changed after creation
}

func isProtectedAttribute(attrName string) bool {
    attrLower := strings.ToLower(attrName)
    for _, protected := range protectedAttributes {
        if attrLower == protected {
            return true
        }
    }
    return false
}

// In handleModify, add check:
for _, change := range req.Changes() {
    attrName := change.Modification().Type_()

    if isProtectedAttribute(attrName) {
        res.SetResultCode(ldap.LDAPResultUnwillingToPerform)
        res.SetDiagnosticMessage(fmt.Sprintf("Cannot modify operational attribute: %s", attrName))
        w.Write(res)
        return
    }

    // ... continue with modification
}
```

**Add validation in add operation (prevent users from providing these):**
```go
// In handleAdd, before creating entry:
for attrName := range attributes {
    if isProtectedAttribute(attrName) {
        res.SetResultCode(ldap.LDAPResultUnwillingToPerform)
        res.SetDiagnosticMessage(fmt.Sprintf("Cannot set operational attribute: %s", attrName))
        w.Write(res)
        return
    }
}
```

---

### Phase 4: Update Tests (1-2 hours)

#### File: `internal/models/entry_test.go` (ADD NEW TESTS)

```go
func TestFormatLDAPTimestamp(t *testing.T) {
    tests := []struct {
        name     string
        time     time.Time
        expected string
    }{
        {
            name:     "specific time",
            time:     time.Date(2025, 1, 25, 14, 30, 45, 0, time.UTC),
            expected: "20250125143045Z",
        },
        {
            name:     "different timezone converts to UTC",
            time:     time.Date(2025, 1, 25, 14, 30, 45, 0, time.FixedZone("EST", -5*3600)),
            expected: "20250125193045Z", // 14:30 EST = 19:30 UTC
        },
        {
            name:     "midnight",
            time:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
            expected: "20250101000000Z",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := FormatLDAPTimestamp(tt.time)
            if result != tt.expected {
                t.Errorf("FormatLDAPTimestamp() = %v, want %v", result, tt.expected)
            }
        })
    }
}

func TestAddOperationalAttributes(t *testing.T) {
    entry := NewEntry("uid=test,dc=example,dc=com", "inetOrgPerson")
    entry.SetAttribute("uid", "test")
    entry.SetAttribute("cn", "Test User")

    // Add operational attributes
    entry.AddOperationalAttributes()

    // Check objectClass was added
    if !entry.HasAttribute("objectclass") {
        t.Error("objectclass attribute was not added")
    }

    // Check createTimestamp was added
    if !entry.HasAttribute("createtimestamp") {
        t.Error("createtimestamp attribute was not added")
    }

    // Check modifyTimestamp was added
    if !entry.HasAttribute("modifytimestamp") {
        t.Error("modifytimestamp attribute was not added")
    }

    // Verify format (should be 14 chars + Z = 15 total)
    createTS := entry.GetAttribute("createtimestamp")
    if len(createTS) != 15 {
        t.Errorf("createtimestamp has wrong length: %d, expected 15", len(createTS))
    }
    if !strings.HasSuffix(createTS, "Z") {
        t.Error("createtimestamp should end with 'Z'")
    }

    // Verify timestamps are different after modification
    time.Sleep(10 * time.Millisecond)
    entry.SetAttribute("cn", "Modified User")
    entry.AddOperationalAttributes()

    modifyTS := entry.GetAttribute("modifytimestamp")
    if createTS == modifyTS {
        t.Error("modifytimestamp should be updated after modification")
    }
}
```

#### File: `internal/store/sqlite_test.go` (UPDATE EXISTING TESTS)

Update existing tests to check for timestamp attributes:
```go
func TestSearchEntriesHasTimestamps(t *testing.T) {
    store := setupTestStore(t)
    ctx := context.Background()
    baseDN := "dc=test,dc=com"

    // Create test entry
    entry := models.NewEntry("uid=test,ou=users,"+baseDN, "inetOrgPerson")
    entry.SetAttribute("uid", "test")
    err := store.CreateEntry(ctx, entry)
    require.NoError(t, err)

    // Search for entry
    entries, err := store.SearchEntries(ctx, baseDN, "(uid=test)")
    require.NoError(t, err)
    require.Len(t, entries, 1)

    found := entries[0]

    // Verify timestamp attributes exist
    assert.True(t, found.HasAttribute("createtimestamp"), "createtimestamp should be present")
    assert.True(t, found.HasAttribute("modifytimestamp"), "modifytimestamp should be present")

    // Verify format
    createTS := found.GetAttribute("createtimestamp")
    assert.Regexp(t, `^\d{14}Z$`, createTS, "createtimestamp should match format YYYYMMDDHHMMSSz")

    modifyTS := found.GetAttribute("modifytimestamp")
    assert.Regexp(t, `^\d{14}Z$`, modifyTS, "modifytimestamp should match format YYYYMMDDHHMMSSz")
}
```

#### Integration Tests

Add integration test with ldapsearch:
```bash
# Search for entries and verify timestamps are present
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w password \
  -b "dc=example,dc=com" \
  "(objectClass=*)" \
  createTimestamp modifyTimestamp

# Expected output should include:
# createTimestamp: 20250125143045Z
# modifyTimestamp: 20250125143045Z
```

---

### Phase 5: Update Documentation (30 minutes)

#### File: `CLAUDE.md` (UPDATE)

Add to "Supported LDAP Operations" section:
```markdown
**Operational Attributes (Automatic):**
- `createTimestamp`: Entry creation time in LDAP Generalized Time format
- `modifyTimestamp`: Last modification time in LDAP Generalized Time format
- `objectClass`: Structural object class

These attributes are automatically added to all entries and cannot be modified by clients.
```

#### File: `README.md` (UPDATE - if exists)

Add note about operational attributes:
```markdown
## Operational Attributes

LDAPLite automatically maintains the following LDAP operational attributes:

- **createTimestamp**: Records when the entry was created (format: `YYYYMMDDHHMMSSz`)
- **modifyTimestamp**: Records when the entry was last modified (format: `YYYYMMDDHHMMSSz`)

These attributes are read-only and automatically updated by the server.
```

---

## Testing Strategy

### Unit Tests

**Test Coverage:**
1. ✅ Timestamp formatting (various dates, timezones, edge cases)
2. ✅ AddOperationalAttributes() adds all required attributes
3. ✅ Timestamps are in correct format
4. ✅ modifyTimestamp changes after modifications
5. ✅ createTimestamp remains constant

**Files:**
- `internal/models/entry_test.go` - Timestamp formatting and helper tests
- `internal/store/sqlite_test.go` - Integration with database queries

### Integration Tests

**Scenarios:**
1. Create entry → verify createTimestamp and modifyTimestamp are present
2. Modify entry → verify modifyTimestamp is updated
3. Search entries → all entries have timestamp attributes
4. Filter by timestamp (future enhancement)
5. Attempt to modify timestamps → should fail with unwillingToPerform

**Manual Testing:**
```bash
# 1. Create a test entry
ldapadd -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w password <<EOF
dn: uid=testuser,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: testuser
cn: Test User
sn: User
mail: test@example.com
EOF

# 2. Search and verify timestamps
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w password \
  -b "uid=testuser,ou=users,dc=example,dc=com" \
  "(objectClass=*)" \
  createTimestamp modifyTimestamp

# Expected output:
# createTimestamp: 20250125143045Z
# modifyTimestamp: 20250125143045Z

# 3. Modify the entry
ldapmodify -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w password <<EOF
dn: uid=testuser,ou=users,dc=example,dc=com
changetype: modify
replace: mail
mail: newemail@example.com
EOF

# 4. Search again - modifyTimestamp should be updated
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w password \
  -b "uid=testuser,ou=users,dc=example,dc=com" \
  "(objectClass=*)" \
  createTimestamp modifyTimestamp

# Expected: modifyTimestamp is newer than createTimestamp

# 5. Try to modify timestamp (should fail)
ldapmodify -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w password <<EOF
dn: uid=testuser,ou=users,dc=example,dc=com
changetype: modify
replace: modifyTimestamp
modifyTimestamp: 20990101000000Z
EOF

# Expected: Error - unwillingToPerform
```

---

## Implementation Checklist

### Phase 1: Helper Functions
- [ ] Add `FormatLDAPTimestamp()` function to `internal/models/entry.go`
- [ ] Add `AddOperationalAttributes()` method to `Entry` struct
- [ ] Add unit tests for timestamp formatting
- [ ] Add unit tests for AddOperationalAttributes()

### Phase 2: Update Store Methods
- [ ] Update `GetEntry()` in `internal/store/sqlite.go`
- [ ] Update `SearchEntries()` in `internal/store/sqlite.go`
- [ ] Update `GetAllEntries()` in `internal/store/sqlite.go`
- [ ] Update `GetChildren()` in `internal/store/sqlite.go`
- [ ] Update user methods in `internal/store/sqlite_users.go`
- [ ] Update group methods in `internal/store/sqlite_groups.go`
- [ ] Update OU methods in `internal/store/sqlite_ous.go`

### Phase 3: Protection
- [ ] Add `protectedAttributes` list to `internal/server/ldap.go`
- [ ] Add `isProtectedAttribute()` helper function
- [ ] Update `handleModify()` to block protected attributes
- [ ] Update `handleAdd()` to block protected attributes
- [ ] Add tests for protection validation

### Phase 4: Testing
- [ ] Run all existing unit tests (ensure no regressions)
- [ ] Add new timestamp-specific tests
- [ ] Add integration tests
- [ ] Manual testing with ldapsearch
- [ ] Manual testing with ldapmodify

### Phase 5: Documentation
- [ ] Update `CLAUDE.md` with operational attributes info
- [ ] Update `README.md` (if exists)
- [ ] Add inline code comments
- [ ] Update this plan with "COMPLETED" status

---

## Expected Benefits

### LDAP Standards Compliance
✅ Implements RFC 4512 operational attributes
✅ Provides standard audit trail information
✅ Better interoperability with LDAP clients

### User Experience
✅ Automatic timestamp tracking (no manual intervention)
✅ Read-only attributes prevent accidental modification
✅ Standard LDAP attribute names (familiar to LDAP users)

### Operational Benefits
✅ Audit trail: know when entries were created/modified
✅ Debugging: timestamps help troubleshoot issues
✅ Future filtering: enable searches by timestamp ranges

---

## Risks & Mitigations

### Risk: Timestamp Format Incompatibility
**Impact:** Some LDAP clients might not parse timestamps correctly
**Mitigation:** Use RFC 4517 Generalized Time format (standard)
**Detection:** Test with multiple LDAP clients (ldapsearch, LDAP Admin, etc.)

### Risk: Performance Impact
**Impact:** Adding attributes to every entry might slow queries
**Mitigation:** Minimal - only formatting existing database fields
**Detection:** Benchmark before/after with large datasets

### Risk: Breaking Changes
**Impact:** Existing code/tests might fail if they don't expect new attributes
**Mitigation:** Comprehensive test updates before release
**Detection:** Run full test suite before commit

---

## Timeline Estimate

| Phase | Effort | Cumulative |
|-------|--------|------------|
| 1. Helper Functions | 30 min | 30 min |
| 2. Update Store Methods | 1 hour | 1.5 hours |
| 3. Protection | 30 min | 2 hours |
| 4. Testing | 1-2 hours | 3-4 hours |
| 5. Documentation | 30 min | 3.5-4.5 hours |

**Total Estimated Time:** 3.5 - 4.5 hours

---

## Future Enhancements

### Phase 6 (Optional): Filter by Timestamp
Enable LDAP searches like:
```
(modifyTimestamp>=20250101000000Z)
```

Requires:
- Timestamp parsing in filter compiler
- Greater/less than comparison support in SQL
- Index on timestamp columns for performance

### Phase 7 (Optional): Additional Operational Attributes
- `entryDN`: The entry's DN (redundant but standard)
- `entryUUID`: Unique identifier for the entry
- `creatorsName`: DN of who created the entry
- `modifiersName`: DN of who last modified the entry
- `subschemaSubentry`: Schema location

---

## References

### LDAP RFCs
- **RFC 4512** - LDAP Directory Information Models (operational attributes)
- **RFC 4517** - LDAP Syntaxes and Matching Rules (Generalized Time)
- **RFC 4519** - LDAP Schema for User Applications (attribute definitions)

### Go Time Formatting
- Go time format: `"20060102150405Z"` for `YYYYMMDDHHMMSSz`
- Always use `time.UTC()` before formatting
- Reference: https://pkg.go.dev/time#Time.Format

### Code Locations
- Entry model: `internal/models/entry.go`
- Store implementation: `internal/store/sqlite*.go`
- LDAP handlers: `internal/server/ldap.go`
- Tests: `internal/models/*_test.go`, `internal/store/*_test.go`

---

**Status:** Ready for Implementation
**Next Steps:** Begin Phase 1 - Add helper functions
