# LabNETCONF

**One process. NETCONF and RESTCONF. A different device per user.**

Laboratory NETCONF 1.0/1.1 over SSH and RESTCONF over HTTP, with
per-user device profiles and ephemeral candidate/running/startup
datastores.

This is not a production NMS, not ConfD, and not netopeer2.

| | |
|---|---|
| Binary | `labnetconf` |
| Schema | `labnetconf.dev/v1alpha1` |
| NETCONF SSH | `:830` · host residual **10830** |
| RESTCONF | `:8303` · host residual **18303** |
| Control | `/v1` `/mcp` `/` · host **18830** |

Status: `labnetconf serve` binds SSH NETCONF and RESTCONF from a
fail-closed `labnetconf.dev/v1alpha1` document. Management `/v1`
binds only with `--management-listen`. Scratch image UID 65532.
Start at `START-HERE.md` and `AGENTS.md`.

When `spec.ui.enabled` is true and management is bound, `GET /` serves
the operator SPA (cookie `labnetconf_session` + `X-LabNETCONF-CSRF`).
Otherwise `GET /` is 404 problem+json. Local Vite: Node **22.14.0**,
`make web-install`.
