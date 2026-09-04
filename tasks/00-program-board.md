# Program board — LabNETCONF 1.0

Status: Proposed
Last reviewed: 2026-09-04

Agents implement one work package per change. Control-plane order is
CFG → APP → API → SEC → MCP. Data plane proceeds after WIRE.

## Work packages

| Order | Task | ID | Depends | Milestone |
| ---: | --- | --- | --- | --- |
| 1 | Repository foundation | FND-001 | — | M0 |
| 2 | Domain + fail-closed YAML | CFG-001 | FND-001 | M0 |
| 3 | Framing + RPC codec | WIRE-001 | CFG-001 | M1 |
| 4 | Path tree + datastores | TREE-001 | CFG-001 | M1 |
| 5 | SSH NETCONF server | SSH-001 | WIRE-001, TREE-001 | M1 |
| 6 | RESTCONF listener | RC-001 | TREE-001 | M1 |
| 7 | Notifications + wait | NOTIF-001 | WIRE-001, TREE-001 | M1 |
| 8 | Snapshot plan/apply/reset | APP-001 | CFG-001, TREE-001 | M2 |
| 9 | REST /v1 | API-001 | APP-001 | M2 |
| 10 | Bearer + CSRF + audit | SEC-001 | API-001 | M2 |
| 11 | MCP + parity | MCP-001 | API-001, SEC-001 | M2 |
| 12 | Observability | OBS-001 | SSH-001, API-001 | M3 |
| 13 | CLI + scratch image | DEP-001 | SSH-001, RC-001, API-001 | M3 |
| 14 | Operator SPA | UI-001 | API-001, SEC-001 | M4 |
| 15 | Integration-lab BOM | SWAP-001 | MCP-001, SEC-001, DEP-001 | M4 |
| 16 | GA hardening | GA-001 | 1–15 | M5 |
| — | TLS NETCONF + call-home + RESTCONF TLS | TLS-001 | DEP-001 | v1.1 |

## Parallelization

- WIRE-001 is the critical path. Do not invent netopeer types while it is open.
- TREE-001 can proceed from CFG-001 in parallel with WIRE.
- SSH-001 needs WIRE + TREE.
- RC-001 after TREE; can parallel SSH.
- APP-001 after CFG + TREE.
- API after APP; SEC after API; MCP after API+SEC.
- UI required for 1.0 GA; rc.1 may be API-complete.
- SWAP-001 is docs+examples in this repo. Integrator PR is out of band.

## Frozen decisions

- Q1: labinfo id and compose name are `labnetconf` from day one.
- Q2: NETCONF SSH + RESTCONF both 1.0.
- Q3: no YANG compiler; candidate+running+startup in 1.0.
- Q4: per-user profiles.
- Q5: NETCONF edits candidate; no writable-running capability.
- Q6: RESTCONF writes running iff candidate clean, else 409 `candidate_dirty`.
- Q7: call-home / RFC 7589 / RESTCONF TLS are v1.1.
- Q8: ports 10830 / 18303 / 18830.
