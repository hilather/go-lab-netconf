# Agent guide — LabNETCONF

Laboratory NETCONF + RESTCONF device. Sibling of LabNTP (responder)
and LabSNMP (per-identity maps). Vendored later by mcp-integration-lab.

Read first: docs/00-family-evaluation.md, 01-architecture.md,
02-netconf-restconf-semantics.md, 04-state-and-configuration.md,
05-control-plane-and-parity.md, relevant ADRs.

Do not invent capability IDs, YANG compiler scope, or ports.

## Identity

| Field | Value |
|---|---|
| Product | LabNETCONF |
| Binary | `labnetconf` |
| Module | `github.com/hilather/go-lab-netconf` |
| Image | `ghcr.io/hilather/labnetconf` |
| Schema | `labnetconf.dev/v1alpha1` |
| Cookie / CSRF | `labnetconf_session` / `X-LabNETCONF-CSRF` |
| labinfo id | `labnetconf` |
| MCP | `netconf_*` / `labnetconf://` / `2026-07-28` |
| Ports | SSH :830/10830, RESTCONF :8303/18303, mgmt :8088/18830 |

## Ground rules

1. Two data planes + one management plane, one process. Data planes
   work if management is off.
2. No Dial in netconfssh/ncserver/ncframing/ncrpc/datastore/restconf/app/notif.
   No call-home. No manager CLI.
3. Never write bootstrap YAML. Reset restores instance trees and
   wipes notifications.
4. KnownFields, camelCase, `spec.auth` bearer-only.
5. Secrets file refs. Token ≥32. Host key file. User password/keys files.
6. REST and MCP are adapters. Order CFG → APP → API → SEC → MCP.
   Production files in `internal/control/rest` must not import
   `internal/web`. `cmd/labnetconf` wires `rest.Config.UI` and
   `UIEnabled` from the live snapshot. Tests in `rest` may import `web`.
7. First-party framing/RPC. x/crypto/ssh only in netconfssh.
   No sysrepo/netopeer2/ConfD/OpenYuma/NSO.
8. No full YANG compiler. No writable-running capability.
9. RESTCONF HTTP only. tls.enabled true rejects.
10. No Prometheus client.
11. Docs ship with the change.
12. Identity for views is SSH/RESTCONF *user → profile*, not client IP.

## Allowed deps

yaml.v3, MCP SDK v1.7.0, ulid, golang.org/x/crypto/ssh (adapter only).

## Locked tests

KnownFields, reserved keys, tls.enabled reject, writableRunning
true reject, Dial AST, forbidden modules, framing 1.0 and 1.1,
commit moves candidate to running, discard restores candidate from
running, candidate isolation (two users / two profiles), commit
visible on RESTCONF GET, unknown user 401 (RESTCONF), reset
restores hostname.
