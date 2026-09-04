# Family evaluation — mcp-integration-lab and LabNETCONF

**Last reviewed:** 2026-09-04  
**Target:** https://github.com/hilather/go-lab-netconf (empty)

`mcp-integration-lab` publishes DNS, LDAP, TACACS+/RADIUS, SMTP, HTTP intercept, NTP, SSO, and (designed) syslog + SNMP. It has **no** NETCONF or RESTCONF appliance. LabNETCONF is an **add**.

## Members and what we copy

| Project | What LabNETCONF copies | What it leaves |
|---|---|---|
| LabNTP | First-party protocol responder, two planes, `spec.auth`, NET_BIND_SERVICE, residual high host port | Clock math |
| LabMail / LabSyslog | Receive-only notification store, reserved-key reject, wait API | SMTP/syslog codecs |
| LabSNMP | Split-horizon named maps (here: device profiles per user), no full schema compiler | SNMP PDUs |
| LabLDAP | Multi-user file-ref credentials | 389ds |
| LabSSO | Dest-vs-escape (RESTCONF dest 443 is *not* taken; residual 18303) | OIDC |
| LabDNS | Task pack, KnownFields, ADRs | Chaos |
| MCPJungle / labinfo / labgraph | Registration JSON, connection block, later fixtures | Integrator-owned |
| go-lab-syslog / go-lab-snmp | Pack format (also empty today) | Those protocols |

## Why this product exists

Controllers under test (`ncclient`, Ansible `netconf_config`, Cisco NSO as *client*, RESTCONF browsers) need a device that:

- Speaks NETCONF 1.1 over SSH subsystem `netconf`
- Speaks RESTCONF JSON on a dedicated port
- Presents a *different* tree to user `ops` vs user `vendor` (split-horizon)
- Supports candidate + commit
- Lets an agent wait for a config-change notification

Not sysrepo. Not netopeer2.

## Ports (lock)

| Plane | IANA | Container | Host residual | Local escape |
|---|---|---|---|---|
| NETCONF SSH | 830 | `:830` | 10830 | `:1830` |
| RESTCONF | 443 | `:8303` | 18303 | `:8303` |
| Management | n/a | `:8088` | 18830 | `:8088` |

Do not steal LabSSO 443/18443. Do not put RESTCONF on management `:8088`.
