# Hub Developer Guidelines

The hub follows a CLEAN-style layout. Keep dependencies pointing inward and keep generated HTTP DTOs at the controller boundary.

## Layers

- `apps/hub/api/hub/v1`: OpenAPI contract, codegen config, and generated Echo server types.
- `apps/hub/internal/app`: layer namespace only. Do not put Go files directly in this directory.
- `apps/hub/internal/pkg/app`: executable hub wiring. It wires config, repositories, services, controllers, event streams, and runtime dependencies.
- `apps/hub/internal/app/config`: config defaults, loading, import/export validation, and bootstrap mapping.
- `apps/hub/internal/app/model`: shared domain models that do not import HTTP, Echo, GORM, SQLite, or generated OpenAPI packages.
- `apps/hub/internal/app/repository`: SQLite lifecycle, migrations, persistence implementations, secret storage, and provider manifest persistence.
- `apps/hub/internal/app/service`: business workflows such as agents, dashboard, home/settings, widgets, integrations, voice, STT, TTS, and wake-word runtime orchestration.
- `apps/hub/internal/app/controller`: Echo/HTTP handlers, SSE broker, generated API implementation, request/response mapping, and HTTP error handling.
- `apps/hub/internal/pkg`: support plumbing, such as executable hub wiring, A2A transport helpers, database setup, request middleware, file sync, logging, paths, MCP transport helpers, registries, and display action event plumbing.
- `apps/hub/tests/mocks`: mockery-generated test doubles.
- `apps/hub/tests/integration`: black-box Ginkgo specs and bootstrap utilities for a running hub.

## Import Direction

Controllers may import services and models. Services may import repositories and models. Repositories may import models and persistence support. Models must remain dependency-light and must not import controller, service, repository, generated API packages, or GORM.

Generated OpenAPI DTOs must not leak into services or repositories. Map generated request/response types in controllers.

`internal/pkg` must not become a domain escape hatch. If code knows about voice policy, dashboard layout, OAuth, widget connection resolution, agent conversation state, or settings semantics, it belongs in `model`, `repository`, `service`, or `controller`.

## HTTP Contract

The hub REST API is contract-first. Update `apps/hub/api/hub/v1/openapi.yaml`, run `make codegen`, and implement the generated Echo interface in `internal/app/controller`.

The `/api/v1/events` SSE endpoint is part of the hub API contract and returns `text/event-stream`.

The MCP Bridge is intentionally excluded from the display OpenAPI contract because it is a separate trusted-agent surface.

The hub must not embed or serve the Svelte display bundle. The display is always a client of the hub API.

## Tests

Unit tests stay beside the package when they need package-private coverage.

Integration specs live under `apps/hub/tests/integration/specs`. They are black-box tests against an already running hub, configured with `JUTE_HUB_BASE_URL` and defaulting to `http://localhost:8787`.

Use:

```sh
make codegen
make generate-mocks
make test
make integration-test-local
```
