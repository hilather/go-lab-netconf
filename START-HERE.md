# Start here

LabNETCONF answers NETCONF and RESTCONF from a compiled device
profile bound to the authenticated user.

Implement from `tasks/00-program-board.md`. After FND-001:

```
go build -o bin/labnetconf ./cmd/labnetconf
./bin/labnetconf version
```

`validate` / `serve` land in CFG-001 / DEP-001. After those:

```
./bin/labnetconf validate --config testdata/config/valid/full.yaml
./bin/labnetconf serve --config testdata/config/valid/full.yaml \
  --netconf-listen=:1830 --restconf-listen=:8303 --management-listen=:8088
```

`--management-listen` defaults off.
