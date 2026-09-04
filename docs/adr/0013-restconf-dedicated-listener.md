# ADR 0013 — RESTCONF is a dedicated listener

- Status: Accepted
- Date: 2026-09-04

`/restconf` is not mounted on management `:8088`. Management REST
`/v1` is the operator/MCP surface. RESTCONF is the SUT surface on
`:8303` / host 18303. LabSSO keeps dest-443.
