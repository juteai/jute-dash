# Local Voice Development

Jute has two local voice paths:

- smoke test: deterministic STT text for checking wake/audio/A2A/TTS routing;
- real local voice: openWakeWord, go-whisper, and Piper invoked as hub-owned command providers.

## Install Local Voice Tools

From the repo root:

```sh
cd examples/config/local
make install-local-voice
source ../../../.jute/local-voice-tools/local-voice.env
```

The installer creates `.jute/local-voice-tools`, installs Python command tools in a local virtualenv, downloads a Piper voice, writes a small `openwakeword` CLI wrapper, and attempts to install `gowhisper` into the same tool directory.

It does not start a wake/STT/TTS server. The hub invokes each tool as a command provider for one request.

## Run Modes

Deterministic pipeline smoke:

```sh
make run-kronk-voice-smoke
```

Real local STT only:

```sh
source ../../../.jute/local-voice-tools/local-voice.env
make run-kronk-whisper
```

Real local wake, STT, and TTS:

```sh
source ../../../.jute/local-voice-tools/local-voice.env
make run-kronk-local-voice
```

By default, `run-kronk-local-voice` uses openWakeWord's built-in `hey jarvis` model because this repo does not yet ship a trained "Hey Jute" wake model. Say "hey jarvis" for local real-wake testing.

To use a trained Hey Jute model:

```sh
JUTE_OPENWAKEWORD_MODEL=/absolute/path/to/hey-jute.onnx \
JUTE_WAKE_MODEL=openwakeword-hey-jute \
JUTE_WAKE_PHRASE="Hey Jute" \
make run-kronk-local-voice
```

## Overrides

Use these when tools are already installed elsewhere:

```sh
JUTE_OPENWAKEWORD_BIN=/absolute/path/to/openwakeword
JUTE_OPENWAKEWORD_MODEL=/absolute/path/to/hey-jute.onnx
JUTE_GO_WHISPER_BIN=/absolute/path/to/gowhisper
JUTE_PIPER_BIN=/absolute/path/to/piper
JUTE_PIPER_MODEL=/absolute/path/to/voice.onnx
```

Use these to choose model IDs passed into provider config:

```sh
JUTE_WAKE_MODEL=hey_jarvis
JUTE_WAKE_PHRASE="hey jarvis"
JUTE_WHISPER_MODEL=tiny.en
```

## Notes

- `make run-kronk` does not fake natural STT.
- `make run-kronk-voice-smoke` is still the fastest routing check.
- `make run-kronk-local-voice` uses `.jute/local-voice-dev`, so old SQLite voice settings from `.jute/local-dev` cannot hide provider changes.
- If provider selections look stale, run `make clean` in `examples/config/local`.

References:

- [openWakeWord](https://github.com/dscripka/openWakeWord)
- [go-whisper](https://github.com/mutablelogic/go-whisper)
- [Piper](https://github.com/OHF-Voice/piper1-gpl)
