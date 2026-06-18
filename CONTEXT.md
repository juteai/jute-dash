# Jute Dash — System

Jute Dash is a local-first home assistant platform. This context covers vocabulary shared across the Hub, Display, and MCP Bridge.

## Language

### Product and components

**Jute Dash**:
The product as a whole — the hub, display, and all associated services running as a household system.
_Avoid_: Jute, the app, the platform

**Hub**:
The Go service that owns configuration, persistence, the agent registry, A2A transport, home adapters, Widget Skill registry, MCP Bridge, and voice services. Authoritative name: Jute Hub (external/marketing). Shorthand: Hub (within the codebase and docs).
_Avoid_: server, backend, API server

**Display**:
The SvelteKit application that renders the dashboard, settings UI, ambient mode, and voice interaction surface. It is a client of the Hub API. Authoritative name: Jute Display (external/marketing). Shorthand: Display.
_Avoid_: frontend, web app, UI, client

**MCP Bridge**:
The optional, Hub-owned MCP surface that exposes Widget Skills, dashboard context, and Hub-mediated tools to trusted local Agents.
_Avoid_: MCP server, bridge, plugin

**Dashboard**:
The widget grid view within the Display. One view among several (settings, ambient mode, voice sheet) that the Display can show.
_Avoid_: home screen, main screen, display (when meaning this view specifically)

### Agents

**Agent**:
An external A2A-compatible service registered with the Hub. Users bring their own agents. The Hub discovers, validates, and communicates with agents over A2A.
_Avoid_: assistant, bot, service, AI

**Agent Card**:
The A2A discovery document an Agent publishes at a well-known URL. The Hub resolves, validates, and caches Agent Cards to learn an Agent's capabilities, supported interfaces, and security requirements.
_Avoid_: agent manifest, agent config

### Widgets

**Widget**:
A self-contained dashboard component committed to the `widgets/` directory of the repo. Widgets are native Svelte components. Every item on the Dashboard is a Widget.
_Avoid_: tile, card, panel, widget pack, component (when meaning a dashboard item)

**Widget Instance**:
A specific placement of a Widget on the Dashboard, with its own position, settings, and identity. Multiple instances of the same Widget type can exist on one Dashboard.
_Avoid_: widget slot, tile instance

**Widget Skill**:
The agent-facing capability declaration for a Widget. Currently defined statically in the widget's Go package under `widgets/{kind}/hub` by returning a `*widgetskills.Definition` from `Skill()`. Describes what context the Widget exposes, what actions the Hub can perform on its behalf, and what prompts help an Agent use it. The Hub reads registered Widget Skill declarations at startup and surfaces them through the MCP Bridge.
_Avoid_: widget capability, widget tool, widget plugin

**Widget Declaration**:
The current runtime declaration of a Widget's identity, settings schema, Adapter Connection requirements, and optional Widget Skill. It lives in Go under `widgets/{kind}/hub`; manifest files such as `widget.yaml` are future work, not the current source of truth.
_Avoid_: widget.yaml (when describing current runtime behavior), widget.json

**Alert Focus**:
The full-screen Display state shown when a timer, alarm, or calendar event alert is due. It is derived from hydrated Widget data plus the current Display clock, and it delegates snooze or dismiss back through Widget actions.
_Avoid_: alarm modal, timer popup, notification screen

**Notification Sound Policy**:
The shared alert sound contract used by alert-capable Widgets and the Display. The v1 supported sounds are `chime`, `bell`, `pulse`, `soft`, and `none`; unsupported values fall back to `chime` unless a valid widget fallback is supplied.
_Avoid_: ringtone system, audio theme, sound plugin

**Alert Time State**:
The time-derived status for an alert item: future, due/ringing, snoozed until a later due time, dismissed, cancelled, or expired after its event end. The Hub owns durable Alert Time State; the Display only derives a renderable focus state from Hub-provided data.
_Avoid_: browser alarm state, local reminder state

### Themes and customization

**Theme**:
A data-only visual customization pack committed to the `themes/` directory. Themes contain design tokens only — no executable code. Contributed via fork and PR, the same model as Widgets.
_Avoid_: theme pack, plugin, skin
