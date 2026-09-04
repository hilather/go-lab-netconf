# Tasks

See [00-program-board.md](00-program-board.md). Take one wave whose
dependencies are complete. Template: [agent-task-template.md](agent-task-template.md).

## Canonical vs stale wave files

MANIFEST lists 17 waves. Extra files in this directory split TREE/DS
and shift later numbers by one. Implement only the canonical
(MANIFEST) file. Harvest extra test names; ignore their DAG and
package splits. Work-package IDs are the program board’s
(`WIRE-001`, `TREE-001`). `FRAME-001` ≡ `WIRE-001`. `DS-001` is
folded into TREE-001.

| Work package | Canonical wave file (MANIFEST) | Stale extra file(s) — ignore DAG |
|---|---|---|
| FND-001 | `wave-01-repository-foundation.md` | — |
| CFG-001 | `wave-02-domain-and-configuration.md` | — |
| WIRE-001 | `wave-03-framing-and-rpc.md` (file title says FRAME-001) | — |
| TREE-001 | `wave-04-yangtree-and-datastores.md` | `wave-04-yang-tree.md` (TREE-001 yangtree-only) + `wave-05-datastores.md` (DS-001) |
| SSH-001 | `wave-05-ssh-netconf.md` | `wave-06-ssh-netconf.md` |
| RC-001 | `wave-06-restconf.md` | `wave-07-restconf.md` |
| NOTIF-001 | `wave-07-notifications.md` | `wave-08-notifications.md` |
| APP-001 | `wave-08-app-snapshot.md` | `wave-09-app-snapshot.md` |
| API-001 | `wave-09-rest-api.md` | `wave-10-rest-api.md` |
| SEC-001 | `wave-10-auth-security.md` | `wave-11-auth-security.md` |
| MCP-001 | `wave-11-mcp.md` | `wave-12-mcp.md` |
| OBS-001 | `wave-12-observability.md` | `wave-13-observability.md` |
| DEP-001 | `wave-13-cli-and-container.md` | `wave-14-cli-and-container.md` |
| UI-001 | `wave-14-web-ui.md` | `wave-15-web-ui.md` |
| SWAP-001 | `wave-15-integration-lab-swap.md` | `wave-16-integration-lab-swap.md` |
| GA-001 | `wave-16-ga-hardening.md` | `wave-17-ga-hardening.md` |
| TLS-001 (v1.1) | `wave-17-tls-and-callhome-v1.1.md` | `wave-18-tls-callhome-v1.1.md` |
