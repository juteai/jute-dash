# Widget Developer Guidelines

## Overview

Jute widgets are small dashboard experiences that can read approved home data, render glanceable UI, and optionally expose safe public context to A2A agents.

There are two widget types:

- first-party native Svelte widgets shipped with Jute;
- custom Widget Packs rendered in sandboxed iframes.

Custom widgets should be Widget Packs unless they are accepted into the core display app.

## Widget Pack Structure

Minimum structure:

```text
my-widget/
  widget.json
  index.html
  assets/
```

The widget entrypoint must be static browser content. It can be built from any framework as long as it speaks the Widget SDK message protocol.

## Manifest

`widget.json` is required:

```json
{
  "id": "com.example.energy-price",
  "name": "Energy Price",
  "version": "1.0.0",
  "entry": "index.html",
  "permissions": ["home:read", "widget:state"],
  "dataNeeds": ["energy.current_tariff", "home.locale"],
  "contextPolicy": {
    "exposeToAgents": true,
    "publicFields": ["tariffName", "currentPrice", "nextCheapWindow"]
  },
  "sizes": ["small", "medium", "wide"]
}
```

Manifest rules:

- `id` must be globally stable and reverse-DNS style.
- `version` must use semantic versioning.
- `entry` must point inside the Widget Pack.
- `permissions` must list every privileged capability the widget needs.
- `dataNeeds` must list the hub data topics the widget consumes.
- `contextPolicy.publicFields` must list exactly which widget fields may be shown to agents.
- `sizes` must include at least one supported size.

## Permissions

Initial permissions:

- `home:read`: read normalized non-sensitive home state.
- `widget:state`: store widget-specific state through the hub.
- `agent:context`: allow public context fields to be included in A2A dashboard context.
- `network:fetch`: ask the hub to fetch approved external URLs.
- `media:display`: render approved image or video sources.

Do not request permissions unless they are required for the widget's core job.

## Host Communication

Widgets do not call the hub API directly. Use the Widget SDK message protocol.

Every message includes:

```json
{
  "type": "jute.widget.ready",
  "widgetId": "com.example.energy-price",
  "requestId": "01HX...",
  "payload": {}
}
```

Widget to host messages:

- `jute.widget.ready`: widget has loaded.
- `jute.widget.resize`: widget requests a supported size.
- `jute.widget.request_data`: widget requests approved data topics.
- `jute.widget.update_state`: widget persists widget state.
- `jute.widget.emit_action`: widget asks the hub to perform an approved action.
- `jute.widget.open_settings`: widget asks Jute to open widget settings.

Host to widget messages:

- `jute.host.theme`: current theme tokens and motion preference.
- `jute.host.data`: approved data response.
- `jute.host.visibility`: visible, hidden, focused, or ambient status.
- `jute.host.permissions`: granted permission set.
- `jute.host.error`: rejected request or runtime error.

## Agent Context

Widgets can expose context to agents only when:

- the widget declares `agent:context`;
- `contextPolicy.exposeToAgents` is true;
- each exposed field is listed in `contextPolicy.publicFields`;
- the user has granted the widget's agent context permission;
- the widget is visible or focused according to the hub context policy.

Never expose:

- secrets;
- raw adapter payloads;
- private notes;
- exact presence or location data;
- camera frames or microphone audio;
- hidden widget state;
- browser local storage.

## UX Requirements

Widgets must:

- render inside the standard `WidgetFrame` defined by [Display UX](../architecture/display-ux.md);
- declare supported size names and minimum grid dimensions;
- fit all declared sizes without overflow;
- declare overflow behavior as `clip`, `scroll`, or `expand`;
- support keyboard focus where interactive;
- expose useful labels for screen readers;
- respect reduced motion and high-contrast settings;
- avoid flashing or rapidly changing ambient-mode UI;
- show a useful empty state when data is unavailable.

Widget frames provide:

- 1px border;
- 8px maximum border radius;
- size-based padding;
- optional header and actions;
- focus ring;
- edit-mode drag handle;
- edit-mode resize handle;
- empty, loading, error, and permission-required states.
- stale and unavailable states when hub data or dependencies are not fresh.

Edit-mode rules:

- drag and resize handles are visible only in edit mode;
- widget actions must remain reachable by keyboard;
- resize must snap to supported grid sizes;
- layout changes persist through the hub, not browser local storage.

## Failure Behavior

Widgets should degrade cleanly:

- use the standard widget states: `loading`, `empty`, `unavailable`, `error`, `permission_required`, and `stale`;
- show a compact inline state if data is unavailable;
- keep single-widget failures inside `WidgetFrame`;
- include a short title, one-line explanation, and optional retry or settings action;
- avoid repeated retry loops;
- tolerate missing optional data fields;
- stop timers or network requests when hidden;
- handle `jute.host.permissions` updates at runtime.

Widgets must not show raw stack traces, raw hub errors, raw adapter payloads, credential references, or private widget state. App-level banners are reserved for hub-level or cross-widget failures as defined in [Resilience And Error UX](../architecture/resilience-error-ux.md).
