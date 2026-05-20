# Kronk A2A Agent

This is an optional developer fixture that exposes a local Kronk-backed ADK agent over A2A.

It lives in its own Go module so the root Jute Dash hub stays lightweight and does not inherit ADK, Kronk, model, or llama.cpp runtime dependencies.

## Commands

```sh
make setup
make check
make server
make run
```

- `make server` starts only the local A2A server.
- `make run` starts the local A2A server and then runs the ADK launcher against it.

## Environment

- `KRONK_A2A_LISTEN`: A2A listen address, default `127.0.0.1:9797`.
- `KRONK_A2A_MODE`: `launcher` or `server`, default `launcher`.
- `KRONK_MODEL_ID`: Kronk catalog model ID, default `Qwen3-0.6B-Q8_0`.
- `KRONK_MODEL_URL`: optional direct model URL.
- `JUTE_MCP_URL`: optional Jute MCP Bridge URL, for example `http://127.0.0.1:8790/mcp`.
- `JUTE_MCP_TOKEN`: optional bearer token for local-token MCP auth.
- `JUTE_MCP_TIMEOUT`: optional timeout such as `5s`.

## Jute Config

When running with the default listen address, add the agent to Jute with:

```yaml
agents:
  - id: kronk-local
    name: Kronk Local
    description: Local Kronk-backed A2A assistant.
    card-url: http://127.0.0.1:9797/.well-known/agent-card.json
    endpoint-url: http://127.0.0.1:9797/invoke
    protocol-binding: JSONRPC
    enabled: true
```

## MCP

When `JUTE_MCP_URL` is set, the agent receives ADK function tools backed by Jute MCP:

- `jute_dashboard_context_get`
- `jute_skill_list`
- `jute_skill_read_context`
- `jute_skill_invoke_action`
- `jute_skill_prompt_get`

These tools are optional. If MCP is unset, the Kronk agent still runs as a normal A2A agent.
