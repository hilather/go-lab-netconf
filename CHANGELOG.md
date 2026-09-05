# Changelog

## [Unreleased]

## [1.0.0-rc.1] - 2026-09-05

### Security

- Pin `golang.org/x/crypto` to v0.56.0 so `govulncheck` is clean on
  `internal/netconfssh` (`ssh.NewServerConn`). v0.41.0 was affected
  by GO-2026-5017, GO-2026-5014, GO-2026-5013, GO-2025-4134,
  GO-2026-6355, GO-2026-6354, and GO-2026-6303.

### Added

- Design pack for LabNETCONF 1.0 (NETCONF over SSH + RESTCONF).
- Module `github.com/hilather/go-lab-netconf` (Go 1.26), Apache-2.0
  LICENSE, `cmd/labnetconf` (`version` via `internal/buildinfo`),
  package-comment stubs for the canonical map, `internal/notif`
  Sink/Waiter/Nop, and Dial / forbidden-module AST tests in
  `internal/testutil`.
- Fail-closed Makefile (LabNTP family target names) and required CI
  jobs: format, lint, unit, race, documentation, changelog,
  security-scan (`govulncheck` v1.1.4). Unimplemented targets exit 1.
- `labnetconf.dev/v1alpha1` fail-closed YAML (`internal/model`,
  `internal/config`, `internal/domainerr`). `labnetconf validate`
  and `canonicalize` load KnownFields camelCase documents, default
  omitted `allowClientCidrs` to loopback, treat `[]` as deny-all,
  and print a SHA-256 revision over secret paths (never bytes).
  `make test-config-compat` is a required CI job.
- NETCONF 1.0 EOM (`]]>]]>`) and 1.1 chunked (`\n#N\n` … `\n##\n`)
  framing in `internal/ncframing`, plus hello/RPC XML codec in
  `internal/ncrpc`. Advertised capabilities are base:1.0, base:1.1,
  candidate, startup, validate, and notification; writable-running
  and xpath are not emitted. `message-id` is preserved. Session
  goldens live under `testdata/sessions`. `make test-fuzz-smoke` is
  a required CI job.
- Compact path tree (`internal/yangtree`) and per-profile-instance
  running/candidate/startup stores (`internal/datastore`). NETCONF
  `edit-config` targets candidate; `Commit` publishes to running
  and increments `storeGeneration`. RESTCONF writes running through
  `WriteRunningIfCandidateClean` when candidate is clean and
  unlocked, else `candidate_dirty`. Locks are per profile-instance.
- RFC 5277 stream `NETCONF` config-change ring in `internal/notif`.
  `Ring` implements Sink and Waiter (OnCommit on commit only). Wait
  returns an existing or inserted record, or `wait_timeout` /
  `store_wiped`. Reset and restart wipe the log. Ids are ULIDs
  (`github.com/oklog/ulid/v2`).
- SSH NETCONF data plane (`internal/netconfssh`, `internal/ncserver`)
  listens on TCP, authenticates `spec.users` via `passwordFile`
  and/or `authorizedKeysFile`, and accepts only subsystem `netconf`.
  `internal/nctest` is the in-repo hello + get-config client.
  Admission omitted CIDRs default to loopback; an empty list is
  deny-all. `create-subscription` succeeds and does not write
  notification messages on the session.
- RFC 8040 JSON RESTCONF listener (`internal/restconf`) on a dedicated
  HTTP port (default `:8303`), not the management mux. Basic auth
  against `spec.users`; management bearer is rejected. Writes call
  `WriteRunningIfCandidateClean` (409 `candidate_dirty` if candidate
  is dirty or locked). Omitted admission CIDRs default to loopback;
  an empty list is deny-all. JSON `application/yang-data+json` only.
- Snapshot compiler (`internal/compiler`, `internal/snapshot`) and
  plan/apply/reset (`internal/app`). Closed apply ops from docs/04
  require `expectedRevision` and `Idempotency-Key`. Reset rereads
  bootstrap, restores running/candidate/startup, drops locks, and
  wipes notifications; the process never writes the bootstrap file.
  `datastore:set` / commit / discard are Service methods, not apply
  verbs.
- Management REST `/v1` adapter (`internal/control/rest`) over the
  shared capability registry. Every PARITY_REQUIRED row plus
  unauthenticated health live/ready; `application/problem+json`
  including `candidate_dirty`. `GET /` is 404 problem+json. `/restconf`
  is not mounted on management. `make generate` / `verify-generated`
  write and check `api/capabilities/v1.json`, `api/openapi/v1.json`,
  and `api/errors/v1.json`.
- slog JSON logging and hand-rolled OpenMetrics (`internal/observability`).
  Frozen series from docs/09 scrape at `GET /v1/metrics`. Ready is
  snapshot loaded plus enabled NETCONF/RESTCONF listeners bound plus
  (management bound or off). `labnetconf healthcheck --url=` probes
  `GET /v1/health/ready`. No Prometheus client.
- Management bearer (`spec.auth` file-ref, ≥32 bytes) and SPA cookie
  `labnetconf_session` with CSRF header `X-LabNETCONF-CSRF` on cookie
  POST. Origins exact-match `allowedOrigins`. Basic is rejected on
  `/v1` (401 Bearer); RESTCONF Basic against `spec.users` is unchanged.
  Administrator has all scopes; reader has `netconf.read`. Mutations
  append to an in-process audit ring. Secret bytes stay out of GET
  state, the users list, info logs, and metric labels.
- MCP Streamable HTTP adapter (`internal/control/mcp`) over the shared
  `app.Service`. Protocol `2026-07-28`, official SDK v1.7.0 only on
  this adapter, `POST /mcp`, Bearer only, `allowLegacyClients` default
  false. Every PARITY_REQUIRED `netconf_*` tool and `labnetconf://`
  resource is registered. MCP does not HTTP-call REST.
  `labnetconf mcp-stdio --config FILE --token-file FILE` requires
  `--token-file`. `make test-parity` is a required CI job and fails
  on a missing twin.
- `labnetconf serve` binds SSH NETCONF, RESTCONF, and optional
  management (`--management-listen` defaults off). `healthcheck`
  probes `GET /v1/health/ready`. `mcp-stdio` requires `--config` and
  `--token-file`. Scratch image `ghcr.io/hilather/labnetconf` runs
  UID 65532 with HEALTHCHECK ready and CMD
  `--management-listen=:8088`. `make test-container` smokes
  `:1830`/`:8303` with `cap_drop ALL`. `examples/compose.smoke.yaml`
  uses the testdata lab host key.
- Operator SPA (`web/` + `internal/web` `go:embed`): React/TS + Vite
  (Node **22.14.0**), login via bearer (`POST /v1/session`; no Basic),
  HttpOnly `labnetconf_session` + in-memory `X-LabNETCONF-CSRF`. Pages
  from docs/12: overview (listeners, sessions, revision, health),
  state, profiles (tree + leaf edit helper), users (secret paths only),
  datastores (running/candidate/startup + commit/discard), notification
  inbox + wait, session list, plan/apply/reset, audit. REST only. No
  localStorage tokens. No call-home or send-RPC-to-remote control.
  `spec.ui.enabled: false` → `GET /` is 404 problem+json. `make
  web-install web-test web-build web-embed` and CI job `web` are
  required. Committed `internal/web/dist` so the scratch image has no
  Node stage. Mira review is requested after this first UI lands.
- Integrator BOM under `examples/` (`labnetconf.yaml`, Jungle
  `labnetconf.json`, labinfo snippet with `urls` + `connection`,
  `profile.env` 10830/18303/18830) matching `docs/13`. Vendor pin
  `v1.0.0-rc.1` dest `third_party/go-lab-netconf` is documented;
  integrator pin is LAST (out of band after GA). No `vendor.go`.
- GA-001: production `serve` injects one `notif.Ring` into
  `datastore.New`, SSH, and APP (Nop is not wired). Live SSH
  `create-subscription` plus `/v1/notifications:wait` and MCP wait
  return the commit record. Soak tests cover session/commit/wait
  races. Release notes `docs/releases/v1.0.0-rc.1.md` and tag-gate
  (`scripts/release-gate`, `.github/workflows/release.yml`). Residual
  limitations documented; TLS/call-home are not claimed. Do not merge
  without the release manager.
