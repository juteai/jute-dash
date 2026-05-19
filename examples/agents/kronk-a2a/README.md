# Kronk A2A Test Agent

This example runs a local Kronk-backed ADK agent, exposes it over A2A, then starts an ADK launcher that consumes the same in-process A2A server as a remote agent.

It is intentionally isolated from the root Jute Dash module. Kronk and ADK currently require a newer Go toolchain and can download large model/runtime assets during manual runs.

## Requirements

- Go 1.26 or newer.
- Network access for the first Kronk runtime/model download.
- Enough disk space for the selected GGUF model.

## Run

From the repository root:

```sh
make kronk-a2a-setup
make kronk-a2a
```

By default the A2A server binds to:

```text
http://127.0.0.1:9797
```

Useful URLs:

```text
Agent Card: http://127.0.0.1:9797/.well-known/agent-card.json
JSON-RPC:   http://127.0.0.1:9797/invoke
```

## Environment

- `KRONK_A2A_LISTEN`: A2A listen address. Defaults to `127.0.0.1:9797`.
- `KRONK_A2A_MODE`: Set to `server` to run only the local A2A server for Jute integration.
- `KRONK_MODEL_ID`: Kronk catalog model id. Defaults to `Qwen3-0.6B-Q8_0`.
- `KRONK_MODEL_URL`: Direct model URL. When set, this overrides `KRONK_MODEL_ID`.
- `JUTE_MCP_URL`: Optional Jute MCP Bridge URL, for example `http://127.0.0.1:8790/mcp`.
- `JUTE_MCP_TOKEN`: Optional local-token auth value for the Jute MCP Bridge.

## Jute Agent Config

Use this local agent as a development Agent Card target:

```yaml
id: kronk-local
name: Kronk Local
description: Local Kronk-backed A2A test agent.
enabled: true
card-url: http://127.0.0.1:9797/.well-known/agent-card.json
endpoint-url: http://127.0.0.1:9797/invoke
protocol-binding: JSONRPC
capabilities:
  - conversation
  - local-a2a
  - mcp-ready
```

The repository also includes `config/jute.dev-a2a.yaml` and a `make dev-a2a` target that wires this server into the local dashboard.

## MCP Testing

When the Jute MCP Bridge is implemented and enabled, start the agent with:

```sh
JUTE_MCP_URL=http://127.0.0.1:8790/mcp make kronk-a2a
```

If the bridge uses local-token auth:

```sh
JUTE_MCP_URL=http://127.0.0.1:8790/mcp JUTE_MCP_TOKEN="$TOKEN" make kronk-a2a
```

The agent still works without MCP. In that mode it should only use prompt text and A2A-provided context.
