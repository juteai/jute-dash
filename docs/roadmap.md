# Roadmap

## Phase 0: Foundation

- Choose the stack and repo shape.
- Complete docs-first architecture pass.
- Define Widget Pack extensibility and A2A dashboard-context strategy.
- Define optional MCP Bridge strategy for trusted local agents.
- Define multi-platform distribution strategy.
- Define clean-slate display UX and mark the current dashboard as throwaway POC.
- Define voice and wake-word architecture, provider boundaries, follow-up listening, and conversation UI.
- Define selectable STT/TTS Voice Provider Packs and TTS playback policy.
- Define local configuration persistence, first-run setup, YAML/JSON bootstrap/import/export, and SQLite runtime ownership.
- Define resilience and error UX for hub disconnects, stale data, no-agent states, widget failures, and safe error copy.
- Add coding-agent guidance in `AGENTS.md` and `CLAUDE.md`.
- Build the Go hub entrypoint.
- Add config loading, validation, public config projection, and starter APIs.
- Add a provisional SvelteKit dashboard shell wired to the hub API.
- Keep starter UI provisional and replaceable until clean-slate UX implementation begins.

## Phase 1: Local Dashboard MVP

- Implement the JSON-RPC A2A client transport.
- Add MCP bridge settings and per-agent MCP scopes, disabled by default.
- Resolve and cache Agent Cards.
- Persist runtime state in SQLite using migrations and safe first-run defaults.
- Add setup status API and settings APIs for household and device profile configuration.
- Add `/api/v1/events` for dashboard and task updates.
- Replace POC dashboard UI with the clean-slate dashboard, edit mode, WidgetFrame, and chat mode.
- Add display resilience UX: startup offline screen, runtime reconnect ribbon, stale dashboard styling, no-agent chat state, and widget error frames.
- Implement Widget Pack manifest validation and iframe host runtime.
- Show agent skills in the UI.
- Send user prompts to a selected agent.
- Stream task updates back to the display through hub events.
- Add a read-only MCP Bridge with dashboard/widget resources and safe context tools for local agents.
- Add settings UI for home name, rooms, tiles, and agent registration.

## Phase 2: Home Context

- Add smart-home adapter interfaces.
- Start with Home Assistant, MQTT, and simple HTTP webhook integrations.
- Normalize rooms, devices, scenes, and sensors into a single home state model.
- Let agents request context through bounded hub APIs instead of direct device access.
- Add approval-gated MCP tools for future smart-home action requests.

## Phase 3: Voice And Presence

- Add hub voice status, conversation state, and voice/conversation SSE events.
- Add Echo Show-style conversation UI to the display.
- Add local display-device voice service with VAD, openWakeWord, and pre-roll buffering.
- Add Voice Provider Pack manifest validation and discovery.
- Add speech-to-text provider selection with Wyoming local/LAN as the first practical path.
- Add optional text-to-speech provider selection, voice listing, preview, playback, and stop controls.
- Route voice turns through the same A2A conversation pipeline as typed messages.
- Add follow-up listening without wake word for the configured window.
- Add local wake-word service support for headless satellite nodes.

## Phase 4: Productization

- Add SQLite backup/export tooling and migration compatibility checks.
- Add device pairing and household profiles.
- Add secure remote access.
- Package kiosk images and desktop wrappers.
- Add plugin templates for agents and integrations.
- Add provider pack contribution templates and conformance tests for STT/TTS providers.
