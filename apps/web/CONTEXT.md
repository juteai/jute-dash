# Display

The SvelteKit application that renders Jute Dash for users. It is a client of the Hub API — it does not own state, does not talk to Agents, and does not call MCP directly.

## Language

### Views

**Dashboard**:
The primary view — a positioned grid of Widget Instances. See system CONTEXT.md for the canonical definition.

**Ambient Mode**:
A low-information, always-on display mode shown when the household is idle. Rendered by the Display, driven by Hub state.
_Avoid_: screensaver, idle mode, sleep mode

**Voice Sheet**:
The overlay surface shown during voice interaction — displays listening state, transcript bubbles, mute/cancel controls.
_Avoid_: voice modal, voice dialog, voice overlay

**Settings UI**:
The Display views for configuring the Hub (agents, widgets, themes, display preferences). Settings are persisted by calling Hub APIs, never browser-local storage.
_Avoid_: preferences, config screen

### Widget rendering

**Widget Chrome**:
The visual frame the Display applies around a Widget Instance. Controlled by the Hub config and Display settings. Modes: `solid`, `clear`, `smoked`, `frosted`, `auto`.
_Avoid_: widget border, widget container, widget wrapper

### Connection states

**Offline**:
The Display cannot reach the Hub at all. Show the startup-offline state per the Resilience and Error UX doc.
_Avoid_: disconnected, down, unreachable

**Reconnecting**:
The Display has lost a previously established Hub connection and is attempting to restore it.
_Avoid_: retrying, loading

**Stale**:
The Display is connected but Hub data has not refreshed within the expected window.
_Avoid_: outdated, cached, old data
