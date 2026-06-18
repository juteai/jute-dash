# Distribution

## Release Targets

Jute ships as several artifacts built from the same hub and display foundation:

- **Hub binary:** Go binary for API, persistence, and local services.
- **Docker image:** multi-arch image for home servers, NAS devices, and development.
- **PWA/kiosk web app:** browser install target deployed separately from the hub.
- **Raspberry Pi/systemd package:** install script and service unit for wall display and headless nodes.
- **Desktop wrapper:** later Tauri wrapper around the display UI for macOS, Windows, and Linux.

The standalone hub binary is the primary v1 distribution target.

## Build Strategy

- Build SvelteKit as static client assets.
- Serve the display app separately from the hub.

## CI/CD

Use GitHub Actions for the default pipeline:

- run Go tests;
- run Svelte type checks and build;
- run Playwright smoke tests against the local hub and display;
- build standalone Go binaries for Linux, macOS, and Windows;
- build Docker images for `linux/amd64` and `linux/arm64`;
- generate checksums, SBOMs, and signed release artifacts;
- publish releases through GoReleaser.

Docker builds use `buildx` so Raspberry Pi and home-server deployments share the same release flow.

## Platform Matrix

Initial supported platforms:

- Linux amd64
- Linux arm64
- macOS arm64
- macOS amd64
- Windows amd64
- Docker linux/amd64
- Docker linux/arm64

Raspberry Pi support targets 64-bit Raspberry Pi OS first.

## Installation Modes

### Local Development

Run the local stack for fast iteration:

```sh
cd examples/config/local
make run
```

The local stack serves the development display at `https://localhost:5173` for browser APIs that require a secure context. Spotify OAuth uses the hub callback `http://127.0.0.1:8787/api/v1/integrations/spotify/callback` because Spotify requires explicit loopback IP redirect URIs for local HTTP callbacks and rejects `localhost`. Plain HTTP remains available through `make run-http` from `examples/config/local` for non-OAuth UI testing.

### Hub Binary

Run one binary that serves the hub API. The display remains a separate client
deployment and is not embedded in the hub binary:

```sh
juted --config /etc/jute/config.yaml
```

Once SQLite persistence exists, `--config` bootstraps an empty runtime store. Runtime settings then live in the data directory.

### Docker

Run the hub API container with config and data mounted:

```sh
docker run --rm \
  -p 8787:8787 \
  -e JUTE_HOME=/data \
  -e JUTE_CONFIG=/config/config.yaml \
  -e JUTE_LISTEN=0.0.0.0:8787 \
  -v "$PWD/config/config.yaml:/config/config.yaml:ro" \
  -v "$PWD/data:/data" \
  ghcr.io/juteai/jute-dash:latest
```

For Compose-based installs, use `examples/compose/docker-compose.yml` as the
starting point. The Compose example mounts `./config/config.yaml` into
`/config/config.yaml` and persists runtime SQLite state under `/data`.

Docker runtime defaults:

- `JUTE_HOME=/data`
- `JUTE_CONFIG=/config/config.yaml`
- `JUTE_LISTEN=0.0.0.0:8787`

The mounted YAML/JSON file is a bootstrap/import source. On first run, the hub
creates `/data/jute.db`, applies the bootstrap config, and then treats SQLite as
runtime truth. Secrets must remain environment variable references, not literal
values in the mounted config.

### systemd

The package installs:

- `/usr/local/bin/juted`
- `/etc/jute/config.yaml`
- `/var/lib/jute/jute.db`
- `jute.service`

The service runs as a dedicated low-privilege user.

## Runtime Data

`JUTE_HOME` is the primary data root.

Default runtime locations:

- local app: platform-specific user data directory;
- Docker: `/data`;
- systemd: `/var/lib/jute`.

The runtime database defaults to `$JUTE_HOME/jute.db`. YAML/JSON config remains bootstrap/import/export and should not be treated as the live source of truth after the database exists.

## Versioning

Use semantic versioning for Jute releases.

- Patch releases fix bugs and security issues.
- Minor releases add compatible widget, API, adapter, or A2A behavior.
- Major releases may change persisted data formats or public extension contracts.

Widget SDK and A2A dashboard-context extension versions are versioned independently.
