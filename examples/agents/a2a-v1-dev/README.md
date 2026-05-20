# A2A 1.0 Dev Agent

This is a tiny standard-library A2A 1.0 JSON-RPC fixture for local Jute Dash development. It is intentionally lightweight and does not run a model.

Run it directly:

```sh
go run ./examples/agents/a2a-v1-dev
```

Environment:

- `A2A_V1_DEV_LISTEN`: listen address, default `127.0.0.1:9797`.
- `JUTE_MCP_URL`: optional Jute MCP Bridge URL, for example `http://127.0.0.1:8790/mcp`.
- `JUTE_MCP_TOKEN`: optional bearer token for local-token MCP auth.
- `JUTE_MCP_TIMEOUT`: optional timeout such as `5s`.

Endpoints:

- Agent Card: `http://127.0.0.1:9797/.well-known/agent-card.json`
- JSON-RPC: `http://127.0.0.1:9797/invoke`

The Agent Card declares A2A 1.0 `supportedInterfaces` and the Jute dashboard-context extension. The response tells you whether the hub sent dashboard context metadata and, when `JUTE_MCP_URL` is set, whether the agent could read Widget Skills from Jute MCP.

For the full local loop:

```sh
make dev-a2a-mcp
```
