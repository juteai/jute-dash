# Development Harnesses

Harnesses are complete local stacks for testing Jute Dash against included A2A fixtures. Each harness owns its config, `.jute/` data directory, and embedded `fixture/` module.

## Mock A2A

Use the deterministic mock agent when you want fast, repeatable A2A 1.0 chat and task-history behavior without a model download:

```sh
cd examples/harnesses/mock-a2a
make dev
```

Use the MCP-enabled variant when testing Widget Skills and dashboard context over the Jute MCP Bridge:

```sh
cd examples/harnesses/mock-a2a-mcp
make dev
```

## Kronk A2A

Use the Kronk harness when you want a local model-backed A2A 1.0 fixture. First startup can take a while because Kronk may download llama.cpp assets and a model.

```sh
cd examples/harnesses/kronk-a2a
make dev
make dev-mcp
```

Each harness owns its own `.jute/` data directory under the repository root and provides `make reset` to remove only that harness state.

Every harness supports:

```sh
make setup
make dev
make reset
make check
```

`make dev` installs missing dependencies, starts the hub, starts the embedded fixture, waits for readiness, starts the Svelte display, and stops its own child processes on Ctrl-C.
