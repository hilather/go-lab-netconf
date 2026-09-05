# ADR 0011 — Candidate, not writable-running

- Status: Accepted
- Date: 2026-09-04

NETCONF `edit-config` targets candidate. Commit publishes to
running. `:writable-running` is not advertised.

RESTCONF writes running when candidate is clean and unlocked;
otherwise 409 `candidate_dirty`. Writing candidate and then
auto-committing is not used.
