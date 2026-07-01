# LDIF Import And Export

LDAPLite includes practical LDIF import and export commands for bootstrap,
safe inspection, and repeatable directory setup.

The commands use the same SQLite store, model validation, password processing,
and group referential-integrity rules as LDAPLite's LDAP and Web UI write
paths. They are not a full OpenLDAP `slapadd` replacement.

## Import

Run a dry run before writing:

```bash
LDAP_BASE_DN=dc=example,dc=com \
LDAP_ADMIN_PASSWORD=YourSecurePassword \
LDAP_DATABASE_PATH=/var/lib/ldaplite/ldaplite.db \
ldaplite import ldif --file ./directory.ldif --dry-run
```

Apply the import:

```bash
LDAP_BASE_DN=dc=example,dc=com \
LDAP_ADMIN_PASSWORD=YourSecurePassword \
LDAP_DATABASE_PATH=/var/lib/ldaplite/ldaplite.db \
ldaplite import ldif --file ./directory.ldif
```

Flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--file` | required | LDIF file to import |
| `--dry-run` | `false` | Parse and validate without writing |
| `--replace-existing` | `false` | Replace existing entries by DN |
| `--allow-generated-passwords` | `false` | Generate random passwords for imported users missing `userPassword` |

Import parses the whole file and validates the whole batch before writing.
Validation checks include:

- DNs must be under `LDAP_BASE_DN`.
- Non-base entries must have an existing parent in the database or import batch.
- Entries must use supported structural object classes.
- Group `member` values must reference existing entries or entries in the same
  batch.
- Client-supplied `entryUUID`, timestamps, and `memberOf` are rejected.
- User entries must satisfy LDAPLite's `inetOrgPerson` validation.

Writes are applied in parent-before-child order. Validation errors fail before
writes. The current store API does not expose a whole-import transaction, so a
storage error after validation can leave a partially applied import.

## Password Handling

Plain `userPassword` values are accepted and processed through LDAPLite's
password hashing path. Passwords are never stored in the generic attributes
table.

Unsupported password schemes are rejected. If
`--allow-generated-passwords` is set, generated passwords are printed once to
stdout and are never stored outside the password hash.

## Export

Export to stdout:

```bash
ldaplite export ldif --file -
```

Export to a file:

```bash
ldaplite export ldif --file ./directory.ldif
```

Flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--file` | `-` | Destination file, or stdout with `-` |
| `--include-operational` | `false` | Include safe operational attributes such as timestamps and `entryUUID` |
| `--include-password-placeholders` | `false` | Emit `userPassword: {REDACTED}` placeholders for user entries |

Export emits base DN, organizational units, users, and groups in
parent-before-child order. It includes `objectClass`, normal attributes, and
group `member` values.

By default export omits:

- `userPassword`
- password hashes
- computed `memberOf`
- operational timestamps and `entryUUID`

Raw password hashes are never exported.

## Minimal LDIF Example

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

## Limits

LDAPLite import/export intentionally does not support:

- LDIF change records such as `changetype: modify`, `delete`, `modrdn`, or
  `moddn`
- schema extension import
- arbitrary third-party password hash import
- raw password-hash export
- replication or incremental sync
