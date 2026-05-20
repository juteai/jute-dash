# Mock A2A Fixture

This directory is harness test infrastructure for Jute Dash. It runs a deterministic local A2A 1.0 JSON-RPC server so the dashboard, hub, A2A client path, task history, and optional MCP bridge can be tested as a full stack.

It is not a template for production agents. External agent authors should use the project guidance in `docs/developer/a2a-agent-guidelines.md` and `docs/developer/mcp-agent-guidelines.md`.

Direct fixture commands are available for debugging only:

```sh
make check
make server
make server-mcp
```

Normal development should run the parent harness with `make dev`.
