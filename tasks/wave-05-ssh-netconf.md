# SSH-001 — SSH NETCONF server

Status: not-started
Depends: WIRE-001, TREE-001
Owns: internal/netconfssh, internal/ncserver

## Goal
Listen TCP, SSH subsystem `netconf`, authenticate spec.users, run a NETCONF session.

## Scope
- x/crypto/ssh adapter, types do not leak
- hostKeyFile
- password + public key
- session table
- management-off still answers

## Tests
nctest hello + get-config; bad password; wrong subsystem name rejected; no Dial.

## Acceptance
A test SSH client on :1830 completes hello and get-config running.
