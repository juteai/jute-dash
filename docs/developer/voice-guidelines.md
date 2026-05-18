# Voice Developer Guidelines

## Architecture Contract

Voice code must preserve the Jute split:

- the Jute Voice Service handles microphone capture, VAD, wake-word detection, buffering, STT, and optional TTS;
- the Go hub owns conversations, agent selection, follow-up windows, A2A task routing, persistence, and event emission;
- the Svelte display renders conversation state from hub APIs and events.

Do not call A2A agents directly from voice providers or the display.

## Provider Interfaces

STT and TTS integrations are Voice Provider Packs. Do not add new providers as Go `plugin` binaries or by giving provider code direct access to hub internals.

Provider transports:

- `wyoming` for preferred local/LAN voice services;
- `http-json` for sidecars and cloud adapters;
- `command` for trusted local wrappers, disabled unless explicitly enabled;
- `builtin` for adapters shipped with Jute.

Each provider has a `jute.voice.provider.json` manifest. The manifest declares provider ID, name, version, kind, transport, supported locales, streaming support, offline/network behavior, audio formats, hardware hints, license, and contribution metadata.

Manifests may declare credential requirements, but they must only contain secret references. Never put raw API keys, tokens, passwords, or OAuth refresh tokens in a provider manifest.

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

Provider packs are different from Widget Packs and A2A agents. A voice provider never renders UI widgets, never receives dashboard context, and never routes turns to A2A agents.

## Wake Word Rules

- Wake word detection runs locally before cloud STT.
- Default engine is openWakeWord.
- Keep wake-word thresholds configurable per device profile.
- Failed wake checks should stay silent unless debug mode is enabled.
- Use local VAD and pre-roll buffering so the first words after the wake word are not clipped.

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

## Privacy Rules

- Do not send raw microphone audio to A2A agents.
- Do not send pre-roll buffers to A2A agents.
- Do not send partial transcripts to A2A agents unless a future explicit policy allows it.
- Do not log raw audio.
- Do not log raw transcripts by default.
- Cloud STT and cloud TTS must be opt-in.
- TTS logs must exclude raw synthesized text by default.
- Voice provider manifests must not contain raw secrets.
- Command providers must be disabled unless explicitly enabled by the household or device profile.
- Redact precise presence and private widget data from conversation context.

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
