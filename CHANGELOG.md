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
