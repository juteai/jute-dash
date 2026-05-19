# A2A 1.0 Dev Agent

This is a tiny standard-library A2A 1.0 JSON-RPC fixture for local Jute Dash development. It is intentionally lightweight and does not run a model.

Run it directly:

```sh
go run ./examples/agents/a2a-v1-dev
```

Environment:

- `A2A_V1_DEV_LISTEN`: listen address, default `127.0.0.1:9797`.

Endpoints:

- Agent Card: `http://127.0.0.1:9797/.well-known/agent-card.json`
- JSON-RPC: `http://127.0.0.1:9797/invoke`

The Agent Card declares A2A 1.0 `supportedInterfaces` and the Jute dashboard-context extension. The response tells you whether the hub sent dashboard context metadata.
