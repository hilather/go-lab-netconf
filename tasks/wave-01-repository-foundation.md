# FND-001 — Repository foundation

Status: complete
Depends: none
Owns: repo root, cmd/labnetconf stub, Makefile, CI

## Goal
Checkout builds, `labnetconf version` works, unimplemented Make
targets exit 1.

## Scope
- go.mod module github.com/hilather/go-lab-netconf, Go 1.26
- Apache-2.0 LICENSE
- Package dirs from docs/01 with package comments only
- Makefile + CI from AGENTS.md
- START-HERE README AGENTS CHANGELOG CONTRIBUTING SECURITY

## Non-scope
Framing, SSH, RESTCONF, REST /v1.

## Acceptance
linux/amd64 build; no import cycles; no sysrepo dependency.
