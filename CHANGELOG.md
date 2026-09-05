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
- NETCONF 1.0 EOM (`]]>]]>`) and 1.1 chunked (`\n#N\n` … `\n##\n`)
  framing in `internal/ncframing`, plus hello/RPC XML codec in
  `internal/ncrpc`. Advertised capabilities are base:1.0, base:1.1,
  candidate, startup, validate, and notification; writable-running
  and xpath are not emitted. `message-id` is preserved. Session
  goldens live under `testdata/sessions`. `make test-fuzz-smoke` is
  a required CI job.
- Compact path tree (`internal/yangtree`) and per-profile-instance
  running/candidate/startup stores (`internal/datastore`). NETCONF
  `edit-config` targets candidate; `Commit` publishes to running
  and increments `storeGeneration`. RESTCONF writes running through
  `WriteRunningIfCandidateClean` when candidate is clean and
  unlocked, else `candidate_dirty`. Locks are per profile-instance.
- SSH NETCONF data plane (`internal/netconfssh`, `internal/ncserver`)
  listens on TCP, authenticates `spec.users` via `passwordFile`
  and/or `authorizedKeysFile`, and accepts only subsystem `netconf`.
  `internal/nctest` is the in-repo hello + get-config client.
  Admission omitted CIDRs default to loopback; an empty list is
  deny-all. `create-subscription` succeeds and does not write
  notification messages on the session.
- RFC 8040 JSON RESTCONF listener (`internal/restconf`) on a dedicated
  HTTP port (default `:8303`), not the management mux. Basic auth
  against `spec.users`; management bearer is rejected. Writes call
  `WriteRunningIfCandidateClean` (409 `candidate_dirty` if candidate
  is dirty or locked). Omitted admission CIDRs default to loopback;
  an empty list is deny-all. JSON `application/yang-data+json` only.
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
- `labnetconf serve` binds SSH NETCONF, RESTCONF, and optional
  management (`--management-listen` defaults off). `healthcheck`
  probes `GET /v1/health/ready`. `mcp-stdio` requires `--config` and
  `--token-file`. Scratch image `ghcr.io/hilather/labnetconf` runs
  UID 65532 with HEALTHCHECK ready and CMD
  `--management-listen=:8088`. `make test-container` smokes
  `:1830`/`:8303` with `cap_drop ALL`. `examples/compose.smoke.yaml`
  uses the testdata lab host key.
