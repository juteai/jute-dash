# Widgets

## Strategy

Jute uses two widget classes:

- **First-party widgets:** native Svelte components shipped with the display app for core experiences such as clock, weather, rooms, energy, camera, media, calendar, and agent status.
- **Widget Packs:** custom third-party widgets loaded from a `widget.json` manifest, rendered in a sandboxed iframe by default, and connected to Jute through a typed postMessage SDK.

This gives first-party widgets polish and performance while keeping user-created widgets framework-independent and isolated.

All widgets are visually hosted in the dashboard `WidgetFrame` specified in [Display UX](display-ux.md). The current POC dashboard styling is not canonical.

## Widget Pack Layout

A Widget Pack is a directory, archive, or URL with this minimum shape:

```text
my-widget/
  widget.json
  index.html
  assets/
```

The manifest is the contract between the widget and the hub:

```json
{
  "id": "com.example.energy-price",
  "name": "Energy Price",
  "version": "1.0.0",
  "entry": "index.html",
  "permissions": ["home:read", "network:fetch"],
  "dataNeeds": ["energy.current_tariff", "home.locale"],
  "contextPolicy": {
    "exposeToAgents": true,
    "publicFields": ["tariffName", "currentPrice", "nextCheapWindow"]
  },
  "sizes": ["small", "medium", "wide"]
}
```

Required fields are `id`, `name`, `version`, `entry`, `permissions`, `dataNeeds`, `contextPolicy`, and `sizes`.

## Runtime Model

- The hub installs and validates Widget Packs.
- The display renders each custom widget in an iframe with `sandbox` restrictions.
- Widget iframe origin is isolated from the display app where possible.
- Widgets receive only the data allowed by their manifest and user-granted permissions.
- Widgets cannot call the hub API directly. They communicate through the display host using the Widget SDK message protocol.
- The display forwards approved widget requests to the hub.

## Widget SDK Messages

All messages include `type`, `widgetId`, `requestId`, and `payload`.

Widget to host:

- `jute.widget.ready`
- `jute.widget.resize`
- `jute.widget.request_data`
- `jute.widget.update_state`
- `jute.widget.emit_action`
- `jute.widget.open_settings`

Host to widget:

- `jute.host.theme`
- `jute.host.data`
- `jute.host.visibility`
- `jute.host.permissions`
- `jute.host.error`

The SDK should be small TypeScript package materialized later under `packages/widget-sdk`.

## Permissions

Widget permissions are explicit and user-visible.

Initial permissions:

- `home:read`: read normalized non-sensitive home state.
- `widget:state`: persist widget-specific state.
- `agent:context`: allow public widget context to be included in A2A dashboard context.
- `network:fetch`: request hub-mediated fetch to approved origins.
- `media:display`: display images or video streams approved by the hub.

No widget receives broad filesystem, raw network, microphone, camera, or secret access in v1.

## Agent Context

Widgets may contribute context to agents only through `contextPolicy.publicFields`. The hub builds agent context from:

- visible widgets;
- focused widget, if any;
- widget title, type, size, and layout location;
- public widget fields declared in the manifest;
- locale, timezone, display profile, and interaction mode.

Hidden widgets, private widget state, secrets, raw auth data, and undeclared fields are never sent to agents.

The same policy applies to the [MCP Bridge](mcp-bridge.md). MCP resources and tools expose only hub-approved public widget context. Widgets do not call MCP directly, and agents do not connect directly to widget iframes.

MCP tool descriptions are hub-authored only. Widget Pack names, descriptions, and manifest text must not become trusted MCP tool instructions.

## Built-In Widgets

First-party widgets use the same conceptual contract as Widget Packs, but they can be native Svelte components. They still declare:

- stable widget ID;
- supported sizes;
- data needs;
- settings schema;
- agent context policy.

This keeps built-in and custom widgets understandable through the same mental model.

Initial built-in widgets:

- `date-time`
- `weather`
- `chat-history`

These first widgets should be implemented inside the standard `WidgetFrame` and persisted through the hub layout model.

## Developer Guidelines

Widget authors should start with [Widget Developer Guidelines](../developer/widget-guidelines.md).
