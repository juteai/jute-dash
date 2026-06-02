# Context Map

## Contexts

- [System](./CONTEXT.md) — shared vocabulary for the Jute Dash product as a whole
- [Hub](./apps/hub/CONTEXT.md) — the Go service that owns config, persistence, agents, and MCP
- [Display](./apps/web/CONTEXT.md) — the SvelteKit app that renders the dashboard and settings UI

## Relationships

- **Display → Hub**: the Display is a client of the Hub API. It does not call agents, home adapters, or MCP directly.
- **Hub → Agents**: the Hub discovers, validates, and communicates with Agents over A2A. The Display never talks to Agents directly.
- **Hub → MCP Bridge**: the MCP Bridge is part of the Hub. It surfaces Widget Skills and dashboard context to trusted local Agents.
- **Hub ↔ Display (Widget Skills)**: the Hub reads static widget manifests at startup and exposes the resulting skill surface through the MCP Bridge. The Display renders widgets; the Hub owns the skill registry.
