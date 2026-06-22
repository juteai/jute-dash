# Local Voice Development

Jute has two local voice paths:

- smoke test: deterministic STT text for checking wake/audio/A2A/TTS routing;
- real local voice: openWakeWord, go-whisper, and Piper invoked as hub-owned command providers.

## Install Local Voice Tools

From the repo root:

```sh
cd examples/config/local
make setup
```

The installer creates `.jute/local-voice-tools`, installs Python command tools in a local virtualenv, downloads a Piper voice, writes a small `openwakeword` CLI wrapper, and downloads the upstream `gowhisper` CLI binary into the same tool directory.

It does not start a wake/STT/TTS server. The hub invokes each tool as a command provider for one request. The local Makefile automatically sources `.jute/local-voice-tools/local-voice.env` when it runs the hub.

## Run Modes

Deterministic pipeline smoke:

```sh
make run-kronk-voice-smoke
```

Real local wake, STT, and TTS:

```sh
make run-kronk
```

By default, the local examples use openWakeWord's built-in `hey jarvis` model because this repo does not yet ship a trained "Hey Jute" wake model. Say "hey jarvis" for local real-wake testing.

To use a trained Hey Jute model:

```sh
JUTE_OPENWAKEWORD_MODEL=/absolute/path/to/hey-jute.onnx make run-kronk
```

Then select the `openwakeword-hey-jute` wake model in Voice settings, or set `voice.wake-word-model-id` to `openwakeword-hey-jute` in `examples/config/local/config.yaml` before the local database is seeded.

## Overrides

Use these when tools are already installed elsewhere:

```sh
JUTE_OPENWAKEWORD_BIN=/absolute/path/to/openwakeword
JUTE_OPENWAKEWORD_MODEL=/absolute/path/to/hey-jute.onnx
JUTE_GO_WHISPER_BIN=/absolute/path/to/gowhisper
JUTE_GOWHISPER_VERSION=v0.0.39
JUTE_PIPER_BIN=/absolute/path/to/piper
JUTE_PIPER_MODEL=/absolute/path/to/voice.onnx
```

Use Voice settings, or these YAML fields before the local database is seeded, to choose model IDs:

```yaml
voice:
    wake-word-model-id: hey_jarvis
    stt-model-id: tiny.en
    tts-model-id: piper-default
```

## Notes

- `make run-kronk` uses real local command providers after `make setup`.
- `make run-kronk-voice-smoke` is still the fastest routing check.
- If provider selections look stale, run `make clean` in `examples/config/local`.

References:

- [openWakeWord](https://github.com/dscripka/openWakeWord)
- [go-whisper](https://github.com/mutablelogic/go-whisper)
- [Piper](https://github.com/OHF-Voice/piper1-gpl)
