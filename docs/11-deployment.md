# 11 — Deployment

Scratch image UID 65532. CMD serve
`--config=/etc/labnetconf/config.yaml --management-listen=:8088`.
Healthcheck hits `/v1/health/ready`.

Appliance smoke: `--netconf-listen=:1830 --restconf-listen=:8303`,
`cap_drop: ALL`. Integrator: host 10830/18303/18830.
`NET_BIND_SERVICE` only if publishing IANA 830.

Flags:

```
--netconf-listen ADDR|off
--restconf-listen ADDR|off
--management-listen ADDR|off   # default off
```

SSH needs a host key file even on :1830. testdata ships a lab-only
key. Never commit a production key.
