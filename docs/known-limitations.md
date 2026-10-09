# Known limitations (1.0)

Honest residual claims for the 1.0 GA tag. **This release does not
ship NETCONF over TLS, RESTCONF TLS, or RFC 8071 call-home.** Those
keys reject `enabled: true` at validate. TLS-001 is v1.1.

- Not a production device. Not netopeer2. Not a manager.
- No YANG 1.1 compiler. Compact schema + YAML instance only.
- No NETCONF over TLS. No call-home. No YANG-Push.
- No NMDA operational datastore.
- No confirmed-commit. No XPath filter.
- RESTCONF HTTP only. JSON only. Cleartext is a laboratory limitation.
- Client `remoteAddr` is not exposed in session rows or notifications,
  because Docker `userland-proxy` SNAT is a NAT collision. Split-horizon
  identity is still SSH/RESTCONF user → profile, not client IP.
- Single replica. Memory store only. Reset and restart wipe the
  notification log and audit ring.
- No OAuth PRM. No Prometheus client. No writable-running capability.
- SSH `authorized_keys` options (`from=`, `command=`, `restrict`, and
  the rest) are parsed but not enforced at login. Only the key material
  is checked. A change to a key's options still counts as a credential
  change and closes that user's SSH connections.
- In-protocol NETCONF `<kill-session>` lets any read-write user kill
  another session. A read-only user gets access-denied. Admin
  `POST /v1/sessions/{id}:kill` is the scoped `netconf.admin` path.
- Integrator vendor pin is LAST and out of band after GA.
