# Widgets

## Strategy

Jute uses two widget classes:

- **First-party widgets:** native Svelte components shipped with the display app for core experiences such as clock, weather, rooms, energy, camera, media, calendar, and agent status.
- **Widget Packs:** custom third-party widgets loaded from a `widget.json` manifest, rendered in a sandboxed iframe by default, and connected to Jute through a typed postMessage SDK.

This gives first-party widgets polish and performance while keeping user-created widgets framework-independent and isolated.

All widgets are visually hosted in the dashboard `WidgetFrame` specified in [Display UX](display-ux.md). The current POC dashboard styling is not canonical.

Widgets also declare agent-facing capabilities through [Widget Skills](widget-skills.md). A Widget Skill describes what an agent may read, which prompts are available, and which hub-mediated actions the widget can safely perform. The hub uses this contract to expose widget capabilities through A2A dashboard context and the optional MCP Bridge.

## Contract Layers

The widget system has four contracts. Implementations should keep these separate:

- **Install contract:** `widget.json` describes identity, entrypoint, permissions, data needs, supported sizes, and optional Widget Skill capabilities.
- **Frame contract:** every widget renders inside `WidgetFrame` and obeys the dashboard layout, sizing, focus, and error-state rules from [Display UX](display-ux.md).
- **Runtime contract:** widgets communicate with Jute through the Widget SDK message protocol. Widget Packs never call the hub API directly.
- **Agent contract:** widgets expose agent-facing context, prompts, and actions through Widget Skills. Widgets never call MCP directly.

This separation lets contributors build widgets without coupling them to the Svelte app internals, Go hub internals, or any single agent implementation.

## Widget Pack Layout

A Widget Pack is a directory, archive, or URL with this minimum shape:

```text
my-widget/
  widget.json
  index.html
  assets/
```

Recommended layout for contributed widgets:

```text
my-widget/
  widget.json
  index.html
  README.md
  assets/
  src/
  tests/
```

Only `widget.json` and the declared `entry` file are required. `README.md`, `src`, and `tests` are strongly recommended for contribution review.

## Manifest Contract

The manifest is the contract between the widget and the hub:

```json
{
  "id": "com.example.energy-price",
  "name": "Energy Price",
  "version": "1.0.0",
  "entry": "index.html",
  "permissions": ["home:read", "network:fetch", "agent:skill"],
  "dataNeeds": ["energy.current_tariff", "home.locale"],
  "agentSkill": {
    "enabled": true,
    "skillId": "com.example.energy-price.current",
    "summary": "Read current energy tariff and identify cheaper upcoming windows.",
    "requiredPermissions": ["agent:skill", "home:read"],
    "visibilityPolicy": "visible_or_focused",
    "context": {
      "fields": [
        { "name": "tariffName", "type": "string" },
        { "name": "currentPrice", "type": "number", "unit": "GBP/kWh" },
        { "name": "nextCheapWindow", "type": "string" }
      ]
    },
    "actions": [
      {
        "id": "refresh",
        "title": "Refresh tariff data",
        "sideEffect": "read",
        "requiresConfirmation": false
      }
    ],
    "prompts": [
      {
        "id": "energy_usage_advice",
        "title": "Energy usage advice"
      }
    ]
  },
  "sizes": ["small", "medium", "wide"]
}
```

Required fields are `id`, `name`, `version`, `entry`, `permissions`, `dataNeeds`, and `sizes`.

`agentSkill` is optional for visual-only widgets. It is required for any widget that wants to expose context, prompts, or actions to agents through A2A dashboard context or MCP.

Manifest field rules:

- `id`: stable reverse-DNS ID. It must not change after publication.
- `name`: short display name.
- `version`: semantic version.
- `entry`: relative path inside the Widget Pack.
- `permissions`: complete list of privileged capabilities.
- `dataNeeds`: hub data topics consumed by the widget.
- `sizes`: supported size names, at least one of `small`, `medium`, `wide`, or `large`.
- `agentSkill`: Widget Skill declaration, if the widget is agent-visible.

The hub must reject manifests with unknown top-level fields, missing required fields, duplicate IDs, invalid entry paths, unsupported permissions, unsupported sizes, invalid `agentSkill` declarations, or raw secrets.

## Runtime Model

- The hub installs and validates Widget Packs.
- The display renders each custom widget in an iframe with `sandbox` restrictions.
- Widget iframe origin is isolated from the display app where possible.
- Widgets receive only the data allowed by their manifest and user-granted permissions.
- Widgets cannot call the hub API directly. They communicate through the display host using the Widget SDK message protocol.
- The display forwards approved widget requests to the hub.

## Validation And Installation

Widget Pack validation happens before installation and again when the widget is enabled.

The hub validates:

- manifest schema and unknown fields;
- stable reverse-DNS widget ID;
- semantic version string;
- entry path containment within the pack;
- declared permissions and data needs;
- supported sizes and minimum layout requirements;
- Widget Skill context fields, action schemas, side-effect levels, and prompt declarations;
- absence of raw secrets, raw credentials, absolute local paths, and executable native hooks.

Validation failure is not fatal to the whole dashboard. The rejected widget is marked unavailable, the user receives a safe install/configuration error, and no Widget Skill or MCP capability is exposed for that widget.

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

SDK message rules:

- `widgetId` must match the installed widget instance.
- `requestId` must be unique per request and echoed by the host response when applicable.
- Widgets must tolerate permission and visibility changes at runtime.
- Widgets must stop timers, polling, and media work when hidden unless a future background policy allows it.
- The host may reject any request that does not match the manifest, current permission set, or current visibility policy.

## Permissions

Widget permissions are explicit and user-visible.

Initial permissions:

- `home:read`: read normalized non-sensitive home state.
- `widget:state`: persist widget-specific state.
- `agent:skill`: allow the widget to expose hub-approved skills, context, prompts, and actions to agents.
- `network:fetch`: request hub-mediated fetch to approved origins.
- `media:display`: display images or video streams approved by the hub.

No widget receives broad filesystem, raw network, microphone, camera, or secret access in v1.

Permission requests should be minimal. A widget that only renders hub-provided weather, date/time, or local display information should not request `network:fetch` or `media:display`.

## Widget Skills And Agent Context

Widgets may contribute context to agents only through `agentSkill.context.fields`. The hub builds agent context from:

- visible widgets;
- focused widget, if any;
- widget title, type, size, and layout location;
- public skill context fields declared in the manifest or built-in skill definition;
- locale, timezone, display profile, and interaction mode.

Hidden widgets, private widget state, secrets, raw auth data, and undeclared fields are never sent to agents.

The same policy applies to the [MCP Bridge](mcp-bridge.md). MCP resources and tools expose only hub-approved Widget Skills. Widgets do not call MCP directly, and agents do not connect directly to widget iframes.

MCP tool descriptions are hub-authored only. Widget Pack names, descriptions, and manifest text must not become trusted MCP tool instructions. Widget-owned operations are exposed as declared skill actions and invoked through the hub.

## Built-In Widgets

First-party widgets use the same conceptual contract as Widget Packs, but they can be native Svelte components. They still declare:

- stable widget ID;
- supported sizes;
- data needs;
- settings schema;
- Widget Skill context, prompts, and actions.

This keeps built-in and custom widgets understandable through the same mental model.

Initial built-in widgets:

- `date-time`
- `weather`
- `chat-history`

These first widgets should be implemented inside the standard `WidgetFrame` and persisted through the hub layout model.

Initial built-in Widget Skills:

- `jute.date_time.current`
- `jute.weather.current`
- `jute.chat_history.current`

Built-in widget metadata should be exposed through the same catalog/skill registry surfaces as Widget Packs so tests, MCP, and future settings UI do not need special cases.

## Contribution Model

Contributors can add widgets in three ways:

- **Built-in widget:** for broadly useful, license-compatible widgets that should ship with Jute and can be maintained in the core app.
- **Widget Pack example:** for sample integrations, experiments, and reference implementations that should stay isolated from core runtime dependencies.
- **Documentation recipe:** for third-party widgets hosted elsewhere.

Widget contributions should include:

- `widget.json` manifest or built-in manifest-equivalent metadata;
- README explaining data needs, permissions, privacy behavior, and supported platforms;
- screenshots or a short visual description of each supported size;
- Widget Skill context/action/prompt documentation when `agentSkill` is enabled;
- tests or a manual verification checklist;
- license statement compatible with the project.

Contributed widgets must not add new runtime dependencies to the hub or display unless the dependency is justified for the core product. Widget Packs should bundle their own static assets and stay sandbox-compatible.

## Developer Guidelines

Widget authors should start with [Widget Developer Guidelines](../developer/widget-guidelines.md).

For a copyable starting point, use [Widget Pack Template](../developer/widget-pack-template.md).
