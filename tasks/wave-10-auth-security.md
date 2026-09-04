# SEC-001 — Bearer, CSRF, audit

Status: not-started
Depends: API-001
Owns: internal/auth, internal/audit

## Goal
Bearer file-ref on /v1. Session cookie + CSRF for the SPA. Audit ring on mutations.

## Tests
missing/short token; CSRF missing on cookie POST; RESTCONF Basic unchanged.
