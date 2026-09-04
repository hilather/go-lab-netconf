# 09 — Observability

slog JSON to stderr. Hand-rolled OpenMetrics. No Prometheus client.

Series: `labnetconf_rpcs_total{rpc,decision}`,
`labnetconf_restconf_requests_total{method,code}`,
`labnetconf_sessions`, `labnetconf_locks`,
`labnetconf_notifications`, `labnetconf_apply_total{result}`,
`labnetconf_http_requests_total{code,route}`,
`labnetconf_build_info`.

Ready: snapshot loaded AND enabled NETCONF/RESTCONF listeners that
are enabled have bound AND (management bound or off).
`labnetconf healthcheck --url=`.
