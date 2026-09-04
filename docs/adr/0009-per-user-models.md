# ADR 0009 — Per-user models

- Status: Accepted
- Date: 2026-09-04

Each SSH/RESTCONF user names exactly one model/profile. Two users
may share a model or use different models (split-horizon). Identity
is the user, not the client IP.
