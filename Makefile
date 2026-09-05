# LabNETCONF task runner. Tool versions are pinned; do not use @latest.

GO ?= go
export GOTOOLCHAIN ?= local
export GOPROXY ?= https://proxy.golang.org,direct

GOLANGCI_LINT_VERSION ?= v2.12.2
GOVULNCHECK_MOD ?= golang.org/x/vuln/cmd/govulncheck@v1.1.4
GOLANGCI_LINT_MOD ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: help fmt format lint vet build generate verify-generated test test-race \
	test-fuzz-smoke test-parity test-config-compat test-docs test-container \
	security-scan test-changelog web-install web-test web-build web-embed

help:
	@printf '%s\n' \
		'LabNETCONF Make targets (Go 1.26; module github.com/hilather/go-lab-netconf)' \
		'  format              go fmt ./...' \
		'  fmt                 alias for format' \
		'  vet                 go vet ./...' \
		'  lint                go vet + golangci-lint $(GOLANGCI_LINT_VERSION)' \
		'  build               go build -o bin/labnetconf ./cmd/labnetconf' \
		'  generate            write api/capabilities, openapi, and errors JSON' \
		'  verify-generated    fail if generate would change those files' \
		'  test                go test ./...' \
		'  test-race           go test -race ./...' \
		'  test-fuzz-smoke     buildinfo + ncframing fuzz corpora' \
		'  test-docs           required documents, metadata, links, and required phrases' \
		'  security-scan       govulncheck' \
		'  test-parity         REST/MCP capability parity goldens (MCP-001)' \
		'  test-config-compat  positive+negative v1alpha1 config fixtures' \
		'  web-install         npm ci in web/ (UI-001)' \
		'  web-test            Vitest operator SPA tests (UI-001)' \
		'  web-build           production Vite build + copy into internal/web/dist (UI-001)' \
		'  web-embed           copy web/dist into internal/web/dist (UI-001)' \
		'  test-container      build scratch image and smoke :1830/:8303 (DEP-001)' \
		'  test-changelog      observable paths require a CHANGELOG.md entry'

fmt: format

format:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint: vet
	$(GO) run $(GOLANGCI_LINT_MOD) run ./...

build:
	$(GO) build -o bin/labnetconf ./cmd/labnetconf

generate:
	$(GO) run ./scripts/generate

verify-generated:
	$(GO) run ./scripts/generate -check

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-fuzz-smoke:
	$(GO) test ./internal/buildinfo -fuzz=FuzzInfoString -fuzztime=5s -count=1
	$(GO) test ./internal/ncframing -fuzz=FuzzRead10 -fuzztime=5s -count=1
	$(GO) test ./internal/ncframing -fuzz=FuzzRead11 -fuzztime=5s -count=1

test-docs:
	$(GO) run ./scripts/checkdocs

security-scan:
	$(GO) run $(GOVULNCHECK_MOD) ./...

test-parity:
	@echo 'test-parity: not implemented until MCP-001' >&2
	@exit 1

test-config-compat:
	$(GO) test ./internal/config -run TestConfigCompat -count=1

web-install:
	@echo 'web-install: not implemented until UI-001' >&2
	@exit 1

web-test:
	@echo 'web-test: not implemented until UI-001' >&2
	@exit 1

web-build:
	@echo 'web-build: not implemented until UI-001' >&2
	@exit 1

web-embed:
	@echo 'web-embed: not implemented until UI-001' >&2
	@exit 1

test-container:
	bash scripts/test-container.sh

test-changelog:
	$(GO) run ./scripts/checkchangelog
