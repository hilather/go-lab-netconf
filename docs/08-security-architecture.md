# 08 — Security architecture

- Management: `spec.auth` bearer file-ref ≥32 bytes. Cookie
  `labnetconf_session`. CSRF `X-LabNETCONF-CSRF`. Origins exact match.
- NETCONF data plane: SSH password and/or public key. Host key file
  ref. Treat passwords as lab secrets.
- RESTCONF data plane: HTTP Basic against the same `spec.users[]`.
  Management bearer does not unlock `/restconf` by default.
- Admission CIDRs on both data planes. Omitted defaults to loopback
  (`127.0.0.0/8`, `::1/128`). Present empty list is deny-all.
- No call-home (no amplifier, no Dial).
- Secrets never in GET state, UI, logs at info, or metrics labels.
- `sharedProfileDatastore` default true so management/SPA/MCP edits couple
  to the live data-plane store; for isolated concurrent testers set
  `sharedProfileDatastore: false` and use two users on two profiles
  (SSH username `alice` alone still collides by design — documented,
  not a NAT bug).
- Docker userland-proxy SNAT does not collapse profiles (identity
  is user). `remoteAddr` on sessions/notifications is best-effort.
  This file and `docs/02-netconf-restconf-semantics.md` must contain
  the phrases `NAT collision` and `userland-proxy`.
