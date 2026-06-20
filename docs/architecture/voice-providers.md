# Voice Provider Packs

## Goal

Jute supports selectable wake-word, speech-to-text, and text-to-speech providers through **Voice Provider Packs**. This gives users a local-first default path while allowing open source contributors to add new engines, cloud adapters, and hardware-specific integrations without changing hub conversation logic.

Voice Provider Packs are not widgets, A2A agents, or in-process Go plugins:

- widgets render dashboard UI and use the Widget SDK;
- A2A agents reason over user turns and dashboard context;
- wake-word providers detect activation locally;
- STT/TTS providers convert audio to text or text to audio;
- the Go hub remains the conversation authority.

The canonical provider runtime is the hub-owned Jute Voice Service calling manifest-declared
provider packs. Browser APIs are not provider packs for v1.

## Decision

Do not use Go's `plugin` package for v1. It is too tied to toolchain, operating system, architecture, and build constraints for Jute's multi-platform goals.

Instead, voice providers are discovered by manifest and called by the hub runtime. The v1 transport
is an explicitly enabled local command owned by the hub.

Supported provider kinds:

- `wake-word`: wake-word detection only;
- `stt`: speech-to-text;
- `tts`: text-to-speech.

Supported transport:

- `command`: trusted local wrapper for installed tools and model CLIs; disabled unless explicitly enabled.

## Ecosystem References

- [sherpa-onnx](https://k2-fsa.github.io/sherpa/onnx/): strong local/offline candidate because it exposes ASR, VAD, TTS, multiple language bindings, and Raspberry Pi-oriented examples.
- [OpenAI speech-to-text](https://developers.openai.com/api/docs/guides/speech-to-text): optional cloud STT provider for higher quality transcription.
- [OpenAI text-to-speech](https://developers.openai.com/api/docs/guides/text-to-speech): optional cloud TTS provider with streaming and multiple output formats.
- [OHF Piper](https://github.com/OHF-Voice/piper1-gpl): fast local neural TTS engine. It should be integrated as an external service or command provider unless Jute makes an explicit licensing decision.
- [go-whisper](https://github.com/mutablelogic/go-whisper): local Whisper STT service. Treat it as a documented external `command` provider for trusted installs in v1; keep command providers opt-in and do not link whisper.cpp or FFmpeg into the hub.
- Browser APIs: display-local fallback candidate only. They are not canonical provider-pack transports for v1.
- [pmdroid/microwakeword](https://github.com/pmdroid/microwakeword): experimental Go wake-word wrapper over TensorFlow Lite and the audio microfrontend. Defer production adoption until native packaging and model compatibility are proven.

## Pack Layout

A provider pack is a directory, archive, URL, or built-in registration with this minimum shape:

```text
my-voice-provider/
  jute.voice.provider.json
  README.md
  assets/
```

The manifest is the stable contract:

```json
{
  "id": "org.example.whisper-local",
  "name": "Example Whisper Local",
  "version": "1.0.0",
  "kind": "stt",
  "transport": {
    "type": "command",
    "command": "/usr/local/bin/jute-stt",
    "args": ["--model", "{modelId}", "--input", "{inputPath}"]
  },
  "capabilities": {
    "streaming": false,
    "partialTranscripts": false,
    "offline": true,
    "languages": ["en", "en-GB", "en-US"],
    "inputFormats": ["audio/wav;rate=16000", "audio/pcm;rate=16000"]
  },
  "credentials": []
}
```

Required fields are `id`, `name`, `version`, `kind`, `transport`, and `capabilities`.

Wake-word providers use the same manifest envelope and add a `wakeWord` section:

```json
{
  "id": "org.example.openwakeword",
  "name": "Example openWakeWord",
  "version": "1.0.0",
  "kind": "wake-word",
  "transport": {
    "type": "command",
    "command": "/usr/local/bin/jute-wake",
    "args": ["--model", "{modelId}", "--input", "{inputPath}"]
  },
  "capabilities": {
    "offline": true,
    "languages": ["en", "en-GB"]
  },
  "credentials": [],
  "wakeWord": {
    "defaultModelId": "hey-jute",
    "phrase": "Hey Jute",
    "languages": ["en"],
    "sensitivity": 0.55,
    "models": [
      {
        "id": "hey-jute",
        "path": "assets/hey-jute.tflite",
        "phrase": "Hey Jute",
        "languages": ["en"],
        "sensitivity": 0.55
      }
    ]
  }
}
```

Wake model paths must reference files inside the provider pack. Absolute paths, parent-directory traversal, and remote model URLs are rejected by manifest validation.

Secrets never appear in manifests. Credential entries are references only:

```json
{
  "credentials": [
    {
      "id": "apiKey",
      "label": "API key",
      "source": "env",
      "env": "EXAMPLE_STT_API_KEY",
      "required": true
    }
  ]
}
```

Manifest validation rejects credential declarations that omit an ID, label, supported source, or
environment-variable reference. Credential IDs must be unique within a provider pack.

## Transport Rules

### Command

Use `command` for trusted local wrappers around installed engines.

Rules:

- disabled by default;
- must be enabled per household or device profile;
- command path must be absolute or resolved from an approved provider directory;
- no shell interpolation;
- arguments are passed as an array;
- STT-capable and wake-word command manifests must include an `{inputPath}` argument placeholder so
  the voice service can pass the captured audio file explicitly;
- STT-capable command manifests must also include a `{modelId}` argument placeholder;
- stdin/stdout formats are declared in the manifest.

## Wake-Word Contract

Wake-word providers report local activation events to the Jute Voice Service. They do not produce final transcripts and do not call A2A agents.

Minimum input:

- microphone stream controlled by the Jute Voice Service;
- selected wake model ID or phrase;
- language hint;
- sensitivity threshold;
- cancellation token.

Minimum output:

- provider ID;
- model ID;
- activation timestamp;
- confidence when available;
- health status;
- last error state when activation fails.

Wake-word thresholds are persisted per device profile. Failed wake checks stay silent unless debug mode is enabled.

## STT Contract

STT providers accept one utterance from the Jute Voice Service and return a final transcript. The provider does not call the hub or A2A agents; the voice service reports final text to `POST /api/v1/voice/transcripts/final`, and the hub owns agent selection, dashboard context redaction, follow-up policy, and A2A dispatch.

The first local STT path uses command providers. Jute writes the captured utterance to a temporary
WAV, invokes the configured command with `{inputPath}` and `{modelId}`, and reads final transcript
JSON. The adapter reports provider/model/language/duration metadata beside the transcript, keeps raw
PCM out of logs, and treats command failures as recoverable provider state.

Minimum input:

- audio content or stream reference;
- sample rate;
- channel count;
- language hint;
- device ID;
- conversation ID when available;
- cancellation token.

Minimum output:

- final transcript;
- normalized language;
- duration;
- provider ID;
- model ID;
- confidence when available;
- timing segments when available;
- error code when transcription fails.

Partial transcripts are optional. Partial transcripts are shown in the UI only and are not sent to A2A agents unless a future explicit policy allows it.

## TTS Contract

TTS providers accept assistant text and return playable audio, a stream, or a local playback request.

The first local TTS path uses command providers. Jute invokes the configured command with
hub-approved text and selected voice/locale/model metadata, then reads safe playback JSON. Provider
failure emits recoverable TTS failure behavior; the visual assistant response remains canonical.

Minimum input:

- text to speak;
- locale;
- voice ID;
- style or instruction string when supported;
- speed;
- volume target;
- output format preference;
- sensitive-output policy;
- cancellation token.

Minimum output:

- provider ID;
- model ID;
- voice ID;
- audio format;
- duration when available;
- playable audio, stream URL, or local playback request;
- error code when synthesis fails.

The display remains useful when TTS is disabled or fails. The assistant response is always rendered visually.

TTS provider manifests declare a `tts` section with `defaultVoiceId`, `defaultModelId`, and a list of voice metadata records. The hub exposes voice IDs, labels, locales, model IDs, provider health, and setup status through `GET /api/v1/tts/voices`. The endpoint uses the default display profile unless a safe `deviceProfileId` query value is supplied. It does not expose raw credential references or provider manifests through this response.

## Provider Selection

Provider selection is persisted per device profile in SQLite and may be bootstrapped from YAML or JSON config.

Persisted settings:

- selected STT provider;
- selected TTS provider;
- model ID and voice ID;
- locale and language hints;
- TTS enabled flag, speed, and volume;
- streaming preference;
- cloud provider opt-in;
- command-provider enablement;
- sensitive-output speech policy;
- provider-specific non-secret settings.

Default selection order:

1. configured command provider when command providers are explicitly enabled;
2. configured built-in provider;
3. configured cloud provider, only when cloud opt-in is enabled;
4. disabled provider with visual-only fallback.

## Health And Test States

Providers must expose health or test behavior so the UI can show clear setup status.

Status values:

- `available`: provider is reachable and configured.
- `misconfigured`: required settings or credential references are missing.
- `offline`: provider endpoint or command is unavailable.
- `degraded`: provider works but reports limited models, high latency, or partial capability.
- `disabled`: provider exists but is not enabled for the device profile.

Health checks must not send household transcripts, live microphone audio, or secrets. Provider test calls use synthetic audio or explicit test text.

## Hub APIs

Implemented foundation provider APIs:

- `GET /api/v1/voice/providers`: list discovered wake/STT/TTS providers and health states.
- `GET /api/v1/tts/voices`: list voices for the selected or requested TTS provider, scoped by
  optional `deviceProfileId`.
- `POST /api/v1/tts/speak`: apply speech policy, synthesize approved assistant text through the
  configured TTS provider when available, and emit safe speak TTS state events.
- `POST /api/v1/tts/stop`: stop current transient TTS state and emit `tts.stopped`.

The foundation TTS control APIs return safe playback metadata for synthesized provider audio. They
do not make spoken output canonical: visual assistant responses remain authoritative when synthesis,
playback, provider setup, or speech policy prevents audio.

Future provider APIs:

- `GET /api/v1/voice/providers/{id}`: provider details, capabilities, and setup status.
- `POST /api/v1/voice/providers/{id}/test`: run a safe provider test.
- `PATCH /api/v1/devices/{id}/voice-settings`: update selected STT/TTS providers and voice settings.

Provider events:

- `voice.provider_discovered`
- `voice.provider_health_changed`
- `tts.started`
- `tts.completed`
- `tts.failed`
- `tts.stopped`

## Contribution Model

Contributors can add providers in three ways:

- built-in adapters for broadly useful, license-compatible integrations;
- provider pack examples for external services;
- documentation recipes for third-party tools.

Provider submissions must document:

- license;
- supported platforms and architectures;
- privacy behavior;
- network behavior;
- required credentials;
- tested locales;
- expected latency;
- failure modes;
- whether the provider can run offline.

Provider packs should include conformance tests once the provider test harness exists. The first harness should use mocked STT/TTS endpoints so CI does not require microphones, speakers, model downloads, or paid APIs.

Bootstrap configs may include `voice-provider-packs` records to seed provider manifests into the hub store for a fresh install or local dev stack. Runtime provider state still lives in SQLite; the bootstrap file is not the live source of truth after seeding.

## Security Rules

- Treat third-party provider manifests as untrusted input; reject unknown fields and trailing JSON before validation.
- Never load arbitrary provider code into the hub process.
- Never place raw secrets in `jute.voice.provider.json`.
- Keep cloud STT and cloud TTS opt-in per household or device profile.
- Keep raw microphone audio local unless a selected STT provider explicitly requires upload and the user has opted in.
- Never send raw microphone audio, pre-roll buffers, or partial transcripts to A2A agents.
- Disable command providers by default.
- Log provider IDs and health transitions, not raw transcripts, raw audio, synthesized sensitive text, or credential values.
