# Voice Developer Guidelines

## Architecture Contract

Voice code must preserve the Jute split:

- the Jute Voice Service handles microphone capture, VAD, wake-word detection, buffering, STT, and optional TTS;
- the Go hub owns conversations, agent selection, follow-up windows, A2A task routing, persistence, and event emission;
- the Svelte display renders conversation state from hub APIs and events.

Do not call A2A agents directly from voice providers or the display.

Canonical runtime boundary:

- build production wake-word, VAD, STT, TTS, follow-up, and mute/cancel behavior in the Go hub voice runtime and hub APIs;
- keep the display as a hub client; browser microphone capture may send PCM to the hub, but browser wake decisions, browser STT, and browser TTS are not v1 runtime paths;
- do not rely on browser `SpeechRecognition` for local-first STT.

## Provider Interfaces

Wake-word, STT, and TTS integrations are Voice Provider Packs. Do not add new providers as Go `plugin` binaries or by giving provider code direct access to hub internals.

Provider transports:

- `command` for trusted local wrappers, disabled unless explicitly enabled.

STT `command` providers receive a temporary WAV path through `{inputPath}` and never run unless command providers are enabled.

Each provider has a `jute.voice.provider.json` manifest. The manifest declares provider ID, name, version, kind, transport, supported locales, streaming support, offline/network behavior, and audio formats.

Manifests may declare credential requirements, but they must only contain secret references. Never put raw API keys, tokens, passwords, or OAuth refresh tokens in a provider manifest.

Supported provider kinds are `wake-word`, `stt`, and `tts`. Wake-word providers use the same provider-pack envelope as STT/TTS providers and add a `wakeWord` section with declared model IDs, provider-pack-relative model paths, phrases, languages, and default sensitivity.

Manifest validation rejects:

- undeclared default wake models;
- wake model paths that are absolute, remote, or escape the provider pack;
- credential declarations with missing IDs, missing labels, unsupported sources, missing env references, duplicate IDs, or raw credential-looking values.

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

TTS providers should accept text and return:

- playable audio or a local playback request;
- provider/model metadata;
- duration when available;
- error state when synthesis fails.

Fallback rules:

- Do not add `htgo-tts` as a built-in or canonical TTS provider.
- `htgo-tts` may only be reconsidered as a trusted command/external sidecar path after command providers are explicitly enabled and manifest health/privacy rules are satisfied.
- Browser `speechSynthesis` is out of scope for v1.
- Fallbacks must emit the same recoverable TTS event states and keep the visual assistant response canonical.

Provider status values should be:

- `available`;
- `misconfigured`;
- `offline`;
- `degraded`;
- `disabled`.

Provider tests must use synthetic audio or explicit test text. Do not use recent household transcripts, live microphone audio, or secrets for health checks.

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
- Default path is a hub-owned command wake provider.
- Command wake providers receive a temporary WAV path through `{inputPath}` and return detection JSON.
- Map detection to `voice.wake_detected`, then `wake_detected` and `capturing_utterance` state changes.
- Keep non-detections silent unless debug mode is enabled.
- Keep wake-word thresholds configurable per device profile.
- Failed wake checks should stay silent unless debug mode is enabled.
- Use local VAD and pre-roll buffering so the first words after the wake word are not clipped.

## Local Capture Rules

- Keep microphone capture behind the `AudioCapture` interface so hub-owned platform drivers can change without changing conversation logic.
- For v1, command capture is the small local driver path: configure `voice.capture-command` to stream signed 16-bit little-endian mono PCM to stdout. Keep the command local and explicitly configured.
- Browser capture may post mono signed 16-bit PCM to `/api/v1/voice/audio`; use `?wake=true` for dashboard wake candidates that must pass hub wake detection before STT.
- The browser must not run wake-word models, STT, or send final transcript text directly from local recognition.
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

## Implementation Order

1. Define hub voice status and conversation state types.
2. Add `/api/v1/events` support for voice and conversation events.
3. Add display conversation UI states using mock events.
4. Add local display-device voice service with wake-word and VAD.
5. Add Voice Provider Pack manifest validation and provider health state.
6. Add STT provider selection, starting with hub-owned command providers.
7. Route final transcripts into the A2A message/task pipeline.
8. Add optional TTS provider selection, voice listing, playback, and stop controls.
