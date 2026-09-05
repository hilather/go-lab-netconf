# 09 — Observability

slog JSON to stderr. Hand-rolled OpenMetrics. No Prometheus client.

Series: `labnetconf_rpcs_total{rpc,decision}`,
`labnetconf_restconf_requests_total{method,code}`,
`labnetconf_sessions`, `labnetconf_locks`,
`labnetconf_notifications`, `labnetconf_apply_total{result}`,
`labnetconf_http_requests_total{code,route}`,
`labnetconf_build_info`.

Never label with client IP, Authorization, Cookie, or secret bytes.

`GET /v1/metrics` scrapes OpenMetrics text. Catalog:
`api/metrics/v1alpha1.json`.

Ready: snapshot loaded AND enabled NETCONF/RESTCONF listeners that
are enabled have bound AND (management bound or off).
`GET /v1/health/live` is process liveness. `GET /v1/health/ready`
is that Ready probe.

`labnetconf healthcheck --url=http://127.0.0.1:8088/v1/health/ready`.
