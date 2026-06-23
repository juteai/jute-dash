# Local Voice Development

Jute local examples use openWakeWord, faster-whisper, and Piper as hub-owned command providers.

## Install Local Voice Tools

From the repo root:

```sh
make setup
```

This prepares dependencies and local voice tools only. Dev servers start from the example `make run-*` targets.

To install or refresh only the local example tools:

```sh
make setup-local-examples
```

The installer creates `.jute/local-voice-tools`, installs Python command tools in a local virtualenv, downloads ONNX wake assets and a Piper voice, and writes small `openwakeword` and `jute-faster-whisper` CLI wrappers into the same tool directory.
The normal `make run`, `make run-mock`, `make run-kronk`, `make run-ollama`, and `make run-gemini` targets install or verify these tools automatically. Re-check an existing install with:

```sh
make voice-check
```

`voice-check` verifies the binaries and runs a short Piper-to-STT smoke test so broken STT command paths fail before you open the dashboard.

It does not start a wake/STT/TTS server. The hub invokes each tool as a command provider for one request. The local Makefile automatically sources `.jute/local-voice-tools/local-voice.env` when it runs the hub.

## Run Modes

Real local wake, STT, and TTS:

```sh
make run-kronk
```

By default, the local examples use openWakeWord's built-in `hey jarvis` model because this repo does not yet ship a trained "Hey Jute" wake model. Say "hey jarvis" for local real-wake testing.
The local config starts with a `0.35` wake threshold so browser microphone chunks are less brittle during development. Override `JUTE_OPENWAKEWORD_THRESHOLD` when you need to tune local detection without editing the provider pack.

To use a trained Hey Jute model:

```sh
JUTE_OPENWAKEWORD_MODEL=/absolute/path/to/hey-jute.onnx make run-kronk
```

Then select the `openwakeword-hey-jute` wake model in Voice settings, or set `voice.wake-word-model-id` to `openwakeword-hey-jute` in `examples/config/local/config.yaml` and restart the local stack.

## Overrides

Use these when tools are already installed elsewhere:

```sh
JUTE_OPENWAKEWORD_BIN=/absolute/path/to/openwakeword
JUTE_OPENWAKEWORD_MODEL=/absolute/path/to/hey-jute.onnx
JUTE_OPENWAKEWORD_THRESHOLD=0.35
JUTE_FASTER_WHISPER_BIN=/absolute/path/to/jute-faster-whisper
JUTE_FASTER_WHISPER_MODEL_DIR=/absolute/path/to/whisper-model-cache
JUTE_FASTER_WHISPER_DEVICE=cpu
JUTE_FASTER_WHISPER_COMPUTE_TYPE=int8
JUTE_PIPER_BIN=/absolute/path/to/piper
JUTE_PIPER_MODEL=/absolute/path/to/voice.onnx
```

Use Voice settings, or these YAML fields before startup, to choose model IDs:

```yaml
voice:
    wake-word-model-id: hey_jarvis
    stt-model-id: tiny.en
    tts-model-id: piper-default
```

## Notes

- The local example targets use real local command providers by default.
- Browser microphone capture requires user permission. Open the dashboard, use the chat microphone button once, and grant microphone access; hands-free browser wake cannot start before the browser grants mic access.
- If provider selections look stale, restart the local stack with `-config`/the local Makefile target. Explicit bootstrap config reconciles voice provider settings into the local SQLite database on startup.

References:

- [openWakeWord](https://github.com/dscripka/openWakeWord)
- [faster-whisper](https://github.com/SYSTRAN/faster-whisper)
- [Piper](https://github.com/OHF-Voice/piper1-gpl)
