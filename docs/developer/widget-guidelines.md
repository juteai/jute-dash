# Widget Developer Guidelines

## Overview

Jute Dash's widget ecosystem is a unified, high-performance, and **monorepo-driven native library**. Both the back-end data fetching and scheduling logic (written in Go) and the front-end display view (written in Svelte) live side-by-side inside a single widget directory under the root `widgets/` folder. 

There are no sandboxed iframes, manifests, or postMessage message protocols. Everything compiles and executes natively within Jute's own runtime.

---

## Widget Folder Structure

Every widget lives in its own subdirectory under `/widgets/`:

```text
widgets/
  [name]/
    [name].go                 # Backend Go provider
    [Name]Widget.svelte       # Frontend Svelte view component
    README.md                 # Usage documentation and settings schema
```

---

## 1. Backend Implementation (Go)

Your Go file must define a package named after the widget's kind (e.g. `package weather` in `widgets/weather/weather.go`) and implement the `Widget` interface defined in `widgets/widget.go`:

```go
type Widget interface {
	// Kind returns the unique string identifier for the widget (e.g. "weather", "rss").
	Kind() string

	// CatalogInfo returns the static registration metadata.
	CatalogInfo() WidgetCatalogItem

	// FetchData gathers and aggregates the latest state/payload for this widget.
	// It is passed the widget's custom settings from the YAML file.
	FetchData(ctx context.Context, settings map[string]any) (any, error)

	// Skill returns the optional agent-facing skill metadata. Returns nil if visual-only.
	Skill() *widgetskills.Definition
}
```

### Self-Registration
During `init()`, register your widget with the global registry:

```go
func init() {
	widgets.Register(&MyWidget{})
}
```

### Server Instantiation (Blank Imports)
To trigger the widget's `init()` block, add a blank import for your subpackage inside Jute's main server file [internal/server/server.go](file:///Users/craig/Repos/jute-dash/internal/server/server.go):

```go
import (
	_ "jute-dash/widgets/mywidget"
)
```
This guarantees dynamic registration upon server boot while preventing Go circular import cycles.

---

## 2. Frontend Implementation (Svelte)

Your Svelte view must be named `[Name]Widget.svelte` (e.g. `WeatherWidget.svelte`).

### Path Alias Resolution
To import your Svelte view inside the main layout displays (like `DashboardGrid.svelte`), use the `$widgets` path alias which points to the repository root:

```typescript
import MyWidget from '$widgets/mywidget/MyWidget.svelte';
```

### Vite File System Permissions
Because widgets live outside the SvelteKit project directory (`apps/web`), Vite restricts file system access by default. Ensure the path is explicitly allowed inside `apps/web/vite.config.ts`'s `server.fs.allow` configuration.

---

## 3. Widget Skills & Agent Context

If your widget is agent-visible, define its `Skill()` return structure to declare what an agent can see and do through A2A or MCP:

- **Context fields**: Expose public fields (e.g., current prices, apparent temperatures) that an agent is allowed to read.
- **Actions**: Define safe, hub-mediated commands (like refreshing data) with input and output JSON schemas.
- **Prompts**: Provide high-level guidance explaining the widget's purpose.

*Note: Never expose secrets, OAuth credentials, raw database rows, or private metadata inside your widget's context fields.*

---

## 4. UI & Styling Guidelines

- **Theme Compliance**: Adhere strictly to the black-on-white (BOW) and white-on-black (WOB) display design system.
- **Hover Micro-Animations**: Use smooth CSS transitions (`transition-all`, `hover:scale-[1.01]`) to make interactions feel premium and responsive.
- **Grids & Layouts**: Design the Svelte component to fit cleanly inside the standard `WidgetFrame` at all supported grid sizes. Expose a clean empty or loading state when data is unavailable.

---

## Contribution Checklist

When contributing a new widget:
1. **Directory**: Create `widgets/[name]/` containing `[name].go` and `[Name]Widget.svelte`.
2. **Dynamic Boot**: Blank import your package inside `internal/server/server.go`.
3. **Dashboard Mapping**: Import and map the component inside `DashboardGrid.svelte`.
4. **Documentation**: Write a `README.md` inside your widget folder detailing its kind, supported sizes, and custom settings schemas.
5. **Quality Verification**: Run `make check` to verify Go compilation, backend package tests (`go test ./...`), and SvelteKit type checks (`make web-check`).
