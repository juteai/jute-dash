# Widgets

## Strategy

Jute Dash's widget ecosystem is designed for maximum speed, visual polish, and ease of development. All widgets are compiled natively as **first-party Svelte components** on the frontend and **Go packages** on the backend. 

There are no sandboxed iframes, postMessage bridges, or third-party runtime sandboxing layers. This choice enables:
- **Flawless UI Integration**: Widgets blend natively with our black-on-white (BOW) and white-on-black (WOB) display design system, supporting smooth layout resizing, theme swapping, and hover micro-animations.
- **Maximum Performance**: Fast, direct client-side Svelte execution and direct Go data aggregation without the processing overhead of multi-process iframe containment.
- **Simplified Development**: To add, understand, or edit a widget, a developer only needs to open one unified directory.

All widgets are visually hosted in the dashboard `WidgetFrame` specified in [Display UX](display-ux.md).

Widget frame styling, transparency, and background blending are host-owned display concerns specified in [Visual Customization](visual-customization.md). Widget code should not hard-code opaque surfaces when theme tokens or widget chrome classes are available.

Widgets also declare agent-facing capabilities through [Widget Skills](widget-skills.md). The hub uses this contract to expose widget capabilities through A2A dashboard context and the optional MCP Bridge.

---

## Monorepo Widgets Library

All widgets live in the unified **monorepo widgets library** under the root `widgets/` directory. 

Each widget occupies its own self-contained subfolder containing:
1. A **Go provider** (`[kind].go`, e.g., `widgets/rss/rss.go`) representing the back-end data aggregator and skill definitions under a subpackage (e.g. `package rss`).
2. A **Svelte view** (`[Name]Widget.svelte`, e.g., `widgets/rss/RSSWidget.svelte`) rendering the front-end interface.

```text
widgets/
  datetime/
    datetime.go
    DateTimeWidget.svelte
  rss/
    rss.go
    RSSWidget.svelte
```

### Svelte Resolution ($widgets alias)
Frontend imports resolve outside the Vite project root via the `$widgets` path alias:
```typescript
import RSSWidget from '$widgets/rss/RSSWidget.svelte';
```
Vite file system permissions are proactively granted inside `apps/web/vite.config.ts` via `server.fs.allow`.

### Dynamic Go Self-Registration
Backend packages self-register with Jute's catalog and dynamic skill registries via package initialization (`init()`). Since Go compiles package files together, importing a package automatically registers its widgets. 

To maintain clean and acyclic Go package dependencies (since subpackages import the root `widgets` package to register), all package blank imports are consolidated inside [main.go](file:///Users/craig/Repos/jute-dash/apps/hub/cmd/juted/main.go):
```go
import (
	_ "jute-dash/widgets/chathistory"
	_ "jute-dash/widgets/datetime"
	_ "jute-dash/widgets/markets"
	_ "jute-dash/widgets/rss"
	_ "jute-dash/widgets/weather"
)
```
This guarantees that any server context automatically compiles and loads all widgets.

### Workspace Tooling Alignment
To align monorepo workspace tooling, a symbolic link `widgets/node_modules -> ../apps/web/node_modules` allows Svelte compilers (`svelte-check`), TypeScript checkers, linters, and IDEs to flawlessly resolve external dependencies inside the root widgets directory.

---

## Contract Layers

The widget system is structured around three key contracts:

- **Frame contract**: every widget renders inside a native Svelte `WidgetFrame` and obeys the dashboard grid layout, sizing coordinates (`x`, `y`, `w`, `h`), and edit-mode rules from [Display UX](display-ux.md).
- **Visual contract**: every widget uses theme tokens and supports the host's `solid`, `clear`, `smoked`, `frosted`, or `auto` widget chrome modes from [Visual Customization](visual-customization.md).
- **Backend contract**: widgets implement the `Widget` Go interface in `widgets/widget.go` to provide static metadata, fetching and caching logic, and agent-facing skills.
- **Agent contract**: widgets expose agent-facing context, prompts, and actions through [Widget Skills](widget-skills.md).

---

## Core Widgets

Jute Dash ships with five built-in widgets:

1. **Date & Time (`date-time`)**: Clock, date, timezone, and locale synchronization.
2. **Weather (`weather`)**: Current apparent temperature, humidity, wind, and conditions using Open-Meteo.
3. **Chat History (`chat-history`)**: Recent conversation turns, active A2A agent status, and a quick re-entry chat button.
4. **RSS Feed (`rss`)**: headlines aggregator from custom RSS xml streams with background caching.
5. **Markets (`markets`)**: Stock, commodity, or crypto tickers watchlist using Yahoo Finance.

---

## Developer Guidelines

To build a new widget:
1. Create a folder `widgets/[name]/`.
2. Implement your backend provider in `widgets/[name]/[name].go` under `package [name]`. Make sure it registers itself inside `init()`.
3. Add a blank import for your subpackage in `apps/hub/cmd/juted/main.go` to trigger auto-registration.
4. Implement your frontend view in `widgets/[name]/[Name]Widget.svelte`.
5. Import your view in `DashboardGrid.svelte` and map it inside the widget render block.
6. Document usage, settings schemas, and examples in a `README.md` file inside your widget folder.
