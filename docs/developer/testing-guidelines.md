# Testing Guidelines

Jute Dash uses the smallest stack that covers the product risks:

- Vitest for pure TypeScript and store logic in `apps/web/src/**/*.test.ts`.
- Playwright for browser user flows in `apps/web/e2e/**/*.spec.ts`.
- Playwright mocked-hub tests for fast PR coverage of display, settings, widgets, chat, and resilience states.
- Playwright visual smoke tests for broad render sanity only.
- A separate real-stack Playwright smoke path for hub/display/A2A boundary checks.
- Ginkgo hub integration specs under `apps/hub/tests/integration/specs` for black-box API, SSE, voice, and display-serving boundary checks against a running hub.

Do not assert the current proof-of-concept CSS, tile styling, or page layout as product truth. Browser tests should assert Display UX and Resilience And Error UX behavior: roles, labels, safe copy, disabled hub-write actions, no horizontal overflow, stale/degraded states, and redaction of internal errors or secrets.

## Locations

- Unit tests: `apps/web/src/**/*.test.ts`.
- Mocked-hub browser tests: `apps/web/e2e/*-mocked.spec.ts`.
- Shared Playwright hub fixtures: `apps/web/e2e/mockHub.ts`.
- Visual smoke: `apps/web/e2e/visual-smoke.spec.ts`.
- Real-stack smoke: `apps/web/e2e/real-stack.spec.ts`, run with `apps/web/playwright.real-stack.config.ts`.
- Hub integration smoke: `apps/hub/tests/integration/specs`, run with `make integration-test-local` after starting the hub. Override the target with `JUTE_HUB_BASE_URL`.

## Locators

Prefer `getByRole`, `getByLabel`, placeholder text, and visible user copy. Add `data-testid` only when the UI has no accessible name and the selector documents a stable product concept, not throwaway styling.

## PR Coverage

PRs run unit coverage, Svelte checks, build, and the fast mocked/visual Playwright suite. The real-stack smoke is manual at first because it starts the Go hub and local mock A2A agent; promote it to scheduled CI when it is stable enough to trust.

Hub API changes must update the OpenAPI contract and generated code with `make codegen`. Interface changes should regenerate mocks with `make generate-mocks`.

## Component Tests

Vitest Browser Mode and Testing Library Svelte are deferred. The current high-risk UI states are integration states driven by hub APIs, Svelte stores, and SSE. Playwright covers those with fewer dependencies. Add component tests when a stable, reusable component has enough local states that full-page Playwright tests become noisy.

## Mocked Hub

Use `createMockHub(page, scenario)` from `apps/web/e2e/mockHub.ts`. It provides deterministic responses for core hub endpoints, fails unknown API calls clearly, records writes, and shims `EventSource` for synthetic display/voice events. Keep fixture payloads redacted: no raw provider tokens, secret values, stack traces, or full remote URLs with credentials.

Initial priority flow inventory:

- startup offline, retry, runtime reconnecting/degraded, and SSE failure;
- dashboard render, widget frame states, phone/desktop overflow;
- edit mode, catalog add, headless widgets, widget settings, save/reset;
- household/room/tile/appearance/connection settings saves;
- no-agent chat state, add/refresh/toggle/remove agent, chat success/failure;
- Spotify/Apple Music auth-adjacent flows without storing raw token material.

Skipped by default: Cypress, WireMock, and Playwright component testing. Add them only when Playwright routes or the real hub smoke path stop being enough.
