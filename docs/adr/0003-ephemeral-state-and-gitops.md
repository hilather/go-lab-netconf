# ADR 0003 — Ephemeral state and GitOps

- Status: Accepted
- Date: 2026-09-04

Desired state is one YAML document. Candidate dirty bits, running
edits beyond bootstrap, and the notification log are not desired
state. Reset rereads bootstrap into running+candidate+startup. The
process never writes the config file.
