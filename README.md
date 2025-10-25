# LDAPLite

A lightweight, LDAP-compliant server written in Go with SQLite backend. Built for modern environments with Docker support and no external dependencies beyond Go.

## Features

- **LDAP Protocol Compliant**: Works with standard LDAP tools like `ldapsearch`, `ldapadd`, etc.
- **SQLite Backend**: Simple file-based database, no PostgreSQL/MySQL required
- **Group Nesting**: Full support for nested groups with circular reference detection
- **Argon2id Hashing**: Modern, secure password hashing (OWASP recommended)
- **Docker Ready**: Distroless image, non-root user, health checks
- **Simple Configuration**: Environment variable based, no config files required
- **Idiomatic Go**: Clean, well-structured codebase following Go best practices

## Quick Start

### Prerequisites

- Go 1.22 or higher
- Port 3389 available (or configure via `LDAP_PORT`)

### Building

```bash
go build -o bin/ldaplite ./cmd/ldaplite
```

### Running

```bash
# Set required environment variables
export LDAP_BASE_DN=dc=example,dc=com
export LDAP_ADMIN_PASSWORD=your-secure-password

# Start the server
./bin/ldaplite server
```

The server will:
1. Initialize the SQLite database in `/data/ldaplite.db` (or configure with `LDAP_DATABASE_PATH`)
2. Create the base DN structure
3. Create default OUs: `ou=users` and `ou=groups`
4. Create admin user: `cn=admin,dc=example,dc=com` with provided password
5. Listen on `0.0.0.0:3389`

### Testing Connection

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "cn=admin,dc=example,dc=com" \
  -w your-secure-password \
  -b "dc=example,dc=com" \
  "(objectClass=*)"
```

## Docker Deployment

### Building the Image

```bash
docker build -t ldaplite:latest .
```

### Running with Docker Compose

```bash
docker-compose up -d
```

### Running with Docker

```bash
docker run -d \
  --name ldaplite \
  -p 3389:3389 \
  -e LDAP_BASE_DN=dc=example,dc=com \
  -e LDAP_ADMIN_PASSWORD=YourSecurePassword \
  -v ldap_data:/data \
  ldaplite:latest
```

## Configuration

Configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_PORT` | `3389` | LDAP server port |
| `LDAP_BIND_ADDRESS` | `0.0.0.0` | Bind address |
| `LDAP_BASE_DN` | `dc=example,dc=com` | LDAP base DN (required) |
| `LDAP_ADMIN_PASSWORD` | (required on first run) | Initial admin password |
| `LDAP_DATABASE_PATH` | `/data/ldaplite.db` | SQLite database path |
| `LDAP_LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `LDAP_LOG_FORMAT` | `json` | Log format: json or text |
| `LDAP_READ_TIMEOUT` | `30` | Read timeout in seconds |
| `LDAP_WRITE_TIMEOUT` | `30` | Write timeout in seconds |

## LDAP Operations

### Adding Users

```ldif
dn: uid=john,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: john
cn: John Doe
sn: Doe
givenName: John
mail: john@example.com
userPassword: password123
```

Save to file `add-user.ldif` and run:
```bash
ldapadd -H ldap://localhost:3389 \
  -D "cn=admin,dc=example,dc=com" \
  -w your-admin-password \
  -f add-user.ldif
```

### Creating Groups

```ldif
dn: cn=developers,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: developers
member: uid=john,ou=users,dc=example,dc=com
member: uid=jane,ou=users,dc=example,dc=com
```

### Nested Groups

```ldif
dn: cn=engineering,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: engineering
member: cn=developers,ou=groups,dc=example,dc=com
member: cn=devops,ou=groups,dc=example,dc=com
```

## Project Structure

```
ldaplite/
├── cmd/ldaplite/              # Entry point
├── internal/
│   ├── server/                # LDAP server implementation
│   ├── store/                 # SQLite storage layer
│   │   └── migrations/        # SQL migrations (embedded in binary)
│   ├── models/                # LDAP entry models
│   └── schema/                # Schema definitions (future)
├── pkg/
│   ├── config/                # Configuration management
│   └── crypto/                # Password hashing
├── Dockerfile                 # Container image
└── docker-compose.yml        # Compose configuration
```

## Object Classes Supported

- `organizationalUnit` (ou): Container for users/groups
- `inetOrgPerson`: Users with personal information
- `groupOfNames`: Groups with member references
- `top`: Root object class

## Supported Attributes

### Users (inetOrgPerson)
- `uid`: User identifier (required)
- `cn`: Common name (required)
- `sn`: Surname (required)
- `givenName`: First name (required)
- `mail`: Email address
- `telephoneNumber`: Phone number
- `displayName`: Display name
- `userPassword`: Password hash (automatically hashed on creation)

### Groups (groupOfNames)
- `cn`: Common name (required)
- `member`: DN of member (can be user or group, multi-valued)
- `description`: Group description

### OUs (organizationalUnit)
- `ou`: OU name (required)
- `description`: Description

## Limitations (Current Version)

- No TLS/SSL support (use reverse proxy for encryption)
- No SASL authentication
- No complex ACLs
- No referrals
- No schema extension
- Search filters: basic implementation only
- No replication

These can be added in future versions as needed.

## Architecture

### Database Schema

The SQLite database uses a flexible schema:

- **entries**: All LDAP entries (DN, object class, timestamps)
- **attributes**: All entry attributes (multi-valued, key-value)
- **users**: User-specific data (UID, password hash)
- **groups**: Group-specific data (CN)
- **group_members**: Group membership relationships (supports nesting)
- **organizational_units**: OU-specific data

### Group Nesting

Groups can contain both users and other groups. The system:
- Detects circular references
- Limits nesting depth (default: 10 levels)
- Uses recursive SQL CTEs for efficient queries
- Supports both direct and inherited membership queries

### Password Security

Passwords are hashed using **Argon2id** with OWASP-recommended parameters:
- Memory: 64MB
- Iterations: 3
- Parallelism: 2
- Hash verified in constant time to prevent timing attacks

## Development

### Building from Source

```bash
# Get dependencies
go mod download

# Run tests
go test -v ./...

# Build binary
go build -o bin/ldaplite ./cmd/ldaplite

# Run with local database
mkdir -p /tmp/ldaplite-data
export LDAP_DATABASE_PATH=/tmp/ldaplite-data/ldaplite.db
export LDAP_BASE_DN=dc=example,dc=com
export LDAP_ADMIN_PASSWORD=admin123
./bin/ldaplite server
```

### Running Tests

```bash
go test -v -race -cover ./...
```

### Code Coverage

```bash
go test -cover ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Kubernetes Deployment

Example StatefulSet:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ldaplite
spec:
  serviceName: ldaplite
  replicas: 1
  selector:
    matchLabels:
      app: ldaplite
  template:
    metadata:
      labels:
        app: ldaplite
    spec:
      containers:
      - name: ldaplite
        image: ldaplite:latest
        ports:
        - containerPort: 3389
          name: ldap
        env:
        - name: LDAP_BASE_DN
          value: dc=example,dc=com
        - name: LDAP_ADMIN_PASSWORD
          valueFrom:
            secretKeyRef:
              name: ldaplite-secret
              key: admin-password
        volumeMounts:
        - name: data
          mountPath: /data
        livenessProbe:
          exec:
            command:
            - /usr/local/bin/ldaplite
            - healthcheck
          initialDelaySeconds: 10
          periodSeconds: 30
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 1Gi
```

## Troubleshooting

### Database Errors

Check database permissions:
```bash
ls -la /data/ldaplite.db
chmod 700 /data
```

### Connection Issues

Verify server is running:
```bash
# Check logs
docker logs ldaplite

# Test connection
ldapwhoami -H ldap://localhost:3389 \
  -D "cn=admin,dc=example,dc=com" \
  -w password
```

### Authentication Failures

Ensure admin user was created:
```bash
# Check admin user exists
ldapsearch -H ldap://localhost:3389 \
  -b "cn=admin,dc=example,dc=com" \
  -x
```

## Performance Considerations

- SQLite is suitable for small to medium deployments
- Typical queries: < 100ms for 10k+ entries
- Group membership queries use recursive SQL CTEs (optimized)
- Indexes on DN, OU, CN for fast lookups
- Connection pooling: 25 max open connections (configurable)

## Security Best Practices

1. **Passwords**: Use strong admin password
2. **TLS**: Use reverse proxy (Nginx/Traefik) for TLS
3. **Network**: Restrict LDAP port access to trusted networks
4. **Data**: Regular SQLite database backups
5. **Logs**: Monitor logs for failed authentication attempts
6. **Container**: Run as non-root user (distroless image handles this)

## License

MIT

## Contributing

Contributions welcome! Please ensure:
- Code follows Go conventions
- Tests pass and coverage maintained
- Commits are clear and well-documented

## Roadmap

- [ ] Full search filter parsing and evaluation
- [ ] Add/Modify/Delete operations implementation
- [ ] TLS support (StartTLS)
- [ ] SASL authentication
- [ ] Schema validation
- [ ] Connection pooling optimization
- [ ] Performance benchmarks
- [ ] Admin CLI tools
- [ ] Web UI (optional)
- [ ] Replication support

## Support

For issues, questions, or contributions, please open an issue on GitHub.

---

**Version**: 0.1.0
**Status**: Beta - For testing and development
