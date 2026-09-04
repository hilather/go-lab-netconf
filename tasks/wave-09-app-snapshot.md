# APP-001 — Snapshot, plan/apply/reset

Status: not-started
Depends: CFG-001, DS-001
Owns: internal/app, internal/compiler, internal/snapshot

Compile snapshot, closed apply ops, reset restores three datastores
and wipes notifications. datastore:set is not an apply verb.
