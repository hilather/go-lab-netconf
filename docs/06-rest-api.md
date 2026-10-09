# 06 — REST API

Frozen routes live in `05-control-plane-and-parity.md`.
`application/problem+json`. Bearer or session+CSRF. Health is
unauthenticated. `GET /v1/metrics` requires a bearer or session with
`netconf.read`. Basic is rejected on `/v1` (401 Bearer).

`GET /v1/profiles` items are `{ "name": ... }`, the same camelCase
field as `GET /v1/profiles/{name}` and MCP `netconf_profiles_list`.

`GET /v1/datastores/{profile}/{store}` returns the instance tree
JSON. `POST …:set` is an overlay/tree write on that store (tester
helper; same engine as edit-config). `POST …:commit` / `:discard`
match the NETCONF RPCs.

`GET /v1/preview/get?user=alice&path=ietf-system:system/hostname`
does not open SSH.

`GET /v1/sessions` lists live NETCONF SSH sessions as
`{ "id", "user", "profile" }`. RESTCONF has no session table.
`POST /v1/sessions/{id}:kill` (`netconf.admin`) ends that session and
drops its locks. It does not close the SSH connection. An unknown id
is 404 `not_found`.

When `spec.ui.enabled` is true and management is bound, `GET /`
serves the operator SPA. Otherwise `GET /` is 404 problem+json.

Cookie `labnetconf_session`. CSRF `X-LabNETCONF-CSRF`.
`POST /v1/session` (bearer) issues the cookie; `GET /v1/session` returns
CSRF for reload recovery; `DELETE /v1/session` clears it. These three
bindings are REST-only (not MCP).
