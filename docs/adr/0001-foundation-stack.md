# ADR 0001: Go Hub With SvelteKit Display

## Status

Accepted

## Context

Jute Dash needs to support touch displays, browsers, desktop-style use, and future headless devices with wake-word behavior. The user prefers Go or Svelte, and wants the project to remain configurable for different home setups.

## Decision

Use Go for the local hub and SvelteKit for the display UI.

The hub owns local configuration, agent registration, A2A transport, smart-home adapters, and future voice services. The display UI consumes hub APIs and can run in a browser, kiosk shell, tablet wrapper, or future native wrapper.

## Consequences

- Headless deployments remain first-class.
- The display can move quickly without coupling agent orchestration to frontend runtime choices.
- The project can later add Tauri, Capacitor, or embedded Linux kiosk packaging without rewriting the core.
- We need to maintain an API contract between hub and display from the beginning.

