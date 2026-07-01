# goldap Protocol Inventory

This inventory captures the protocol surface that must survive replacing
`github.com/lor00x/goldap`.

## Current Dependency Boundary

`goldap/message` currently appears in:

- `go.mod` and `go.sum`
- `internal/protocol/transport.go`
- `internal/protocol/connection.go`
- `internal/protocol/response.go`
- `internal/protocol/extended_response.go`
- protocol tests under `internal/protocol/`
- server handlers under `internal/server/`
- server tests under `internal/server/`

The server currently treats `goldap/message` as its internal LDAP AST. The
replacement should first move that AST behind LDAPLite-owned types, then replace
BER encoding and decoding.

## Request Operations Decoded Today

`protocol.Connection.dispatch` currently switches on these `goldap` request
types:

- `message.BindRequest`
- `message.SearchRequest`
- `message.AddRequest`
- `message.ModifyRequest`
- `message.DelRequest`
- `message.CompareRequest`
- `message.ExtendedRequest`
- `message.UnbindRequest`

`internal/protocol/transport.go` performs TCP framing and then calls
`message.ReadLDAPMessage`.

## Response Operations Encoded Today

`internal/protocol/response.go` builds these response types:

- `message.BindResponse`
- `message.SearchResultEntry`
- `message.SearchResultDone`
- `message.AddResponse`
- `message.ModifyResponse`
- `message.DelResponse`
- `message.CompareResponse`
- `message.ExtendedResponse`

`internal/protocol/extended_response.go` also builds the WhoAmI extended
response. It uses `unsafe` to set `ExtendedResponse.responseValue` because
`goldap` does not expose a setter.

`protocol.Connection.WriteResponse` wraps a response protocol op in
`message.LDAPMessage` and writes the BER bytes returned by `msg.Write()`.

## Request Fields Used By Server Handlers

Bind:

- simple bind DN from `BindRequest.Name()`
- simple password from `BindRequest.AuthenticationSimple()`

Search:

- base DN from `SearchRequest.BaseObject()`
- scope from `SearchRequest.Scope()`
- filter from `SearchRequest.Filter()`
- requested attributes from `SearchRequest.Attributes()`
- types-only flag from `SearchRequest.TypesOnly()`

Add:

- entry DN from `AddRequest.Entry()`
- attributes from `AddRequest.Attributes()`

Modify:

- target DN from `ModifyRequest.Object()`
- changes from `ModifyRequest.Changes()`
- change operation values `0` add, `1` delete, `2` replace

Delete:

- target DN from the `message.DelRequest` value

Compare:

- currently only returns compare false; request assertion fields are not used

Extended:

- request OID from `ExtendedRequest.RequestName()`
- WhoAmI request value is not used

Unbind:

- no request fields are used

## Search Filter Forms

`internal/server/search.go` currently serializes these `goldap` filter forms:

- `message.FilterEqualityMatch`
- `message.FilterPresent`
- `message.FilterAnd`
- `message.FilterOr`
- `message.FilterNot`
- `message.FilterGreaterOrEqual`
- `message.FilterLessOrEqual`
- `message.FilterApproxMatch`
- `message.FilterSubstrings`
- `message.SubstringInitial`
- `message.SubstringAny`
- `message.SubstringFinal`

Unknown filter values currently fall back to `(objectClass=*)` unless their
string representation already starts with `(`.

Functional tests currently cover practical client-facing filters including:

- `(objectClass=*)`
- `(uid=jane)`
- `(cn=Jane Doe)`
- `(mail=jane@example.com)`
- `(sAMAccountName=jane)`
- `(userPrincipalName=jane@example.com)`
- `(&(objectClass=inetOrgPerson)(uid=jane))`
- `(|(uid=jane)(mail=jane@example.com))`
- `(!(uid=missing))`
- `(cn=Jane*)`
- `(member=uid=jane,ou=users,dc=example,dc=com)`
- `(cn=Literal \2a User)`
- `(cn=Literal * User)`

## Compatibility Fixtures Added For Replacement Work

Milestone 1 added protocol tests that pin:

- current BER decoding of representative bind, search, add, delete, and compare
  requests through `protocol.ReadLDAPMessage`;
- exact BER bytes emitted for success bind, search done, add, modify, delete,
  compare false, and WhoAmI extended responses.

These tests are intentionally close to the current protocol boundary. Later
milestones should update them to assert LDAPLite-owned message types while
preserving the same observable BER behavior.
