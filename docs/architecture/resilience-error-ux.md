# Resilience And Error UX

## Goal

Jute Dash should make runtime problems visible, calm, and actionable without becoming an admin console.

The display must not silently hide hub failures behind fake data. Fallback data can keep the interface renderable during development, but the product UX must clearly show when data is unavailable, stale, degraded, or blocked by setup.

## Principles

- Keep the last useful dashboard visible whenever it is safe to do so.
- Prefer inline and non-modal recovery over blocking dialogs.
- Use clear household language, not internal service language.
- Explain what is unavailable and what the user can do next.
- Preserve touch targets and kiosk readability during failures.
- Never expose raw Go errors, stack traces, credential names, secret references, full remote URLs, or internal payloads in user-facing text.
- Keep detailed diagnostics behind future settings or debug surfaces.

## App Connection States

The display tracks one app-level connection state:

- `starting`: the display is loading initial data or checking the hub.
- `connected`: the hub is reachable and core display data is fresh.
- `reconnecting`: the hub was reachable, then a request or event stream failed, and reconnect attempts are active.
- `offline`: the hub is unreachable and no fresh data is available.
- `degraded`: the hub is reachable, but one or more important subsystems are unavailable.

The state is display-local but derived from hub API calls, `/healthz`, future `/api/v1/status`, and future `/api/v1/events` health.

## Startup Offline UX

If the display cannot reach the hub during initial load, show a full-screen offline state.

Required content:

- Jute logo using the active light or dark asset;
- short title: `Hub not reachable`;
- configured hub URL or host, shortened when needed;
- primary action: `Retry`;
- secondary action later: `Change hub`;
- concise troubleshooting copy.

The startup offline screen should not show a fake dashboard as if it were live. It may show the app shell and theme, but it must clearly state that hub data has not loaded.

Recommended copy:

```text
Hub not reachable
Jute Dash cannot connect to the local hub at 127.0.0.1:8787.
Check that the hub is running, then retry.
```

Do not include stack traces, connection implementation details, or credential references.

## Runtime Disconnect UX

If the display already has a dashboard and the hub disconnects:

- keep the last in-memory dashboard visible;
- mark hub-backed data as stale;
- show a persistent non-modal status ribbon;
- continue local-only UI actions where possible, such as opening menus or leaving chat;
- disable actions that require hub writes, such as sending messages or saving layout edits;
- reconnect automatically with backoff.

The stale dashboard is in-memory only for v1. Durable offline caching is a later explicit design.

Recommended ribbon copy:

```text
Reconnecting to hub
Showing the last dashboard state.
```

When reconnection succeeds:

- refresh hub-backed data;
- clear stale styling;
- show a brief `Connected` confirmation;
- do not interrupt the user with a modal.

## Degraded UX

Use `degraded` when the hub is reachable but a subsystem is partially unavailable.

Examples:

- event stream disconnected while normal polling still works;
- weather provider unavailable;
- agent health checks failing;
- voice provider offline;
- widget pack runtime failed;
- setup is incomplete.

Degraded state appears as a status ribbon only when the issue affects more than one widget or primary user flow. Single-widget failures stay inside that widget.

## Status Ribbon And Banners

The app-level status ribbon is for hub-level or cross-feature problems.

Use it for:

- reconnecting or offline after initial data has loaded;
- degraded hub status;
- setup incomplete when it blocks primary actions;
- event stream disconnected;
- global permission or storage problems.

Do not use it for:

- one widget's missing data;
- one message send failure;
- no chat history;
- disabled optional features.

Ribbon requirements:

- non-modal;
- visible in dashboard, edit, and chat modes;
- accessible with `role="status"` or `aria-live="polite"`;
- concise copy;
- optional retry or settings action;
- no auto-dismiss while the problem remains.

## Data Freshness

Hub-backed payloads should carry enough information for the display to communicate freshness.

Minimum display behavior:

- show `updatedAt` when a widget has meaningful data freshness;
- mark stale widgets when the app is reconnecting or offline;
- avoid showing stale data as if it were live;
- keep stale styling subtle and readable.

Weather already includes `updatedAt` and `status`. Future home, agent, voice, and widget payloads should follow the same pattern.

## Agent Availability

No configured or enabled agent is a setup-needed state, not a danger error.

When no agent is available:

- the dashboard chat button remains visible;
- chat opens to an empty agent state;
- the composer is disabled;
- the chat-history widget says no agent is connected;
- the primary CTA points to future agent setup.

Recommended copy:

```text
No agent connected
Add an A2A agent to start conversations.
```

Agent unavailable states are distinct:

- `disabled`: configured but intentionally off;
- `missing_credentials`: authentication is configured but not available;
- `unsupported_binding`: no supported A2A binding is available;
- `unhealthy`: the hub health check failed;
- `offline`: the agent endpoint cannot be reached;
- `unknown`: health has not been checked yet.

Unavailable agents can be listed, but they cannot be selected for new turns unless the state becomes available.

## Chat Failures

Message send and task failures stay in chat, close to the failed turn.

Rules:

- keep the user's unsent text when a send fails before acceptance;
- show accepted-but-failed task errors as assistant/system rows;
- provide retry when retrying is safe;
- provide cancel when a turn is still in progress;
- disable send while the hub is offline or no agent is available;
- do not show raw transport errors, tokens, or full remote URLs.

Recommended copy:

```text
Message not sent
The hub is reconnecting. Try again when Jute is connected.
```

For future A2A task failures, show the safe public error from the hub, not the raw remote error.

## Widget Error States

All widgets render inside `WidgetFrame` and use the same state vocabulary:

- `loading`: data or widget runtime is starting;
- `empty`: no data exists yet, but nothing is broken;
- `unavailable`: dependency is offline or disabled;
- `error`: the widget failed to render or load required data;
- `permission_required`: user action is needed to grant access;
- `stale`: widget is showing last known data while reconnecting or offline.

Widget failures stay inside the widget frame unless they indicate a hub-level problem.

Widget error states should include:

- short title;
- one-line explanation;
- optional retry or settings action;
- no stack traces;
- no raw payload dumps.

## Setup Gaps

Setup gaps are treated as incomplete configuration.

Examples:

- household setup incomplete;
- no weather location;
- no enabled A2A agent;
- voice provider not selected;
- required widget permission not granted.

Setup-needed states should use calm copy and clear next action. They should not use danger styling unless the setup gap creates a security or safety issue.

## Public Interfaces

Future display types:

```ts
type AppConnectionState =
  | 'starting'
  | 'connected'
  | 'reconnecting'
  | 'offline'
  | 'degraded';

type AgentAvailability =
  | 'available'
  | 'disabled'
  | 'missing_credentials'
  | 'unsupported_binding'
  | 'unhealthy'
  | 'offline'
  | 'unknown';

type WidgetRenderState =
  | 'loading'
  | 'empty'
  | 'unavailable'
  | 'error'
  | 'permission_required'
  | 'stale';

type UserFacingIssue = {
  code: string;
  severity: 'info' | 'warning' | 'error';
  title: string;
  message: string;
  action?: {
    label: string;
    target: 'retry' | 'settings' | 'setup' | 'details';
  };
};
```

Future status APIs:

- `GET /api/v1/status`: setup, store, event stream, agent, widget, voice, provider, and degraded summary.
- `GET /api/v1/agents/{id}/status`: one agent's availability and safe public reason.

Existing reachability API:

- `GET /healthz`: minimal hub process reachability.

Future event types:

- `hub.connected`
- `hub.reconnecting`
- `hub.disconnected`
- `hub.degraded`
- `agent.health_changed`
- `message.failed`
- `widget.error`

## Redaction And Safe Copy

User-facing error payloads are public display data.

Never show:

- raw Go errors;
- stack traces;
- SQL errors;
- token names or credential references;
- raw Agent Card payloads;
- full remote URLs with query strings;
- widget private state;
- raw adapter payloads;
- raw prompt, transcript, or dashboard context payloads.

The hub should map internal failures to stable issue codes and safe public messages. The display should render the safe public fields and keep raw details out of the DOM.

## Implementation Order

1. Document this UX contract.
2. Add display connection state and startup offline screen.
3. Add runtime reconnect ribbon and stale dashboard styling.
4. Add no-agent and agent-unavailable chat states.
5. Add widget-level state components.
6. Add `/api/v1/status`, agent status, and event reconnect semantics.
