# ADR 0005 — Lab static bearer

- Status: Accepted
- Date: 2026-09-04

Management auth lives at `spec.auth`. `spec.management.auth` is
unknown. Mode is `bearer` only. Data-plane SSH/RESTCONF users are
`spec.users[]` and are not management tokens.
