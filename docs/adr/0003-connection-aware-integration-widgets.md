# ADR 0003: Connection-Aware Integration Widgets

## Status

Accepted

## Context

The first modular widget branch split music and smart-home behavior into provider-specific widgets, but credentials, action routes, runtime payloads, and setup flows were still drifting back into per-widget settings and ad hoc endpoints.

Jute needs widgets to stay easy to contribute while keeping shared runtime concerns inside the hub. A Spotify widget, Apple Music widget, Philips Hue widget, or Zigbee2MQTT widget should be self-contained for provider behavior, display, docs, and tests, but it should not own secret storage, connection health policy, action dispatch policy, or public error mapping.

## Decision

Use connection-aware Integration Widgets.

- Widgets remain self-contained contributions under `widgets/`.
- Integration Widgets declare required Adapter Connections in their catalog metadata.
- Widget instances store `connectionRefs: Record<string,string>` separately from non-secret `settings`.
- Adapter Connections are household durable SQLite state and can be shared by many widget instances.
- Raw credentials are never stored in widget settings. Connection records store secret references only; resolved secret material exists only inside the hub process.
- The hub resolves connection references into adapter-scoped material before invoking connection-aware widget runtime code.
- All hydrated widget data uses one normalized runtime payload: `{ status, issue?, updatedAt?, data? }`.
- Widget actions execute through `POST /api/v1/widgets/{widgetInstanceId}/actions/{actionId}`.
- `sideEffect` and `requiresConfirmation` in Widget Skill actions are authoritative. Actions marked `requiresConfirmation` require confirmation for every actor.

## Consequences

- The display no longer needs provider-specific setup or action routes.
- Widget settings sheets choose shared connections; Settings `Connections` owns setup/linking records.
- Agent, MCP, and display paths can share the same hub action policy instead of bypassing each other.
- Existing development data from the partial modular-widget branch may be reset or migrated; this branch is not preserving the old raw widget credential shape.
- Provider widgets must map internal adapter failures to safe user-facing issues, never raw errors.
