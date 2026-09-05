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
- RFC 5277 stream `NETCONF` config-change ring in `internal/notif`.
  `Ring` implements Sink and Waiter (OnCommit on commit only). Wait
  returns an existing or inserted record, or `wait_timeout` /
  `store_wiped`. Reset and restart wipe the log. Ids are ULIDs
  (`github.com/oklog/ulid/v2`). `Nop` remains for tests. `serve` is
  not wired yet, so `cmd/labnetconf` is unchanged.
