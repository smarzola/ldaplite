# LDAPLite Quick Start

Get LDAPLite running locally, verify LDAP bind/search, and seed a small
directory.

## Option 1: Docker Compose

Prerequisites:

- Docker Compose
- `ldapsearch` and `ldapwhoami` for LDAP smoke tests

Start the server:

```bash
docker compose up -d
docker compose logs -f ldaplite
```

The checked-in compose file starts:

- LDAP on `ldap://localhost:3389`
- Web UI on `http://localhost:8080`
- base DN `dc=example,dc=com`
- admin bind DN `uid=admin,ou=users,dc=example,dc=com`
- admin password `ChangeMe123!`

Verify LDAP bind:

```bash
ldapwhoami -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w "ChangeMe123!"
```

Search the directory:

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w "ChangeMe123!" \
  -b "dc=example,dc=com" \
  "(objectClass=*)"
```

Open the Web UI at `http://localhost:8080` and sign in with username `admin`
and password `ChangeMe123!`.

Stop the server:

```bash
docker compose down
```

Remove the local data volume too:

```bash
docker compose down -v
```

## Option 2: Local Binary

Prerequisites:

- Go 1.25 or newer
- npm, for embedded Web UI assets
- `ldapsearch` and `ldapwhoami` for LDAP smoke tests

Build:

```bash
make build
```

Run:

```bash
export LDAP_BASE_DN=dc=example,dc=com
export LDAP_ADMIN_PASSWORD=SecurePassword123!
export LDAP_DATABASE_PATH=/tmp/ldaplite-quickstart/ldaplite.db
export LDAP_WEB_UI_ENABLED=true
export LDAP_WEB_UI_PORT=8080

./bin/ldaplite server
```

In another terminal, verify bind:

```bash
ldapwhoami -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w "SecurePassword123!"
```

## Seed Users And Groups With LDIF

Create `quickstart.ldif`:

```ldif
dn: uid=john,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: john
cn: John Doe
sn: Doe
givenName: John
mail: john@example.com
userPassword: john-password

dn: cn=developers,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: developers
member: uid=john,ou=users,dc=example,dc=com
```

Validate it without writing:

```bash
LDAP_BASE_DN=dc=example,dc=com \
LDAP_ADMIN_PASSWORD=SecurePassword123! \
LDAP_DATABASE_PATH=/tmp/ldaplite-quickstart/ldaplite.db \
./bin/ldaplite import ldif --file quickstart.ldif --dry-run
```

Import it:

```bash
LDAP_BASE_DN=dc=example,dc=com \
LDAP_ADMIN_PASSWORD=SecurePassword123! \
LDAP_DATABASE_PATH=/tmp/ldaplite-quickstart/ldaplite.db \
./bin/ldaplite import ldif --file quickstart.ldif
```

Export a safe LDIF snapshot:

```bash
LDAP_BASE_DN=dc=example,dc=com \
LDAP_ADMIN_PASSWORD=SecurePassword123! \
LDAP_DATABASE_PATH=/tmp/ldaplite-quickstart/ldaplite.db \
./bin/ldaplite export ldif --file -
```

## Add Entries With LDAP Tools

You can also use standard LDAP write tools:

```bash
ldapadd -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w "SecurePassword123!" \
  -f quickstart.ldif
```

## Next Steps

- Read [README.md](README.md) for configuration and architecture details.
- Browse [docs/README.md](docs/README.md) for operator documentation.
- Review [docs/import-export.md](docs/import-export.md) for LDIF behavior and
  limits.
- Review [docs/scim.md](docs/scim.md) for SCIM provisioning.
- Review [docs/authorization.md](docs/authorization.md) for LDAPLite roles.
