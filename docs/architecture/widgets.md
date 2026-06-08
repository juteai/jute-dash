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

The runtime widget architecture is illustrated below:

```mermaid
flowchart TD
    subgraph Browser [Svelte Display App]
        Shell[DisplayShell.svelte] -->|1. Mounts| Grid[DashboardGrid.svelte]
        Grid -->|2. Renders| Frame[WidgetFrame.svelte]
        Frame -->|3. Loads component from| Registry[widgets/widget-registry.ts]
        Registry -->|4. Instantiates| SvelteComp["Widget View Component\n(e.g., WeatherWidget.svelte)"]
        ConfigSheet[WidgetSettingsSheet.svelte] -->|5. Edits Settings| SvelteComp
    end

    subgraph Hub [Go Hub Backend]
        API[REST & Event API] -->|Hydrates JSON state| SvelteComp
        ConfigSheet -->|6. Saves config changes| API
        API -->|7. Updates| DB[(SQLite Runtime Store)]

        DB -->|Loads Layouts & Settings| RegistryGo["Go Widget Registry\n(widgets/registry.go)"]
        RegistryGo -->|8. Instantiates| GoWidget["Go Widget Package\n(e.g., weather/weather.go)"]

        GoWidget -->|9. Implements| Catalog["CatalogInfo & SettingsSchema"]
        Catalog -.->|Generates Settings UI Form| ConfigSheet

        Scheduler[Background Runner] -->|10. Calls Cadence| GoWidget
        GoWidget -->|FetchData| ExtData["External API / Adapter"]
        GoWidget -->|Produces Payload| DB
    end
```

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

- **Frame contract**: every widget renders inside a native Svelte `WidgetFrame` and obeys the dashboard grid layout, sizing coordinates (`x`, `y`, `w`, `h`) on the 12-column base grid, and edit-mode rules from [Display UX](display-ux.md).
- **Visual contract**: every widget uses theme tokens and supports the host's `solid`, `clear`, `smoked`, `frosted`, or `auto` widget chrome modes from [Visual Customization](visual-customization.md).
- **Backend contract**: widgets implement the `Widget` Go interface in `widgets/widget.go` to provide static metadata, a settings schema, fetching and caching logic, and agent-facing skills.
- **Agent contract**: widgets expose agent-facing context, prompts, and actions through [Widget Skills](widget-skills.md).

---

## Widget Modes

Each widget instance has a `mode`:

- `ui`: the widget renders a tile in the dashboard grid and (if it declares one) also exposes its skill/context to agents.
- `headless`: the widget renders **no** tile. It still runs `FetchData` on its normal cadence and still exposes its skill/context to agents.

Headless mode is for context-only or sensor-like use: a widget contributes household context to the conversation pipeline without occupying screen space. Because every built-in widget already declares a skill, any widget can be added as headless.

`mode` is distinct from the `visible` field. `visible: false` means the instance has been removed from the profile (no fetch, no context). `mode: headless` means the instance is active (fetch + context) but not drawn. The hub includes a widget instance in data hydration and agent context when it is not removed — that is, when it is `ui` **or** `headless` — and excludes only removed instances.

Headless instances are added, configured, and removed from the **headless tray** in edit mode (see [Display UX](display-ux.md)).

## Settings Schema

Widgets declare a typed settings schema so the display can render a settings form without bespoke per-widget UI, and so the hub can validate and introspect settings.

The Go `Widget` interface exposes `CatalogInfo() WidgetCatalogItem`, which contains a `SettingsSchema []SettingField` list. Each field declares:

- `id`: settings key;
- `type`: one of `string`, `number`, `boolean`, `enum`, `string-list`, or `object-list`;
- `label`: human-facing label;
- `default`: default value;
- `options`: allowed values for `enum`;
- `fields`: nested `SettingField` list for `object-list` items (e.g. RSS `feeds` of `{name, url}`).

The schema is surfaced through the widget catalog API alongside the catalog metadata, and a single generic form renderer in the display builds the settings sheet from it. Frame-level settings (title, chrome, size, and `mode`) are common to all widgets and are not part of the per-widget schema.

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
2. Implement your backend provider in `widgets/[name]/[name].go` under `package [name]`. Make sure it registers itself inside `init()`, and declare a `SettingsSchema()` so the widget is configurable from the UI.
3. Add a blank import for your subpackage in `apps/hub/cmd/juted/main.go` to trigger auto-registration.
4. Implement your frontend view in `widgets/[name]/[Name]Widget.svelte`.
5. Import and register your view inside [widget-registry.ts](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/widget-registry.ts).
6. Document usage, settings schemas, and examples in a `README.md` file inside your widget folder.
