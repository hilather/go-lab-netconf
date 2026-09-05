# Changelog

## [Unreleased]

### Added

- Design pack for LabNETCONF 1.0 (NETCONF over SSH + RESTCONF).
- Module `github.com/hilather/go-lab-netconf` (Go 1.26), Apache-2.0
  LICENSE, `cmd/labnetconf` (`version` via `internal/buildinfo`),
  package-comment stubs for the canonical map, `internal/notif`
  Sink/Waiter/Nop, and Dial / forbidden-module AST tests in
  `internal/testutil`.
- Fail-closed Makefile (LabNTP family target names) and required CI
  jobs: format, lint, unit, race, documentation, changelog,
  security-scan (`govulncheck` v1.1.4). Unimplemented targets exit 1.
- `labnetconf.dev/v1alpha1` fail-closed YAML (`internal/model`,
  `internal/config`, `internal/domainerr`). `labnetconf validate`
  and `canonicalize` load KnownFields camelCase documents, default
  omitted `allowClientCidrs` to loopback, treat `[]` as deny-all,
  and print a SHA-256 revision over secret paths (never bytes).
  `make test-config-compat` is a required CI job.
- Compact path tree (`internal/yangtree`) and per-profile-instance
  running/candidate/startup stores (`internal/datastore`). NETCONF
  `edit-config` targets candidate; `Commit` publishes to running
  and increments `storeGeneration`. RESTCONF writes running through
  `WriteRunningIfCandidateClean` when candidate is clean and
  unlocked, else `candidate_dirty`. Locks are per profile-instance.
- Snapshot compiler (`internal/compiler`, `internal/snapshot`) and
  plan/apply/reset (`internal/app`). Closed apply ops from docs/04
  require `expectedRevision` and `Idempotency-Key`. Reset rereads
  bootstrap, restores running/candidate/startup, drops locks, and
  wipes notifications; the process never writes the bootstrap file.
  `datastore:set` / commit / discard are Service methods, not apply
  verbs.
- Management REST `/v1` adapter (`internal/control/rest`) over the
  shared capability registry. Every PARITY_REQUIRED row plus
  unauthenticated health live/ready; `application/problem+json`
  including `candidate_dirty`. `GET /` is 404 problem+json. `/restconf`
  is not mounted on management. `make generate` / `verify-generated`
  write and check `api/capabilities/v1.json`, `api/openapi/v1.json`,
  and `api/errors/v1.json`.
- Management bearer (`spec.auth` file-ref, ≥32 bytes) and SPA cookie
  `labnetconf_session` with CSRF header `X-LabNETCONF-CSRF` on cookie
  POST. Origins exact-match `allowedOrigins`. Basic is rejected on
  `/v1` (401 Bearer); RESTCONF Basic against `spec.users` is unchanged.
  Administrator has all scopes; reader has `netconf.read`. Mutations
  append to an in-process audit ring. Secret bytes stay out of GET
  state, the users list, info logs, and metric labels.
- MCP Streamable HTTP adapter (`internal/control/mcp`) over the shared
  `app.Service`. Protocol `2026-07-28`, official SDK v1.7.0 only on
  this adapter, `POST /mcp`, Bearer only, `allowLegacyClients` default
  false. Every PARITY_REQUIRED `netconf_*` tool and `labnetconf://`
  resource is registered. MCP does not HTTP-call REST.
  `labnetconf mcp-stdio --config FILE --token-file FILE` requires
  `--token-file`. `make test-parity` is a required CI job and fails
  on a missing twin.
