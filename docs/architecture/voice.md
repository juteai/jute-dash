# Voice And Wake Word Architecture

## Goal

Jute voice should feel natural on an always-on home display without weakening the local-first architecture. The first implementation target is a dashboard or kiosk device with a microphone and visible conversation UI. The same design must also support later headless voice satellites on Raspberry Pi-style hardware.

The chosen posture is **local-first hybrid**:

- wake word detection runs locally before any cloud service is used;
- voice activity detection runs locally;
- speech-to-text and text-to-speech are provider interfaces with local and optional cloud implementations;
- the Go hub remains the conversation authority;
- A2A agents receive transcripts and redacted context, not raw microphone audio.

Canonical voice runtime is the local Jute Voice Service beside the hub. Browser-side voice in the
Svelte display is allowed only as an explicit degraded or preview mode. It may request microphone
permission for a foreground push-to-talk experiment, show browser `speechSynthesis` previews, or run
measured WebAssembly/WebGPU STT experiments such as Transformers.js or sherpa-onnx WASM, but it must
not become the durable wake-word, follow-up, provider-selection, A2A routing, or conversation
authority. Browser `SpeechRecognition` is not a canonical provider because support is uneven and some
browser implementations use server-side recognition, which can break offline and local-first
expectations.

Browser fallback policy:

- browser capture is opt-in per device session, foreground-only, and unavailable for headless
  satellites;
- browser microphone permission is transient runtime state, not durable voice enablement. The
  display must request it only from a visible user action, stop capture on cancel, mute, navigation,
  tab/background suspension, or permission revocation, and recover through hub state rather than
  assuming a remembered browser grant;
- browser wake word is unavailable in v1 except as a non-canonical spike because always-on PWA and
  kiosk microphone behavior varies by browser and operating system;
- browser STT/TTS can be used only when clearly labeled as preview or degraded mode and must report
  final transcripts back to the hub instead of calling A2A agents;
- cloud-backed browser features require the same household or device-profile cloud opt-in as hub
  providers;
- when offline, the display may use only already-installed local browser assets and must otherwise
  degrade to visual controls and hub events.

Browser runtime evidence:

- MDN currently marks `SpeechRecognition` as limited availability and notes that Chrome may use a
  server-backed recognizer, so browser STT cannot satisfy the canonical local-first/offline contract
  without per-browser proof;
- Transformers.js supports browser ASR and TTS tasks, but model download size, cold start, memory,
  WebGPU/WASM behavior, and kiosk/PWA compatibility must be measured before any display fallback is
  promoted beyond preview;
- sherpa-onnx documents a WebAssembly ASR path, but Jute still needs packaging, model delivery, and
  device-performance evidence before treating it as a supported display fallback;
- browser wake-word SDKs such as Porcupine Web are non-canonical because they introduce vendor
  access-key/licensing concerns and still do not solve headless satellite or browser background
  lifecycle constraints.

## Ecosystem References

- [openWakeWord](https://github.com/dscripka/openWakeWord): first local wake-word baseline.
- [Home Assistant wake-word architecture](https://www.home-assistant.io/voice_control/about_wake_word/): useful model for server-side wake detection and voice satellites.
- [Wyoming Protocol](https://www.home-assistant.io/integrations/wyoming): integration-compatible local voice service boundary for wake word, STT, and TTS systems.
- [sherpa-onnx](https://k2-fsa.github.io/sherpa/onnx/): local/offline speech toolkit with ASR, VAD, and TTS paths.
- [OpenAI speech-to-text](https://developers.openai.com/api/docs/guides/speech-to-text): optional cloud STT provider for higher transcription quality.
- [OpenAI text-to-speech](https://developers.openai.com/api/docs/guides/text-to-speech): optional cloud TTS provider.
- [OHF Piper](https://github.com/OHF-Voice/piper1-gpl): local TTS reference for external service or command-provider integration.

## Component Architecture

```mermaid
flowchart LR
  mic["Microphone input"]
  voice["Jute Voice Service"]
  vad["Local VAD"]
  wake["Local wake word\nopenWakeWord first"]
  buffer["Pre-roll + utterance buffer"]
  stt["STT Voice Provider Pack\nlocal or cloud"]
  tts["TTS Voice Provider Pack\noptional"]
  hub["Jute Hub\nconversation authority"]
  a2a["A2A task pipeline"]
  display["Jute Display\nconversation UI"]
  events["/api/v1/events\nSSE"]

  mic --> voice
  voice --> vad
  voice --> wake
  voice --> buffer
  buffer --> stt
  stt --> hub
  hub --> a2a
  a2a --> hub
  hub --> events
  events --> display
  hub --> tts
  tts --> voice
```

## Jute Voice Service

The Jute Voice Service is a local process or module that runs beside the Go hub on display devices. Later it can run on headless satellite devices and connect back to the hub.

Responsibilities:

- capture microphone audio;
- apply local voice activity detection;
- run local wake-word detection;
- maintain a short pre-roll buffer so the beginning of speech is not lost;
- capture one utterance at a time;
- call the selected STT provider;
- optionally call the selected TTS provider for spoken responses;
- report state, partial transcripts, and final transcripts to the hub.

The voice service does not call A2A agents directly. It sends transcripts and state to the hub, and the hub decides whether to start or continue a conversation.

The foundation service loop is driver-neutral. It accepts a pluggable PCM `AudioCapture`, runs a
pluggable local VAD over fixture or device frames, maintains a time-windowed pre-roll buffer, emits
safe state updates, and returns captured utterance frames to the next local provider step. Platform
microphone drivers are added behind the capture interface; tests use synthetic PCM fixtures so CI
does not require a microphone.

Service state and health:

- disabled devices remain `idle`;
- muted devices remain `muted` and stop active capture;
- enabled and unmuted devices enter `wake_listening`;
- local VAD speech transitions to `capturing_utterance`;
- capture failures transition to `error` with a safe service status and sanitized message;
- cancel stops active capture and returns to the configured resting state.

The service never logs raw PCM. Pre-roll and utterance frames stay in memory until a selected STT
provider receives them through an explicit local provider call.

## Hub Conversation Authority

The Go hub owns conversation identity, turn ordering, agent selection, follow-up windows, and event emission.

Hub responsibilities:

- start a conversation after wake-word activation or push-to-talk;
- continue an existing conversation during the follow-up window;
- send user turns through the same A2A message/task pipeline as typed messages;
- attach redacted dashboard context when the target agent supports it;
- emit voice, conversation, and task events over `/api/v1/events`;
- persist conversation summaries when history is enabled;
- enforce mute, cancel, timeout, and privacy policy.

## Voice State Machine

```mermaid
stateDiagram-v2
  [*] --> idle
  idle --> muted: user mutes mic
  muted --> idle: user unmutes mic
  idle --> wake_listening: voice enabled
  wake_listening --> wake_detected: wake word detected
  wake_detected --> capturing_utterance: acknowledge wake
  capturing_utterance --> transcribing: end of speech
  transcribing --> agent_thinking: final transcript
  agent_thinking --> assistant_responding: agent output begins
  assistant_responding --> followup_listening: response complete
  followup_listening --> capturing_utterance: speech detected
  followup_listening --> wake_listening: timeout or max turns
  capturing_utterance --> error: capture failure
  transcribing --> error: STT failure
  agent_thinking --> error: agent failure
  assistant_responding --> error: TTS or response failure
  error --> followup_listening: recoverable
  error --> wake_listening: unrecoverable
  wake_listening --> muted: user mutes mic
  followup_listening --> muted: user mutes mic
```

State definitions:

- `muted`: microphone is disabled by user or policy.
- `idle`: voice feature is inactive or not yet configured.
- `wake_listening`: local wake-word engine is active.
- `wake_detected`: wake word fired and acknowledgement begins.
- `capturing_utterance`: user speech is being recorded.
- `transcribing`: STT provider is producing a transcript.
- `agent_thinking`: hub has sent the turn to an A2A agent.
- `assistant_responding`: response is being displayed or spoken.
- `followup_listening`: user can continue without wake word.
- `error`: recoverable or terminal voice failure state.

## Wake And Follow-Up Flow

```mermaid
sequenceDiagram
  participant User
  participant Voice as Jute Voice Service
  participant Hub as Jute Hub
  participant Agent as A2A Agent
  participant UI as Jute Display

  User->>Voice: Says wake word and request
  Voice->>Voice: Detects wake word locally
  Voice->>Hub: voice.wake_detected
  Hub->>UI: voice.state_changed wake_detected
  Voice->>Voice: Captures utterance with pre-roll + VAD
  Voice->>Hub: voice.transcript.final
  Hub->>Hub: Starts conversation or creates turn
  Hub->>Agent: Sends A2A message/task with transcript
  Agent-->>Hub: Streams task status and response
  Hub-->>UI: conversation and task events
  Hub->>Voice: Optional TTS response
  Hub->>UI: conversation.followup_started
  User->>Voice: Follow-up without wake word
  Voice->>Hub: voice.transcript.final
  Hub->>Agent: Sends next A2A turn
```

Wake flow:

1. Device continuously listens locally for the configured wake word while unmuted.
2. On wake, Jute plays or displays an acknowledgement state.
3. Jute captures the utterance using VAD and a short pre-roll buffer.
4. STT produces a transcript.
5. The voice service sends the transcript to the hub.
6. The hub creates or continues a conversation and forwards the turn to the selected A2A agent.

Follow-up flow:

1. After an assistant response completes, the hub enters `followup_listening`.
2. Default follow-up window is 8 seconds.
3. During this window, user speech starts a new turn without the wake word.
4. Each valid follow-up resets the 8-second window.
5. Maximum continuous follow-up session is 45 seconds or 5 turns, whichever comes first.
6. Manual cancel, timeout, mute, or long silence returns to `wake_listening`.

Error flow:

- Failed wake detection stays silent unless debug mode is enabled.
- Failed STT shows "I didn't catch that" and briefly returns to follow-up listening.
- Failed agent response shows a recoverable conversation error and exits follow-up.

## Provider Strategy

STT and TTS integrations use [Voice Provider Packs](voice-providers.md). Provider packs are selectable, manifest-driven integrations that run through process or network boundaries. They are not Go in-process dynamic plugins.

Wake-word providers:

- default: openWakeWord through a loopback or LAN Wyoming service;
- later: Porcupine, microWakeWord, and Wyoming-compatible engines.

The first wake-word adapter speaks Wyoming over TCP to `wyoming-openwakeword` or a compatible mock.
It sends a `detect` request for the selected model names, treats `detection` as a wake event, and
keeps `not-detected` silent unless debug mode is enabled. A detection causes the hub/display stream
to emit `voice.wake_detected`, followed by `voice.state_changed` transitions through
`wake_detected` and `capturing_utterance`. The adapter accepts only loopback or LAN-scoped endpoints
and rejects endpoint URLs with embedded credentials.

At runtime, the hub resolves the active wake provider from SQLite by scanning installed wake-word
Provider Packs in deterministic order. It activates only local/offline Wyoming packs whose health is
`available` or `degraded`, whose required credentials are satisfied, and whose manifest declares the
selected wake model. If no device-profile model is selected, the provider pack default model is used.
Unsupported, unsafe, cloud, unhealthy, or credential-incomplete wake packs fail closed.

STT providers:

- default path: Wyoming-compatible local/LAN STT services;
- local/offline candidates: sherpa-onnx ASR and Whisper-compatible sidecars;
- optional cloud providers such as OpenAI speech-to-text for higher accuracy;
- cloud upload requires explicit household or device profile opt-in.

The first STT adapter speaks Wyoming over TCP to a local or LAN ASR provider. It sends
`transcribe`, `audio-start`, one or more `audio-chunk` events with raw PCM payloads, and
`audio-stop`, then accepts either a final `transcript` event or streaming
`transcript-start`/`transcript-chunk`/`transcript-stop` events. The adapter returns the final text
plus provider ID, model ID, language, and utterance duration to the hub-owned voice transcript
pipeline. It does not call A2A agents directly and does not log raw audio.

At runtime, the hub resolves the selected STT Provider Pack from SQLite and activates an STT
provider only when voice is enabled, the manifest is a local/offline Wyoming or `http-json` STT
pack, required credentials are satisfied, and provider health is `available` or `degraded`. Public
provider summaries continue to omit transport endpoints and credential metadata.

Captured utterances flow through a hub-owned STT turn processor: the local voice service hands off a
cloned utterance, the selected provider returns transcript metadata, and only the sanitized final
transcript is submitted through the same server-owned final transcript sink used by
`/api/v1/voice/transcripts/final`. Provider connection details and raw audio stay out of the
submitted transcript payload. The server constructs this processor from the active STT provider
store and the device profile before connecting it to local capture. The local voice service builder
remains hardware-free at startup: platform capture and VAD implementations are passed in explicitly,
then synthetic PCM tests exercise the capture-to-STT-to-final-transcript path. If STT processing
fails, the hub emits a safe degraded `voice.state_changed` error state and does not create an A2A
turn.

Wyoming STT health uses the same safe provider states as the provider list:

- invalid or unsafe endpoint: `misconfigured`;
- unreachable endpoint: `offline`;
- successful local/LAN connection: `available`;
- cloud STT providers remain `disabled` until cloud opt-in is enabled.

TTS providers:

- optional in v1;
- default path: Wyoming-compatible local/LAN TTS services;
- embedded/local candidate: sherpa-onnx TTS through a provider pack;
- Piper/OHF Piper should be external service or command-provider integration unless a future licensing decision changes this;
- optional cloud providers such as OpenAI text-to-speech require explicit opt-in;
- the visual conversation UI must remain fully useful when TTS is disabled.

The provider interfaces should support health status, model name, language, latency metrics, and last error state.

TTS-specific playback, caching, and speech policy details are specified in [Text-To-Speech Architecture](text-to-speech.md).

## Headless Satellite Constraints

Headless voice satellites are specified in [Headless Voice Satellites](voice-satellites.md). They
are not part of the v1 display-device implementation, but the v1 voice contracts reserve the shape:

- satellites run the same local capture, VAD, wake-word, and pre-roll responsibilities as the display-side Jute Voice Service;
- satellites report state and final transcripts to the hub through hub-owned voice APIs, not directly to A2A agents;
- each satellite maps to a device profile so provider choices, wake sensitivity, cloud opt-in, command-provider enablement, microphone profile, and preferred agent stay device-scoped;
- satellite connections must be local-first, paired, authenticated, and bound to loopback or explicitly configured LAN addresses by default;
- satellites must not send raw microphone audio, pre-roll buffers, or partial transcripts to A2A agents;
- hub events must continue to expose safe device and conversation identifiers without raw audio, provider credentials, private widget state, or precise presence data.

[Headless Voice Satellites](voice-satellites.md) is the planning gate for implementation. It defines:

- pairing, authentication, revocation, and trust bootstrap;
- device-profile binding and whether per-satellite settings inherit from household defaults or
  require explicit overrides;
- which provider packs run on the satellite versus hub-adjacent services;
- safe hub API/event payloads for satellite state, wake, final transcript, health, version, and
  update status;
- update and provisioning strategy for Raspberry Pi-class hardware;
- multi-room routing, room attribution, and privacy boundaries without leaking precise presence.

Full satellite provisioning, pairing, update, and multi-room routing remain out of scope for the
current display-device implementation.

## Foundation Implementation Slices

The first voice implementation creates durable voice settings, safe hub status/control APIs, display mute/listening status, event emission, selected local Wyoming STT/TTS provider resolution, and the hub-owned final-transcript-to-A2A handoff. The visual conversation remains the reliable baseline when provider resolution or synthesis/transcription fails.

Implemented v1 foundation APIs:

- `GET /api/v1/voice/status`: returns enabled/muted state, selected provider IDs, follow-up window, device profile, and safe service status.
- `GET /api/v1/voice/providers`: returns discovered provider pack summaries; this may be an empty list before provider discovery exists.
- `PATCH /api/v1/voice/settings`: updates durable device-profile voice settings through the hub, including enablement, wake/STT/TTS provider selections, wake sensitivity, locale, follow-up window, cloud opt-in, command-provider enablement, microphone profile, and sensitive-output policy. Unknown fields are rejected and raw secret values are never accepted or returned.
- `POST /api/v1/voice/mute`: marks the device voice state muted.
- `POST /api/v1/voice/unmute`: marks the device voice state unmuted.
- `POST /api/v1/voice/cancel`: clears transient voice activity when present and returns the current safe status.
- `POST /api/v1/voice/transcripts/final`: accepts final text from the local voice service, starts or continues a hub-owned voice conversation, dispatches the text through the configured A2A agent runner, emits ordered voice/conversation events, and returns the conversation detail plus follow-up deadline.

Implemented local voice-service foundation:

- pluggable PCM capture interface for display devices and future headless satellites;
- pluggable VAD interface with fixture-audio tests;
- configurable pre-roll and silence windows;
- in-memory captured utterance handoff for the selected STT provider path;
- STT turn processing from cloned captured utterance to sanitized final transcript submission;
- server-side local voice service construction from capture/VAD interfaces plus active STT provider;
- safe state transitions for `idle`, `muted`, `wake_listening`, `capturing_utterance`, and `error`;
- cancel and mute controls that stop active capture without logging raw audio.

Foundation state names are `muted`, `idle`, `wake_listening`, `capturing_utterance`, and `error`. The service status is `not_configured` until voice is enabled and an STT provider is selected. The display may show microphone controls and voice settings from this state, but browser microphone capture is still out of scope.

## API Contracts

Hub APIs:

- `GET /api/v1/voice/status`: returns current state, mute status, selected provider IDs, and follow-up settings. Future status payloads may include active conversation ID and follow-up deadline.
- `GET /api/v1/voice/providers`: returns discovered STT/TTS provider packs and health states.
- `PATCH /api/v1/voice/settings`: persists safe device-profile voice settings in SQLite. The request may include `deviceProfileId`, enablement, wake-word model or phrase, wake sensitivity, STT/TTS provider and model IDs, TTS voice/locale/speed/volume, preferred agent ID, cloud opt-in, command-provider enablement, sensitive-output policy, follow-up window seconds, and microphone profile. The hub validates bounds, rejects unknown fields, and emits `voice.state_changed` with a redacted state summary after a successful save.
- `POST /api/v1/voice/mute`: mutes microphone capture.
- `POST /api/v1/voice/unmute`: unmutes microphone capture.
- `POST /api/v1/voice/cancel`: cancels active capture, transcription, response, or follow-up session.
- `POST /api/v1/voice/transcripts/final`: hub-owned voice-service ingress for final transcripts. The request accepts `text`, optional `deviceProfileId`, optional `deviceId`, optional `conversationId`, and optional `agentId`. Unknown fields are rejected so raw audio, pre-roll buffers, and provider internals cannot slip through this API.

Future hub APIs:

- `GET /api/v1/voice/providers/{id}`: returns provider details, capabilities, and setup status.
- `POST /api/v1/voice/providers/{id}/test`: runs a safe provider health or preview test.

Typed display chat currently uses the standard A2A JavaScript SDK through `/api/v1/proxy/agents/{agentId}`. Voice remains hub-owned because the hub must enforce wake-word, privacy, follow-up-window, and routing policy before sending final transcripts to agents.

## Event Contracts

Voice and conversation events are emitted over `/api/v1/events`:

- `voice.state_changed`
- `voice.wake_detected`
- `voice.provider_discovered`
- `voice.provider_health_changed`
- `voice.transcript.partial`
- `voice.transcript.final`
- `conversation.started`
- `conversation.turn_started`
- `conversation.turn_completed`
- `conversation.followup_started`
- `conversation.ended`

Every event includes `id`, `type`, `createdAt`, `deviceId`, optional `conversationId`, and `payload`.

## Conversation UI

The display chat experience is specified in [Display UX](display-ux.md). Voice uses the same chat mode primitives for listening, thinking, streaming, error, mute, cancel, and follow-up states.

The display should use an Echo Show-style conversation flow that transitions from the dashboard into focused chat mode.

UI requirements:

- bottom sheet on wide and tablet layouts;
- side sheet on large wall displays when it leaves widgets more visible;
- large listening orb or ring for `wake_detected`, `capturing_utterance`, and `followup_listening`;
- transcript bubbles for user and assistant turns;
- compact task progress states while the agent is thinking;
- always-visible mute and cancel controls while voice is active;
- clear visual distinction between wake listening and follow-up listening;
- ambient mode shows only listening/responding status by default, not full transcripts.

The conversation UI consumes hub events. It should not infer conversation state only from local browser state.

The display settings UI also renders the safe voice settings projection served by the hub. It may let users enable or disable voice for the selected device profile, pick wake/STT/TTS providers and voices, adjust wake sensitivity and follow-up timing within hub-defined bounds, opt in to cloud providers, and use mute/cancel controls. It must label cloud providers clearly, show setup status or hints instead of credential values, and save durable changes through `PATCH /api/v1/voice/settings` rather than browser-local durable storage.

## Persisted Settings

Persist these settings per device profile in SQLite:

- wake word phrase or model ID;
- wake sensitivity threshold;
- voice service provider;
- STT provider pack;
- TTS provider pack;
- STT/TTS model IDs;
- TTS voice ID;
- cloud STT/TTS opt-in;
- command-provider enablement;
- sensitive-output speech policy;
- follow-up window seconds, default `8`;
- maximum follow-up session seconds, default `45`;
- maximum follow-up turns, default `5`;
- mute default;
- microphone profile;
- preferred voice language;
- per-device preferred agent.

YAML or JSON config may bootstrap these values, but runtime changes are saved through the hub settings API.

## Privacy Rules

- Raw microphone audio stays local by default.
- Wake-word and VAD processing happen before any cloud provider is called.
- Cloud STT and cloud TTS are opt-in per household or device profile.
- A2A agents receive final transcripts and redacted dashboard context only.
- Raw audio, pre-roll buffers, and partial transcripts are not sent to A2A agents.
- Voice Provider Pack manifests never contain raw secrets.
- Voice logs exclude raw audio and raw transcripts by default.
- Ambient mode avoids showing transcripts unless the user has enabled visible conversation history.
