# LDAPLite Quick Start

Get LDAPLite running in less than 5 minutes.

## Option 1: Docker Compose (Recommended)

### Prerequisites
- Docker and Docker Compose installed

### Steps

```bash
# Clone or navigate to the ldaplite directory
cd ldaplite

# Start the server
docker-compose up -d

# Verify it's running
docker-compose logs ldaplite
```

The server is now running at `localhost:3389`.

### Test Connection

```bash
# From your host machine (requires ldap-utils installed)
ldapsearch -H ldap://localhost:3389 \
  -D "cn=admin,dc=example,dc=com" \
  -w ChangeMe123! \
  -b "dc=example,dc=com" \
  "(objectClass=*)"
```

You should see the base DN entry and default OUs.

## Option 2: Local Binary

### Prerequisites
- Go 1.22 or higher
- Port 3389 available

### Steps

```bash
# Build the binary
go build -o ldaplite ./cmd/ldaplite

# Set environment variables
export LDAP_BASE_DN=dc=example,dc=com
export LDAP_ADMIN_PASSWORD=secure-password-here
export LDAP_DATABASE_PATH=./ldaplite.db

# Run the server
./ldaplite server
```

The server will start and you'll see:
```
INFO     LDAP server starting address=0.0.0.0:3389
```

### Test Connection

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "cn=admin,dc=example,dc=com" \
  -w secure-password-here \
  -b "dc=example,dc=com" \
  "(objectClass=*)"
```

## Adding Your First User

Create a file `add-user.ldif`:

```ldif
dn: uid=john,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: john
cn: John Doe
sn: Doe
givenName: John
mail: john@example.com
userPassword: john-password
```

Add the user:

```bash
ldapadd -H ldap://localhost:3389 \
  -D "cn=admin,dc=example,dc=com" \
  -w ChangeMe123! \
  -f add-user.ldif
```

Verify:

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "cn=admin,dc=example,dc=com" \
  -w ChangeMe123! \
  -b "uid=john,ou=users,dc=example,dc=com"
```

## Creating a Group

Create `add-group.ldif`:

```ldif
dn: cn=developers,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: developers
member: uid=john,ou=users,dc=example,dc=com
```

Add the group:

```bash
ldapadd -H ldap://localhost:3389 \
  -D "cn=admin,dc=example,dc=com" \
  -w ChangeMe123! \
  -f add-group.ldif
```

## Testing with Apache Directory Studio

1. Download Apache Directory Studio from: https://directory.apache.org/studio/
2. Create new connection:
   - Network Parameter: `localhost:3389`
   - Authentication: Simple
   - Bind DN: `cn=admin,dc=example,dc=com`
   - Bind password: `ChangeMe123!`
3. Connect and browse the directory

## Configuration

Edit environment variables in `docker-compose.yml` or shell:

```bash
export LDAP_BASE_DN=dc=mycompany,dc=com          # Change domain
export LDAP_ADMIN_PASSWORD=very-secure-password   # Change admin password
export LDAP_PORT=10389                            # Change port
export LDAP_LOG_LEVEL=debug                       # Enable debug logging
```

## Docker Compose Shutdown

```bash
docker-compose down
```

Add `-v` to also remove the data volume:

```bash
docker-compose down -v
```

## Making Changes

If you modify the code and want to rebuild:

```bash
docker-compose down -v
docker-compose build
docker-compose up -d
```

## Useful Commands

### Stop the server

```bash
docker-compose stop
```

### View logs

```bash
docker-compose logs -f ldaplite
```

### Enter container shell

```bash
docker-compose exec ldaplite /bin/sh
```

### Test LDAP bind

```bash
ldapwhoami -H ldap://localhost:3389 \
  -D "cn=admin,dc=example,dc=com" \
  -w ChangeMe123!
```

## Troubleshooting

### Connection refused

- Check port: `netstat -an | grep 3389`
- Check Docker: `docker-compose ps`
- Check logs: `docker-compose logs ldaplite`

### Authentication failed

- Verify password in docker-compose.yml
- Check user exists: `ldapsearch ... -b "cn=admin,dc=example,dc=com"`

### Data not persisting

- Check volume: `docker-compose ps`
- Verify mount point: `docker inspect ldaplite | grep -A 5 Mounts`

## Next Steps

1. Read the full [README.md](README.md) for advanced configuration
2. Check [IMPLEMENTATION.md](IMPLEMENTATION.md) for architecture details
3. Browse the code in `cmd/` and `internal/` directories
4. Create more users and groups as needed
5. Set up TLS with a reverse proxy (Nginx/Traefik)

## Questions?

See the Troubleshooting section in [README.md](README.md) or check the logs:

```bash
docker-compose logs ldaplite -n 50
```

---

Enjoy LDAPLite! 🚀
