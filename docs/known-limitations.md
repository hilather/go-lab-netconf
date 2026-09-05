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
- Notification remoteAddr best-effort under Docker userland-proxy
  (NAT collision). Split-horizon identity is still SSH/RESTCONF user
  → profile, not client IP.
- Single replica. Memory store only. Reset and restart wipe the
  notification log and audit ring.
- No OAuth PRM. No Prometheus client. No writable-running capability.
- Integrator vendor pin is LAST and out of band after GA.
