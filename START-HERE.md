# Start here

LabNETCONF answers NETCONF and RESTCONF from a compiled device
profile bound to the authenticated user.

Implement from `tasks/00-program-board.md`.

```
go build -o bin/labnetconf ./cmd/labnetconf
./bin/labnetconf version
./bin/labnetconf validate --config testdata/config/valid/full.yaml
./bin/labnetconf canonicalize --config testdata/config/valid/full.yaml
./bin/labnetconf serve --config testdata/config/valid/full.yaml \
  --netconf-listen=:1830 --restconf-listen=:8303 --management-listen=:8088
```

`--management-listen` defaults off. Appliance smoke image:
`examples/compose.smoke.yaml` (`cap_drop: ALL`, testdata lab host key).
`GET /` is 404 until `spec.ui.enabled: true`. Local Vite: Node 22.14.0,
`make web-install`, `npm --prefix web run dev`.
