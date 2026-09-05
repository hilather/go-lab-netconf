# 13 — Integration with MCP Integration Lab

Last reviewed: 2026-09-04

Integrator change is LAST (D24). This document is the BOM. Product
logic never moves into `mcp-integration-lab`. There is **no**
NETCONF service in the lab today — SWAP-001 adds the overlay files
in *this* repo only. **Do not implement `vendor.go`. Do not change
`mcp-integration-lab` in this PR.** The integrator pin is out of
band after the GA tag.

Copy-paste sources live under `examples/`.

| This repo | Integrator destination |
|---|---|
| `examples/labnetconf.yaml` | `profiles/default/labnetconf/bootstrap.yaml` |
| `examples/mcpjungle/labnetconf.json` | `profiles/default/mcpjungle/servers/labnetconf.json` |
| `examples/labinfo/services-labnetconf.yaml` | merge into `profiles/default/labinfo/services.yaml` |
| `examples/profile.env` | merge into `profiles/default/profile.env` |
| `examples/compose.smoke.yaml` | local appliance smoke only (`:1830`/`:8303`, `cap_drop: ALL`) |

Do not recopy `testdata/config/valid/full.yaml` into the lab overlay.

## Naming

| Kind | Value |
|---|---|
| Compose service | `labnetconf` |
| labinfo id | `labnetconf` |
| Jungle name / file | `labnetconf` / `labnetconf.json` |
| Token | `secrets/labnetconf-token` `0o644` |
| Host key | `secrets/labnetconf-hostkey` `0o644` |
| Alice password | `secrets/netconf-alice` `0o644` |
| Config mount | `/etc/labnetconf/config.yaml` |
| Image local | `labnetconf:local` from `./third_party/go-lab-netconf` |

## Vendor pin

Documented here for the later integrator PR. Not applied in this
repo. Pin LAST after the GA tag.

```
URL:  https://github.com/hilather/go-lab-netconf
Dest: third_party/go-lab-netconf
Ref:  v1.0.0-rc.1
```

## Compose

Integrator compose (not `examples/compose.smoke.yaml`):

```yaml
labnetconf:
  image: labnetconf:local
  build: ./third_party/go-lab-netconf
  command: ["serve", "--config=/etc/labnetconf/config.yaml", "--management-listen=:8088"]
  user: "65532:65532"
  read_only: true
  tmpfs: ["/tmp"]
  cap_drop: [ALL]
  cap_add: [NET_BIND_SERVICE]
  security_opt: ["no-new-privileges:true"]
  ports:
    - "${LABNETCONF_SSH_PORT:-10830}:830/tcp"
    - "${LABNETCONF_RESTCONF_PORT:-18303}:8303/tcp"
    - "${LABNETCONF_REST_PORT:-18830}:8088/tcp"
  volumes:
    - ${MCPLAB_PROFILE_DIR:-./profiles/default}/labnetconf/bootstrap.yaml:/etc/labnetconf/config.yaml:ro
    - ./secrets/labnetconf-token:/run/secrets/labnetconf-token:ro
    - ./secrets/labnetconf-hostkey:/run/secrets/labnetconf-hostkey:ro
    - ./secrets/netconf-alice:/run/secrets/netconf-alice:ro
  healthcheck:
    test: ["CMD", "/labnetconf", "healthcheck", "--url=http://127.0.0.1:8088/v1/health/ready"]
    interval: 5s
    timeout: 3s
    retries: 12
    start_period: 3s
```

`cap_add NET_BIND_SERVICE` is required because the process binds
`:830` inside the container even when the host publish is 10830.

## profile.env

Names in `examples/profile.env`:

```
LABNETCONF_SSH_PORT=10830
LABNETCONF_RESTCONF_PORT=18303
LABNETCONF_REST_PORT=18830
```

IANA dest is 830. Residual is the shipped default because many
hosts already run something on 830 or cannot bind it. RESTCONF dest
443 belongs to LabSSO; do not move RESTCONF there.

## labinfo

Id `labnetconf`. Snippet: `examples/labinfo/services-labnetconf.yaml`.
Must include `urls` + `connection`.

Connection endpoints:

- `netconf-ssh` `${LAB_PUBLIC_HOST}:${LABNETCONF_SSH_PORT}` subsystem `netconf`
- `restconf` `http://${LAB_PUBLIC_HOST}:${LABNETCONF_RESTCONF_PORT}/restconf`
- MCP `http://${LAB_PUBLIC_HOST}:${LABNETCONF_REST_PORT}/mcp`

Parameters: SSH user alice, no TLS on RESTCONF in 1.0, profile `router-a`.

## Jungle

Filename = name `labnetconf`. File: `examples/mcpjungle/labnetconf.json`.

```json
{
  "name": "labnetconf",
  "transport": "streamable_http",
  "url": "http://labnetconf:8088/mcp",
  "bearer_token": "${LABNETCONF_TOKEN}"
}
```

Append `labnetconf` to the integration tool group.

## Smoke

Executed by the integrator after the pin, documented here:

1. NETCONF hello + get-config running hostname via test client on :10830
2. unauth GET /v1/state → 401
3. RESTCONF GET `/restconf/data/ietf-system:system/hostname` as alice
4. edit-config candidate + commit → RESTCONF GET sees new hostname
5. `netconf_notifications_wait` after that commit

Do not put framing or datastore logic in `internal/lab`.
