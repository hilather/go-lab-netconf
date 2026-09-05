# NOTIF-001 — Notification ring

Status: complete
Depends: WIRE-001, TREE-001
Owns: internal/notif

## Goal
config-change events on commit. create-subscription for a session. Tester wait API.

## Scope
- Bounded ring, wait, wipe
- RFC 5277 stream NETCONF only
- No outbound socket

## Tests
AST no Dial; wait existing/inserted/timeout/wipe.

## Acceptance
commit then notifications:wait returns the record.
