# 12 — Operator UI

Status: Proposed normative behavior
Owners: Operator UI
Last reviewed: 2026-09-04
Related ADRs: 0004, 0005, 0007

Required for 1.0 GA; rc.1 may ship without it. The operator SPA is a
same-origin Vite + React app embedded with `go:embed` of
`internal/web/dist`. It talks REST `/v1` only. REST and MCP remain the
control plane; the SPA is an adapter.

## Screens

Exactly these routes. There is no call-home control and no “send RPC
to remote” control.

| Path | Page |
|---|---|
| `/login` | Exchange a bearer for cookie `labnetconf_session` |
| `/` | Overview: listeners, sessions, revision, health live/ready |
| `/state` | Redacted `GET /v1/state` |
| `/profiles` | Named profiles, instance tree, compact-path leaf edit helper |
| `/users` | Data-plane users; secret file contents are never shown |
| `/datastores` | running / candidate / startup + commit / discard / :set |
| `/notifications` | Inbox + `POST /v1/notifications:wait` |
| `/sessions` | NETCONF SSH and RESTCONF HTTP session list |
| `/apply` | Plan / apply / gated reset (`netconf.admin`) |
| `/audit` | In-process audit ring (`netconf.audit.read`) |

Leaf edit helper writes **candidate** via `POST /v1/datastores/{profile}/{store}:set`.
Commit publishes to running. Discard restores candidate from running.

Reset rereads bootstrap YAML, restores trees, wipes notifications, never
writes the file. Phrase `RESET`, checkbox, optional reason. Submit
requires `netconf.admin`.

## Auth

Cookie `labnetconf_session` HttpOnly SameSite=Lax Path=/. CSRF header
`X-LabNETCONF-CSRF` is held in process memory (`web/src/api/client.ts`).
Never `localStorage` / `sessionStorage` tokens. Never HTTP Basic. Vitest
`assertNoTokenStorage` locks this.

`GET`/`HEAD` never send CSRF. Mutations attach the in-memory secret.
`credentials: "same-origin"` always.

Login copy: exchange a scoped API bearer for an HttpOnly session cookie.

`POST /v1/session` (bearer) issues the cookie. `GET /v1/session` returns
the CSRF secret for reload recovery. `DELETE /v1/session` clears it.

## `spec.ui.enabled`

`false` (or UI handler unset) → `GET /` is **404 `application/problem+json`**.
REST `/v1` and MCP `/mcp` are unchanged. Omitted `ui.enabled` stays false.

There is no live Apply op for `spec.ui`. Rewrite bootstrap YAML, then Reset
or process restart. `UIEnabled` reads the live snapshot’s
`Canonical.Spec.UI.Enabled`.

`--management-listen` still defaults **off**. Image CMD binds `:8088` so
HEALTHCHECK and the SPA work in compose.

## Origins

The SPA is **same-origin**. Overlay `allowedOrigins: []` stays deny-all
(no `*`). Missing Origin is allowed. Loopback (`http://127.0.0.1:8088`,
`http://localhost:5173` Vite) is exempt.

## Embed and CI

`make web-install web-test web-build web-embed` build the Vite app and copy
`web/dist` → `internal/web/dist`. Dockerfile has **no Node stage**. The
committed `internal/web/dist` is what `go:embed`, `go test`, `docker build`,
and GHCR ship.

CI job `web` (Node **22.14.0**) asserts the **checkout** is a real Vite tree
**before** `make web-build`: `internal/web/dist/index.html` has
`<title>LabNETCONF</title>` or `#root`, the stub sentence is absent, hashed JS
exists. Then `web-install web-test web-build` proves `web/src` still
compiles. There is **no** full-tree `git diff` of `dist` (Vite is not
bit-identical across runners).

Go unit test `TestCommittedDistIsProduction` fails if `Files()` is the stub
page (`UI assets were not copied`).

`internal/control/rest` production files must not import `internal/web`.
`cmd/labnetconf` sets `rest.Config.UI` when serve lands. Tests in `rest`
may import `web`.

## Local Vite

Node **22.14.0**, npm ≥10.9.0.

```bash
make web-install
npm --prefix web run dev
```

Dev server proxies `/v1` and `/mcp` to `http://127.0.0.1:8088`. Serve
LabNETCONF with `--management-listen=:8088` and `spec.ui.enabled: true`.

## Mira

Mira reviews the operator UI after this implementation lands on the branch
that will be tagged. Mira is not a merge gate, tag gate, or GHCR gate.
Security defects (localStorage tokens, CSRF missing, Basic auth) are a tag
blocker because CI must be green.
