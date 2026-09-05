# LabNETCONF operator SPA

React + TypeScript + Vite (Node **22.14.0**). The UI talks REST only (`/v1`).

Browser auth is `POST /v1/session` (bearer only — no HTTP Basic) → HttpOnly
`labnetconf_session` + CSRF in the JSON body / `GET /v1/session` reload
recovery. Mutations send `X-LabNETCONF-CSRF`. The token is never written to
`localStorage` or `sessionStorage`.

Pages: sign-in, overview (listeners, sessions, revision, health), state,
profiles (tree + leaf edit helper), users (secret paths only), datastores
(running/candidate/startup + commit/discard), notification inbox + wait,
session list, plan/apply/reset, audit. There is no call-home or send-RPC-to-remote
control.

`web/go.mod` is a nested-module fence so parent `go test ./...` does not walk
`node_modules`. Do not import `github.com/hilather/go-lab-netconf/web` from the
parent module. `//go:embed` cannot leave a module, so `make web-build` copies
`web/dist` into `internal/web/dist`. The committed fallback is
`internal/web/stub`.

```bash
npm --prefix web test
npm --prefix web run typecheck
npm --prefix web run build
```

Dev server proxies `/v1` and `/mcp` to `http://127.0.0.1:8088`.
