# Development Harnesses

Harnesses are complete local stacks for testing Jute Dash against included A2A fixtures. Each harness owns its config, `.jute/` data directory, and embedded `fixture/` module.

Every harness supports:

```sh
make run
```

`make run` is the default target. It installs missing dependencies, starts the hub with the Jute MCP Bridge enabled, starts the embedded fixture, waits for readiness, and then starts the Svelte display. Press `Ctrl-C` to stop all running processes.

## Mock A2A

Use the deterministic mock agent when you want fast, repeatable A2A 1.0 chat and task-history behavior without a model download:

```sh
cd examples/harnesses/mock-a2a
make run
```

## Kronk A2A

Use the Kronk harness when you want a local model-backed A2A 1.0 fixture. First startup can take a while because Kronk may download llama.cpp assets and a model.

```sh
cd examples/harnesses/kronk-a2a
make run
```

---

To clean up data directories or perform checks across all modules, run targets from the repository root:

- `make reset` to clear the local development databases (`.jute/dev-mock-a2a` and `.jute/dev-kronk-a2a`).
- `make check` to run checks and tests across the entire codebase.
