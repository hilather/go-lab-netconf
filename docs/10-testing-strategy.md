# 10 — Testing strategy

Unit next to the package. Session goldens from ncclient / netconf-console
and curl RESTCONF traces under `testdata/sessions`.

Interop: `_test.go` may call `ncclient` or `curl` if present (skip if
missing). Production packages must not import those.

AST fences (FND-001, `internal/testutil`): Dial in production
packages `netconfssh`, `ncserver`, `ncframing`, `ncrpc`,
`datastore`, `restconf`, `app`, `notif`; forbidden modules
(sysrepo/netopeer2/confd/openyuma/nso/freeconf/yangson); forbidden
exec basenames `netopeer2-server`, `sysrepod`, `confd`.

Config matrix under testdata/config (`valid/defaults.yaml`,
`valid/full.yaml`, `valid/split-horizon.yaml`,
`valid/both-credentials.yaml`, omitted vs empty
`allowClientCidrs`). REST contracts. MCP parity.
Fuzz ncframing. Container script on :1830 / :8303.
Locks: KnownFields unknown fields, reserved-key reject,
`tls.enabled: true` reject, `writableRunning: true` reject,
framing 1.0 and 1.1, candidate isolation, commit visible on
RESTCONF GET, discard restores, unknown user 401.
