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

Status: fail-closed `labnetconf.dev/v1alpha1` YAML loads via
`labnetconf validate`. NETCONF 1.0/1.1 framing and hello/RPC XML
codecs are in `internal/ncframing` and `internal/ncrpc`. Start at
`START-HERE.md` and `AGENTS.md`.
