# LDAPS TLS Sidecar Deployment

LDAPLite intentionally keeps the LDAP listener plain by default. For clients
that require `ldaps://`, run LDAPLite on a private listener and terminate TLS in
a sidecar or reverse proxy that forwards raw TCP to LDAPLite.

The functional suite includes a TLS-terminating TCP proxy test that verifies an
LDAPS client can bind and search through this pattern.

## Topology

```text
LDAP client --ldaps://:636--> TLS sidecar --ldap://:3389--> LDAPLite
```

Recommended network shape:

- Expose only the sidecar's TLS port to client networks.
- Bind LDAPLite to `127.0.0.1` for single-host deployments, or to a private
  container/Kubernetes network.
- Do not expose LDAPLite's plain LDAP port directly outside the trusted network.

## LDAPLite Environment

```bash
LDAP_BASE_DN=dc=example,dc=com
LDAP_ADMIN_PASSWORD=change-me
LDAP_BIND_ADDRESS=127.0.0.1
LDAP_PORT=3389
LDAP_DATABASE_PATH=/var/lib/ldaplite/ldaplite.db
```

For container deployments, set `LDAP_BIND_ADDRESS=0.0.0.0` only inside a
private network that is not directly published to the host or internet.

## Minimal stunnel Example

`stunnel.conf`:

```ini
foreground = yes

[ldaps]
accept = 0.0.0.0:636
connect = 127.0.0.1:3389
cert = /etc/stunnel/ldaplite.crt
key = /etc/stunnel/ldaplite.key
```

Certificate requirements:

- The certificate must include the DNS name clients use for LDAPLite.
- Use a CA-trusted certificate for production.
- For internal deployments, install your private CA into each client that
  validates LDAPS certificates.

## Client URLs

Use the sidecar endpoint in LDAP clients:

```text
ldaps://ldap.example.com:636
```

LDAPLite itself still listens on:

```text
ldap://127.0.0.1:3389
```

## Smoke Tests

With certificate validation:

```bash
LDAPTLS_CACERT=/path/to/ca.crt ldapwhoami \
  -H ldaps://ldap.example.com:636 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD"
```

For a local self-signed smoke test only:

```bash
LDAPTLS_REQCERT=never ldapsearch \
  -H ldaps://localhost:636 \
  -D "uid=appbind,ou=users,dc=example,dc=com" \
  -w "$LDAP_APP_BIND_PASSWORD" \
  -b "dc=example,dc=com" \
  "(objectClass=*)"
```

## Current Limits

- LDAPLite does not currently implement native LDAPS or StartTLS.
- The sidecar must be a raw TCP proxy. Do not use an HTTP reverse proxy mode.
- LDAP healthchecks and telemetry continue to target LDAPLite's plain listener
  unless your deployment adds separate sidecar healthchecks.
