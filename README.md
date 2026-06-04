# Jute Dash

Jute Dash is a local-first home assistant surface for bring-your-own agents. The first target is an Echo Show-style dashboard that runs on tablets, kiosks, desktops, and browsers, with a Go hub that can also run headless for wake-word and device-only deployments later.

## Architecture

- **Core hub:** Go HTTP service for configuration, local state, agent registry, and future A2A client orchestration.
- **Display UI:** SvelteKit with a shadcn-svelte-ready component structure for the touch dashboard.
- **Packaging path:** browser/kiosk first, optional Tauri/Capacitor/native wrappers later, and pure Go headless mode for small devices.
- **Agent model:** bring your own A2A-compatible agents, registered by Agent Card URL, endpoint URL, protocol binding, and optional secret references.

Architecture docs:

- [Architecture Index](docs/architecture/index.md)
- [System Architecture](docs/architecture/system.md)
- [Configuration And Persistence](docs/architecture/configuration-persistence.md)
- [Display UX](docs/architecture/display-ux.md)
- [Visual Customization](docs/architecture/visual-customization.md)
- [Widgets](docs/architecture/widgets.md)
- [Widget Skills](docs/architecture/widget-skills.md)
- [A2A Compatibility](docs/architecture/a2a.md)
- [Voice And Wake Word Architecture](docs/architecture/voice.md)
- [Voice Provider Packs](docs/architecture/voice-providers.md)
- [Text-To-Speech](docs/architecture/text-to-speech.md)
- [Distribution](docs/architecture/distribution.md)
- [UX Customization](docs/architecture/ux-customization.md)
- [Security And Privacy](docs/architecture/security-privacy.md)

## Getting Started

Install web dependencies once:

```sh
make setup
```

Start the hub and web UI together:

```sh
make dev
```

The hub runs at `http://127.0.0.1:8787` and the web UI runs at `http://127.0.0.1:5173` by default. The web UI expects the hub at `http://127.0.0.1:8787`; override that with `VITE_JUTE_API_URL`.

The root `Makefile` is intentionally core-only. Fixture-backed development stacks live under `examples/harnesses`.

To run the dashboard against the deterministic mock A2A 1.0 fixture:

```sh
cd examples/harnesses/mock-a2a
make dev
```

That self-contained harness owns its fixture, config, and `.jute/dev-mock-a2a` store. `make dev` installs missing dependencies, starts the hub, starts the embedded fixture, waits for readiness, and then starts the web UI.

To run the same mock stack with the Jute MCP Bridge enabled:

```sh
cd examples/harnesses/mock-a2a-mcp
make dev
```

To run the local Kronk-backed A2A 1.0 model harness:

```sh
cd examples/harnesses/kronk-a2a
make dev
make dev-mcp
```

MCP-enabled harnesses run the bridge at `http://127.0.0.1:8790/mcp` with loopback-only, no-token dev auth. Production-style config keeps MCP disabled by default and supports local bearer-token auth through `JUTE_MCP_TOKEN`.
MCP clients should send `X-Jute-Agent-ID` so the hub can apply the configured agent's MCP scopes.

## Current UI Status

The current `apps/web` dashboard is throwaway proof-of-concept UI. It is useful for checking the hub contract, but its CSS, layout, side panel, tile structure, and visual styling are not canonical. Future frontend work should follow [Display UX](docs/architecture/display-ux.md) and may replace the POC UI wholesale.

Useful local commands:

```sh
make run        # hub only
make web-dev    # web UI only
make test       # Go tests
make web-check  # Svelte checks
make check      # Go tests and Svelte checks
```

Each harness includes its own `fixture/` module. Run `make server` inside that module only when you intentionally want the agent process without the full Jute stack.

Optional MCP smoke request:

```sh
curl -s http://127.0.0.1:8790/mcp \
  -H 'Content-Type: application/json' \
  -H 'X-Jute-Agent-ID: mock-a2a-agent' \
  -d '{"jsonrpc":"2.0","id":1,"method":"resources/list"}'
```

## Project Layout

```text
apps/hub/cmd/juted/             Go entrypoint for the local hub
apps/hub/internal/app/          Internal hub application layers (config, agents, homestate, mcp, voice, etc.)
apps/hub/internal/pkg/          Reusable hub packages (a2a client/mock, database, registry, displayactions, etc.)
apps/web/                       SvelteKit Display application (dashboard, chat sheet, settings)
themes/                         Theme Pack definitions (design tokens only)
widgets/                        Native dashboard widgets (Svelte components + Go providers)
config/                         Generic example local configuration
docs/                           Architecture notes, decisions (ADRs), and developer guidelines
examples/harnesses/             Self-contained local dev stacks with embedded fixtures
```

## Widget Development

Custom widgets are contributed as Widget Packs: static browser content plus a `widget.json` manifest. The manifest declares identity, permissions, data needs, supported sizes, and optional Widget Skills for agent/MCP exposure.

Start here:

- [Widgets Architecture](docs/architecture/widgets.md)
- [Widget Skills](docs/architecture/widget-skills.md)
- [Widget Developer Guidelines](docs/developer/widget-guidelines.md)
- [Widget Pack Template](docs/developer/widget-pack-template.md)
- [Theme Developer Guidelines](docs/developer/theme-guidelines.md)

Widgets must render inside `WidgetFrame`, communicate through the Widget SDK message protocol, and never call the hub API, MCP, A2A agents, camera, microphone, filesystem, or raw network directly.

Themes are contributed as data-only Theme Packs. They can change UI tokens, backgrounds, and widget chrome, but not widget behavior or agent-facing capabilities.

## Current API Surface

- `GET /healthz`
- `GET /api/v1/status`
- `GET /api/v1/config`
- `GET /api/v1/home`
- `GET /api/v1/agents`
- `POST /api/v1/agents`
- `PATCH /api/v1/agents/{id}`
- `DELETE /api/v1/agents/{id}`
- `POST /api/v1/agents/{id}/refresh-card`
- `GET /api/v1/setup/status`
- `GET /api/v1/settings/household`
- `PATCH /api/v1/settings/household`
- `GET /api/v1/settings/rooms`
- `PUT /api/v1/settings/rooms`
- `GET /api/v1/settings/tiles`
- `PUT /api/v1/settings/tiles`
- `GET /api/v1/widgets/catalog`
- `GET /api/v1/widgets/layout`
- `PUT /api/v1/widgets/layout`
- `POST /api/v1/widgets/layout/reset`
- `GET /api/v1/voice/status`
- `POST /api/v1/voice/mute`
- `POST /api/v1/voice/unmute`
- `POST /api/v1/voice/cancel`
- `GET /api/v1/voice/providers`
- `POST /api/v1/messages`
- `GET /api/v1/conversations`
- `POST /api/v1/conversations`
- `GET /api/v1/conversations/{id}`
- `POST /api/v1/conversations/{id}/turns`
- `POST /api/v1/conversations/{id}/turns/stream`
- `GET /api/v1/events`

The optional MCP bridge is a separate local surface, not part of `/api/v1`. When enabled, it exposes Widget Skills and dashboard context through MCP Streamable HTTP at the configured path, defaulting to `/mcp`.

`POST /api/v1/conversations/{id}/turns` is the primary chat path. It uses A2A `SendStreamingMessage` when the selected Agent Card supports streaming, falls back to blocking `SendMessage`, and reads conversation history back from the selected agent with A2A `ListTasks` and `GetTask` when that agent supports task history. Jute does not store conversation transcripts locally in this pre-v1 slice; unsupported agents show a clear history-unavailable state.

`POST /api/v1/messages` remains as a blocking compatibility endpoint for simple smoke tests.

## A2A Assumptions

The project tracks A2A as an external protocol rather than inventing a custom agent API. The current design assumes:

- agents publish an Agent Card, usually at `/.well-known/agent-card.json`;
- Jute acts as an A2A client and local orchestrator;
- standard A2A bindings are selected in this order: `JSONRPC`, `HTTP+JSON`, then `GRPC`;
- Jute dashboard context uses an optional A2A extension instead of a custom protocol binding;
- secrets stay outside public config and are referenced through environment variables or a future OS keyring integration.

Loopback Agent Card URLs are allowed by default for local development. Remote Agent Card discovery must be explicitly allowed with `a2a.allowed-agent-card-urls`, using exact Agent Card URLs or a leading host wildcard such as `https://*.agents.example.com/.well-known/agent-card.json`.

## Configuration Direction

Runtime settings generally live in SQLite. YAML config is the preferred human-authored bootstrap/import/export format, and JSON remains supported for machine-friendly compatibility. During the pre-v1 settings slice, household/display/weather, rooms, and dashboard tiles save to SQLite in store-backed mode and back to the active YAML config in harness mode. Configured agents are also saved back to the active YAML config so local users can add, disable, and remove A2A agents without editing SQLite directly. The hub owns durable settings, and public config responses are redacted projections.

`JUTE_HOME` is the planned data root. The runtime database defaults to `$JUTE_HOME/jute.db`, with Docker using `/data` and systemd using `/var/lib/jute`.

References:

- [A2A Specification](https://a2a-protocol.org/latest/specification/)
- [A2A Extensions](https://a2a-protocol.org/latest/topics/extensions/)
- [A2A Agent Discovery](https://a2a-protocol.org/latest/topics/agent-discovery/)

## Roadmap

See [docs/roadmap.md](docs/roadmap.md).

## License

Jute Dash is licensed under the [GNU Affero General Public License v3.0](LICENSE), identified as `AGPL-3.0-only`.

## Agent Guidance

- [AGENTS.md](AGENTS.md)
- [CLAUDE.md](CLAUDE.md)
