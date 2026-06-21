# Goal: Reach 100% Compatibility for LDAPLite First-Milestone AD-Like Functional Tests

You are working in `/Users/smarzola/ldaplite`.

Your objective is to implement and iterate on a functional test suite until LDAPLite achieves **100% passing compatibility** for the first AD-like compatibility milestone described below.

## Repository Rules

Follow the repository's `AGENTS.md` instructions.

Important reminders:

- Use `ast-grep` for Go structural searches.
- Use `rg` for ordinary text/file searches.
- Keep changes scoped.
- Do not revert unrelated user changes.
- Prefer black-box functional tests over internal package tests for this milestone.
- Run relevant tests before finishing.
- If a test exposes a real compatibility gap, fix LDAPLite rather than weakening the test.

## Definition of Done

The goal is complete only when:

1. A functional test suite exists for the first AD-like compatibility milestone.
2. The suite starts LDAPLite as a real subprocess or equivalent black-box server.
3. Tests communicate with LDAPLite using a real LDAP client library, preferably `github.com/go-ldap/ldap/v3`.
4. The suite covers all first-milestone behaviors listed below.
5. All milestone tests pass.
6. Existing unit tests still pass, or any unrelated failures are clearly documented.
7. The final response summarizes:
   - files changed
   - test coverage added
   - compatibility fixes made
   - exact test commands run

## First-Milestone Compatibility Scope

Implement functional coverage for these AD-like LDAP behaviors:

### Server Startup

- Build or run LDAPLite in test mode.
- Start the server on a dynamically selected local TCP port.
- Use a temporary SQLite database.
- Set required environment variables:
  - `LDAP_BASE_DN=dc=example,dc=com`
  - `LDAP_ADMIN_PASSWORD=ChangeMe123!`
  - `LDAP_DATABASE_PATH=<temp db path>`
  - `LDAP_PORT=<random free port>`
- Wait until the LDAP port accepts connections.
- Shut down cleanly after tests.

### Basic Directory Fixture

The tests should create or verify this layout:

```text
dc=example,dc=com
ou=users,dc=example,dc=com
ou=groups,dc=example,dc=com
uid=jane,ou=users,dc=example,dc=com
cn=engineering,ou=groups,dc=example,dc=com
```

The user entry should include realistic AD/client-facing attributes where LDAPLite supports generic attributes:

```text
uid: jane
cn: Jane Doe
givenName: Jane
sn: Doe
mail: jane@example.com
sAMAccountName: jane
userPrincipalName: jane@example.com
objectClass: inetOrgPerson
userPassword: Password123!
```

The group entry should include:

```text
cn: engineering
objectClass: groupOfNames
member: uid=jane,ou=users,dc=example,dc=com
```

### Bind Compatibility

Test:

- Admin/simple bind succeeds.
- User bind succeeds with correct password.
- User bind fails with wrong password.
- Wrong-password failure returns LDAP invalid credentials result code `49`.
- Bind to a missing user fails with an appropriate LDAP error.

### Search Compatibility

Test subtree searches from `dc=example,dc=com`.

Required filters:

```text
(objectClass=*)
(uid=jane)
(cn=Jane Doe)
(mail=jane@example.com)
(sAMAccountName=jane)
(userPrincipalName=jane@example.com)
(&(objectClass=inetOrgPerson)(uid=jane))
(|(uid=jane)(mail=jane@example.com))
(!(uid=missing))
(cn=Jane*)
(member=uid=jane,ou=users,dc=example,dc=com)
```

For each successful search:

- Assert expected result count.
- Assert returned DN.
- Assert important attributes.
- Normalize attribute names case-insensitively where appropriate.
- Do not depend on response ordering unless LDAPLite explicitly guarantees it.

### Attribute Behavior

Test:

- `userPassword` is not returned in ordinary LDAP search results.
- `objectClass` is returned.
- Operational attributes are present where expected:
  - `createTimestamp`
  - `modifyTimestamp`
- Operational timestamp format is LDAP generalized time:
  - `YYYYMMDDHHMMSSZ`

### Modify Compatibility

Test:

- Modify a user's `mail`.
- Search confirms the updated value.
- Modify a user's password.
- Old password no longer binds.
- New password binds.
- `userPassword` still does not appear in search results.

### Delete Compatibility

Test:

- Delete a user.
- Searching for that user no longer returns it.
- Binding as that user fails.
- Deleting a missing DN returns LDAP no such object result code `32`, if observable through the client.

### Error Code Compatibility

Where the LDAP client exposes result codes, assert:

- Invalid credentials: `49`
- No such object: `32`
- Constraint violation for unsupported password schemes: `19`
- Object class violation for invalid required attributes: `65`

## Suggested Implementation Shape

Prefer a new functional test package such as:

```text
tests/functional/
  ad_compat_test.go
  testserver_test.go
```

Use a build tag:

```go
//go:build functional
```

The suite should run with:

```bash
go test -tags=functional -v ./tests/functional/...
```

Add a Make target if appropriate:

```make
test-functional:
	go test -tags=functional -v ./tests/functional/...
```

## Iteration Instructions

Work in a loop:

1. Inspect the existing server, CLI, store, and test patterns.
2. Add the smallest useful black-box functional test harness.
3. Add the milestone tests.
4. Run the functional suite.
5. For each failure:
   - Determine whether the test expectation is valid for the milestone.
   - If valid, fix LDAPLite.
   - If invalid, adjust the test and document why.
6. Re-run until the milestone suite is 100% passing.
7. Run normal tests:
   ```bash
   go test -v ./...
   ```
   or the repository's preferred command if different.
8. Stop only when the Definition of Done is satisfied.

## Compatibility Philosophy

This milestone does **not** require full Active Directory compatibility.

Do not attempt to implement:

- Kerberos
- SASL/GSSAPI
- TLS/LDAPS
- Global Catalog
- DirSync
- paging controls
- server-side sorting controls
- AD recursive matching rule `1.2.840.113556.1.4.1941`
- complete Microsoft schema behavior

The goal is practical AD-like compatibility for common LDAP clients using simple bind, subtree search, ordinary attributes, users, groups, modification, deletion, and predictable LDAP result codes.

## Final Response Required

When complete, report:

- Whether the milestone suite is 100% passing.
- Commands run and results.
- Files changed.
- Any intentional compatibility limits left outside this milestone.
