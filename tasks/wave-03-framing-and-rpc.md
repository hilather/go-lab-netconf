# FRAME-001 — NETCONF framing and RPC codec

Status: not-started
Depends: CFG-001
Owns: internal/ncframing, internal/ncrpc, testdata/sessions

## Goal
Encode/decode hello + RPCs with 1.0 EOM and 1.1 chunked framing.

## Non-scope
SSH, datastore.

## Tests
Goldens; fuzz decoder; request-id preservation.
