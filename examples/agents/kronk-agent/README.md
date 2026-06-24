# Kronk A2A Agent

A local model-backed assistant agent using `llama.cpp` (via `ardanlabs/kronk` and `craigh33/adk-go-kronk`) exposed over A2A 1.0.

## Hardware Support (macOS / Metal vs CPU)

On first run, the Makefile executes a self-test to probe whether Metal acceleration is stable on your system. If successful, it caches the configuration in `.kronk-processor.choice` to use GPU inference. Otherwise, it falls back to CPU.

If the server still fails while starting with a cached Metal choice, the run target retries on CPU and updates `.kronk-processor.choice`.

To force a re-probe:

```sh
make reset-probe
```

## Running the Agent

To run the agent standalone:

```sh
make run
```

This runs the HTTP server at `http://127.0.0.1:9797`. Note that first startup might download llama.cpp dependencies and a model (Qwen3-8B).

To run the full stack (Jute Hub, web dashboard, and this agent), see [examples/config/local/README.md](../../config/local/README.md).
