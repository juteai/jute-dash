# Configuration And Persistence

## Goal

Jute should be easy to run out of the box without hand-editing files, but advanced users still need portable configuration, mounted config files, environment overrides, and future import/export.

The decision:

- YAML is the preferred bootstrap, import, and export format.
- JSON remains supported for machine-friendly compatibility.
- SQLite is runtime truth.
- Secrets are references only.
- The hub owns all durable settings.
- Browser state is transient only.

## Storage Stack

Use SQLite for the runtime store. The Go driver is [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) because it is a CGo-free `database/sql` driver and supports the multi-platform binary path.

Use SQLite WAL mode for normal runtime. [SQLite WAL](https://www.sqlite.org/wal.html) allows readers and writers to proceed concurrently, which fits Jute's shape: the hub is the single writer, while displays, widgets, and local tools mostly read.

Runtime rules:

- open the database through an `internal/store` package;
- run migrations before serving APIs;
- enable WAL mode for normal local filesystems;
- do not place the runtime database on a network filesystem;
- keep one hub process as the writer for a household;
- expose settings only through hub APIs.

## Configuration Layers

Effective configuration is built in this order:

1. compiled safe defaults;
2. boot-only CLI and environment overrides;
3. optional bootstrap YAML or JSON, applied only when initializing an empty runtime store;
4. SQLite household settings;
5. SQLite device profile overrides;
6. transient browser state for non-durable UI only.

Boot-only values include:

- config path;
- data directory;
- listen address;
- headless mode;
- development asset overrides;
- log level.

Boot-only values are not treated as ordinary persisted settings unless the user explicitly saves them through a future settings flow.

## Local Paths

`JUTE_HOME` is the primary local data root.

Default data roots:

- macOS: `~/Library/Application Support/Jute Dash`
- Linux: `$XDG_DATA_HOME/jute-dash` or `~/.local/share/jute-dash`
- Windows: `%AppData%/Jute Dash`
- Docker: `/data`
- systemd: `/var/lib/jute`

Default runtime database:

```text
$JUTE_HOME/jute.db
```

Default bootstrap config:

- development: `config/jute.example.yaml`;
- Docker: `/config/config.yaml`;
- systemd: `/etc/jute/config.yaml`;
- ordinary local install: optional.

No bootstrap config is required for first run.

## First Run

When no runtime database exists, the hub:

1. creates the data directory;
2. creates `jute.db`;
3. runs all migrations;
4. inserts safe defaults;
5. applies bootstrap YAML or JSON if a bootstrap file was provided;
6. exposes setup status through the local API.

The first-run UI should collect only the minimum useful settings:

- home name;
- locale and timezone;
- weather enabled state and lat/lon location;
- default display profile;
- optional first A2A agent;
- optional voice provider choices when voice is enabled.

Production first-run should not include fake remote agents or call remote services until the user configures them. The existing example config can stay richer for development demos.

## Runtime Store Model

Jute uses `internal/store` as the runtime persistence package, built on `database/sql` plus `modernc.org/sqlite`.

The store owns:

- database opening and pragmas;
- migrations;
- transactions;
- backup/export helpers;
- setting reads and writes;
- redacted public projections.

Add an app-managed migration table:

```sql
CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at TEXT NOT NULL
);
```

Use normalized tables for stable product concepts, with JSON columns only for flexible non-secret settings.

Initial table families:

- `household_settings`
- `device_profiles`
- `layout_profiles`
- `rooms`
- `tiles`
- `agents`
- `agent_card_cache`
- `widget_packs`
- `widget_instances`
- `widget_permissions`
- `voice_provider_packs`
- `voice_settings`
- `mcp_settings`
- `agent_mcp_scopes`
- `adapter_connections`
- `setting_audit_log`

Provider manifests and widget manifests are validated records. They are not executable code and are not durable sources of truth outside the hub store.

Use stable string IDs for user-facing records so import/export and future sync can preserve identity.

Use UTC ISO-8601 strings for public API timestamps.

## Setting Classification

Before adding a new setting, classify it as one of:

- `boot-only`: read from CLI/env at process start and not modified by UI.
- `household durable`: shared setting stored in SQLite.
- `device-profile durable`: per-display or per-headless-node override stored in SQLite.
- `install record`: widget pack, voice provider pack, adapter, or agent registration metadata.
- `cache`: refreshable data such as Agent Cards, health checks, provider status, and weather snapshots.
- `secret reference`: environment variable name, keyring key, or OAuth credential reference.
- `transient UI state`: open menus, drag state, local focus state, and unsaved form edits.

Do not store durable settings only in browser local storage.

## YAML Bootstrap Format

YAML is the recommended format for hand-authored Jute configuration because it is easier to scan and maintain for homes, agents, widgets, layouts, and future setup records.

Rules:

- `.yaml` and `.yml` are preferred for local bootstrap config;
- `.json` remains accepted for generated config, tests, and compatibility;
- YAML uses kebab-case keys such as `listen-address`, `card-url`, and `wind-speed-unit`;
- YAML decoding is strict and rejects unknown fields;
- YAML is not watched or automatically reloaded in v1.

Jute intentionally does not copy Glance's live config reload model in v1. Runtime edits happen through the hub and persist to SQLite. Editing a YAML file after `jute.db` exists does not silently overwrite runtime settings; use a future explicit import flow for that.

## Secrets

Secrets are never stored directly in YAML, JSON, or ordinary SQLite settings rows.

v1 secret references:

- environment variable names;
- local token file references outside repo paths when explicitly configured;
- auth configured booleans in public projections.

Future secret storage:

- OS keyring;
- OAuth device flow records;
- encrypted backup/export support.

Public APIs may expose whether a secret is configured. They must not expose raw values.

## Import And Export

YAML export is the preferred human-readable portable projection of runtime settings. JSON export remains available for machine-readable workflows.

Export includes:

- household settings;
- device profiles when selected;
- layouts;
- rooms and tiles;
- agents without raw credentials;
- widget installation records and permissions;
- voice provider pack records and non-secret settings;
- MCP bridge settings and per-agent scopes without raw tokens;
- adapter connection records without raw credentials.

Export excludes by default:

- raw secrets;
- raw transcripts;
- raw audio;
- TTS cache contents;
- Agent Card cache;
- provider health cache;
- logs;
- volatile runtime state.

Import writes through the same validation path as setup/settings APIs. Imports should report conflicts before overwriting existing records.

## APIs

Keep:

- `GET /api/v1/config`: redacted effective public config view.

Setup and settings APIs:

- `GET /api/v1/setup/status`

Future setup and settings APIs:

- `POST /api/v1/setup/complete`
- `GET /api/v1/settings/household`
- `PATCH /api/v1/settings/household`
- `GET /api/v1/devices`
- `GET /api/v1/devices/{id}/settings`
- `PATCH /api/v1/devices/{id}/settings`
- `POST /api/v1/config/import`
- `GET /api/v1/config/export`

Future MCP settings APIs:

- `GET /api/v1/mcp/settings`
- `PATCH /api/v1/mcp/settings`
- `GET /api/v1/agents/{id}/mcp-scopes`
- `PATCH /api/v1/agents/{id}/mcp-scopes`

Settings writes go through the hub, update SQLite, and emit relevant events.

Settings-related events:

- `settings.changed`
- `device_profile.changed`
- `home.updated`
- `widget.updated`
- `agent.card_updated`
- `voice.provider_health_changed`

## CLI

Future CLI behavior:

- `juted init`: create the data directory, database, and optional bootstrap config.
- `juted --data-dir`: override runtime data path.
- `juted --config`: provide YAML or JSON bootstrap/import config.
- `juted --listen`: boot-only listen override.
- `juted --headless`: start without serving the display.

The current `--config` behavior is provisional. Once SQLite persistence exists, `--config` is bootstrap input for an empty store unless an explicit import command is used.

## Backup And Recovery

Use SQLite-aware backup/export paths instead of copying only `jute.db` while the hub is running. WAL mode can create `jute.db-wal` and `jute.db-shm` companion files.

Backup options:

- YAML export for portable, secret-free human configuration;
- JSON export for machine-readable compatibility;
- SQLite backup API for full local backup;
- stopped-service file copy for advanced users;
- future encrypted export for secrets and sensitive history.

Recovery flow:

1. start from safe defaults if no backup exists;
2. restore SQLite backup for full local state;
3. import YAML or JSON for portable settings;
4. reattach secrets through environment variables or future keyring setup.

## Development Defaults

Development keeps using `config/jute.example.yaml` for a rich demo.

Production defaults should be quieter:

- no fake agent endpoints;
- weather enabled only when a location is configured or accepted in setup;
- loopback listen address;
- no cloud STT/TTS opt-in;
- no command providers enabled;
- no remote access.
