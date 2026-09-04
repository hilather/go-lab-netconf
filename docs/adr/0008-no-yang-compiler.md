# ADR 0008 — Compact schema, not a YANG compiler

- Status: Accepted
- Date: 2026-09-04

1.0 serves YAML instance trees described by a compact path/type
schema. Module `sourceFile`s are returned by get-schema. There is
no libyang/pyang/sysrepo compile step. Full YANG 1.1 parse is v1.1+.
