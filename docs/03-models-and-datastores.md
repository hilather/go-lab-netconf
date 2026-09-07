# 03 — Models and datastores

Status: Proposed
Last reviewed: 2026-09-04

## Models (SNMP-map analog)

A **model** (`spec.profiles[]`) is a named device profile:

- `modules[]` advertised through a stub ietf-yang-library
- `schema[]` compact path → type map used by validate
- `instance` YAML tree that becomes running at boot
- optional `startup` YAML tree; default = instance

There is **no YANG 1.1 compiler** in 1.0 (ADR 0008). Names like
`ietf-system` are identifiers. Optional `moduleFile` is stored for
later get-schema; 1.0 may ignore the body.

Two users may share a profile or point at different ones
(split-horizon). With the default `spec.netconf.sharedProfileDatastore: true`,
all users bound to the same profile share one datastore triple, so a
management/SPA/MCP edit dirties the candidate that NETCONF/RESTCONF see.
Set `sharedProfileDatastore: false` and use two users on two profiles for
isolated concurrent testers.

## Path addressing

Internal paths are module-qualified:

```
ietf-system:system/hostname
ietf-interfaces:interfaces/interface[name=eth0]/enabled
```

RESTCONF URI maps RFC 8040 encoding onto the same path.
NETCONF subtree filters map onto the same path.
Unknown path on get → empty data. Unknown path on edit →
`unknown-element` / RESTCONF 400.

## Types (1.0 compact schema)

`string | boolean | int32 | int64 | uint32 | uint64 | decimal64 | identityref | enumeration | leaf-list | list | container | empty`

Range / pattern optional. `valueFrom: processUptime` is the only
dynamic getter (LabSNMP analog). Dynamic leaves are not writable.

## Datastores

| Store | Role |
|---|---|
| running | What get / RESTCONF GET see after commit. RESTCONF writes running when candidate is clean and unlocked. |
| candidate | What NETCONF edit-config targets. Not the RESTCONF write target. |
| startup | copy-config source/dest; wiped on reset |

RESTCONF PATCH/PUT/POST/DELETE call `WriteRunningIfCandidateClean`:
under the profile-instance lock, if candidate is clean and unlocked,
apply the edit to running and copy-forward onto candidate. If
candidate is dirty or locked, the write fails with `candidate_dirty`
(HTTP 409). NETCONF `edit-config` remains `Edit` on candidate plus
`Commit`. RESTCONF must not `Edit(candidate)` then `Commit`.

Lock: one lock per datastore **per profile-instance** (not
process-global). Two profile-instances may hold `candidate`
independently. lock-denied if that instance's datastore is held by
another session. kill-session drops that session's locks.

Generation counter increments on commit and on copy-config that
mutates running or startup. Candidate-only edits do not change
bootstrap `runtimeRevision` and do not increment `storeGeneration`.

## Commit / discard

- commit: validate candidate, running = candidate, notify
- discard-changes: candidate = running
- Failed validate leaves candidate intact

Reset restores all three from bootstrap and drops locks + notif log.
