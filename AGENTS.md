# AGENTS.md

This file is the working guide for Codex/agent changes in this repository. Keep it short and operational. Put broad product docs in `README.md` or `docs/` instead of duplicating them here.

## Project Snapshot

LDAPLite is a lightweight LDAP v3 server written in Go with a SQLite backend.

Key stack:
- Go 1.25.3
- SQLite via `modernc.org/sqlite`
- Repo-owned LDAP BER encoding/decoding in `internal/protocol/`
- CLI via Cobra
- Password hashing with Argon2id
- Web UI assets built with npm/Vite/React/Tailwind/shadcn/ui

## Search And Editing Rules

Use `ast-grep` for Go structural searches and refactors:

```bash
ast-grep --lang=go --pattern 'func $NAME($$$) $$$' cmd internal pkg
ast-grep --lang=go --pattern 'type $NAME struct { $$$ }' internal pkg
ast-grep --lang=go --pattern '$OBJ.$METHOD($$$)' internal pkg
```

Use `rg` for ordinary text and file searches.

Do not rely on stale line numbers in docs or comments. Prefer symbol names, tests, and direct source inspection.

## Common Commands

### Build

```bash
make build
# or
npm run build:css
go build -o bin/ldaplite ./cmd/ldaplite
```

`make build` builds embedded Web UI assets before compiling the binary.

### Test

```bash
make test
make test-functional
make test-coverage
```

`make test` runs `npm run build:css` and then `go test -v -race ./...`.

`make test-functional` runs the AD-like black-box compatibility suite:

```bash
go test -tags=functional -v ./tests/functional/...
```

The functional suite starts a real `ldaplite server` subprocess on a random local port, uses a temporary SQLite database, and drives LDAP operations through `github.com/go-ldap/ldap/v3`.

Direct `go test ./...` requires embedded Web UI assets to already exist because the Web UI embeds `internal/web/static/`. Prefer `make test` on a fresh checkout.

### Run Locally

```bash
export LDAP_BASE_DN=dc=example,dc=com
export LDAP_ADMIN_PASSWORD=your-secure-password
export LDAP_DATABASE_PATH=/tmp/ldaplite-data/ldaplite.db
./bin/ldaplite server
```

### LDAP Smoke Tests

```bash
ldapsearch -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w ChangeMe123! \
  -b "dc=example,dc=com" \
  "(objectClass=*)"

ldapwhoami -H ldap://localhost:3389 \
  -D "uid=admin,ou=users,dc=example,dc=com" \
  -w ChangeMe123!
```

### Docker

```bash
make docker-build
make docker-run
make docker-stop
make docker-logs
```

## CI And Release

GitHub Actions workflows live in `.github/workflows/`.

### Branch And PR Validation

`test.yml` runs on pushes to all branches and on pull requests. It:
- installs Node dependencies with `npm ci`
- builds CSS with `npm run build:css`
- sets up Go 1.25
- runs `go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...`
- runs `make test-functional`
- uploads coverage to Codecov on a best-effort basis
- runs `go vet ./...`
- checks formatting with `gofmt -l .`
- runs `go build -v ./...`

### Release Pipeline

`release.yml` runs only when a tag matching `v*.*.*` is pushed. It:
- runs the normal Go test suite and `make test-functional`
- builds Linux `amd64` and `arm64` tarballs
- builds and pushes multi-arch Docker images to GHCR
- creates a GitHub Release with tarballs attached
- derives release artifact versions from the tag with `${GITHUB_REF#refs/tags/}`
- injects the tag into release binaries with `-X main.version=...`
- passes the tag to Docker builds as the `VERSION` build arg

Release artifact runtime versions are tag-driven. Source defaults still exist for local/dev builds in:
- `cmd/ldaplite/main.go`
- `Makefile`
- `package.json`
- `package-lock.json`

Keep those defaults and `CHANGELOG.md` aligned with the intended release tag, but remember the tag controls released binary/container versions.

### Release Checklist

1. Decide the next semver tag, e.g. `v0.8.0`.
2. Update local/default versions:
   - `cmd/ldaplite/main.go`
   - `Makefile`
   - `package.json`
   - `package-lock.json`
3. Add a `CHANGELOG.md` entry.
4. Verify locally:
   ```bash
   npm ci
   npm run build:css
   go test -v -race ./...
   make test-functional
   ```
5. Commit the release changes.
6. Tag and push:
   ```bash
   git tag vX.Y.Z
   git push origin <branch>
   git push origin vX.Y.Z
   ```
7. Confirm the `Release` workflow publishes the GitHub Release and GHCR images.

Do not add a second CI or release workflow unless the existing `test.yml` and `release.yml` cannot support the change.

## Source Map

- CLI entry point: `cmd/ldaplite/main.go`
- LDAP server and operation handlers: `internal/server/ldap.go`
- BER transport/connection/response helpers: `internal/protocol/`
- SQLite store and migrations: `internal/store/`
- LDAP models: `internal/models/`
- LDAP filter parsing/compilation: `internal/schema/`
- Config: `pkg/config/`
- Password hashing: `pkg/crypto/`
- Web UI: `internal/web/`
- Functional compatibility tests: `tests/functional/`
- Roadmap and broader docs: `README.md`, `docs/ROADMAP.md`

## Critical Invariants

Password handling:
- `userPassword` must never be stored in the generic `attributes` table.
- Password hashes live only in `users.password_hash`.
- LDAP searches must not return `userPassword`.
- Add/Modify password changes must go through `ProcessPassword()`.
- Unsupported password schemes must return LDAP constraint violation (`19`).
- Bind must retrieve password hashes only through `GetUserPasswordHash` and verify with constant-time password verification.

Storage and directory behavior:
- Generic LDAP attributes belong in the EAV `attributes` table unless they are security-sensitive or relationship-only data.
- Specialized tables should remain minimal: passwords and referential-integrity markers/relationships.
- Entry placement must stay within the configured base DN and below an existing parent, except for the base DN itself.
- Group `member` references must point to existing entries.
- `memberOf` is computed from `group_members`, not stored as a normal attribute.

LDAP behavior:
- Return meaningful LDAP result codes instead of generic operations errors when the failure class is known.
- Bind/search/write access policy is security-sensitive; run relevant server tests and functional tests when changing it.
- RootDSE and schema discovery are public; normal searches and writes require the configured bind behavior.
- Operational/structural attributes such as `objectClass`, `createTimestamp`, and `modifyTimestamp` are server-managed and protected from client modification.

Compatibility testing:
- Keep `tests/functional/` black-box: use a real server subprocess and real LDAP client library.
- The AD-like suite is practical client compatibility, not full Active Directory emulation.
- Out of scope unless explicitly requested: Kerberos, SASL/GSSAPI, Global Catalog, DirSync, paging controls, server-side sorting controls, AD recursive matching rule, and full Microsoft schema semantics.

## Adding Features Safely

New LDAP attributes:
- Prefer generic attributes through `models.Entry.Attributes`.
- Add model/server special handling only when validation, security, or computed behavior requires it.

New LDAP operations:
- Add handlers in `internal/server/ldap.go`.
- Use existing store APIs where possible: `CreateEntry`, `GetEntry`, `UpdateEntry`, `DeleteEntry`, `SearchEntries`, `EntryExists`.
- Add protocol/server tests and functional coverage when behavior is client-visible.

New object classes:
- Add or update model validation in `internal/models/`.
- Update `CreateEntry`/store handling only for required validation or specialized storage.
- Add migrations only when new non-attribute storage is truly required.

## Current Intentional Limits

- Native LDAPS and StartTLS use operator-provided PEM certificate/key files.
- No SASL authentication.
- No extensible matching or AD recursive matching rule.
- No full schema extension system.
- No replication or high availability.
- Single-file SQLite storage, intended for small-to-medium deployments.
