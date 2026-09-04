# ADR 0002 — First-party NETCONF and RESTCONF

- Status: Accepted
- Date: 2026-09-04

## Context

Wrapping netopeer2/sysrepo makes "never call-home" a config flag and
couples the lab to a C stack.

## Decision

- First-party framing + RPC + datastore + RESTCONF router.
- `golang.org/x/crypto/ssh` allowed behind `internal/netconfssh`.
- Production must not import sysrepo bindings, must not exec
  netopeer2, sysrepod, ConfD, or yangson.
- Tests may use ncclient / scrapli / netopeer2-cli as *clients*.

## Consequences

Wire semantics live in docs/02 and docs/03. AST tests prove the fence.
