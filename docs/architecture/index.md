# Jute Dash Architecture

Jute Dash is a local-first home assistant platform built around a headless-capable Go hub and a browser/kiosk-friendly SvelteKit display. The product should feel like an Echo Show for dashboards, while remaining useful on tablets, desktops, wall displays, Raspberry Pi devices, and headless voice nodes.

## Architecture Map

- [System Architecture](system.md): hub, display, API boundaries, persistence, event streams, and deployment modes.
- [Configuration And Persistence](configuration-persistence.md): local data paths, first-run setup, SQLite runtime store, YAML/JSON bootstrap/import/export, migrations, and secrets.
- [Display UX](display-ux.md): clean-slate dashboard, widget frame, edit mode, chat mode, brand constants, and first widgets.
- [Resilience And Error UX](resilience-error-ux.md): hub disconnects, stale data, no-agent states, widget failures, safe error copy, and recovery behavior.
- [Widgets](widgets.md): built-in widgets, custom Widget Packs, iframe sandboxing, SDK messages, and dashboard context.
- [A2A Compatibility](a2a.md): Agent Card discovery, A2A 1.0 bindings, streaming, caching, credentials, and the Jute dashboard-context extension.
- [MCP Bridge](mcp-bridge.md): optional local MCP tool and context surface for trusted A2A agents.
- [Voice And Wake Word Architecture](voice.md): local-first hybrid voice, wake word, STT/TTS providers, follow-up listening, and conversation UI.
- [Voice Provider Packs](voice-providers.md): selectable STT/TTS provider plugins, manifests, transports, health checks, and contribution rules.
- [Text-To-Speech](text-to-speech.md): spoken response policy, provider choices, playback, caching, and TTS UI contracts.
- [Distribution](distribution.md): multi-platform builds, release artifacts, Docker, PWA/kiosk, Raspberry Pi, and future native wrappers.
- [UX Customization](ux-customization.md): dashboard experience, themes, layout profiles, ambient mode, and persistence.
- [Security And Privacy](security-privacy.md): local-first defaults, widget isolation, context redaction, secrets, and network exposure.

## Foundational Decisions

- The product foundation is **Go hub + SvelteKit/shadcn-svelte display**, not Qt.
- The Go hub owns configuration, local state, agent registry, A2A client transport, integration adapters, event streams, and future headless voice services.
- The SvelteKit app owns the touch dashboard, customization UI, widget surface, and kiosk/PWA experience.
- The current Svelte UI is throwaway proof-of-concept work. The clean display UX is defined in [Display UX](display-ux.md).
- Runtime failures must be visible, calm, and actionable. Do not silently hide hub disconnects, missing agents, stale data, or widget failures.
- Voice is local-first hybrid: wake word and VAD run locally, while STT/TTS use selectable Voice Provider Packs with local/LAN defaults and optional cloud implementations.
- Voice Provider Packs are manifest-driven process or network integrations. Do not use Go in-process dynamic plugins for v1.
- SQLite is runtime truth. YAML config is preferred for bootstrap, import, and export, while JSON remains supported.
- Secrets are references only and are not stored in YAML, JSON, or ordinary settings rows.
- Custom widgets use Widget Packs by default, loaded from a manifest and rendered in sandboxed iframes with a typed postMessage SDK.
- Jute-specific dashboard context is sent to agents through an optional A2A extension, not a custom A2A protocol binding.
- The optional Jute MCP Bridge runs inside the Go hub and exposes local dashboard context and safe tools to trusted local agents.
- Standard A2A protocol bindings are used for v1, in this order: `JSONRPC`, `HTTP+JSON`, then `GRPC`.

## Current State

The starter code in this repo is provisional. It establishes the first Go hub shape, example config, and a small HTTP contract. The existing `apps/web` dashboard is POC UI and may be replaced wholesale. The next implementation work should follow these architecture docs before expanding behavior.

## Related Docs

- [Roadmap](../roadmap.md)
- [ADR 0001: Go Hub With SvelteKit Display](../adr/0001-foundation-stack.md)
- [Widget Developer Guidelines](../developer/widget-guidelines.md)
- [A2A Agent Guidelines](../developer/a2a-agent-guidelines.md)
- [MCP Agent Guidelines](../developer/mcp-agent-guidelines.md)
- [Voice Developer Guidelines](../developer/voice-guidelines.md)
