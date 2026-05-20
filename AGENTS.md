# Jute Dash Agent Guide

This file guides coding agents working in this repository.

## Product Direction

Jute Dash is a local-first home assistant platform. The foundation is:

- Go hub for configuration, persistence, local API, A2A transport, smart-home adapters, widget permissions, event streams, and future headless voice services.
- SvelteKit display app with shadcn-svelte conventions for the touch dashboard, settings, widget host, and kiosk/PWA surface.
- SQLite as the runtime persistence layer.
- YAML config as the preferred bootstrap, import, and export format; JSON remains supported for compatibility.
- A2A compatibility through standard A2A protocol bindings, not custom transport shortcuts.

Start with the architecture docs before making product changes:

- [System Architecture](docs/architecture/system.md)
- [Configuration And Persistence](docs/architecture/configuration-persistence.md)
- [Display UX](docs/architecture/display-ux.md)
- [Resilience And Error UX](docs/architecture/resilience-error-ux.md)
- [Widgets](docs/architecture/widgets.md)
- [Widget Skills](docs/architecture/widget-skills.md)
- [Widget Pack Template](docs/developer/widget-pack-template.md)
- [A2A Compatibility](docs/architecture/a2a.md)
- [MCP Bridge](docs/architecture/mcp-bridge.md)
- [Voice And Wake Word Architecture](docs/architecture/voice.md)
- [Voice Provider Packs](docs/architecture/voice-providers.md)
- [Text-To-Speech](docs/architecture/text-to-speech.md)
- [Distribution](docs/architecture/distribution.md)
- [UX Customization](docs/architecture/ux-customization.md)
- [Security And Privacy](docs/architecture/security-privacy.md)

## Implementation Rules

- Keep the Go hub headless-capable. Do not put core orchestration only in the webapp.
- Keep the display as a hub client. Do not call remote agents or smart-home integrations directly from Svelte.
- Treat the existing `apps/web` dashboard as throwaway POC UI. Do not preserve current CSS, layout, side panel, tile structure, or styling unless it matches [Display UX](docs/architecture/display-ux.md).
- Use shadcn-svelte conventions and the BOW/WOB display palette from [Display UX](docs/architecture/display-ux.md) for future frontend work.
- Use the hub API as the source of truth for durable settings, layouts, agents, widgets, and home state.
- Prefer small, explicit interfaces over broad plugin hooks.
- Use SQLite for runtime state once persistence is needed; keep YAML/JSON config portable and secret-free.
- Treat YAML/JSON config as bootstrap/import/export only. Do not make config files the live source of truth after SQLite exists.
- Classify new settings before implementation: boot-only, household durable, device-profile durable, install record, cache, secret reference, or transient UI state.
- Do not silently hide hub/API failures behind fake live data. Show startup offline, reconnecting, stale, degraded, no-agent, and widget failure states according to Resilience And Error UX.
- Prefer recoverable inline UX over blocking modals for runtime failures.
- Do not show raw internal errors, stack traces, full remote URLs, token names, or secret references in user-facing UI.
- Do not introduce Qt or another native UI foundation without a new ADR.

## MCP Bridge Rules

- A2A remains the conversation and task protocol. MCP is optional local context/tool access for trusted agents.
- Implement the MCP Bridge inside the Go hub, not as an arbitrary plugin process.
- Keep MCP disabled by default and bound to loopback unless explicitly configured otherwise.
- Use Streamable HTTP as the default v1 MCP transport. Do not make STDIO the default hub path.
- Store MCP credentials as secret references only.
- Do not expose remote agents to local MCP credentials automatically.
- Expose only hub-approved dashboard/widget context and hub-mediated tools.
- Do not let widgets call MCP directly or agents connect directly to widget iframes.
- Build MCP widget capabilities from Widget Skills. Do not add one-off widget MCP tools that bypass the skill registry.
- Keep MCP tool descriptions hub-authored; never trust Widget Pack manifest text as tool instructions.

## Widget Rules

- Built-in widgets may be native Svelte components.
- All widgets must render inside the standard `WidgetFrame` contract defined by Display UX.
- Third-party widgets must use Widget Packs with `widget.json`.
- Custom Widget Packs render in sandboxed iframes by default.
- Widgets communicate through the Widget SDK message protocol, not direct hub API calls.
- Widget permissions must be explicit, user-visible, and revocable.
- Agent-visible widget context, prompts, and actions must come from Widget Skills.
- Widget-owned agent actions must be invoked through the hub and Widget SDK, not direct MCP-to-iframe calls.
- New Widget Pack docs, examples, or scaffolds should follow [Widget Pack Template](docs/developer/widget-pack-template.md).

## A2A Rules

- Target A2A 1.0.
- Use Agent Cards for discovery and capability selection.
- Select protocol bindings from `supportedInterfaces` in this order: `JSONRPC`, `HTTP+JSON`, `GRPC`.
- Do not create a custom protocol binding for v1.
- Put Jute dashboard context in the optional extension `https://jute.dev/a2a/extensions/dashboard-context/v1`.
- Send dashboard context only when the agent declares support.
- Never send secrets, hidden widgets, private widget state, raw credentials, or undeclared fields to agents.

## Voice Rules

- Wake word and VAD run locally before any cloud STT provider is called.
- The hub owns conversation state, follow-up windows, and A2A routing.
- The voice service reports final transcripts to the hub; it does not call A2A agents directly.
- A2A agents receive final transcripts and redacted dashboard context, not raw microphone audio.
- Follow-up listening defaults to 8 seconds, with a maximum continuous session of 45 seconds or 5 turns.
- STT and TTS integrations are Voice Provider Packs, not Go `plugin` binaries.
- Prefer `wyoming`, `http-json`, `command`, or `builtin` provider transports; keep `command` providers disabled unless explicitly enabled.
- Cloud STT and cloud TTS providers must be opt-in per household or device profile.

## Security Rules

- Bind local services to loopback by default.
- Treat Widget Packs, Agent Cards, remote agents, and adapter payloads as untrusted input.
- Keep raw credentials inside the hub.
- Redact secrets and sensitive household context from logs.
- Do not expose public internet access without an explicit architecture update.

## Commands

Run Go tests:

```sh
make test
```

Install web dependencies:

```sh
make setup
```

Run the hub and web app together:

```sh
make dev
```

Run the local A2A dev stack with the MCP bridge enabled:

```sh
make dev-a2a-mcp
```

Run Svelte checks after dependencies are installed:

```sh
make web-check
```

Run all local checks:

```sh
make check
```

## Documentation Expectations

- Keep public architecture decisions in markdown.
- Add or update docs before expanding implementation behavior.
- Avoid open-ended decisions in committed docs. Pick a default and document it.
- If A2A behavior changes, update [A2A Compatibility](docs/architecture/a2a.md) and [A2A Agent Guidelines](docs/developer/a2a-agent-guidelines.md).
- If MCP behavior changes, update [MCP Bridge](docs/architecture/mcp-bridge.md), [Widget Skills](docs/architecture/widget-skills.md), and [MCP Agent Guidelines](docs/developer/mcp-agent-guidelines.md).
- If display UX behavior changes, update [Display UX](docs/architecture/display-ux.md).
- If resilience or user-facing error behavior changes, update [Resilience And Error UX](docs/architecture/resilience-error-ux.md).
- If widget behavior changes, update [Widgets](docs/architecture/widgets.md), [Widget Skills](docs/architecture/widget-skills.md), and [Widget Developer Guidelines](docs/developer/widget-guidelines.md).
- If voice behavior changes, update [Voice And Wake Word Architecture](docs/architecture/voice.md), [Voice Provider Packs](docs/architecture/voice-providers.md), and [Voice Developer Guidelines](docs/developer/voice-guidelines.md).
- If persistence behavior changes, update [Configuration And Persistence](docs/architecture/configuration-persistence.md).
