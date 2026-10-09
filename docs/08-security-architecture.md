# 08 — Security architecture

- Management: `spec.auth` bearer file-ref ≥32 bytes. Cookie
  `labnetconf_session`. CSRF `X-LabNETCONF-CSRF`. Origins exact match.
- NETCONF data plane: SSH password and/or public key. Host key file
  ref. Treat passwords as lab secrets.
- RESTCONF data plane: HTTP Basic against the same `spec.users[]`.
  Management bearer does not unlock `/restconf` by default.
- Admission CIDRs on both data planes. Omitted defaults to loopback
  (`127.0.0.0/8`, `::1/128`). Present empty list is deny-all, including
  a live `replaceAdmission` with `[]`. Omitted or null still becomes
  loopback. Admission and user access on the running listeners are
  taken from the live snapshot.
- No call-home (no amplifier, no Dial).
- A password or authorized-keys change closes that user's SSH
  connections. Equality is the password bytes, with only trailing CR
  and LF removed, plus the multiset of parsed authorized-key lines
  (key material and options such as `from=`, `command=`, and
  `restrict`). Blank lines, key-line spacing, comment text, and
  key-line order do not close them. A trailing space in a password,
  or an option-value change, does close the connection. Options are
  part of the comparison and are not enforced at handshake. An apply
  that leaves a user's credentials unchanged does not close that
  user's SSH connection for credential rotation. Admission changes
  still close connections that no longer match the CIDR list. Reset
  still ends every NETCONF session, even when credentials are
  unchanged, because profile datastore handles are rebuilt (see
  `docs/05-control-plane-and-parity.md`). Admin
  `POST /v1/sessions/{id}:kill` and in-protocol `<kill-session>` end
  one NETCONF session and its locks and do not close SSH. In-protocol
  `<kill-session>` still lets any read-write NETCONF user kill another
  session; a read-only user gets access-denied.
- Secrets never in GET state, UI, logs at info, or metrics labels.
- `sharedProfileDatastore` default true so management/SPA/MCP edits couple
  to the live data-plane store; for isolated concurrent testers set
  `sharedProfileDatastore: false` and use two users on two profiles
  (SSH username `alice` alone still collides by design — documented,
  not a NAT bug).
- Docker userland-proxy SNAT does not collapse profiles (identity
  is user). It is a NAT collision for the client address the process
  sees. `GET /v1/sessions` returns `id`, `user`, and `profile` and
  does not include `remoteAddr`. Notifications do not include
  `remoteAddr` either. This file and
  `docs/02-netconf-restconf-semantics.md` must contain the phrases
  `NAT collision` and `userland-proxy`.
