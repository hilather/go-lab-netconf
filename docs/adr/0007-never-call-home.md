# ADR 0007 — Never call home

- Status: Accepted
- Date: 2026-09-04

RFC 8071 is outbound Dial. Production must not Dial a NETCONF or
RESTCONF peer. `callHome.enabled: true` rejects in 1.0. Reserved
keys `callHome*`, `manager*`, `remote*` reject. RFC 5277
subscriptions stay in-process.
