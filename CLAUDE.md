# Claude Guidance For Jute Dash

Jute Dash is a local-first home assistant platform. Treat this repo as architecture-led: read the docs before expanding implementation.

## Core Architecture

- Hub: Go service for config, persistence, local API, A2A transport, smart-home adapters, widget permissions, event streams, and future headless voice.
- Display: SvelteKit with shadcn-svelte conventions for dashboard, settings, widget host, PWA, and kiosk use.
- Persistence: SQLite for runtime data, YAML for preferred bootstrap/import/export, and JSON for compatibility.
- Agents: bring-your-own A2A servers discovered through Agent Cards.
- Widgets: native Svelte components contributed to the `widgets/` directory via fork and PR.

Key docs:

- [Architecture Index](docs/architecture/index.md)
- [System Architecture](docs/architecture/system.md)
- [Configuration And Persistence](docs/architecture/configuration-persistence.md)
- [Display UX](docs/architecture/display-ux.md)
- [Visual Customization](docs/architecture/visual-customization.md)
- [Resilience And Error UX](docs/architecture/resilience-error-ux.md)
- [Widgets](docs/architecture/widgets.md)
- [Widget Skills](docs/architecture/widget-skills.md)
- [Widget Developer Guidelines](docs/developer/widget-guidelines.md)
- [Theme Developer Guidelines](docs/developer/theme-guidelines.md)
- [A2A Compatibility](docs/architecture/a2a.md)
- [MCP Bridge](docs/architecture/mcp-bridge.md)
- [Voice And Wake Word Architecture](docs/architecture/voice.md)
- [Voice Provider Packs](docs/architecture/voice-providers.md)
- [Text-To-Speech](docs/architecture/text-to-speech.md)
- [Distribution](docs/architecture/distribution.md)
- [UX Customization](docs/architecture/ux-customization.md)
- [Security And Privacy](docs/architecture/security-privacy.md)

## Working Rules

- Keep the hub headless-capable.
- Keep the display as a client of the hub API.
- Treat the current `apps/web` dashboard as throwaway POC UI. Do not preserve current CSS, layout, side panel, tile structure, or styling unless it matches Display UX.
- Use shadcn-svelte conventions and Theme Pack tokens from Visual Customization for future frontend work.
- Do not call remote A2A agents directly from the Svelte app.
- Do not put durable settings only in browser storage.
- Treat SQLite as runtime truth once persistence exists. Treat YAML/JSON config as bootstrap/import/export.
- Classify new settings before implementation: boot-only, household durable, device-profile durable, install record, cache, secret reference, or transient UI state.
- Do not silently hide hub/API failures behind fake live data. Show startup offline, reconnecting, stale, degraded, no-agent, and widget failure states according to Resilience And Error UX.
- Prefer recoverable inline UX over blocking modals for runtime failures.
- Do not show raw internal errors, stack traces, full remote URLs, token names, or secret references in user-facing UI.
- Do not store raw secrets in YAML/JSON config or public API responses.
- Do not create a custom A2A protocol binding for v1.
- Use the optional Jute A2A extension for dashboard context: `https://jute.dev/a2a/extensions/dashboard-context/v1`.
- Send dashboard context only when an agent declares support.
- Treat MCP as optional local context/tool access for trusted agents. A2A remains the conversation and task protocol.
- Implement the MCP Bridge inside the Go hub, disabled by default, loopback-bound by default, and using Streamable HTTP as the default v1 transport.
- Store MCP credentials as secret references only and never expose remote agents to local MCP credentials automatically.
- Expose only hub-approved dashboard/widget context and hub-mediated tools through MCP.
- Do not let widgets call MCP directly.
- Build MCP widget capabilities from Widget Skills. Do not add one-off widget MCP tools that bypass the skill registry.
- Keep MCP tool descriptions hub-authored.
- Keep wake word and VAD local before any cloud STT call.
- Route voice transcripts through the hub conversation pipeline, not directly to agents.
- Never send raw microphone audio to A2A agents.
- Implement STT/TTS integrations as Voice Provider Packs, not Go dynamic plugins.
- Keep cloud STT/TTS opt-in and command providers disabled unless explicitly enabled.
- Treat themes as data-only Theme Packs, not executable plugins.
- Keep widget transparency host-owned through `WidgetFrame` chrome modes: `solid`, `clear`, `smoked`, `frosted`, and `auto`.
- Keep background images local-first; do not add remote background URLs for v1.

## Widget Rules

- All widgets are native Svelte components contributed to `widgets/` via fork and PR.
- Each widget declares its identity, settings schema, Adapter Connection requirements, and optional agent-facing skill in its `widgets/{kind}/hub` Go package. Widget manifests such as `widget.yaml` are future work, not the current runtime contract.
- Expose agent context, prompts, and actions only through Widget Skills.
- Invoke widget-owned agent actions through the hub skill registry, not direct MCP-to-widget calls.
- Keep permissions explicit and revocable.
- New widget contributions should follow [Widget Developer Guidelines](docs/developer/widget-guidelines.md).

## Commands

```sh
make setup
make dev
make test
make web-check
make check
```

Self-contained dev stacks live in `examples/config/local/`, for example:

```sh
cd examples/config/local
make run-mock
```

## Documentation Discipline

- Update docs before implementing new architecture behavior.
- Do not leave open-ended decisions in architecture docs.
- When adding display behavior, update Display UX and do not treat the current POC UI as canonical.
- When adding visual customization behavior, update Visual Customization and Theme Developer Guidelines.
- When adding resilience or user-facing error behavior, update Resilience And Error UX.
- When adding A2A behavior, check the current official A2A docs and update Jute docs with links.
- When adding MCP behavior, check the current official MCP docs and update MCP Bridge, Widget Skills, and MCP Agent Guidelines.
- When adding widget behavior, update Widgets, Widget Skills, and Widget Developer Guidelines.
- When adding voice behavior, update voice architecture, provider pack architecture, and developer guidelines.

## Agent skills

### Issue tracker

Issues live in GitHub Issues for this repo. See `docs/agents/issue-tracker.md`.

### Triage labels

Default label vocabulary (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Multi-context repo — `CONTEXT-MAP.md` at root points to per-context `CONTEXT.md` files. See `docs/agents/domain.md`.
