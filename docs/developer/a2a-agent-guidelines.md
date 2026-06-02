# A2A Agent Guidelines

## Overview

Jute Dash works with bring-your-own A2A agents. Agents remain independent A2A servers. Jute acts as the A2A client, local orchestrator, dashboard context provider, and credential boundary.

Agents should follow the current [A2A specification](https://a2a-protocol.org/latest/specification/) and publish an Agent Card.

## Agent Card Requirements

Agents should provide:

- `name`, `description`, and `version`;
- `supportedInterfaces` with at least one standard binding;
- `capabilities`, including streaming support when available;
- `skills` with useful names, descriptions, tags, examples, and input/output modes;
- `securitySchemes` and `securityRequirements` when authentication is required.

Jute prefers protocol bindings in this order:

1. `JSONRPC`
2. `HTTP+JSON`
3. `GRPC`

The first implemented send path is blocking JSON-RPC A2A 1.0. Agents used with the current dashboard chat should expose a `JSONRPC` interface with `protocolVersion: "1.0"` and implement `SendMessage`.

Example:

```json
{
  "name": "House Concierge",
  "description": "Helps with household summaries, reminders, and routing.",
  "version": "1.0.0",
  "supportedInterfaces": [
    {
      "url": "https://agent.example.com/a2a/v1",
      "protocolBinding": "JSONRPC",
      "protocolVersion": "1.0"
    }
  ],
  "capabilities": {
    "streaming": true,
    "extensions": [
      {
        "uri": "https://jute.dev/a2a/extensions/dashboard-context/v1",
        "description": "Accepts redacted Jute dashboard context in message metadata.",
        "required": false
      }
    ]
  },
  "defaultInputModes": ["text/plain"],
  "defaultOutputModes": ["text/plain", "application/json"],
  "skills": [
    {
      "id": "home-summary",
      "name": "Home Summary",
      "description": "Summarizes visible household dashboard state.",
      "tags": ["home", "summary", "dashboard"],
      "examples": ["What needs attention at home?"]
    }
  ]
}
```

## Dashboard-Context Extension

Jute defines this optional extension:

```text
https://jute.dev/a2a/extensions/dashboard-context/v1
```

Agents must declare support in their Agent Card before Jute sends dashboard context. The extension is optional. If it is absent, the agent receives the user's normal A2A message without dashboard context.

The context arrives in A2A message metadata and includes:

- display profile;
- locale and timezone;
- interaction mode;
- visible widget IDs;
- focused widget ID when present;
- widget titles, kinds, sizes, and public context fields.

Agents must treat context as advisory, not authoritative. The hub remains responsible for device control, permissions, and action execution.

When Jute activates the extension, it sends `A2A-Extensions: https://jute.dev/a2a/extensions/dashboard-context/v1` and places the redacted context under the same URI key in message metadata.

## Response Contract

Agents must return only user-facing assistant output in A2A message text and text artifacts.

Do not include:

- private reasoning;
- scratchpad or hidden chain-of-thought text;
- planning notes;
- tool-selection notes;
- function-call commentary;
- statements such as "I should call a tool" or "no need to call tools";
- raw tool JSON unless the user explicitly asks for diagnostic output.

Recommended base instruction:

```text
Return only the final user-facing answer. Never include private reasoning, scratchpad text, analysis, tool-selection notes, or function-call plans in assistant output. Use Jute tools only when the user's request requires dashboard context or a declared Jute action. For simple greetings or ordinary chat, do not call tools. When tools are useful, choose the narrowest relevant tool and do not invent capabilities not returned by tool discovery.
```

Jute defensively filters common reasoning wrappers such as `<think>...</think>`, but agents should not rely on the hub to repair unsafe output. The correct behavior is to keep private reasoning private and send only the final answer over A2A.

## Privacy Expectations

Agents must not assume they are entitled to hidden or private dashboard data. If context is missing, ask the user or continue with the visible user message.

Agents should not request:

- raw credentials;
- hidden widget state;
- precise presence data;
- raw camera or microphone data;
- browser storage;
- adapter debug payloads.

## Streaming

Agents should support A2A streaming when possible. Streaming lets Jute show progress, status, and artifacts on the dashboard while work is ongoing.

Long-running tasks should emit useful task status messages. Artifacts should use supported output modes and avoid leaking sensitive data in titles, filenames, or metadata.

## Authentication

Declare authentication in the Agent Card, but do not embed static secrets in it. Jute obtains credentials out of band and sends them according to the selected A2A binding.

Agents should support credential rotation and should reject unauthorized requests with standard A2A-compatible errors.

## Graceful Degradation

Agents should continue to work when:

- the dashboard-context extension is not activated;
- a widget is hidden or unavailable;
- Jute selects a non-preferred binding;
- streaming is unavailable;
- context fields are redacted.
