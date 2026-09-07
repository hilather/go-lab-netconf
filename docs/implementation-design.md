# LabNETCONF implementation design

**Status:** accepted for implementation
**Date:** 2026-09-04
**Target:** https://github.com/hilather/go-lab-netconf
**API group:** `labnetconf.dev/v1alpha1`
**License:** Apache-2.0

## Overview

Single-process Go lab appliance. SUTs speak NETCONF over SSH :830
and RESTCONF over HTTP :8303 against YAML device profiles bound per
user. Candidate is the edit overlay; commit publishes to running;
reset restores bootstrap. Never wrap sysrepo/netopeer2. Never call-home.

Closest siblings: LabNTP (first-party responder), LabSNMP (per-identity
maps + no schema compiler), LabMail (receive-only wait store).

## Goals (1.0)

- NETCONF 1.0 + 1.1 framing, SSH subsystem `netconf`
- RESTCONF JSON on a dedicated listener
- running + candidate + startup
- Per-user named profiles (split-horizon)
- edit-config / RESTCONF / commit / discard / lock
- RFC 5277 config-change notifications in-process
- YAML GitOps, REST `/v1`, MCP `2026-07-28`, operator UI
- Scratch image UID 65532
- Examples BOM for mcp-integration-lab

## Non-goals (1.0)

- sysrepo / netopeer2 / ConfD / OpenYuma / NSO wrap
- Full YANG 1.1 compiler
- NETCONF over TLS RFC 7589
- Call-home RFC 8071
- YANG-Push
- NMDA operational datastore
- confirmed-commit
- RESTCONF / NETCONF on dest-443 (LabSSO owns 443)
- Product logic in mcp-integration-lab

## Decisions

| ID | Decision |
|---|---|
| D1 | Module `github.com/hilather/go-lab-netconf` |
| D2 | First-party framing + RPC; ssh transport-only in netconfssh |
| D3 | Three listeners, one process |
| D4 | Never call-home / never Dial a peer (ADR 0007) |
| D5 | YAML KnownFields; camelCase; kebab reject |
| D6 | `spec.auth` bearer-only; `spec.management.auth` unknown |
| D7 | MCP `2026-07-28`; tools `netconf_*`; resources `labnetconf://` |
| D8 | Control-plane order REST → Auth → MCP |
| D9 | User → exactly one profile |
| D10 | NETCONF edits target candidate; commit publishes; writableRunning true rejects |
| D11 | RESTCONF writes running if candidate clean else 409 candidate_dirty |
| D12 | RFC 7589 / RFC 8071 / RESTCONF TLS enable rejected in 1.0 |
| D13 | Host residual 10830 / 18303 / 18830 |
| D14 | Local escape `:1830` / `:8303` |
| D15 | labinfo id `labnetconf` from day one |
| D16 | UI required for 1.0 GA |
| D17 | `allowLegacyClients` default false; lab overlay true |
| D18 | Hand-rolled OpenMetrics |
| D19 | Official MCP SDK only on the adapter |
| D20 | `valueFrom: processUptime` is the only dynamic source |
| D21 | PasswordFile and/or authorizedKeysFile; no inline secrets |
| D22 | No userland-proxy readiness gate (identity is user) |
| D23 | Placeholder Make targets fail closed |
| D24 | Integrator pin LAST |
| D25 | Go 1.26, Apache-2.0, image `ghcr.io/hilather/labnetconf` |
| D26 | Cookie `labnetconf_session`; CSRF `X-LabNETCONF-CSRF` |
| D27 | Ready = enabled listeners bound + snapshot + (mgmt bound or off) |
| D28 | sharedProfileDatastore default true (Matt revise for #3) |
| D29 | Do not advertise xpath in 1.0 |
| D30 | JSON RESTCONF required; XML optional later |

## Package map

```
cmd/labnetconf
internal/{model,config,compiler,snapshot,yangtree,datastore,
          ncframing,ncrpc,netconfssh,ncserver,restconf,notif,
          app,capabilities,control/rest,control/mcp,auth,audit,
          domainerr,observability,buildinfo,web,testutil,nctest}
api/{jsonschema,openapi,mcp,capabilities,metrics,errors}
web testdata examples docs tasks scripts
```

## CLI

```
labnetconf version
labnetconf validate --config FILE
labnetconf canonicalize --config FILE
labnetconf serve --config FILE [--netconf-listen ADDR|off] [--restconf-listen ADDR|off] [--management-listen ADDR|off]
labnetconf healthcheck --url URL
labnetconf mcp-stdio --config FILE --token-file FILE
```

## PR plan

See `tasks/00-program-board.md`. WIRE-001 (framing + RPC) is the
critical path. Do not let CFG or UI invent netopeer types.
