# Kronk A2A Agent

This is an optional developer fixture that exposes a local Kronk-backed ADK agent over A2A 1.0.

It lives in its own Go module so the root Jute Dash hub stays lightweight and does not inherit ADK, Kronk, model, or llama.cpp runtime dependencies.

## Status

This fixture now serves the A2A 1.0 Agent Card and JSON-RPC method shapes that the Jute hub expects:

- Agent Card declares `supportedInterfaces` with `protocolVersion: "1.0"`.
- JSON-RPC endpoint supports `SendMessage`, `SendStreamingMessage`, `ListTasks`, and `GetTask`.
- The A2A serving layer uses `google.golang.org/adk v1.3.0`, `google.golang.org/adk/server/adka2a/v2`, and `github.com/a2aproject/a2a-go/v2`.
- The example module imports and pins A2A v2 directly; it does not depend on the old non-v2 `github.com/a2aproject/a2a-go` module.

## Commands

```sh
make setup
make check
make server
make server-mcp
make console
make reset-probe
```

- `make server` starts only the local A2A server.
- `make server-mcp` starts the local A2A server with `JUTE_MCP_URL` set.
- `make console` starts the local A2A server and a simple console loop against the same Kronk agent.
- `make reset-probe` clears the cached Metal-vs-CPU choice so the next `make server` re-probes.

On first run (and any time the cache is cleared) the agent target probes whether Metal works on the current host by spinning the model up in `KRONK_A2A_MODE=selftest`, running one short inference, and falling back to CPU if the probe aborts. The result is cached in `.kronk-processor.choice`. Set `KRONK_PROCESSOR=metal` or `KRONK_PROCESSOR=cpu` to bypass the probe entirely.

## Environment

- `KRONK_A2A_LISTEN`: A2A listen address, default `127.0.0.1:9797`.
- `KRONK_A2A_MODE`: `server`, `console`, or legacy alias `launcher`; default `server`.
- `KRONK_MODEL_ID`: Kronk catalog model ID, default `Qwen3-0.6B-Q8_0`.
- `KRONK_MODEL_URL`: optional direct model URL.
- `JUTE_MCP_URL`: optional Jute MCP Bridge URL, for example `http://127.0.0.1:8790/mcp`.
- `JUTE_MCP_TOKEN`: optional bearer token for local-token MCP auth.
- `JUTE_MCP_TIMEOUT`: optional timeout such as `5s`.

### Local stability defaults

The Makefile sets a few conservative defaults so `make server` / `make
server-mcp` are stable on Go 1.26 + macOS 26 (Tahoe) + Apple Silicon, which
otherwise hits a native `SIGABRT` inside the first `llama_tokenize` call.
Override any of them when running on a different host or once upstream
fixes ship.

- `KRONK_LIB_VERSION` (default `b9194`): pins llama.cpp to the build the
  bundled `yzma` v1.14.0 expects. Without a pin, kronk auto-upgrades the
  on-disk library on every checkout and can drift away from yzma's FFI
  signatures.
- `KRONK_PROCESSOR` (no default; autodetected): when unset, the agent
  target probes Metal first and falls back to CPU on any abort. Set
  this explicitly to `metal` or `cpu` to skip the probe.
- `KRONK_A2A_GO_LDFLAGS` (default `-linkmode=external`): rsc's
  recommended Go 1.26 workaround in
  [golang/go#77917](https://github.com/golang/go/issues/77917); forces
  the external linker so the binary picks up the host macOS SDK version
  in `LC_BUILD_VERSION` instead of the Go default of 12.0. Required for
  Metal to compile its newer shader kernels at all on this stack.

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

For the full local loops:

```sh
cd ../../harnesses/kronk-a2a
make dev
make dev-mcp
```
