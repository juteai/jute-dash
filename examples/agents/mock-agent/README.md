# Mock A2A Agent

A deterministic local A2A 1.0 fixture agent written in pure Go (using only standard libraries) for testing Jute Dash chat and dashboard context without requiring any external model or API keys.

## Running the Agent

To run the agent standalone:

```sh
make run
```

This runs the HTTP server at `http://127.0.0.1:9797`.

To run the full stack (Jute Hub, web dashboard, and this agent), see [examples/config/local/README.md](../../config/local/README.md).
