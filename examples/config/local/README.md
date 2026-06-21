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

`make run` uses a repo-local data directory at `.jute/local-dev` and seeds local voice provider packs for wake, STT, and TTS. The providers are command-backed dev shims, so the Voice settings screen starts with provider choices selected without requiring a microphone model, Whisper install, or TTS engine. The dev STT shim only returns text when `JUTE_DEV_STT_TEXT` is set; otherwise it fails instead of pretending to transcribe. On macOS, the dev TTS shim uses `say` for audible local playback; elsewhere it returns metadata only.

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

  Real local STT with a go-whisper command:
  ```sh
  make run-kronk-whisper
  ```

  `run-kronk-whisper` selects the `local-whisper-stt` command provider and uses a separate `.jute/local-whisper-dev` data directory so existing local settings do not hide the config change. It expects `gowhisper` on `PATH`, or set:
  ```sh
  JUTE_GO_WHISPER_BIN=/absolute/path/to/gowhisper make run-kronk-whisper
  ```

  To choose the Whisper model ID passed to go-whisper:
  ```sh
  make run-kronk-whisper JUTE_WHISPER_MODEL=tiny.en
  ```

  Plain `make run-kronk` does not fake natural STT.

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
