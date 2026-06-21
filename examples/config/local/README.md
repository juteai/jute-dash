# Local Configuration & Dev Stacks

This directory provides the unified local configuration and a helper `Makefile` to run the Jute Dash stack (Hub + Svelte touch dashboard) either standalone or together with any of the example A2A agents.

## Quick Start

All commands should be run from this directory:

```sh
cd examples/config/local
```

### Standalone Jute Dash
Install local voice tools first:

```sh
make setup
```

Then run the Hub and the web dashboard without starting any agent:

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

`make run` uses `.jute/local-dev` and seeds local voice provider packs for wake, STT, and TTS. By default, the local examples select command-backed local providers: openWakeWord wake, go-whisper STT, and Piper TTS. The Makefile automatically sources `.jute/local-voice-tools/local-voice.env` when it exists, so `make setup` is enough for normal local runs.

See [Local Voice Development](../../../docs/developer/local-voice-dev.md) for real wake/STT/TTS setup.

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

  Voice routing smoke test with deterministic dev STT:
  ```sh
  make run-kronk-voice-smoke
  ```

  To choose the transcript used by the dev STT shim:
  ```sh
  make run-kronk-voice-smoke JUTE_VOICE_SMOKE_TEXT="turn on the kitchen lights"
  ```

  Plain `make run-kronk` uses the real local voice providers after `make setup`. `make run-kronk-voice-smoke` is the deterministic no-engine routing check.

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
