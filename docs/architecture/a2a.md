# A2A Compatibility

## Compatibility Target

Jute targets A2A 1.0 and treats A2A as the external agent interoperability layer. Jute is an A2A client and local orchestrator. Remote or local agents are A2A servers.

The optional [MCP Bridge](mcp-bridge.md) is complementary. A2A remains the conversation and task protocol. MCP is a richer local pull/tool surface for trusted agents that can connect to the hub.

Primary references:

- [A2A specification](https://a2a-protocol.org/latest/specification/)
- [A2A extensions](https://a2a-protocol.org/latest/topics/extensions/)
- [A2A agent discovery](https://a2a-protocol.org/latest/topics/agent-discovery/)
- [A2A custom protocol bindings](https://a2a-protocol.org/latest/topics/custom-protocol-bindings/)

## Discovery

Agents are registered by direct configuration, registry lookup, or well-known Agent Card URL. Public agents should normally expose:

```text
https://{agent-domain}/.well-known/agent-card.json
```

The hub resolves the Agent Card, validates it, caches it, and records:

- identity and provider metadata;
- `supportedInterfaces`;
- capabilities, including streaming and extensions;
- skills and supported input/output modes;
- security requirements;
- icon and documentation links.

## Protocol Binding Selection

Jute does not create a custom protocol binding for v1. It selects from standard A2A bindings in this order:

1. `JSONRPC`
2. `HTTP+JSON`
3. `GRPC`

The hub reads the Agent Card `supportedInterfaces` list in preference order and chooses the first interface that both the agent and Jute support. If no compatible binding exists, the agent is visible as incompatible and cannot be selected for tasks.

## Credentials

Agent Card security metadata describes requirements, but credentials are supplied out of band. Jute config stores credential references, not raw secrets.

v1 credential sources:

- environment variable reference;
- local development token file outside repo paths when explicitly configured.

Future credential sources:

- OS keyring;
- OAuth device flow;
- mTLS identity;
- household pairing service.

The display never receives raw agent credentials.

## Messaging And Streaming

User turns in the display become A2A send-message requests through the hub. The hub records the local task mapping and forwards task status and artifact updates to displays over `/api/v1/events`.

Voice turns follow the same path after transcription. The Jute Voice Service sends final transcripts to the hub; the hub sends text turns to A2A agents. Raw microphone audio, pre-roll buffers, and partial transcripts are not sent to A2A agents.

For agents that support streaming, Jute uses the streaming operation for responsive dashboard updates. For agents without streaming, the hub polls task state or waits for push notification support in later releases.

Current implementation status:

- JSON-RPC A2A 1.0 blocking chat is implemented with `SendMessage`.
- JSON-RPC A2A 1.0 streaming chat is implemented with `SendStreamingMessage` when the selected Agent Card advertises streaming support.
- The hub sends `A2A-Version: 1.0`.
- The hub persists conversations, messages, returned `contextId` values, and latest task IDs in SQLite so follow-up turns can continue the same A2A context.
- The display uses `/api/v1/conversations`, `/api/v1/conversations/{id}/turns`, and `/api/v1/events` for durable chat history and live updates.
- Polling and task subscriptions remain future work.

## Agent Card Caching

The hub honors standard HTTP caching headers when fetching Agent Cards:

- use `ETag` with `If-None-Match` when available;
- use `Last-Modified` with `If-Modified-Since` when available;
- use a conservative default cache duration when no caching headers are present;
- refresh manually when the user asks or when a task fails due to capability mismatch.

Cached cards live in SQLite with fetch time, expiry, ETag, content hash, and selected interface.

Current implementation status:

- The hub fetches configured Agent Cards, selects A2A 1.0 `JSONRPC` first, and caches the selected interface, skills, streaming flag, dashboard-context support, and safe card status in SQLite.
- The development `make dev-a2a` path uses the lightweight `examples/agents/a2a-v1-dev` fixture.

## Jute Dashboard-Context Extension

Jute-specific dashboard context uses an optional A2A extension:

```text
https://jute.dev/a2a/extensions/dashboard-context/v1
```

Agents declare support in their Agent Card capabilities. Jute activates the extension only for agents that declare support.

The extension payload travels in A2A message metadata. It does not add fields to core A2A types and does not change protocol binding behavior.

Payload shape:

```json
{
  "schema": "https://jute.dev/a2a/extensions/dashboard-context/v1",
  "display": {
    "deviceId": "kitchen-display",
    "profile": "wall-display",
    "locale": "en-GB",
    "timezone": "Europe/London",
    "interactionMode": "touch"
  },
  "dashboard": {
    "layoutId": "morning",
    "visibleWidgetIds": ["clock", "weather", "energy"],
    "focusedWidgetId": "energy"
  },
  "widgets": [
    {
      "id": "energy",
      "kind": "energy.summary",
      "title": "Energy",
      "size": "medium",
      "publicContext": {
        "currentPrice": "21.2p/kWh",
        "nextCheapWindow": "22:30"
      }
    }
  ]
}
```

The hub redacts or omits:

- hidden widgets;
- private widget state;
- raw smart-home payloads not explicitly exposed;
- secrets and credential references;
- camera frames, audio, transcripts, and precise presence data unless the user grants a future explicit permission.

## MCP Bridge Relationship

A2A dashboard-context metadata is the compact push path. It is appropriate for remote or cloud agents and for agents that do not connect to local MCP.

The MCP Bridge is the richer pull path for trusted local agents. It exposes safe dashboard and widget context as MCP resources and safe hub-mediated actions as MCP tools. It does not replace A2A task messaging and does not create a custom A2A protocol binding.

Remote agents do not receive MCP credentials automatically. If an agent cannot use MCP, it still receives the user's turn through standard A2A and may receive compact dashboard context only when it declares the Jute A2A extension.

## Graceful Degradation

If an agent does not support the Jute dashboard-context extension, it still receives the user's text/audio turn through standard A2A. The display can show that the agent is responding without screen context.

If an agent also lacks MCP access, it must proceed without local dashboard resources or Jute tools.
