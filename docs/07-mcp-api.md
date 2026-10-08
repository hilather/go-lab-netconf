# 07 — MCP API

Protocol `2026-07-28`. Streamable HTTP `POST /mcp`. Stateless.
SDK `github.com/modelcontextprotocol/go-sdk` v1.7.0. Bearer only.
`labnetconf mcp-stdio --config FILE --token-file FILE` keeps the
startup bearer. mcp-stdio re-checks `--token-file` on reset and apply
and fails closed when that secret no longer authenticates.

Tools and resources: `docs/05-control-plane-and-parity.md`.
MCP must not HTTP-call REST. `allowLegacyClients` default false;
lab overlay sets true.

RESTCONF is not an MCP transport.
