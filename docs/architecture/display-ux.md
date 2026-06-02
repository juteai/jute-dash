# Display UX

## Goal

This document defines the first real Jute Dash display UX. It covers the dashboard, widget frame system, agent chat mode, transitions, first widgets, and implementation constraints for the SvelteKit display.

Jute should feel calm, direct, and home-native: a useful always-on assistant surface rather than a web admin dashboard.

## Current UI Status

The existing `apps/web` dashboard is throwaway proof-of-concept work.

Rules:

- current CSS classes are not canonical;
- current page layout, side panel, tile structure, and visual styling are not canonical;
- current component names may be reused only if they fit the new architecture;
- future implementation may replace the dashboard UI from scratch;
- only hub API contracts, architecture decisions, and validated product behavior should be preserved.

Do not use screenshots or styles from the current POC as design targets.

## Design Constants

Brand constants:

- light-mode logo source: `/Users/craig/Repos/jute/docs/brand/logo_dark.svg`;
- dark-mode logo source: `/Users/craig/Repos/jute/docs/brand/logo_light.svg`;
- logo treatment: monochrome mark only, no recoloring for v1.

UI system constants:

- use [shadcn-svelte conventions](https://www.shadcn-svelte.com/llms.txt) for buttons, sheets, dialogs, menus, tabs, inputs, scroll areas, command surfaces, and accessible primitives;
- keep cards and widget frames at 8px border radius or less;
- use lucide-svelte icons for common controls;
- keep display text concise and functional.

Palette:

- light mode is BOW: black-on-white;
- dark mode is WOB: white-on-black;
- use neutral gray borders and surfaces;
- reserve non-neutral colors for semantic states such as error, warning, success, active voice, or recording;
- do not introduce a broad brand color palette in v1.

Visual customization is specified in [Visual Customization](visual-customization.md). The BOW/WOB palette is the default `jute-mono` Theme Pack, not a permanent limit on future contributed themes.

## Display Modes

The display has three primary modes:

- `dashboard`: default widget canvas.
- `edit`: dashboard customization mode.
- `chat`: focused agent conversation mode.

```mermaid
stateDiagram-v2
  [*] --> dashboard
  dashboard --> edit: long press or edit button
  edit --> dashboard: done or cancel
  dashboard --> chat: command, voice, widget, or agent action
  edit --> chat: explicit chat action
  chat --> dashboard: close or minimize
  chat --> dashboard: conversation ended
```

The mode is UI state, but durable layout changes are saved through the hub.

## Resilience And Error States

Runtime error behavior is specified in [Resilience And Error UX](resilience-error-ux.md).

Display requirements:

- initial load with no hub shows a full-screen offline state, not a silent fake dashboard;
- runtime hub disconnect keeps the last in-memory dashboard visible, marks it stale, and shows a persistent status ribbon;
- app-level banners are reserved for hub-level or cross-feature issues;
- single-widget failures stay inside `WidgetFrame`;
- no configured or enabled agent is a setup-needed state, not a danger error;
- chat send failures appear inline near the failed turn with retry or cancel where possible.

App connection states:

- `starting`;
- `connected`;
- `reconnecting`;
- `offline`;
- `degraded`.

## Dashboard

The dashboard is the default first screen.

Layout rules:

- date and time appear in the top-left by default;
- top-left date/time uses configured locale and timezone;
- the dashboard includes the Jute logo, home name, active layout profile, and compact controls;
- dashboard controls use icons for chat, voice/mute, settings, and edit mode;
- widget canvas scrolls vertically when widgets exceed the viewport;
- horizontal overflow is not allowed;
- dashboard chrome remains minimal so widgets are the main content.

## Settings UX

The pre-v1 settings surface is an in-app panel opened from the dashboard header, chat empty states, and agent diagnostics.

Initial sections:

- `Household`: home name, timezone, locale, theme, weather enablement, location, coordinates, and units;
- `Rooms`: editable room IDs, names, summaries, and simple status text;
- `Tiles`: editable dashboard tile IDs, kinds, labels, values, and details;
- `Agents`: add an agent by Agent Card URL, enable or disable agents, remove agents, and refresh Agent Cards;
- `MCP`: read-only bridge status and startup configuration summary;
- `Voice`: read-only voice/provider status until provider selection is implemented;
- `About`: version, setup, config mode, and enabled-agent summary.

Settings writes go through the hub. Store-backed runs persist to SQLite. YAML-backed local harnesses persist the same records to the active YAML config for easy developer iteration. Browser storage is not durable settings storage.

Responsive behavior:

- phone and narrow tablets use a single-column canvas;
- tablets use a 2 to 4 column canvas;
- desktop and wall displays use a wider responsive grid;
- large wall displays may keep chat as a side focus area, but ordinary displays use full chat focus mode.

Spacing:

- base spacing unit is 8px;
- outer page padding defaults to 16px on small screens and 24px on larger screens;
- grid gaps default to 12px or 16px depending on density;
- touch targets are at least 44px.

## Dashboard Grid

The dashboard grid is draggable and resizable.

Persisted widget layout fields:

- `id`: widget instance ID;
- `kind`: widget kind matching the widget's registered type;
- `x`: grid column start;
- `y`: grid row start;
- `w`: grid width;
- `h`: grid height;
- `minW`: minimum grid width;
- `minH`: minimum grid height;
- `size`: named size such as `small`, `medium`, `wide`, or `large`;
- `settings`: non-secret widget settings;
- `visible`: whether the widget appears on the current profile.

Implementation guidance:

- v1 uses a small custom Svelte grid editor for the built-in widget set;
- revisit a proven Svelte-compatible drag/resize grid library only when denser layouts make the custom editor too costly;
- preserve layout through hub APIs, not browser local storage;
- debounce layout saves while dragging;
- commit the final layout when drag or resize ends;
- keep keyboard alternatives for move and resize.

Current v1 layout APIs:

- `GET /api/v1/widgets/catalog`: built-in widget catalog.
- `GET /api/v1/widgets/layout`: current device layout profile.
- `PUT /api/v1/widgets/layout`: replace the current layout with a validated full layout document.
- `POST /api/v1/widgets/layout/reset`: restore the default built-in layout.

Widget additions can come from:

- setup config bootstrap;
- SQLite layout settings;
- in-app widget catalog.

## Edit Mode

Edit mode is activated by:

- long press on touch;
- explicit edit button on desktop and keyboard/remote surfaces.

Long press defaults:

- press duration: 650ms;
- cancel when the pointer moves beyond a small drag threshold;
- provide haptic feedback where the platform allows it.

Edit mode supports:

- add widget;
- move widget;
- resize widget;
- remove widget;
- configure widget;
- duplicate widget when the widget supports multiple instances;
- reset layout profile.

Edit mode UI:

- show a subtle grid overlay;
- show drag handles and resize handles;
- show a top or bottom edit toolbar;
- show clear Done and Cancel actions;
- avoid accidental deletes by requiring confirmation or undo.

## Widget Frame

All widgets render inside a standard `WidgetFrame`.

Frame contract:

- visible 1px border;
- 8px maximum border radius;
- internal padding based on size;
- optional header with title and actions;
- edit-mode drag handle;
- edit-mode resize handle;
- focus ring for keyboard navigation;
- empty, loading, error, and permission-required states;
- stale and unavailable states when hub data or dependencies are not fresh;
- declared overflow behavior.
- host-owned widget chrome using `solid`, `clear`, `smoked`, `frosted`, or `auto` from [Visual Customization](visual-customization.md).

Overflow modes:

- `clip`: content is clipped to the frame.
- `scroll`: content scrolls inside the frame.
- `expand`: widget may request a larger supported size.

All widgets are native Svelte components compiled directly into the display application that render inside the same frame contract.

## First Widgets

Initial built-in widgets:

- `date-time`: clock, date, timezone, and optional next relevant household moment.
- `weather`: current Open-Meteo state from the hub, with unavailable and disabled states.
- `chat-history`: recent conversations, active agent status, no-agent state, and quick re-entry into chat mode.

Default dashboard profile:

- `date-time` anchored top-left;
- `weather` near the top row;
- `chat-history` visible when at least one agent is configured;
- additional status widgets may be added later, but these three define the first clean layout.

## Chat Mode

Chat is a focused mode for conversations with an A2A agent.

Entry points:

- dashboard command input;
- voice wake or push-to-talk;
- chat-history widget;
- agent action;
- notification or task continuation.

Transition:

- use smooth zoom/fade or sheet expansion from the dashboard;
- respect reduced-motion preferences;
- keep the dashboard context visually connected, but make chat the focus;
- closing chat returns to the previous dashboard scroll position.

Chat layout:

- conversation header with agent name, status, close/minimize, mute, and cancel;
- markdown-rendered message stream;
- user and assistant message bubbles;
- task/progress rows for A2A status;
- bottom input bar with text entry, send, voice, and cancel controls;
- optional side metadata on wide displays for sources, widgets in context, or task artifacts.

Markdown rendering:

- support paragraphs, headings, lists, code blocks, links, tables, and inline code;
- sanitize untrusted markdown;
- open external links with clear affordance;
- never allow raw HTML execution from agent messages.

Chat states:

- `idle`: ready for input.
- `listening`: voice or push-to-talk capture is active.
- `thinking`: agent turn started and response is pending.
- `streaming`: response is arriving.
- `error`: recoverable failure with retry or close.

Agent availability:

- `available`: selectable for new turns.
- `disabled`: configured but intentionally off.
- `missing_credentials`: authentication is configured but unavailable.
- `unsupported_binding`: no supported A2A binding is available.
- `unhealthy`: the hub health check failed.
- `offline`: the agent endpoint cannot be reached.
- `unknown`: health has not been checked yet.

When no agent is available, chat remains reachable but opens to a setup-needed state with the composer disabled.

Activity animation:

- use a restrained pulsing ring or three-dot shimmer;
- do not use decorative blobs or large gradients;
- keep animation disabled or simplified under reduced motion.

The Svelte app does not call agents directly. Chat sends turns to the hub and renders hub conversation/task events.

## Voice And Chat

Voice UI uses the same chat mode primitives.

Voice-specific requirements:

- listening state is visually distinct from thinking and streaming;
- mute and cancel remain visible during voice activity;
- follow-up listening can keep chat open briefly after response completion;
- ambient mode may show only listening/speaking state, not full transcripts;
- TTS playback state appears in the chat header or input area.

Detailed voice behavior remains in [Voice And Wake Word Architecture](voice.md).

## Persistence

The hub is the durable source of truth.

Persist through SQLite:

- layout profiles;
- widget instance layout;
- widget settings;
- theme selection, background policy, and widget chrome settings;
- edit-mode saved changes;
- selected theme mode;
- default display profile;
- conversation summaries when history is enabled.

Do not persist durable layout or chat state only in browser local storage.

## Accessibility

The display must support:

- keyboard navigation for dashboard, edit mode, and chat;
- focus-visible rings;
- screen-reader labels for icon controls;
- reduced motion;
- high contrast;
- large touch targets;
- text that does not overflow controls;
- safe markdown semantics for chat content;
- non-pointer alternatives for move and resize.

## Implementation Notes

When the clean UI implementation starts:

- start from the architecture docs, not the current POC CSS;
- copy or install the logo assets into the display app through a planned asset path;
- define shadcn-svelte theme tokens for BOW/WOB;
- implement `WidgetFrame` before individual widget polish;
- implement dashboard, edit mode, and chat as separate mode-level components;
- verify with Playwright screenshots on mobile, tablet, desktop, and wall-display widths.
