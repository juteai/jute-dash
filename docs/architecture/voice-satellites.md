# Headless Voice Satellites

## Goal

Headless voice satellites extend Jute voice into rooms without a display while preserving the same local-first privacy model as the dashboard. A satellite is a small trusted device, such as a Raspberry Pi-class node, that runs microphone capture, local VAD, local wake word, and pre-roll buffering near the user, then reports safe voice state and final transcripts to the hub.

Satellites are not A2A agents, widgets, MCP clients, or arbitrary plugin hosts. The hub remains the conversation authority.

## Scope

This document defines the follow-up architecture required before satellite runtime implementation begins. It covers:

- pairing, authentication, and revocation;
- device-profile binding and settings inheritance;
- default network posture;
- safe hub API and event payloads;
- provider placement between satellite and hub-side services;
- provisioning and update constraints for small Linux devices;
- multi-room routing and privacy boundaries.

Full remote access, public internet satellite access, native image builds, and automatic room-level presence inference are out of scope until separate architecture decisions exist.

## Satellite Responsibilities

A satellite owns only the local audio edge:

- capture microphone PCM through the same `AudioCapture` contract used by display devices;
- run local VAD before any STT provider is called;
- run local wake-word detection before capture is promoted to an utterance;
- maintain a short in-memory pre-roll buffer;
- call an allowed local or LAN STT provider when configured to do so;
- report safe state, health, wake, and final transcript events to the hub;
- accept hub commands for mute, unmute, cancel, settings refresh, and service restart when supported.

A satellite must not:

- call A2A agents directly;
- call widgets, widget skills, or MCP tools;
- receive dashboard context;
- store raw provider credentials;
- send raw microphone audio, pre-roll buffers, or partial transcripts to the hub or agents;
- expose browser fallback voice APIs, because it has no display session.

## Pairing And Trust

Satellites require explicit pairing before the hub accepts events or commands.

Pairing flow:

1. The satellite starts unpaired and advertises only a local pairing capability on an explicitly configured LAN interface.
2. The hub starts a short-lived pairing window from an authenticated settings flow.
3. The satellite displays or prints a pairing code, or exposes it through a local console command for headless setup.
4. The hub exchanges the pairing code for a satellite identity, device profile binding, and secret reference.
5. The hub stores the satellite install record and a secret reference in SQLite. Raw secret material remains outside portable YAML/JSON exports.
6. The satellite stores only the minimum credential required to authenticate to the hub.

Authentication requirements:

- every satellite request includes a satellite identity and authentication proof;
- credentials are revocable per satellite;
- credential rotation must be possible without changing device profile identity;
- failed authentication emits a safe audit event without raw token values;
- paired satellites are unavailable until the hub setting for LAN satellite access is enabled.

Revocation:

- revoking a satellite immediately stops accepting its events;
- revocation does not delete historical conversation summaries;
- a revoked satellite must be re-paired before it can reconnect;
- settings UI should show revoked, offline, update-required, and misconfigured states distinctly.

## Network Posture

The hub remains loopback-bound by default. Satellite support requires explicit LAN enablement because a headless device is usually separate from the hub host.

Defaults:

- no satellite-facing listener is enabled in a default install;
- LAN satellite access requires an explicit household setting;
- satellite APIs bind only to configured private LAN interfaces;
- TLS or a mutually authenticated local transport is required before production use;
- public internet satellite access is not a v1 feature.

Development may use loopback or local test fixtures, but tests must not depend on a physical microphone, model download, or LAN device.

## Device Profiles And Settings

Each satellite binds to exactly one device profile. The profile is the durable scope for voice settings, privacy policy, and routing defaults.

Satellite-scoped settings:

- enabled or disabled;
- room or coarse location label;
- wake-word provider and model;
- wake sensitivity;
- microphone profile;
- STT provider and model;
- TTS provider and voice, if the satellite has a speaker;
- language or locale;
- follow-up window within hub limits;
- cloud STT/TTS opt-in;
- command-provider enablement;
- preferred A2A agent or routing policy;
- sensitive-output speech policy.

Inheritance:

- household defaults may seed a satellite profile at pairing time;
- runtime changes are persisted per device profile through the hub settings APIs;
- cloud provider opt-in and command-provider enablement never inherit silently onto a satellite;
- provider credentials remain hub-owned secret references unless a future provider explicitly requires satellite-local secrets and a separate security design approves it.

## Provider Placement

Provider placement is explicit because satellites have constrained CPU, memory, storage, and privacy surfaces.

Allowed on satellite:

- microphone capture;
- VAD;
- wake-word detection;
- pre-roll buffering;
- local wake-word model files;
- local STT only when the selected provider is marked satellite-capable and its resource budget fits the device profile;
- local TTS only when the device has an approved playback path.

Hub-side or hub-adjacent by default:

- provider discovery and selection;
- cloud STT/TTS credential access;
- command providers;
- A2A routing;
- dashboard context redaction;
- conversation history and summaries;
- TTS cache policy.

Provider Pack manifests should eventually declare satellite capability hints, including CPU architecture, memory estimate, model size, offline support, and whether credentials are required locally.

## Hub API Shape

The first implementation slice should add a satellite-specific ingress surface rather than overloading browser/display endpoints.

Future hub APIs:

- `POST /api/v1/voice/satellites/pairing-sessions`: opens a short pairing window.
- `POST /api/v1/voice/satellites/register`: exchanges a pairing code for a satellite identity.
- `GET /api/v1/voice/satellites`: lists safe satellite status.
- `GET /api/v1/voice/satellites/{id}`: returns safe status, profile binding, version, and health.
- `PATCH /api/v1/voice/satellites/{id}`: updates binding, display name, coarse room, enabled state, or revocation state.
- `POST /api/v1/voice/satellites/{id}/events`: accepts safe satellite events.
- `POST /api/v1/voice/satellites/{id}/transcripts/final`: accepts final transcripts and routes them through the hub-owned conversation pipeline.
- `POST /api/v1/voice/satellites/{id}/commands/{command}`: sends hub-owned mute, unmute, cancel, restart, or refresh commands when supported.

Safe satellite event payloads may include:

- satellite ID;
- device profile ID;
- coarse room label;
- state;
- wake model ID;
- provider IDs;
- provider health state;
- final transcript text;
- transcript language;
- utterance duration;
- local processing latency;
- version and update channel;
- safe error code.

Payloads must not include:

- raw microphone audio;
- pre-roll buffers;
- VAD frames;
- partial transcripts;
- provider credentials or secret reference names;
- private widget state;
- raw dashboard context;
- exact presence confidence or continuous location traces;
- full internal errors, stack traces, or provider URLs with query strings.

## Event Contracts

Satellite events are normalized into the existing `/api/v1/events` stream after hub validation.

Future event types:

- `voice.satellite_registered`
- `voice.satellite_state_changed`
- `voice.satellite_health_changed`
- `voice.satellite_wake_detected`
- `voice.satellite_version_changed`
- `voice.satellite_update_available`
- `voice.satellite_revoked`

Conversation events remain the same as display-device voice. The event payload identifies the satellite and bound device profile, but does not reveal raw audio, partial transcripts, provider credentials, or precise presence.

## Provisioning And Updates

Raspberry Pi-class support should start with a simple service package before full image distribution.

Initial provisioning constraints:

- Linux service unit or container with documented user, group, and device permissions;
- explicit microphone device selection;
- local config contains only hub address, satellite ID, and secret reference material;
- no provider credentials in portable config;
- clear diagnostics for microphone unavailable, wake provider unavailable, clock skew, auth failed, and hub unreachable.

Update constraints:

- satellite reports version, build channel, and update-required state;
- hub can show update status but does not silently push arbitrary code in v1;
- future automatic updates require signed artifacts and rollback design;
- command providers remain disabled unless explicitly enabled for that satellite profile.

## Raspberry Pi-Class Provisioning

The first supported provisioning path is a small Linux service on Raspberry Pi-class hardware. This is a manual package or source build path, not an appliance image.

Supported assumptions for v1 documentation:

- Debian or Raspberry Pi OS Lite on `arm64` or `armhf`;
- systemd as the service manager;
- a single service user named `jute-voice`;
- microphone access through ALSA or PipeWire-compatible device nodes;
- local filesystem storage for the satellite runtime config and any local wake/STT model assets;
- a hub reachable on the same trusted LAN only after the hub has explicitly enabled satellite LAN access.

Do not promise:

- automatic image builds;
- unattended OS provisioning;
- automatic LAN discovery;
- public internet satellite access;
- hub-pushed code updates.

Recommended service account setup:

```sh
sudo useradd --system --home /var/lib/jute-voice --create-home --shell /usr/sbin/nologin jute-voice
sudo usermod -aG audio jute-voice
sudo install -d -o jute-voice -g jute-voice -m 0750 /etc/jute-voice /var/lib/jute-voice
```

Microphone setup:

- select the microphone explicitly rather than relying on whichever device is first at boot;
- document the ALSA card/device or PipeWire source in the local device profile notes;
- grant only the service user or `audio` group access needed for capture;
- test with a local fixture or short local capture command before pairing;
- treat `microphone_unavailable` as a local permissions, missing device, or driver issue, not as a hub issue.

The fixture-driven runtime can be smoke-tested without microphone hardware. From the repository root,
generate a deterministic 16 kHz mono PCM WAV fixture with the benchmark helper:

```sh
mkdir -p /tmp/jute-voice

go run ./apps/hub/cmd/jute-voice-benchmark \
  -tone-fixture /tmp/jute-voice/smoke.wav \
  -tone-duration 320ms \
  -tone-frequency 440 \
  -tone-amplitude 0.3
```

To verify the runtime command shape without a running hub, run the command package smoke test. It uses
an in-process mock hub and proves the fixture audio can drive final transcript submission without a
microphone, model download, or LAN device:

```sh
go test ./apps/hub/cmd/jute-voice-satellite -run TestRunFixtureSatelliteCommand -count=1 -v
```

To run the actual satellite command, point it at a paired hub or local development hub that has the
satellite install record and auth proof configured:

```sh
export JUTE_SATELLITE_AUTH='replace-with-paired-auth-proof'

go run ./apps/hub/cmd/jute-voice-satellite \
  -hub-url http://127.0.0.1:8787 \
  -satellite-id sat-kitchen \
  -auth-secret-env JUTE_SATELLITE_AUTH \
  -fixture-wav /tmp/jute-voice/smoke.wav \
  -transcript "turn on the kitchen lights"
```

The command prints a safe JSON result. When the hub accepts the transcript, the result includes the
hub-owned voice `conversationId` plus follow-up state such as `followupActive`, `followupTurns`, and
`followupMaxTurns`. To smoke-test a follow-up turn, pass that conversation ID back to the command:

```sh
go run ./apps/hub/cmd/jute-voice-satellite \
  -hub-url http://127.0.0.1:8787 \
  -satellite-id sat-kitchen \
  -auth-secret-env JUTE_SATELLITE_AUTH \
  -fixture-wav /tmp/jute-voice/smoke.wav \
  -transcript "make it ten minutes" \
  -conversation-id voice-conversation-from-previous-run
```

The hub still owns follow-up limits, device-profile binding, preferred-agent routing, dashboard
context redaction, and A2A dispatch. A satellite-provided `conversationId` is only a continuation
request; it is rejected if the hub session expired, reached its turn limit, or belongs to another
device profile/device source.

The command exits non-zero with safe diagnostics such as `hub_unreachable` or `auth_failed` if the hub
is not running, is still loopback-only from a remote satellite, lacks the paired satellite record, or
rejects the configured auth proof.
If the fixture run lacks a safe wake model identifier, it exits with `wake_provider_unavailable`
before posting final transcripts.

For a portable runtime config, keep only references and non-secret local settings:

```json
{
  "hubUrl": "http://192.168.1.10:8787",
  "satelliteId": "sat-kitchen",
  "authSecretEnv": "JUTE_SATELLITE_AUTH",
  "fixtureWav": "/var/lib/jute-voice/fixtures/smoke.wav",
  "transcript": "turn on the kitchen lights",
  "conversationId": "voice-conversation-from-previous-run",
  "version": "0.1.0",
  "wakeModelId": "fixture-wake"
}
```

Use `conversationId` only for short-lived local smoke runs or an operator-controlled follow-up test.
Do not preserve household conversation IDs in exported examples, shared support bundles, or long-lived
portable config.

Portable satellite config must not contain raw credentials, provider API keys, OAuth tokens, pre-roll audio, raw PCM, transcripts from real household conversations, provider endpoint URLs with embedded credentials, or secret reference values. The `authSecretEnv` value is an environment variable name only. The raw value belongs in the service manager environment, local keyring, or another future approved secret store outside exportable config. Runtime config JSON is schema-strict: unknown fields and trailing JSON are rejected before the fixture runtime starts.

A minimal systemd service shape:

```ini
[Unit]
Description=Jute Voice Satellite
After=network-online.target sound.target
Wants=network-online.target

[Service]
User=jute-voice
Group=jute-voice
SupplementaryGroups=audio
Environment=JUTE_SATELLITE_AUTH=replace-with-local-secret
ExecStart=/usr/local/bin/jute-voice-satellite -config /etc/jute-voice/satellite.json
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/jute-voice

[Install]
WantedBy=multi-user.target
```

Production packages should replace inline `Environment=` secrets with a systemd credential, local keyring, root-owned environment file, or another approved secret-reference mechanism. The service file above is a shape, not a recommendation to commit secrets.

## Pairing And Recovery Runbook

Pairing requires both a hub-side pairing window and a satellite-side pairing command or console flow.

Operator sequence:

1. Enable satellite LAN access on the hub for a private LAN interface.
2. Open a short hub pairing window from authenticated settings.
3. Start the satellite in unpaired setup mode.
4. Enter or confirm the pairing code.
5. Store the returned satellite ID and local auth proof on the satellite.
6. Bind the satellite to the intended device profile and coarse room label in hub settings.
7. Run the fixture command against the hub before enabling real microphone capture.
8. If follow-up listening is enabled, repeat the fixture command once with the returned `conversationId`
   and confirm the hub accepts or safely rejects the continuation according to the follow-up window.

Authentication failure recovery:

- `auth_failed` means the hub rejected the satellite proof;
- verify the satellite ID, hub URL, clock, and local secret source;
- do not copy secrets into YAML/JSON config to troubleshoot;
- if the stored proof is unknown or suspected leaked, revoke the satellite and re-pair.

Revocation recovery:

- revoked satellites must not be re-enabled by silently accepting their old credential;
- the old credential remains unusable after revocation;
- recovery is a new pairing flow that creates or rotates the accepted credential and refreshes the install record.

Hub unreachable recovery:

- verify that the hub has satellite LAN access enabled;
- verify the hub binds a private LAN address, not only `127.0.0.1`;
- verify local firewall rules;
- keep public internet exposure out of scope for v1.

Clock skew recovery:

- enable NTP or another trusted time source on the satellite;
- treat severe skew as a pairing/authentication risk;
- do not relax token expiry or replay protections to compensate for an untrusted clock.

## Provider Placement For Satellites

Default placement for Raspberry Pi-class nodes:

- VAD: satellite-local.
- Wake word: satellite-local, using a lightweight local model when available.
- Pre-roll: satellite-local and memory-only.
- STT: hub-side or LAN local provider by default; satellite-local only when the selected provider declares satellite capability and fits the device.
- TTS: hub-side or a local playback provider only when the satellite has a configured speaker path.
- Command providers: hub-owned and disabled unless explicitly enabled for that satellite profile.
- Cloud STT/TTS: disabled until household or device-profile opt-in is explicit.

Provider credentials remain hub-owned secret references by default. A satellite-local provider that needs credentials requires a separate security design before support is added.

## Troubleshooting States

| State or diagnostic | Meaning | Safe operator action |
| --- | --- | --- |
| `microphone_unavailable` | The satellite cannot open or read the configured microphone or fixture. | Check device path, `audio` group membership, service sandbox paths, and local driver status. |
| `wake_provider_unavailable` | Wake model or wake service is missing, unhealthy, or unreachable. | Check local model files, provider process status, and selected wake model ID. |
| `clock_skew` | Satellite time is too far from hub time for safe auth or event ordering. | Fix NTP/time sync before pairing or retrying auth. |
| `hub_unreachable` | The satellite cannot reach the hub API. | Check hub LAN enablement, address, firewall, and private LAN routing. |
| `auth_failed` | The hub rejected the satellite identity or proof. | Verify secret source and satellite ID; revoke and re-pair if uncertain. |
| `update_required` | The satellite version is below the hub-supported or recommended version. | Plan a manual package update; v1 does not silently push code. |
| `revoked` | The hub refuses events from this satellite credential. | Re-pair the device; do not reuse the old credential. |
| `misconfigured` | Safe settings are incomplete or invalid. | Review device profile binding, provider selection, and local config references. |

Diagnostics and logs must use stable issue codes like the table above. They must not include raw PCM, pre-roll buffers, partial transcripts, provider internals, provider credentials, token names, secret reference values, private widget state, exact presence confidence, or full remote URLs with embedded query strings.

## Multi-Room Privacy

Satellites improve room coverage but can easily become presence sensors. Jute should treat room attribution as coarse routing data, not precise occupancy tracking.

Rules:

- satellite room labels are user-assigned or coarse;
- no continuous presence stream is inferred from wake, VAD, or microphone levels;
- wake events may route the follow-up conversation to the originating satellite or display;
- transcript routing uses the bound device profile and preferred agent unless the hub policy overrides it;
- ambient displays may show that a nearby room is responding, but not full transcripts unless visible history is enabled;
- multi-room handoff requires explicit future design before conversations follow users across rooms.

## First Implementation Issues

The first end-to-end satellite runtime should be split into small slices:

1. Add satellite install records, pairing session model, and revocation state in the hub.
2. Add authenticated satellite event ingress with safe payload validation and logging tests.
3. Add final transcript ingress for satellites that reuses the hub-owned A2A conversation path.
4. Add satellite status/settings projection in the display settings UI.
5. Add a minimal Linux satellite runtime command using fixture audio and mocked providers.
6. Add provisioning documentation for Raspberry Pi-class devices.

These slices should land before full image builds, multi-room handoff, or satellite-local cloud credentials.
