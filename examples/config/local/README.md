# Local Configuration & Dev Stacks

This directory provides the unified local configuration and a helper `Makefile` to run the Jute Dash stack (Hub + Svelte touch dashboard) either standalone or together with any of the example A2A agents.

## Quick Start

All commands should be run from this directory:

```sh
cd examples/config/local
```

### Standalone Jute Dash
To run the Hub and the web dashboard without starting any agent:

```sh
make run
```

The local stack serves the dashboard over HTTPS by default at:

```text
https://localhost:5173
```

The browser may ask you to accept the local self-signed certificate the first time. This default supports local OAuth flows such as Spotify without extra setup. For non-OAuth UI testing over plain HTTP, run:

```sh
make run-http
```

### Running with a Specific Agent
Each target starts the local Jute stack and launches the respective agent module from `examples/agents/` in parallel:

* **Mock Agent**: Deterministic mock assistant (no models/keys needed)
  ```sh
  make run-mock
  ```

* **Kronk Agent**: Local LLM assistant using `llama.cpp`
  ```sh
  make run-kronk
  ```

* **Ollama Agent**: Local LLM assistant using `Ollama`
  ```sh
  make run-ollama
  ```

* **Gemini Agent**: Cloud LLM assistant using `Google Gemini` (requires `GEMINI_API_KEY`)
  ```sh
  make run-gemini
  ```

Press `Ctrl-C` to stop all running processes.

## Cleanup

To clear local SQLite databases and development store directories:

```sh
make clean
```
