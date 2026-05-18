# Voice Provider Packs

## Goal

Jute supports selectable speech-to-text and text-to-speech providers through **Voice Provider Packs**. This gives users a local-first default path while allowing open source contributors to add new engines, cloud adapters, and hardware-specific integrations without changing hub conversation logic.

Voice Provider Packs are not widgets, A2A agents, or in-process Go plugins:

- widgets render dashboard UI and use the Widget SDK;
- A2A agents reason over user turns and dashboard context;
- voice providers convert audio to text or text to audio;
- the Go hub remains the conversation authority.

## Decision

Do not use Go's `plugin` package for v1. It is too tied to toolchain, operating system, architecture, and build constraints for Jute's multi-platform goals.

Instead, voice providers are discovered by manifest and connected through isolated process or network boundaries. The Jute Voice Service calls the selected provider, and the hub stores provider choices and conversation state.

Supported provider kinds:

- `stt`: speech-to-text;
- `tts`: text-to-speech;
- `stt-tts`: both STT and TTS.

Supported transports:

- `wyoming`: preferred local/LAN protocol for open voice services.
- `http-json`: preferred transport for sidecars and cloud adapters.
- `command`: trusted local wrapper for installed tools and model CLIs; disabled unless explicitly enabled.
- `builtin`: adapters shipped with Jute, implemented through the same contract.

## Ecosystem References

- [Wyoming Protocol](https://www.home-assistant.io/integrations/wyoming): local protocol boundary used by Home Assistant for speech-to-text, text-to-speech, and wake-word systems.
- [sherpa-onnx](https://k2-fsa.github.io/sherpa/onnx/): strong local/offline candidate because it exposes ASR, VAD, TTS, multiple language bindings, and Raspberry Pi-oriented examples.
- [OpenAI speech-to-text](https://developers.openai.com/api/docs/guides/speech-to-text): optional cloud STT provider for higher quality transcription.
- [OpenAI text-to-speech](https://developers.openai.com/api/docs/guides/text-to-speech): optional cloud TTS provider with streaming and multiple output formats.
- [OHF Piper](https://github.com/OHF-Voice/piper1-gpl): fast local neural TTS engine. It should be integrated as an external service or command provider unless Jute makes an explicit licensing decision.

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
    "type": "wyoming",
    "endpoint": "tcp://127.0.0.1:10300"
  },
  "capabilities": {
    "streaming": false,
    "partialTranscripts": false,
    "offline": true,
    "languages": ["en", "en-GB", "en-US"],
    "inputFormats": ["audio/wav;rate=16000", "audio/pcm;rate=16000"],
    "outputFormats": ["application/json"]
  },
  "hardware": {
    "cpu": true,
    "gpu": false,
    "coreml": false,
    "cuda": false,
    "raspberryPi": true
  },
  "credentials": [],
  "license": {
    "name": "MIT",
    "url": "https://example.org/license"
  },
  "contribution": {
    "source": "https://example.org/provider",
    "maintainers": ["example-org"]
  }
}
```

Required fields are `id`, `name`, `version`, `kind`, `transport`, `capabilities`, `hardware`, `credentials`, `license`, and `contribution`.

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

## Transport Rules

### Wyoming

Use `wyoming` for local services and LAN voice pipelines. It is the preferred integration path for Home Assistant-compatible voice tooling and Raspberry Pi-style deployments.

Rules:

- endpoint must be loopback or LAN-scoped;
- provider health checks must fail closed when the endpoint is unreachable;
- remote internet Wyoming endpoints are not supported in v1;
- auto-discovery can be added later, but manual endpoint configuration is enough for v1.

### HTTP JSON

Use `http-json` for sidecars, cloud adapters, and provider services that need ordinary HTTP request/response semantics.

Rules:

- cloud providers must declare `offline: false`;
- credentials are read by the hub or voice service from secret references;
- TLS is required for non-local endpoints;
- timeout, retry, and payload-size limits are controlled by Jute, not the provider pack.

### Command

Use `command` only for trusted local wrappers around installed engines.

Rules:

- disabled by default;
- must be enabled per household or device profile;
- command path must be absolute or resolved from an approved provider directory;
- no shell interpolation;
- arguments are passed as an array;
- stdin/stdout formats are declared in the manifest.

### Builtin

Use `builtin` for providers shipped with Jute. Built-ins still expose a manifest-equivalent description so the settings UI and tests do not need special cases.

## STT Contract

STT providers accept one utterance from the Jute Voice Service and return a final transcript.

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
- cache eligibility;
- error code when synthesis fails.

The display remains useful when TTS is disabled or fails. The assistant response is always rendered visually.

## Provider Selection

Provider selection is persisted per device profile in SQLite and may be bootstrapped from JSON config.

Persisted settings:

- selected STT provider;
- selected TTS provider;
- model ID and voice ID;
- locale and language hints;
- streaming preference;
- cloud provider opt-in;
- command-provider enablement;
- sensitive-output speech policy;
- provider-specific non-secret settings.

Default selection order:

1. configured local/LAN Wyoming provider;
2. configured built-in provider;
3. configured HTTP JSON sidecar;
4. configured cloud provider, only when cloud opt-in is enabled;
5. disabled provider with visual-only fallback.

## Health And Test States

Providers must expose health or test behavior so the UI can show clear setup status.

Status values:

- `available`: provider is reachable and configured.
- `misconfigured`: required settings or credential references are missing.
- `offline`: provider endpoint or command is unavailable.
- `degraded`: provider works but reports limited models, high latency, or partial capability.
- `disabled`: provider exists but is not enabled for the device profile.

Health checks must not send household transcripts, live microphone audio, or secrets. Provider test calls use synthetic audio or user-confirmed preview text.

## Future Hub APIs

Provider APIs:

- `GET /api/v1/voice/providers`: list discovered STT/TTS providers and health states.
- `GET /api/v1/voice/providers/{id}`: provider details, capabilities, and setup status.
- `POST /api/v1/voice/providers/{id}/test`: run a safe provider test.
- `PATCH /api/v1/devices/{id}/voice-settings`: update selected STT/TTS providers and voice settings.

TTS APIs:

- `GET /api/v1/tts/voices`: list voices for the selected or requested TTS provider.
- `POST /api/v1/tts/preview`: synthesize a short user-confirmed preview phrase.
- `POST /api/v1/tts/speak`: speak a conversation response or explicit UI action.
- `POST /api/v1/tts/stop`: stop current playback.

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

## Security Rules

- Treat third-party provider manifests as untrusted input.
- Never load arbitrary provider code into the hub process.
- Never place raw secrets in `jute.voice.provider.json`.
- Keep cloud STT and cloud TTS opt-in per household or device profile.
- Keep raw microphone audio local unless a selected STT provider explicitly requires upload and the user has opted in.
- Never send raw microphone audio, pre-roll buffers, or partial transcripts to A2A agents.
- Disable command providers by default.
- Log provider IDs and health transitions, not raw transcripts, raw audio, synthesized sensitive text, or credential values.

