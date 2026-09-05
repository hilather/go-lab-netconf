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
- RFC 8040 JSON RESTCONF listener (`internal/restconf`) on a dedicated
  HTTP port (default `:8303`), not the management mux. Basic auth
  against `spec.users`; management bearer is rejected. Writes call
  `WriteRunningIfCandidateClean` (409 `candidate_dirty` if candidate
  is dirty or locked). Omitted admission CIDRs default to loopback;
  an empty list is deny-all. JSON `application/yang-data+json` only.
