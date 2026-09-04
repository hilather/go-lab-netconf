# APP-001 — Snapshot, plan/apply/reset

Status: not-started
Depends: CFG-001, TREE-001
Owns: internal/app, internal/compiler, internal/snapshot

## Goal
Compile snapshot, swap atomically, plan/apply closed ops, reset drops dirty + notifs.

## Scope
- Closed operations from docs/04
- expectedRevision + idempotency
- datastore set/commit/discard paths

## Tests
revision mismatch; idempotent apply; reset restores hostname after commit.
