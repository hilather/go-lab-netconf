# Examples

Two consumers, two files:

- `compose.smoke.yaml` — local appliance smoke (DEP-001):
  `--netconf-listen=:1830 --restconf-listen=:8303`, `cap_drop: ALL`,
  testdata lab host key. Never a production key.
- Integrator BOM (SWAP-001) — copy-paste into `mcp-integration-lab`.
  Referenced by [docs/13](../docs/13-integration-lab-swap.md).

| This repo | Integrator destination |
|---|---|
| `labnetconf.yaml` | `profiles/default/labnetconf/bootstrap.yaml` |
| `mcpjungle/labnetconf.json` | `profiles/default/mcpjungle/servers/labnetconf.json` (filename = name `labnetconf`) |
| `labinfo/services-labnetconf.yaml` | merge into `profiles/default/labinfo/services.yaml` |
| `profile.env` | merge into `profiles/default/profile.env` |

`profile.env` names `LABNETCONF_SSH_PORT=10830`,
`LABNETCONF_RESTCONF_PORT=18303`, `LABNETCONF_REST_PORT=18830`.

Vendor pin (documented, not applied here): URL
`https://github.com/hilather/go-lab-netconf`, dest
`third_party/go-lab-netconf`, ref `v1.0.0-rc.1`. Integrator pin is
LAST (out of band after the GA tag). Do not implement `vendor.go` in
this repo. Append `labnetconf` to the Jungle integration tool group
when the integrator PR lands.
