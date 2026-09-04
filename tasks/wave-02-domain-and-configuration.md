# CFG-001 — Domain and fail-closed YAML

Status: not-started
Depends: FND-001
Owns: internal/model, internal/config, testdata/config

## Goal
labnetconf.dev/v1alpha1 loads, normalizes, rejects unknown and
reserved keys, prints revision.

## Tests
valid/defaults.yaml, valid/split-horizon.yaml, invalid callHome,
invalid netconfTls enabled, missing model ref, short token.
