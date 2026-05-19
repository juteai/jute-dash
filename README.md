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
- [Widgets](docs/architecture/widgets.md)
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

To run the dashboard against the local Kronk-backed A2A test agent:

```sh
make kronk-a2a-setup
make dev-a2a
```

`make dev-a2a` starts the Kronk A2A server, waits for its Agent Card, then starts the hub with `config/jute.dev-a2a.yaml` and the web UI. First run may take a while because Kronk can download model/runtime assets. Reset only that dev store with:

```sh
make dev-a2a-reset
```

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

Optional local agent examples:

```sh
make kronk-a2a-setup  # download dependencies for the Kronk A2A example module
make kronk-a2a        # run the local Kronk-backed A2A test agent
make kronk-a2a-server # run only the local A2A server for Jute integration tests
make kronk-a2a-check  # compile/test the isolated example module
```

## Project Layout

```text
cmd/juted/             Go entrypoint for the local hub
internal/a2a/          A2A protocol-facing types and constants
internal/config/       Config loading, defaults, validation, public config projection
internal/home/         Home dashboard state assembled from config
internal/registry/     Runtime view of configured agents
internal/server/       HTTP API surface consumed by the UI and future clients
internal/store/        SQLite runtime store, migrations, seeding, and setup status
internal/weather/      Open-Meteo weather client and weather state mapping
apps/web/              SvelteKit dashboard app, currently throwaway POC UI
config/                Example local configuration
docs/                  Architecture notes, roadmap, and decisions
examples/agents/       Optional local test agents and integration fixtures
```

## Current API Surface

- `GET /healthz`
- `GET /api/v1/config`
- `GET /api/v1/home`
- `GET /api/v1/agents`
- `GET /api/v1/setup/status`
- `POST /api/v1/messages`

`POST /api/v1/messages` sends a blocking JSON-RPC A2A turn for enabled `JSONRPC` agents. Streaming task events are still future work.

## A2A Assumptions

The project tracks A2A as an external protocol rather than inventing a custom agent API. The current design assumes:

- agents publish an Agent Card, usually at `/.well-known/agent-card.json`;
- Jute acts as an A2A client and local orchestrator;
- standard A2A bindings are selected in this order: `JSONRPC`, `HTTP+JSON`, then `GRPC`;
- Jute dashboard context uses an optional A2A extension instead of a custom protocol binding;
- secrets stay outside public config and are referenced through environment variables or a future OS keyring integration.

## Configuration Direction

Runtime settings live in SQLite. YAML config is the preferred human-authored bootstrap/import/export format, and JSON remains supported for machine-friendly compatibility. The hub owns durable settings, and public config responses are redacted projections.

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
