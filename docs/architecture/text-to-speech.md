# Text-To-Speech Architecture

## Goal

Jute should speak assistant responses when a household wants that experience, but spoken output must remain optional, configurable, and local-first. The visual conversation UI remains the reliable baseline; TTS adds presence and accessibility without becoming a hard dependency.

TTS uses the same [Voice Provider Packs](voice-providers.md) model as speech-to-text. Jute does not create a separate TTS-only plugin system.

## Recommended Provider Stack

Default architecture path:

1. **Wyoming-compatible local TTS service** for local/LAN deployments and Home Assistant-compatible voice stacks.
2. **sherpa-onnx provider pack** for embedded/local TTS where Jute can call a sidecar or future built-in adapter.
3. **Piper/OHF Piper provider pack** as an external service or command wrapper. Jute should not bundle GPL TTS engines without a future licensing decision.
4. **Browser `speechSynthesis` fallback** for display-only preview or degraded mode. It is not the headless or canonical voice path.
5. **OpenAI text-to-speech provider pack** as optional cloud-quality TTS, using user-provided credentials and explicit cloud opt-in.

The first implementation should prioritize playback, cancellation, and provider health over advanced voice styling.

## Component Flow

```mermaid
flowchart LR
  agent["A2A agent response"]
  hub["Jute Hub\nconversation authority"]
  policy["Speech policy\nredaction + sensitivity"]
  provider["Selected TTS Provider Pack"]
  voice["Jute Voice Service\nplayback owner"]
  ui["Jute Display\nconversation sheet"]
  events["/api/v1/events"]

  agent --> hub
  hub --> policy
  policy --> provider
  provider --> voice
  voice --> events
  hub --> events
  events --> ui
```

The hub decides whether a response should be spoken. The provider only synthesizes text that the hub has approved for speech.

## Playback Sequence

```mermaid
sequenceDiagram
  participant Hub as Jute Hub
  participant Provider as TTS Provider
  participant Voice as Jute Voice Service
  participant UI as Jute Display
  participant User

  Hub->>Provider: synthesize approved assistant text
  Provider-->>Hub: stream or playable audio reference
  Hub->>Voice: play response
  Hub->>UI: tts.started
  Voice-->>UI: playback progress through events
  User->>Voice: barge-in or cancel
  Voice->>Hub: playback stopped
  Hub->>Provider: cancel if still synthesizing
  Hub->>UI: tts.stopped
```

## Speech Policy

Before calling a TTS provider, the hub applies a speech policy.

Default policy:

- speak ordinary assistant responses;
- do not speak secrets, credentials, door codes, private calendar details, precise presence, or sensitive widget context;
- use visual-only output for sensitive responses;
- require explicit cloud opt-in before cloud TTS;
- disclose AI-generated speech where required by the selected provider policy.

Supported sensitive-output modes:

- `visual_only_sensitive`: default; render sensitive content visually but do not synthesize it.
- `ask_before_sensitive`: prompt the user before speaking sensitive content.
- `speak_all`: advanced setting only, hidden behind explicit confirmation.

## Provider Capabilities

TTS provider manifests declare:

- supported locales;
- supported voice IDs;
- voice labels;
- streaming support;
- supported formats, such as `audio/pcm`, `audio/wav`, `audio/opus`, or `audio/mpeg`;
- style or instruction support;
- speed support;
- offline status;
- network requirements;
- expected latency class;
- cache eligibility.

For low-latency local playback, prefer PCM or WAV. For network transfer, Opus or MP3 may be acceptable when supported.

## API Contracts

Future hub APIs:

- `GET /api/v1/tts/voices`: returns voices for the selected provider or a requested provider ID.
- `POST /api/v1/tts/preview`: synthesizes a short user-confirmed preview phrase.
- `POST /api/v1/tts/speak`: queues speech for approved assistant text or explicit UI action.
- `POST /api/v1/tts/stop`: stops current playback.

Future events:

- `tts.started`: synthesis or playback has begun.
- `tts.chunk`: optional streaming progress event for chunked playback.
- `tts.completed`: playback completed.
- `tts.failed`: synthesis or playback failed.
- `tts.stopped`: user, policy, barge-in, or timeout stopped playback.

Every TTS event includes `id`, `type`, `createdAt`, `deviceId`, optional `conversationId`, optional `turnId`, and `payload`.

## UI Requirements

The conversation UI must make spoken output controllable:

- show when an assistant response is being spoken;
- keep stop and mute controls visible;
- support barge-in so user speech can stop playback and begin capture;
- show a visual response even when TTS fails;
- show selected voice and provider health in settings;
- offer preview before saving a voice;
- show clear labels for cloud providers.

Ambient mode may show only speaking/listening status. It should not reveal full sensitive text by default.

## Caching

TTS caching is optional and disabled for sensitive text.

Cache keys include:

- provider ID;
- model ID;
- voice ID;
- locale;
- normalized text hash;
- style or instruction hash;
- speed;
- output format.

Rules:

- do not cache sensitive responses by default;
- do not cache cloud TTS output unless the user enables it;
- provide a settings action to clear the cache;
- never use raw text as a cache filename.

## Failure Behavior

TTS failure must not fail the conversation.

Failure handling:

- if synthesis fails, emit `tts.failed` and show the assistant response visually;
- if playback fails, stop audio state and keep the response visible;
- if provider health is `offline` or `misconfigured`, skip synthesis and show a setup hint in settings;
- if user cancels, stop playback and keep the conversation recoverable;
- if user barges in, stop playback and enter utterance capture or follow-up listening.

## Persisted Settings

Persist TTS settings per device profile:

- enabled;
- selected provider ID;
- selected model ID;
- selected voice ID;
- locale;
- style or instruction string when supported;
- speed;
- volume;
- streaming preference;
- output target;
- cache policy;
- sensitive-output speech policy;
- cloud opt-in.

YAML or JSON config may bootstrap these values, but runtime edits are saved through the hub settings API.

## Security And Privacy

- TTS text is household data and may include sensitive assistant output.
- Cloud TTS is opt-in and clearly labeled.
- Provider packs never receive raw credentials from manifests.
- TTS logs exclude raw synthesized text by default.
- Audio cache entries are treated as private household data.
- Browser `speechSynthesis` is only a display fallback and must not become the headless voice path.
