# 04 — State and configuration

Last reviewed: 2026-09-04

Desired state is one YAML document. Candidate dirty trees and the
notification log are not desired state. The process never writes
the bootstrap file.

## Document

```yaml
apiVersion: labnetconf.dev/v1alpha1
kind: LabNETCONF
metadata:
  name: lab-device
spec:
  listeners:
    netconf:
      enabled: true
      address: ":830"
      hostKeyFile: /run/secrets/labnetconf-hostkey
    restconf:
      enabled: true
      address: ":8303"
      tls:
        enabled: false
    management:
      address: ":8088"
      restPath: /v1
      mcpPath: /mcp
  auth:
    mode: bearer
    tokens:
      - id: admin
        role: administrator
        secretFile: /run/secrets/labnetconf-token
  ui:
    enabled: true
  netconf:
    versions: ["1.0", "1.1"]
    writableRunning: false
    notifications:
      enabled: true
    sharedProfileDatastore: false
  restconf:
    json: true
    xml: false
  admission:
    allowClientCidrs:
      - "10.99.42.0/24"
      - "127.0.0.0/8"
      - "::1/128"
  profiles:
    - name: router-a
      modules:
        - name: ietf-system
          revision: "2014-08-06"
          namespace: "urn:ietf:params:xml:ns:yang:ietf-system"
      schema:
        - path: "ietf-system:system/hostname"
          type: string
          access: write
      instance:
        ietf-system:
          system:
            hostname: "lab-rtr-a"
  users:
    - name: alice
      passwordFile: /run/secrets/netconf-alice
      profile: router-a
      access: read-write
```

`KnownFields(true)`. camelCase wire names. kebab aliases reject.

## Field notes

| Field | Default | Notes |
|---|---|---|
| `listeners.netconf.address` | `:830` | reset-only |
| `listeners.netconf.hostKeyFile` | required if NETCONF enabled | file ref |
| `listeners.restconf.address` | `:8303` | reset-only |
| `listeners.restconf.tls.enabled` | false | `true` → 1.0 validate error |
| `listeners.callHome.enabled` | false | `true` → 1.0 validate error |
| `listeners.netconfTls.enabled` | false | `true` → 1.0 validate error |
| `auth.mode` | `bearer` | `spec.management.auth` unknown |
| `netconf.writableRunning` | false | `true` → 1.0 validate error |
| `admission.allowClientCidrs` | loopback (`127.0.0.0/8`, `::1/128`) if omitted | present `[]` is deny-all; lab overlay sets compose subnet |
| `spec.users[]` credentials | at least one file path | `passwordFile` and/or `authorizedKeysFile` (both allowed) |

At least one user and one profile required if either data plane is
enabled. Each user `profile` must exist. User names unique.
Host key, password, authorized-keys, and token `secretFile` paths
must be non-empty at `validate`; bytes must exist to *serve*.
If a token file is present, it must be at least 32 bytes.

Unknown fields and kebab-case aliases reject (`unknown_field`).
Reserved keys `callHome*` / `manager*` / `remote*` reject except
the 1.0 schema keys `listeners.callHome` and `listeners.netconfTls`
(those reject only when `enabled: true`).

## Revision

SHA-256 of canonical YAML. Secret **paths** included, secret **bytes**
never. Candidate dirty state does not change revision.
`storeGeneration` increments on commit and on copy-config that
mutates running or startup. Candidate-only edits do not increment it.

## Live vs reset-only vs data-plane

| Live via plan/apply | Reset-only | Data-plane (not apply) |
|---|---|---|
| replaceProfiles / upsert / remove | listener addresses | edit-config / RESTCONF |
| replaceUsers / upsert / remove | host keys, auth tokens | commit / discard |
| replaceAdmission | tls/callHome/netconfTls flags | notification insert |
| replaceNetconfCaps | ui.enabled, management.address | |
| replaceObservability | | |

## Closed apply operations (1.0)

`replaceProfiles`, `upsertProfile`, `removeProfile`,
`replaceUsers`, `upsertUser`, `removeUser`,
`replaceAdmission`, `replaceNetconfCaps`, `replaceObservability`.

Apply requires `expectedRevision` + `Idempotency-Key`.

Reset: reread bootstrap, drop candidate dirty + locks + notif log,
swap snapshot.
