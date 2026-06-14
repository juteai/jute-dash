# Jute Dash Design Specification

Jute Dash is a local-first, privacy-respecting home assistant surface designed for bring-your-own agents. It targets tablet kiosks, desktop surfaces, and browser displays, utilizing a modular widget structure and standard agent protocols.

---

## 1. System Architecture & Component Roles

Jute Dash is split into two primary applications, keeping the hub useful without a screen and the display portable across browsers, tablets, and wall displays.

```mermaid
flowchart TB
  subgraph Client Surface
    display["Jute Display\n(SvelteKit / PWA)"]
    widgets["Native Widgets\n($widgets alias)"]
    display --> widgets
  end

  subgraph Local Daemon Boundary
    hub["Jute Hub\n(Go Daemon)"]
    sqlite["SQLite Store\n(GORM / WAL mode)"]
    config["Bootstrap Config\n(YAML/JSON)"]
    mcp["MCP Bridge\n(Streamable HTTP)"]
    voice["Voice Service\n(VAD / STT / TTS)"]
    filesync["FileSync Sync Worker\n(River Queue)"]

    hub --> sqlite
    hub --> config
    hub --> mcp
    hub --> voice
    sqlite -- Sync job -- > filesync
    filesync -- Save YAML -- > config
  end

  subgraph Bring-Your-Own-Agents
    agent["A2A Agent\n(External)"]
    agentCard["Agent Card\n(Discovery)"]
  end

  display -- HTTP + SSE -- > hub
  agent -- A2A 1.0 Protocol -- > hub
  agent -- Resolve Card -- > agentCard
  agent -- Context / Tools -- > mcp
```

### Component Breakdown
* **Jute Hub (Go):** Headless-capable daemon that manages configuration loading, SQLite-backed persistence, event streaming, wake words, local API routes, agent connectivity, and home automation state.
* **Jute Display (SvelteKit):** Touch-first client that renders the dashboard grid, voice conversation sheet, ambient states, and settings UI. It consumes the Hub API and event stream.
* **Jute MCP Bridge:** An optional HTTP-based bridge that securely surfaces dashboard context, widget skills, and hub-mediated display actions to trusted local agents without exposing databases or raw credentials.
* **Jute Voice Service:** Local voice boundaries managing microphone capture, Voice Activity Detection (VAD), wake-word detection, utterance buffering, and voice provider packs (STT/TTS).
* **FileSync Sync Worker:** A background task processor using **River** (`riverdriver/riversqlite`) that runs a single-threaded queue. When settings are written to SQLite, the worker is enqueued to write changes back to the bootstrap YAML config file atomically.

---

## 2. Approach & Core Inspiration

Jute Dash's interface, branding, and interaction models are inspired by modern minimalist operating systems and spatial hardware panels, moving away from verbose, busy web administration panels.

### Brand Identity & Antigravity Inspiration
* **Antigravity Website Aesthetic:** Jute’s brand identity draws inspiration from the clean, high-contrast, spacious design of the Antigravity website. This inspires the central Jute branding lockups and the header image in the repository (`docs/images/readme-header.svg`).
* **Jute Logo Assets:**
  * Light-Mode Logo: `apps/web/static/brand/logo_dark.svg`
  * Dark-Mode Logo: `apps/web/static/brand/logo_light.svg`
  * The logos are treated as monochrome marks only, avoiding ad-hoc recoloring.
* **Antigravity Stardust Canvas:** The interface features a subtle, floating ambient particle overlay (`.stardust-canvas`) using hardware-accelerated CSS animations. This stardust drifts quietly in the background during setup, empty state, and voice listening modes, projecting a premium, active feel.

### Component Architecture: shadcn-svelte & Alexa Guidelines
* **shadcn-svelte Conventions:** The front-end uses [shadcn-svelte conventions](https://www.shadcn-svelte.com/llms.txt) for all complex layout controls (buttons, sheets, drawers, dialogs, dropdowns, inputs, scroll areas, and tabs). Raw HTML form controls are disallowed.
* **Widget Settings Panel:** Per-widget configuration is rendered inside a slide-in sheet (`WidgetSettingsSheet.svelte`) supporting title edits, appearance chrome options, and list editors.
* **Alexa Design Guidelines:** Jute adopts Amazon's voice-plus-screen principles:
  * **Situational UX:** Voice is voice-forward, and the screen behaves as a visual companion.
  * **Echo Show Model:** When the display is "awoken" (via wake word or push-to-talk), a dedicated conversation view (voice sheet) slides in.
  * **Dashboard Context-Awareness:** The A2A conversation agent reads the current visible dashboard state over the MCP resource `jute://dashboard/current`, allowing it to direct visual focus (via `jute_display_focus_widget`) or push notifications (via `jute_display_notification`) dynamically.

---

## 3. UI Shape & Visual Identity

The interface acts as a spatial widget board. All visible dashboard widgets are native Svelte components hosted inside a responsive grid layout.

### Authored Base Grid & Responsive Scaling
* **12-Column Base Grid:** Layouts are authored and stored at a `BASE_COLUMNS = 12` grid resolution.
* **Proportional 1fr Columns:** The dashboard canvas uses CSS grid with `1fr` columns and rows, scaling widgets proportionally to fill the viewport width.
* **Responsive Column Remapping:** When displaying on narrow viewports, the display re-flows the base layout:
  * **Desktop / Wall Displays (>= 1024px):** 12 columns
  * **Tablets (>= 768px):** 6 columns
  * **Large Phones / Small Tablets (>= 480px):** 4 columns
  * **Phones (< 480px):** 2 columns
* **Widget Execution Modes:**
  * `ui`: The widget is rendered on the dashboard grid.
  * `headless`: The widget executes, gathers data, and feeds A2A/MCP assistant context, but its visual tile is hidden from the dashboard.

### Liquid Glass Spatial UX
Jute Dash implements a "Liquid Glass" spatial container model for widgets:
1. **Dynamic Lensing:** Widget borders are slim (`1px` border using `var(--border)`). Elements scatter and bend underlying content light using CSS `backdrop-filter`. When a widget is active or focused, it warps local light more strongly.
2. **Responsive Tactility:** Widgets adapt physically to cursor and gesture interaction:
   * **Hover Glow:** On hover, borders transition smoothly to `var(--active)` or `var(--foreground)` with a subtle radial glow (`box-shadow: 0 0 12px color-mix(in srgb, var(--active) 25%, transparent)`).
   * **Compliance Press:** Under click or touch-down, elements scale down slightly (`transform: scale(0.985)`) to simulate physical push compliance.
3. **Contextual Viscosity:** Glass panels adjust their blur levels dynamically based on visual density:
   * Content-light widgets (e.g., `date-time`) default to transparent (`clear` or `smoked` with `backdrop-filter: blur(8px)`).
   * Dense content panels (e.g., `chat-history`, settings panel) upgrade to heavy blur (`frosted` with `backdrop-filter: blur(24px)`) to maintain contrast and legibility.

### "Less is More" Minimalist Typography
All widgets must strictly follow a non-verbose layout hierarchy:
* **Icon-First Indicators:** Verbose labels (like `"apparent temperature"`, `"last update"`, `"current index"`) are forbidden. They must be replaced with lightweight `lucide-svelte` icons.
* **Value Dominance:** The primary metric (e.g., `22°`, `10:15`, `+0.4%`) dominates the frame visually, using large typography weights and zero adjacent labeling.
* **Single-Line Metadata:** Secondary metrics are restricted to a single row of compact text at the bottom of the widget frame.

---

## 4. Stable Design System CSS Tokens

CSS Custom Properties are defined globally at the display root inside [app.css](file:///Users/craighutcheon/Repos/Other/jute-dash/apps/web/src/app.css):

| CSS Variable | Light Mode (BOW) | Dark Mode (WOB) | Semantic Purpose |
| :--- | :--- | :--- | :--- |
| `--background` | `#ffffff` | `#000000` | Display base background |
| `--foreground` | `#000000` | `#ffffff` | Primary readable text |
| `--surface` | `#ffffff` | `#000000` | Core card and widget container background |
| `--surface-muted`| `#f7f7f7` | `#111111` | Muted fields and input elements |
| `--surface-strong`| `#eeeeee` | `#1f1f1f` | Hover and high-contrast surfaces |
| `--border` | `#d8d8d8` | `#333333` | Standard structural borders |
| `--border-strong`| `#000000` | `#ffffff` | High-contrast borders |
| `--muted` | `#5f5f5f` | `#a6a6a6` | Secondary labels and helper text |
| `--muted-strong` | `#2d2d2d` | `#dddddd` | Strong secondary description text |
| `--inverse` | `#ffffff` | `#000000` | Inverse text and icons |
| `--accent` | `#111111` | `#ffffff` | Brand indicators and markers |
| `--focus` | `#000000` | `#ffffff` | Focus rings and active accessibility outlines |
| `--shadow` | `rgba(0, 0, 0, 0.12)` | `rgba(255, 255, 255, 0.12)`| Overlay drop shadows |

### Semantic State Colors
Semantic colors are reserved purely for state indication:
* `--danger` (Light: `#b42318` / Dark: `#ffb4ab`): Widget errors, critical warnings.
* `--warning` (Light: `#8a5a00` / Dark: `#ffd28a`): Hub disconnect, connection retries.
* `--success` (Light: `#147a3d` / Dark: `#8de6ad`): Connection restoral, action completions.
* `--active` (Light: `#155eef` / Dark: `#adc6ff`): Microphone active listening, input focus.

---

## 5. Configuration & CLI Definition

The Hub operates on a merged configuration model where compiled defaults are overridden by CLI environment parameters, bootstrap files, and SQLite.

### Command-Line Interface
The daemon `juted` uses the following parameters for bootstrapping and configuration:

* `-config` (Env: `JUTE_CONFIG`): Path to Jute bootstrap config YAML or JSON.
* `-data-dir` (Env: `JUTE_DATA_DIR`): Location of Jute SQLite database and files.
* `-listen` (Env: `JUTE_LISTEN`): Address for the Hub server (defaults to `127.0.0.1:8787`).

### Config Schema Structure
Defined in [config.go](file:///Users/craighutcheon/Repos/Other/jute-dash/apps/hub/internal/app/config/config.go), the config represents:
```yaml
home:
  locale: "en-US"
  timezone: "UTC"
server:
  listenAddress: "127.0.0.1:8787"
mcp:
  enabled: false
  listenAddress: "127.0.0.1:8788"
  path: "/mcp/v1"
a2a:
  loopback: true
display:
  colorMode: "system"
  themeId: "jute-mono"
  density: "comfortable"
  motion: "full"
weather:
  enabled: true
  provider: "open-meteo"
dashboard:
  widgets:
    - id: "date-time"
      type: "date-time"
      title: "Date & Time"
      x: 0
      y: 0
      w: 6
      h: 2
      visible: true
      mode: "ui"
```

---

## 6. Widget & Skill Interfaces

All Jute Dash widgets are first-party Svelte views and Go backend data aggregators compiled into the codebase.

### Backend Go Interface
Every widget implements the Go interface in [widgets/widget.go](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/widget.go):

```go
type Widget interface {
	Kind() string
	CatalogInfo() WidgetCatalogItem
	FetchData(ctx context.Context, settings map[string]any) (any, error)
	Skill() *widgetskills.Definition
}
```

### Core Built-in Widgets
1. **Date & Time (`date-time`):** Digital clock, date, timezone, and locale synchronization.
2. **Weather (`weather`):** apparent temperature, humidity, wind, and conditions using Open-Meteo.
3. **Chat History (`chat-history`):** Recent conversations, active A2A agent status, and a quick re-entry chat button.
4. **RSS Feed (`rss`):** Headings aggregator from custom RSS XML streams with background caching.
5. **Markets (`markets`):** Stock, commodity, or cryptocurrency trackers utilizing Yahoo Finance API.

### Widget Skills Registry schemas
The MCP Bridge and dashboard context communicate using the following canonical schemas:
* **Dashboard Context:** `https://jute.dev/mcp/resources/dashboard-context/v1`
* **Visible Widgets:** `https://jute.dev/mcp/resources/visible-widgets/v1`
* **Widget Skills List:** `https://jute.dev/mcp/resources/widget-skills/v1`
* **Widget Skill Context:** `https://jute.dev/mcp/resources/widget-skill-context/v1`

---

## 7. Resilience & Runtime Status Vocabulary

Jute Dash ensures that hub failures, degraded networks, or agent timeouts are immediately visible without displaying raw console errors.

### App Connection States
The client display maintains one of the following states:
* `starting`: Initializing and checking Hub reachability.
* `connected`: Reachable, and core Display data is fresh.
* `reconnecting`: Request or event stream disconnected; auto-retries active.
* `offline`: Hub is completely unreachable.
* `degraded`: Hub is reachable, but one or more sub-features are offline.

### Agent Availability States
Agents configured over A2A report their status using these values:
* `available`: Ready for turns.
* `disabled`: Configured but intentionally turned off.
* `missing_credentials`: Authentication configured but credential missing.
* `unsupported_binding`: Binding mismatch (not `JSONRPC`, `HTTP+JSON`, or `GRPC`).
* `unhealthy`: Active agent fails ping/health checks.
* `offline`: Endpoint host cannot be resolved.
* `unknown`: Awaiting initial health probe.

### Widget Render States
Widgets render inside their frames using one of the following states:
* `loading`: Widget is loading required API data.
* `empty`: Configured and functional but has no data.
* `unavailable`: Required upstream provider is offline.
* `error`: Rendering or fetch logic failed.
* `permission_required`: Prompt needed for user access grant.
* `stale`: Displays last known memory state while Hub is reconnecting.

---

## 8. Security & Redaction Protocol

Jute Dash acts as a secure firewall between home data and remote agents.

* **Loopback Bindings:** By default, all local services (including the Hub API and the MCP bridge) bind to `127.0.0.1`.
* **Redaction Rules:** Raw Go errors, database stack traces, token strings, credentials, secret references, and full remote URLs containing query parameters must **never** be exposed in the client DOM or user-facing logs.
* **A2A Context Redaction:** Only authorized and safe dashboard widgets, layouts, and settings mapped explicitly by Hub-owned Widget Skill declarations may be packaged and sent to conversation agents. Widget manifests such as `widget.yaml` are future work, not the current runtime contract.
