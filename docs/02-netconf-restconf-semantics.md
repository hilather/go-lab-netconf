# 02 — NETCONF and RESTCONF semantics

**Last reviewed:** 2026-09-04

## NETCONF over SSH (RFC 6241 + 6242)

- SSH server, subsystem name exactly `netconf`. Other subsystems refused.
- Framing: NETCONF 1.0 `]]>]]>` and 1.1 chunked (`\n#N\n` … `\n##\n`). Both required.
- Session starts with `<hello>` exchanging capabilities.
- Advertised in 1.0: `base:1.0`, `base:1.1`, `candidate:1.0`, `startup:1.0`, `validate:1.0`, `notification:1.0` (RFC 5277 stream `NETCONF` config-change only).
- **Not** advertised: `writable-running`, `confirmed-commit`, `xpath`, `url`, `with-defaults` beyond trim, NMDA `origin`.

### Operations

| RPC | 1.0 behavior |
|---|---|
| get | running tree, subtree filter only |
| get-config | source `running` \| `candidate` \| `startup` |
| edit-config | target `candidate` only (default-operation merge) |
| copy-config | running↔candidate↔startup |
| delete-config | `startup` only |
| lock / unlock | single global lock per datastore |
| commit | candidate → running; emit config-change; clear dirty |
| discard-changes | candidate ← running |
| validate | path + declared types only (no YANG when-must) |
| close-session | graceful |
| kill-session | admin session |
| create-subscription | stream `NETCONF`, in-process; no Dial |

Subtree filter is the only filter. XPath filter is `unknown-element` / `op-not-supported`.

## RESTCONF (RFC 8040)

Dedicated listener. **Not** the management mux.

| Path | Role |
|---|---|
| `/.well-known/host-meta` | XRD pointing at `/restconf` |
| `/restconf/data` | datastore root (running) |
| `/restconf/data/{module}:{path}` | resource |
| `/restconf/operations` | RPC list (empty except lab-defined) |
| `/restconf/yang-library-version` | yang-library date |

Methods: GET, POST, PUT, PATCH (plain JSON merge-patch), DELETE. JSON `application/yang-data+json` required. XML `application/yang-data+xml` is 1.0 if cheap; otherwise document as 1.1.

RESTCONF writes running directly **or** we freeze: RESTCONF writes running (typical device) while NETCONF writes candidate. That split confuses testers.

**Freeze:** RESTCONF PATCH/PUT/POST/DELETE apply to **running** and copy-forward into candidate so they stay equal unless a NETCONF candidate session is dirty. If candidate is locked or dirty, RESTCONF write returns 409 `candidate_dirty`. This keeps one story: commit is the NETCONF transaction; RESTCONF is immediate running edit when candidate is clean.

## Identity

SSH user (passwordFile XOR authorizedKeysFile) → profile + access `read` \| `read-write`.
RESTCONF Basic against the same `spec.users[]`.
Unknown user: SSH auth fail / RESTCONF 401.
Wrong profile isolation: user B cannot read user A's tree.

## What this is not

- Call-home, NETCONF/TLS, YANG-Push, NMDA operational, confirmed-commit, writable-running.

## NAT and userland-proxy

Identity is SSH/RESTCONF **user**, not client IP. Docker `userland-proxy` SNAT is a **NAT collision** only for `remoteAddr` display on sessions and notifications. It does not collapse profiles. Do not make source-preserving TCP a 1.0 readiness gate.
