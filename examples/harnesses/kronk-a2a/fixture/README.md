# Kronk A2A Fixture

This directory is harness test infrastructure for Jute Dash. It runs a local Kronk-backed ADK agent behind an A2A 1.0 server so the dashboard can be tested against a model-backed A2A service.

It is not a template for production agents. External agent authors should use the project guidance in `docs/developer/a2a-agent-guidelines.md` and `docs/developer/mcp-agent-guidelines.md`.

Direct fixture commands are available for debugging only:

```sh
make check
make server
make server-mcp
make console
```

Normal development should run the parent harness with `make dev` or `make dev-mcp`. First startup may download llama.cpp assets and a model.
