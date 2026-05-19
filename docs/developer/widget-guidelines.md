# Widget Developer Guidelines

## Overview

Jute widgets are small dashboard experiences that can read approved home data, render glanceable UI, and optionally expose safe capabilities to agents through Widget Skills.

There are two widget types:

- first-party native Svelte widgets shipped with Jute;
- custom Widget Packs rendered in sandboxed iframes.

Custom widgets should be Widget Packs unless they are accepted into the core display app.

Start new custom widgets from [Widget Pack Template](widget-pack-template.md).

## Widget Pack Structure

Minimum structure:

```text
my-widget/
  widget.json
  index.html
assets/
```

Recommended contribution structure:

```text
my-widget/
  widget.json
  index.html
  README.md
  assets/
  src/
  tests/
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
  "permissions": ["home:read", "widget:state", "agent:skill"],
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

Manifest rules:

- `id` must be globally stable and reverse-DNS style.
- `version` must use semantic versioning.
- `entry` must point inside the Widget Pack.
- `permissions` must list every privileged capability the widget needs.
- `dataNeeds` must list the hub data topics the widget consumes.
- `agentSkill` is optional for visual-only widgets and required for agent-visible widgets.
- `agentSkill.context.fields` must list exactly which widget fields may be shown to agents.
- `agentSkill.actions` must list every operation an agent can ask the hub to perform for the widget.
- `agentSkill.prompts` must list prompt purposes, not raw trusted instructions.
- `sizes` must include at least one supported size.

Do not include:

- raw secrets, API keys, tokens, passwords, or OAuth refresh tokens;
- absolute local filesystem paths;
- native executable hooks;
- direct hub API URLs;
- direct MCP, A2A, camera, microphone, or filesystem access instructions.

## Permissions

Initial permissions:

- `home:read`: read normalized non-sensitive home state.
- `widget:state`: store widget-specific state through the hub.
- `agent:skill`: allow hub-approved agents to see the widget's skill, public context, prompt guidance, and declared actions.
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

## Message Lifecycle

The normal widget startup flow is:

1. Widget loads in the frame or sandboxed iframe.
2. Widget sends `jute.widget.ready`.
3. Host replies with `jute.host.theme`, `jute.host.visibility`, and `jute.host.permissions`.
4. Widget requests declared data topics with `jute.widget.request_data`.
5. Host returns approved data with `jute.host.data`.
6. Widget renders, emits safe state updates, and handles permission or visibility changes.

Rules:

- never assume a permission was granted because it appears in `widget.json`;
- never request undeclared data topics;
- never persist state outside `jute.widget.update_state`;
- never keep polling while hidden unless a future background policy grants it;
- treat `jute.host.error` as recoverable and show an inline widget state.

## Widget Skills

Widget Skills are described in [Widget Skills](../architecture/widget-skills.md). They are the agent-facing contract for what a widget can read, explain, and safely do.

Widgets can expose skills to agents only when:

- the widget declares `agent:skill`;
- `agentSkill.enabled` is true;
- each exposed field is listed in `agentSkill.context.fields`;
- every exposed action is listed in `agentSkill.actions`;
- every prompt purpose is listed in `agentSkill.prompts`;
- the user has granted the widget's agent skill permission;
- the widget is visible or focused according to the hub context policy.

Never expose:

- secrets;
- raw adapter payloads;
- private notes;
- exact presence or location data;
- camera frames or microphone audio;
- hidden widget state;
- browser local storage.

Skill action rules:

- use stable action IDs;
- define JSON Schema inputs and outputs;
- mark the side-effect level as `read`, `display`, `configure`, or future `home_action`;
- require confirmation for configure, home-action, and other high-impact operations;
- return safe public results;
- tolerate rejected or unavailable action requests.

Prompt rules:

- declare the prompt purpose and title;
- do not put secrets, private state, or manipulative instructions in prompt text;
- expect the hub to wrap, rewrite, or replace third-party prompt text before exposing it through MCP;
- treat prompts as guidance, not permission grants.

## Contribution Checklist

Widget PRs should include:

- manifest or built-in manifest-equivalent metadata;
- README with purpose, data needs, permissions, privacy behavior, and supported sizes;
- screenshots or a short visual description for each supported size;
- accessibility notes for keyboard, screen reader, high contrast, and reduced motion;
- Widget Skill documentation when `agentSkill` is enabled;
- test notes or manual verification steps;
- license statement compatible with Jute Dash.

Reviewers should reject widgets that:

- request unnecessary permissions;
- expose private state through Widget Skills;
- rely on direct hub, MCP, A2A, filesystem, camera, or microphone access;
- require unsandboxed third-party code for normal operation;
- add global display styling or assume the current POC dashboard layout;
- fail to render useful empty, loading, unavailable, error, permission-required, and stale states.

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
- first-party built-in widgets are added from the hub-provided widget catalog;
- v1 built-in widgets are single-instance unless the catalog explicitly says `allowMultiple: true`.

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
