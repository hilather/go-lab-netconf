# 07 — MCP API

Protocol `2026-07-28`. Streamable HTTP `POST /mcp`. Stateless.
SDK `github.com/modelcontextprotocol/go-sdk` v1.7.0. Bearer only.
`labnetconf mcp-stdio --config FILE --token-file FILE` keeps the
startup bearer. On each reset and apply, mcp-stdio re-authenticates
the startup secret (read once from `--token-file` at start) against
the live verifier and drops the pin when it no longer matches. It
does not re-read the file.

Tools and resources: `docs/05-control-plane-and-parity.md`.
MCP must not HTTP-call REST. `allowLegacyClients` default false;
lab overlay sets true.

RESTCONF is not an MCP transport.
