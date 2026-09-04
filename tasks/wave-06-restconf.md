# RC-001 — RESTCONF listener

Status: not-started
Depends: TREE-001
Owns: internal/restconf

## Goal
HTTP :8303 RFC 8040 JSON against the same datastores.

## Scope
- /restconf/data GET PUT POST PATCH DELETE
- /restconf/yang-library-version
- /.well-known/host-meta
- Basic auth against spec.users
- Write running iff candidate clean; else 409 candidate_dirty
- tls.enabled true already rejected at config

## Tests
GET hostname; PATCH when clean updates running+candidate; PATCH when dirty → 409; 401; management token does not unlock RESTCONF.

## Acceptance
curl against :8303 sees a NETCONF commit from :1830.
