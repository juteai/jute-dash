# Contributing to Jute Dash

Thank you for investing your time in contributing to Jute Dash! This guide covers the development setup, tools, and technical specifications for developers working on the repository.

---

## Development Setup

To build and run Jute Dash locally, you need the following system dependencies:

- **Go**: 1.22+ (for the Go hub)
- **Node.js & npm**: (for SvelteKit display app)
- **pre-commit**: (for code check hooks)

### Homebrew (macOS)

If you use macOS and Homebrew, you can install all required dependencies (Go, npm/node, pre-commit, golangci-lint, goreleaser) at once using the included `Brewfile`:

```bash
make setup
```

This target runs `brew bundle install` followed by `npm install` inside the web app directory and installs the pre-commit git hooks.

### Manual Setup (Non-macOS / Non-Homebrew)

If you do not use macOS or Homebrew, please install the tools manually, then run:

```bash
# Setup web app dependencies
cd apps/web && npm install

# Install pre-commit hooks
make pre-commit-install
```

---

## Root Makefile Targets

The root `Makefile` defines targets for setting up the environment, code validation, linting, and clearing temporary stores. It does *not* run example stacks (run those directly inside their harness directories).

| Target | Description |
|--------|-------------|
| `make setup` | Runs `brew bundle install` (if available), installs web app node packages, and configures pre-commit hooks. |
| `make pre-commit-install` | Installs the pre-commit git hooks. |
| `make test` | Runs the hub backend unit tests (`go test ./...`). |
| `make lint` | Runs the hub backend `golangci-lint` runner. |
| `make web-check` | Runs Svelte compiler and TypeScript checks (`npm run check` in `apps/web`). |
| `make web-lint` | Runs ESLint and formatting checks for the frontend. |
| `make web-test` | Runs the frontend test runner. |
| `make check` | Runs all checks: Go tests, Go linting, frontend checks, tests, and build. |
| `make reset` | Clears all local development `.jute` stores and databases. |

Before you push or open a PR, make sure to run:

```bash
pre-commit run --all-files
make check
```

---

## Development Harnesses (Running Examples)

To run the Jute Dash stack against A2A mock or model fixtures, navigate to the harness directories:

- **Mock Agent (MCP Enabled by Default)**: Fast, deterministic chat.
  ```bash
  cd examples/harnesses/mock-a2a
  make run
  ```
- **Kronk Local Agent (MCP Enabled by Default)**: Backed by a local model.
  ```bash
  cd examples/harnesses/kronk-a2a
  make run
  ```

For more info, see the [Harnesses README](examples/harnesses/README.md).

---

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

---

## Widget & Theme Development

Custom widgets are Svelte components located in the `widgets/` directory. Each widget registers its metadata and capabilities (Widget Skills) in Go.

- [Widgets Architecture](docs/architecture/widgets.md)
- [Widget Skills](docs/architecture/widget-skills.md)
- [Widget Developer Guidelines](docs/developer/widget-guidelines.md)
- [Theme Developer Guidelines](docs/developer/theme-guidelines.md)

Widgets must render inside a standard `WidgetFrame` and use the Widget SDK protocol. They do not have direct network, filesystem, or database access.

---

## Hub API Surface

The hub exposes the following HTTP routes at `http://127.0.0.1:8787`:

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

The local MCP Bridge serves Streamable HTTP requests at `/mcp` (typically `http://127.0.0.1:8790/mcp`) when enabled.

---

## Technical Architecture & Protocols

### A2A Assumptions
Jute Dash acts as an A2A client, consuming remote Agent Cards to discover capabilities and route messages.
- Standard protocol bindings: `JSONRPC`, `HTTP+JSON`, then `GRPC`.
- Dashboard context uses the extension `https://jute.dev/a2a/extensions/dashboard-context/v1` when supported.
- More details: [A2A Compatibility](docs/architecture/a2a.md).

### Configuration & Persistence
- Configuration bootstrap uses YAML or JSON.
- Runtime changes (rooms, widgets, dashboard layout, household settings) persist in a local SQLite database at `jute.db`.
- Secret tokens remain inside environment variables and are never saved to SQLite.
- More details: [Configuration and Persistence](docs/architecture/configuration-persistence.md).

---

## Coding Guidelines (Agents)

If you are an AI coding agent making changes to this codebase, you must review the developer guidelines in:
- [AGENTS.md](AGENTS.md)
- [CLAUDE.md](CLAUDE.md)
