# ADR 0010 — Container :830 and NET_BIND_SERVICE

- Status: Accepted
- Date: 2026-09-04

Product YAML listens `:830`. Integrator compose adds
`CAP_NET_BIND_SERVICE`. This repo's tests bind `:1830` / `:8303`
without the cap. RESTCONF `:8303` does not need the cap.
