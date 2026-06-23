# Import And Export Design

This document defines the practical bootstrap/import/export path for LDAPLite.
It is the committed Milestone 7 design until the commands are implemented.

## Goals

- Seed users, groups, organizational units, and app bind users without using the
  Web UI.
- Preserve LDAPLite's password security rules.
- Preserve group member referential integrity.
- Provide an export path that is safe by default and does not leak password
  hashes.
- Keep commands repeatable enough for container bootstrap and GitOps-style
  setup.

## Non-Goals

- Full OpenLDAP `slapadd` compatibility.
- Schema extension import.
- Importing password hashes from arbitrary LDAP servers.
- Exporting password hashes in the default mode.
- Replication, incremental sync, or conflict-free merge semantics.

## Commands

### `ldaplite import ldif`

```bash
ldaplite import ldif \
  --file ./directory.ldif \
  --dry-run

ldaplite import ldif \
  --file ./directory.ldif
```

Configuration comes from the existing environment variables:

```bash
LDAP_BASE_DN=dc=example,dc=com
LDAP_ADMIN_PASSWORD=change-me
LDAP_DATABASE_PATH=/var/lib/ldaplite/ldaplite.db
```

Flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--file` | required | LDIF file to import |
| `--dry-run` | `false` | Parse and validate without writing |
| `--replace-existing` | `false` | Replace existing entries by DN |
| `--allow-generated-passwords` | `false` | Generate random passwords for imported users missing `userPassword` |

Behavior:

- Initialize the SQLite store and migrations before import.
- Parse all LDIF records before writing.
- Validate the full batch in memory before writing:
  - every DN must be under `LDAP_BASE_DN`;
  - every non-base entry must have an existing parent in the database or import
    batch;
  - every entry must have exactly one supported structural `objectClass`;
  - every group `member` DN must exist in the database or import batch;
  - client-supplied `entryUUID`, `uuid`, timestamps, and `memberOf` are
    rejected;
  - user entries must satisfy existing `inetOrgPerson` validation.
- Apply writes in parent-before-child order inside one transaction where
  possible. If the current store API cannot do a single transaction across the
  whole import, the implementation must fail before writes for validation
  errors and document any partial-write risk for storage errors.

Password handling:

- Plain `userPassword` values are accepted and passed through
  `ProcessPassword()` so storage still uses LDAPLite's Argon2id hashing.
- Supported LDAPLite password schemes remain accepted only through the existing
  password processor.
- Unsupported password schemes return LDAP constraint violation semantics.
- If `--allow-generated-passwords` is set, generated passwords are printed once
  to stdout as a separate summary and never stored outside the password hash.

### `ldaplite export ldif`

```bash
ldaplite export ldif \
  --file ./directory.ldif
```

Flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--file` | `-` | Destination file, or stdout with `-` |
| `--include-operational` | `false` | Include safe operational attributes such as timestamps and `entryUUID` |
| `--include-password-placeholders` | `false` | Emit `userPassword: {REDACTED}` placeholders for user entries |

Behavior:

- Export base DN, OUs, users, and groups in parent-before-child order.
- Export `objectClass` and normal attributes.
- Export group `member` attributes.
- Do not export `userPassword` hashes by default.
- Never export raw password hashes unless a future backup mode is designed with
  explicit encryption, access controls, and tests.
- Do not export computed `memberOf` as a stored attribute.

## Minimal LDIF Shape

```ldif
dn: uid=appbind,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: appbind
cn: Application Bind
sn: Bind
userPassword: change-me

dn: cn=ldaplite.readonly,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: ldaplite.readonly
member: uid=appbind,ou=users,dc=example,dc=com
```

## Implementation Notes

- Prefer a small internal LDIF parser package over ad hoc line splitting in the
  CLI command.
- Reuse `models.Entry`, `models.User`, `models.Group`, and existing store write
  validation rather than duplicating validation in command code.
- Keep import/export independent of the LDAP server listener.
- Keep the Web UI out of scope for the first implementation.

## Required Tests

Unit tests:

- Parse single and multi-entry LDIF.
- Reject malformed records.
- Reject entries outside the base DN.
- Reject child entries whose parent is missing.
- Reject group members that do not exist in the database or batch.
- Reject client-supplied `entryUUID`, `uuid`, timestamps, and `memberOf`.
- Process plaintext user passwords through the password processor.
- Export omits `userPassword` by default.
- Export omits computed `memberOf`.

Command tests:

- `ldaplite import ldif --dry-run --file fixture.ldif` validates without
  writing.
- `ldaplite import ldif --file fixture.ldif` imports a user, group, and
  read-only bind group.
- `ldaplite export ldif --file -` emits importable LDIF without password hashes.
- Invalid import input exits non-zero and prints the failing DN and reason.

Functional follow-up:

- Start LDAPLite against an imported database.
- Bind as an imported user.
- Bind as an imported read-only app user and confirm search succeeds while
  writes return `insufficientAccessRights`.
