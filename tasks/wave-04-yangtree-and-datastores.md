# TREE-001 — Path tree and datastores

Status: complete
Depends: CFG-001
Owns: internal/yangtree, internal/datastore

## Goal
Compile a profile instance into running/candidate/startup. get / edit / commit / discard / lock.

## Scope
- Path parse (module:container/list[k=v]/leaf)
- compact schema type check
- subtree filter (no xpath)
- per-profile-instance datastores
- sharedProfileDatastore default true (Matt revise for #3; set false for isolated concurrent testers)

## Tests
Commit visible on get running; discard restores; lock-denied; unknown path.

## Acceptance
Two users on two profiles cannot see each other's candidate.
