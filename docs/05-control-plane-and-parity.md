# 05 — Control plane and parity

Last reviewed: 2026-09-04

REST and MCP are adapters over one `internal/app.Service`. They must
not call each other. Implementation order CFG → APP → API → SEC → MCP.

RESTCONF `/restconf` is a **data plane** and is not in this table.

## Frozen capabilities

REST_ONLY_PROTOCOL: health live/ready, session, metrics scrape, SPA.

PARITY_REQUIRED:

| REST | MCP | Scope |
|---|---|---|
| `GET /v1/version` | `netconf_version_get` | `netconf.read` |
| `GET /v1/capabilities` | `netconf_capabilities_get` | `netconf.read` |
| `GET /v1/status` | `netconf_status_get` | `netconf.read` |
| `GET /v1/schema/config` | `netconf_schema_get` | `netconf.read` |
| `GET /v1/features` | `netconf_features_list` | `netconf.read` |
| `GET /v1/state` | `netconf_state_get` | `netconf.read` |
| `POST /v1/state:validate` | `netconf_state_validate` | `netconf.admin` |
| `GET /v1/state:export` | `netconf_state_export` | `netconf.admin` |
| `POST /v1/state:reset` | `netconf_state_reset` | `netconf.admin` |
| `POST /v1/changes:plan` | `netconf_change_plan` | `netconf.admin` |
| `POST /v1/changes:apply` | `netconf_change_apply` | `netconf.admin` |
| `GET /v1/profiles` | `netconf_profiles_list` | `netconf.read` |
| `GET /v1/profiles/{name}` | `netconf_profile_get` | `netconf.read` |
| `GET /v1/users` | `netconf_users_list` | `netconf.read` |
| `GET /v1/datastores/{profile}/{store}` | `netconf_datastore_get` | `netconf.read` |
| `POST /v1/datastores/{profile}/{store}:set` | `netconf_datastore_set` | `netconf.write` |
| `POST /v1/datastores/{profile}:commit` | `netconf_datastore_commit` | `netconf.write` |
| `POST /v1/datastores/{profile}:discard` | `netconf_datastore_discard` | `netconf.write` |
| `GET /v1/sessions` | `netconf_sessions_list` | `netconf.read` |
| `POST /v1/sessions/{id}:kill` | `netconf_session_kill` | `netconf.admin` |
| `GET /v1/notifications` | `netconf_notifications_list` | `netconf.read` |
| `GET /v1/notifications/{id}` | `netconf_notification_get` | `netconf.read` |
| `POST /v1/notifications:wait` | `netconf_notifications_wait` | `netconf.read` |
| `POST /v1/notifications:clear` | `netconf_notifications_clear` | `netconf.write` |
| `GET /v1/preview/get` | `netconf_preview_get` | `netconf.read` |
| `GET /v1/audit` | `netconf_audit_query` | `netconf.audit.read` |

Resources: `labnetconf://state`, `labnetconf://profiles/{name}`,
`labnetconf://datastores/{profile}/{store}`,
`labnetconf://notifications/{id}`, `labnetconf://schema/config`.

`netconf_preview_get` evaluates a user + path against running
without opening SSH.

`netconf_users_list` redacts secret file contents.

Scopes: `netconf.read` `netconf.write` `netconf.admin` `netconf.audit.read`.
Administrator has all. Reader has `netconf.read`.

## Errors

`application/problem+json` with `code`. Frozen codes include
`validation_failed`, `unknown_field`, `reserved_key`, `immutable_field`,
`revision_mismatch`, `not_found`, `lock_denied`, `not_writable`,
`wait_timeout`, `store_wiped`, `unauthorized`, `forbidden`,
`origin_not_allowed`, `tls_unsupported`, `callhome_unsupported`,
`candidate_dirty`.
