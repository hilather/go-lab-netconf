# ADR 0011 — Candidate, not writable-running

- Status: Accepted
- Date: 2026-09-04

edit-config and RESTCONF writes target candidate. Commit publishes
to running. `:writable-running` is not advertised. RESTCONF
`autoCommit` default true is a lab shortcut that commit()s after
each HTTP write.
