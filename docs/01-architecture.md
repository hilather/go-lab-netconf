# 01 — Architecture

**Last reviewed:** 2026-09-04

## Problem

QA needs a NETCONF/RESTCONF device that is identity-accurate and tree-accurate. One controller commits hostname via NETCONF as `ops`. Another PATCHes RESTCONF as `vendor` and must *not* see `ops` tree. Reset restores bootstrap. The lab host must not run netopeer2.

## Naming

| Kind | Value |
|---|---|
| Product | LabNETCONF |
| Module | `github.com/hilather/go-lab-netconf` |
| Binary | `labnetconf` |
| Image | `ghcr.io/hilather/labnetconf` |
| Schema | `labnetconf.dev/v1alpha1` |
| Kind | `LabNETCONF` |
| NETCONF | container `:830` host `10830` |
| RESTCONF | container `:8303` host `18303` |
| Management | container `:8088` host `18830` |
| Cookie / CSRF | `labnetconf_session` / `X-LabNETCONF-CSRF` |
| labinfo id | `labnetconf` |

## Invariants

1. Three listeners, one process. Data planes keep accepting if management is off.
2. Never call-home. No Dial. Accept only.
3. Never write the bootstrap file.
4. KnownFields camelCase.
5. Secrets file-ref. Management token ≥32 bytes.
6. User → exactly one device profile.
7. Candidate + running + startup in 1.0. No writable-running capability.
8. RFC 7589 / RFC 8071 enabled:true rejected in 1.0.
9. No full YANG compiler in 1.0.
10. RESTCONF and NETCONF share the same running after commit.

## Process model

```text
SUT SSH :830  --> netconfssh --> ncserver --> ncframing/ncrpc --> datastore
SUT HTTP :8303 --> restconf  -------------------------------------^
                                              yangtree
                     atomic.Pointer[Snapshot]
                              ^
                     compiler.Compile(YAML profiles + users)
```

## Packages

Canonical Go packages are `internal/ncframing` + `internal/ncrpc`
(not `netconfwire`) and `internal/notif` (not `store`). See
[implementation-design.md](implementation-design.md).

| Package | Role |
|---|---|
| `cmd/labnetconf` | CLI |
| `internal/ncframing` `ncrpc` | Hello, RPC XML, 1.0 `]]>]]>` and 1.1 chunked framing |
| `internal/netconfssh` | `x/crypto/ssh` adapter, subsystem `netconf` |
| `internal/yangtree` | Path tree (SNMP `mibtree` analog) |
| `internal/datastore` | running / candidate / startup + lock |
| `internal/ncserver` | NETCONF session state machine |
| `internal/restconf` | RFC 8040 adapter over yangtree |
| `internal/notif` | RFC 5277 config-change ring (Sink/Waiter; Nop for tests) |
| `internal/compiler` `snapshot` `config` `model` | GitOps |
| `internal/app` | plan/apply/reset/preview |
| `internal/auth` `audit` `capabilities` | control |
| `internal/control/rest` `control/mcp` | adapters |
| `internal/web` | SPA embed |
| `internal/nctest` | test client |

## Import fence

- framing/rpc/ssh/yangtree/datastore/ncserver/restconf/notif must not import control/web
- production rest must not import web
- forbidden wrap: sysrepo, netopeer2, confd, openyuma, nso, freeconf-as-server
- `golang.org/x/crypto/ssh` only in `internal/netconfssh`
- no Dial; no exec `netopeer2-server`, `sysrepod`, `confd`
