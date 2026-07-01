# SCIM Provisioning API

LDAPLite exposes a practical SCIM 2.0-compatible HTTP API for provisioning
users and groups alongside LDAP.

The first implementation targets common IdP provisioning flows. It is not a
full SCIM implementation.

## Enablement And Authentication

SCIM is served by the existing embedded HTTP server. Enable it the same way as
the Web UI:

```bash
export LDAP_WEB_UI_ENABLED=true
export LDAP_WEB_UI_PORT=8080
```

SCIM uses HTTP Basic authentication with LDAPLite user credentials.

- Read endpoints require `directory.read`.
- Write endpoints require `directory.write`.
- Members of `cn=ldaplite.admin,ou=groups,<baseDN>` have write access.
- Ordinary authenticated users can read but cannot provision resources.

Use HTTPS or a trusted private network when exposing the HTTP server.

## Discovery

```bash
curl -u admin:ChangeMe123! \
  http://localhost:8080/scim/v2/ServiceProviderConfig

curl -u admin:ChangeMe123! \
  http://localhost:8080/scim/v2/Schemas

curl -u admin:ChangeMe123! \
  http://localhost:8080/scim/v2/ResourceTypes
```

Discovery advertises:

- Users and Groups resource types.
- HTTP Basic authentication.
- Filtering support for the documented subset.
- No PATCH support.
- No bulk support.
- No ETag support.

## User Mapping

| SCIM field | LDAPLite field |
| --- | --- |
| `id` | `entryUUID` |
| `userName` | `uid` |
| `displayName` | `cn` |
| `name.givenName` | `givenName` |
| `name.familyName` | `sn` |
| `emails[0].value` | `mail` |
| `password` | write-only password input |
| `meta.created` | `createTimestamp` / entry creation time |
| `meta.lastModified` | `modifyTimestamp` / entry modification time |

Passwords are accepted only on create and replace requests. SCIM responses never
return plaintext passwords, password hashes, or `userPassword`.

`active` is not supported. LDAPLite currently deletes users on
`DELETE /scim/v2/Users/{id}` instead of soft-disabling them.

## Group Mapping

| SCIM field | LDAPLite field |
| --- | --- |
| `id` | `entryUUID` |
| `displayName` | `cn` |
| `members[].value` | referenced member `entryUUID` |
| `members[].display` | referenced member `cn`, `uid`, or DN fallback |
| `members[].type` | `User` or `Group` |
| `meta.created` | `createTimestamp` / entry creation time |
| `meta.lastModified` | `modifyTimestamp` / entry modification time |

Group writes resolve each SCIM member id back to an LDAP DN before calling the
shared directory service. Members must already exist, and groups must have at
least one member because LDAPLite stores groups as `groupOfNames`.

## Users

List users:

```bash
curl -u admin:ChangeMe123! \
  'http://localhost:8080/scim/v2/Users?startIndex=1&count=100'
```

Filter users:

```bash
curl -u admin:ChangeMe123! \
  'http://localhost:8080/scim/v2/Users?filter=userName%20eq%20%22jdoe%22'
```

Supported user filters:

- `id eq "..."`
- `userName eq "..."`
- `displayName eq "..."`

Create a user:

```bash
curl -u admin:ChangeMe123! \
  -H 'Content-Type: application/scim+json' \
  -d '{
    "userName": "jdoe",
    "displayName": "John Doe",
    "name": {
      "givenName": "John",
      "familyName": "Doe"
    },
    "emails": [
      { "value": "jdoe@example.com", "primary": true }
    ],
    "password": "Secret123!"
  }' \
  http://localhost:8080/scim/v2/Users
```

Replace a user:

```bash
curl -u admin:ChangeMe123! \
  -X PUT \
  -H 'Content-Type: application/scim+json' \
  -d '{
    "userName": "jdoe",
    "displayName": "John Doe",
    "name": {
      "givenName": "John",
      "familyName": "Doe"
    },
    "emails": [
      { "value": "john.doe@example.com", "primary": true }
    ]
  }' \
  http://localhost:8080/scim/v2/Users/<entryUUID>
```

Delete a user:

```bash
curl -u admin:ChangeMe123! \
  -X DELETE \
  http://localhost:8080/scim/v2/Users/<entryUUID>
```

## Groups

List groups:

```bash
curl -u admin:ChangeMe123! \
  'http://localhost:8080/scim/v2/Groups?startIndex=1&count=100'
```

Filter groups:

```bash
curl -u admin:ChangeMe123! \
  'http://localhost:8080/scim/v2/Groups?filter=displayName%20eq%20%22developers%22'
```

Supported group filters:

- `id eq "..."`
- `displayName eq "..."`

Create a group:

```bash
curl -u admin:ChangeMe123! \
  -H 'Content-Type: application/scim+json' \
  -d '{
    "displayName": "developers",
    "members": [
      { "value": "<user-entryUUID>" }
    ]
  }' \
  http://localhost:8080/scim/v2/Groups
```

Replace group membership:

```bash
curl -u admin:ChangeMe123! \
  -X PUT \
  -H 'Content-Type: application/scim+json' \
  -d '{
    "displayName": "developers",
    "members": [
      { "value": "<user-entryUUID>" },
      { "value": "<group-entryUUID>" }
    ]
  }' \
  http://localhost:8080/scim/v2/Groups/<group-entryUUID>
```

Delete a group:

```bash
curl -u admin:ChangeMe123! \
  -X DELETE \
  http://localhost:8080/scim/v2/Groups/<group-entryUUID>
```

## Current Limits

The following SCIM features are intentionally unsupported:

- PATCH.
- Bulk operations.
- Enterprise User schema.
- Bearer-token management.
- Full SCIM filter grammar.
- ETags and version preconditions.
- Soft disable through `active`.
- Schema extensions.

Unsupported filters and fields return SCIM error responses instead of being
silently ignored.
