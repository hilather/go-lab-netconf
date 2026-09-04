# DEP-001 — CLI and scratch image

Status: not-started
Depends: SSH-001, RC-001, API-001
Owns: cmd/labnetconf, Dockerfile, examples/compose.smoke.yaml

## Goal
serve/validate/canonicalize/healthcheck/mcp-stdio. Image UID 65532.

## Tests
test-container on :1830/:8303 with cap_drop ALL.
