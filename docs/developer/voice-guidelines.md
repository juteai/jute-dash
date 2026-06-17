# Voice Developer Guidelines

## Architecture Contract

Voice code must preserve the Jute split:

- the Jute Voice Service handles microphone capture, VAD, wake-word detection, buffering, STT, and optional TTS;
- the Go hub owns conversations, agent selection, follow-up windows, A2A task routing, persistence, and event emission;
- the Svelte display renders conversation state from hub APIs and events.

Do not call A2A agents directly from voice providers or the display.

Canonical runtime boundary:

- build production wake-word, VAD, STT, TTS, follow-up, and mute/cancel behavior in the local Jute Voice Service and hub APIs;
- use browser microphone capture only for explicit preview or degraded foreground push-to-talk flows;
- do not rely on browser `SpeechRecognition` for canonical local-first STT, because support and offline behavior vary;
- browser `speechSynthesis`, Transformers.js, sherpa-onnx WASM, Porcupine Web, or similar display-side experiments must be labeled non-canonical until measured and approved;
- browser fallback must report final transcripts to `/api/v1/voice/transcripts/final`, never directly to A2A;
- browser fallback is unavailable for headless satellites and must not require cloud features unless cloud opt-in is enabled for the household or device profile;
- browser microphone permission must be requested from an explicit foreground user action and treated
  as revocable session state. Stop browser capture on cancel, mute, navigation, tab/background
  suspension, permission loss, or hub disconnect, then re-enter through hub voice state instead of
  resuming from browser-local durable state.
- browser fallback evidence must cover the actual target browser and shell, including desktop,
  kiosk/PWA, and offline behavior, before it is exposed outside a preview or degraded mode.

## Provider Interfaces

Wake-word, STT, and TTS integrations are Voice Provider Packs. Do not add new providers as Go `plugin` binaries or by giving provider code direct access to hub internals.

Provider transports:

- `wyoming` for preferred local/LAN voice services;
- `http-json` for sidecars and cloud adapters;
- `command` for trusted local wrappers, disabled unless explicitly enabled;
- `builtin` for adapters shipped with Jute.

Each provider has a `jute.voice.provider.json` manifest. The manifest declares provider ID, name, version, kind, transport, supported locales, streaming support, offline/network behavior, audio formats, hardware hints, license, and contribution metadata.

Manifests may declare credential requirements, but they must only contain secret references. Never put raw API keys, tokens, passwords, or OAuth refresh tokens in a provider manifest.

Supported provider kinds are `wake-word`, `stt`, `tts`, and `stt-tts`. Wake-word providers use the same provider-pack envelope as STT/TTS providers and add a `wakeWord` section with declared model IDs, provider-pack-relative model paths, phrases, languages, and default sensitivity.

Manifest validation rejects:

- undeclared default wake models;
- wake model paths that are absolute, remote, or escape the provider pack;
- unsafe remote Wyoming endpoints;
- endpoint URLs containing embedded credentials or tokens;
- credential declarations with missing IDs, missing labels, unsupported sources, missing env references, duplicate IDs, or raw credential-looking values;
- missing license metadata.

Wake-word providers should report:

- provider ID;
- model ID or wake phrase;
- supported languages;
- threshold or sensitivity;
- health status;
- last activation timestamp;
- last error.

STT providers should accept one utterance and return:

- final transcript;
- optional partial transcripts;
- confidence when available;
- language;
- duration;
- provider/model metadata.

Wyoming STT adapters should:

- accept only loopback or LAN-scoped TCP endpoints;
- send `transcribe`, `audio-start`, `audio-chunk`, and `audio-stop` events;
- support both final `transcript` and streaming transcript chunk responses;
- return transcript, provider ID, model ID, language, and duration metadata to the hub;
- never log raw PCM payloads or raw provider internals;
- map unsafe endpoint configuration to `misconfigured` and unreachable providers to `offline`.

TTS providers should accept text and return:

- playable audio or a local playback request;
- provider/model metadata;
- duration when available;
- error state when synthesis fails.

Wyoming TTS adapters should:

- accept only loopback or LAN-scoped TCP endpoints;
- send `synthesize` with hub-approved text and selected voice/locale metadata;
- read `audio-start`, `audio-chunk`, and `audio-stop` responses;
- return playable audio bytes with provider, voice, locale, format, sample rate, channel, and duration metadata;
- map unsafe endpoint configuration to `misconfigured` and unreachable providers to `offline`;
- never fail the visual conversation when synthesis or playback fails.

Fallback rules:

- Do not add `htgo-tts` as a built-in or canonical TTS provider.
- `htgo-tts` may only be reconsidered as a trusted command/external sidecar path after command providers are explicitly enabled and manifest health/privacy rules are satisfied.
- Browser `speechSynthesis` is display-only preview/degraded speech. It must be triggered only after hub speech policy approves the text.
- Browser `speechSynthesis` must not be used for headless satellites, background household TTS, or sensitive output that canonical providers would be forbidden to speak.
- Fallbacks must emit the same recoverable TTS event states and keep the visual assistant response canonical.

Provider status values should be:

- `available`;
- `misconfigured`;
- `offline`;
- `degraded`;
- `disabled`.

Provider tests must use synthetic audio or user-confirmed preview text. Do not use recent household transcripts, live microphone audio, or secrets for health checks.

## Provider Contribution Rules

Contributed providers must include:

- manifest;
- README with setup and privacy behavior;
- license information;
- supported platforms and architectures;
- network behavior;
- required credential references;
- supported languages and voices;
- expected latency;
- known failure modes.

Broad, license-compatible integrations can become built-in adapters. GPL or licensing-sensitive engines should be documented as external services or command providers unless Jute later makes an explicit licensing decision.

Voice Provider Packs are distinct from widgets and A2A agents. A voice provider never renders UI widgets, never receives dashboard context, and never routes turns to A2A agents.

## Wake Word Rules

- Wake word detection runs locally before cloud STT.
- Default engine is openWakeWord via `wyoming-openwakeword` or a Wyoming-compatible wake provider.
- Wyoming wake adapters must accept only loopback or LAN-scoped endpoints and must reject embedded credentials.
- Map Wyoming `detection` to `voice.wake_detected`, then `wake_detected` and `capturing_utterance` state changes.
- Keep Wyoming `not-detected` silent unless debug mode is enabled.
- Keep wake-word thresholds configurable per device profile.
- Failed wake checks should stay silent unless debug mode is enabled.
- Use local VAD and pre-roll buffering so the first words after the wake word are not clipped.

## Local Capture Rules

- Keep microphone capture behind the `AudioCapture` interface so display devices and headless satellites can use different platform drivers without changing hub conversation logic.
- Keep VAD behind the `VoiceActivityDetector` interface and run it before STT provider calls.
- Maintain a time-windowed pre-roll buffer in memory only.
- Unit and integration tests should use synthetic PCM fixture frames, not real microphones.
- Do not log raw PCM, frame payloads, pre-roll buffers, or transcript text while testing capture.
- Cancel and mute must stop active capture and return the service to the configured resting state.
- Capture failures should emit safe `error` state and sanitized status text, not platform driver details.

## Follow-Up Rules

- Default follow-up window is 8 seconds after assistant response completion.
- Valid follow-up speech resets the 8-second window.
- Maximum continuous follow-up session is 45 seconds or 5 turns.
- Mute, cancel, timeout, and long silence return to wake listening.
- Follow-up capture must be visually obvious in the conversation UI.

## Conversation UI Rules

- Render a bottom or side conversation sheet over the dashboard.
- Keep mute and cancel controls visible while voice is active.
- Show transcript bubbles for user and assistant turns.
- Show compact task progress while the agent is working.
- Show a large listening orb or ring for active listening states.
- Do not show full transcripts in ambient mode by default.
- The UI consumes hub events and must not be the source of durable conversation truth.

## Settings UI Rules

- Save durable voice settings through hub APIs, currently `PATCH /api/v1/voice/settings`.
- Treat voice settings as device-profile durable settings, not browser-local durable state.
- Let users enable or disable voice, select wake/STT/TTS providers and voices, adjust wake sensitivity, choose locale, set follow-up timing within hub limits, and configure microphone profile when the hub exposes those fields.
- Label cloud STT/TTS providers clearly and require explicit opt-in before selection has effect.
- Show provider health, setup status, and setup hints from safe hub projections only.
- Never render raw credential values, token names, full remote URLs, stack traces, or secret references in the display.
- Keep mute, unmute, and cancel available from settings as well as the conversation surface.

## Privacy Rules

- Do not send raw microphone audio to A2A agents.
- Do not send pre-roll buffers to A2A agents.
- Do not send partial transcripts to A2A agents unless a future explicit policy allows it.
- Report final transcripts to the hub through `/api/v1/voice/transcripts/final`; do not route from the voice service or a provider pack directly to A2A.
- Do not log raw audio.
- Do not log raw transcripts by default.
- Cloud STT and cloud TTS must be opt-in.
- TTS logs must exclude raw synthesized text by default.
- Voice provider manifests must not contain raw secrets.
- Voice provider endpoints must not embed credentials in URL userinfo or query strings; use secret references instead.
- Command providers must be disabled unless explicitly enabled by the household or device profile.
- Redact precise presence and private widget data from conversation context.

## Headless Satellite Rules

- Follow [Headless Voice Satellites](../architecture/voice-satellites.md) before adding satellite runtime code.
- Treat headless satellites as voice services attached to device profiles.
- Keep satellite capture, VAD, wake-word detection, and pre-roll local to the satellite.
- Route only safe state and final transcripts to the hub.
- Require explicit pairing/authentication before accepting satellite events.
- Bind satellite-facing services to loopback by default, and require explicit LAN configuration for multi-device deployments.
- Do not let satellites call A2A agents, widgets, MCP tools, or provider credentials directly.
- Satellite implementation must preserve pairing/revocation, device-profile binding, provider
  placement, safe hub payloads, update strategy, and multi-room privacy boundaries from the
  architecture plan.
- Raspberry Pi-class documentation must describe a manual Linux service path first. Do not promise
  automatic image builds, unattended provisioning, hub-pushed code updates, or public internet
  satellite access without a new architecture decision.
- Portable satellite config must contain only non-secret settings and secret references such as
  `authSecretEnv`. Raw auth proofs, provider API keys, OAuth tokens, raw PCM, pre-roll buffers, real
  household transcripts, and provider-internal payloads must stay out of YAML/JSON config, tests, and
  docs examples.
- The fixture-driven satellite runtime should remain runnable in CI and local development without
  microphone hardware, model downloads, or LAN devices.

## Implementation Order

1. Define hub voice status and conversation state types.
2. Add `/api/v1/events` support for voice and conversation events.
3. Add display conversation UI states using mock events.
4. Add local display-device voice service with wake-word and VAD.
5. Add Voice Provider Pack manifest validation and provider health state.
6. Add STT provider selection, starting with Wyoming-compatible local/LAN providers.
7. Route final transcripts into the A2A message/task pipeline.
8. Add optional TTS provider selection, voice listing, preview, playback, and stop controls.
9. Add headless satellite support through the same hub conversation APIs.
