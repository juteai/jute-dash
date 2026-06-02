# Widget Skills

## Goal

Widget Skills are the contract that lets agents understand what a Jute widget can see and safely do.

Widgets are not just visual panels. A widget may also expose:

- public context the agent can read;
- prompts that explain how to use that context;
- actions that the hub can safely perform on the widget's behalf;
- ability metadata that helps the agent choose the right widget for a task.

The hub is the only authority that turns widget declarations into agent-visible capabilities. Widgets never call MCP directly, and agents never call widget components directly.

## Relationship To A2A And MCP

A2A remains the conversation and task protocol. Widget Skills do not replace A2A messages or Agent Cards.

MCP is the local pull/tool surface for trusted agents. The MCP Bridge exposes Widget Skills as resources, prompts, and tools after applying hub policy, widget permissions, visibility, and per-agent scopes.

The compact A2A dashboard-context extension may include a summary of currently relevant Widget Skills, but full action invocation goes through MCP or a future hub-approved action API.

## Skill Model

Each widget instance may expose one Widget Skill. Multiple instances of the same widget type can expose separate skills because their state, settings, room, and visible context may differ.

Skill identity:

- `skillId`: stable capability ID, such as `jute.weather.current`.
- `widgetInstanceId`: the dashboard widget instance exposing the skill.
- `widgetKind`: widget type, such as `weather`, `date-time`, or `chat-history`.
- `displayName`: short user-facing name.
- `summary`: hub-reviewed capability summary for agents.

Skill availability is dynamic. A skill is available only when the widget exists, is allowed by policy, has required permissions, and matches the configured visibility policy.

## Manifest Contract

Every widget declares its agent-facing skill in `widget.yaml`, committed alongside the widget source under `widgets/`. The Hub reads all `widget.yaml` files at startup and builds the Widget Skill Registry from them.

```yaml
id: com.example.energy-price
name: Energy Price
version: 1.0.0
sizes: [small, medium, wide]
settings:
  # widget-specific settings schema

agentSkill:
  enabled: true
  skillId: com.example.energy-price.current
  summary: Read current energy tariff and identify cheaper upcoming usage windows.
  visibilityPolicy: visible_or_focused
  context:
    fields:
      - name: tariffName
        type: string
        description: Current tariff display name.
      - name: currentPrice
        type: number
        unit: GBP/kWh
        description: Current import electricity price.
      - name: nextCheapWindow
        type: string
        description: Next known cheaper usage window.
  actions:
    - id: refresh
      title: Refresh tariff data
      description: Ask the widget to refresh its tariff data through the hub.
      sideEffect: read
      requiresConfirmation: false
  prompts:
    - id: energy_usage_advice
      title: Energy usage advice
      purpose: Guide the agent when answering questions about cheaper appliance usage times.
```

`agentSkill` is optional. Visual-only widgets should omit it or set `enabled` to false. Agent-visible widgets must include a complete `agentSkill` block.

Required `agentSkill` fields:

- `enabled`
- `skillId`
- `summary`
- `requiredPermissions`
- `visibilityPolicy`
- `context.fields`

Optional `agentSkill` fields:

- `actions`
- `prompts`

Field rules:

- `skillId` uses reverse-DNS or `jute.*` naming and must be stable across versions.
- `summary` is a short capability description, not an instruction to the model.
- `requiredPermissions` must be a subset of the widget's manifest permissions.
- `visibilityPolicy` is `visible`, `focused`, or `visible_or_focused` for v1.
- `context.fields`, `actions`, and `prompts` must use stable IDs because agents may learn or cache them.

## Context

Skill context is the safe public state an agent may read.

Rules:

- context fields must be explicitly declared;
- each value must be produced by the hub or by hub-approved widget state;
- context includes freshness metadata where possible;
- hidden widgets do not expose context unless a future background policy explicitly allows it;
- exact presence, private notes, raw adapter payloads, secrets, camera frames, microphone audio, browser storage, and undeclared fields are never context.

`contextPolicy.publicFields` is replaced by `agentSkill.context.fields`. Existing POC code may still use public fields internally until the new contract is implemented, but new architecture work should use Widget Skills.

Supported context field types for v1:

- `string`
- `number`
- `integer`
- `boolean`
- `enum`
- `datetime`
- `duration`
- `object`
- `array`

Each context field must include:

- `name`
- `type`
- `description`

Optional field metadata:

- `unit`
- `enumValues`
- `nullable`
- `freshness`
- `sensitivity`

Only `sensitivity: public` fields may be exposed to agents. If omitted, the hub treats the field as public only when it is declared in `agentSkill.context.fields` and produced by hub-approved state.

## Actions

Actions are hub-mediated operations a widget skill can perform.

Action side-effect levels:

- `read`: refresh, recalculate, or retrieve widget-owned data.
- `display`: focus, highlight, notify, expand, or otherwise change the display.
- `configure`: change widget settings or layout after explicit user approval.
- `home_action`: future smart-home action request requiring hub policy and confirmation.

Action rules:

- every action must have a stable ID and JSON Schema input/output;
- display, configure, and home actions require opt-in scopes;
- actions execute through the hub, not through direct MCP-to-widget calls;
- actions return safe public results;
- failed actions return recoverable safe errors;
- high-impact actions require confirmation even if an agent has the scope.

For the POC, only `read` and low-risk `display` actions are in scope.

Required action fields:

- `id`
- `title`
- `description`
- `sideEffect`
- `requiresConfirmation`
- `inputSchema`
- `outputSchema`

Action IDs are local to the skill and should be stable. The MCP Bridge invokes actions as `{ skillId, widgetInstanceId?, actionId, arguments }` through the generic `jute_skill_invoke_action` tool.

## Prompts

Widget Skill prompts are reusable guidance fragments for agents.

Prompt rules:

- hub-authored prompts are trusted project guidance;
- prompt declarations in `widget.yaml` are reviewed as part of the contribution PR — they are trusted once merged;
- MCP prompt output should be generated from stable hub templates;
- prompt text must not include secrets, hidden state, or private widget data;
- prompts guide the agent but never grant permission.

For third-party widgets, the manifest may declare prompt purpose and expected use, but the hub decides the final prompt content exposed to agents.

Required prompt fields:

- `id`
- `title`
- `purpose`

Optional prompt fields:

- `arguments`
- `examples`

Prompt declarations must describe intended use. They must not contain raw model instructions that override Jute policy, ask the model to ignore permissions, or imply hidden context exists.

## MCP Mapping

The MCP Bridge exposes Widget Skills through generic skill resources and tools.

Resources:

- `jute://skills`: available Widget Skills for the connected agent.
- `jute://skills/{skillId}`: skill definition, visible widget instances, context schema, action summaries, and prompt summaries.
- `jute://skills/{skillId}/context`: current public context for a skill.
- `jute://widgets/{widgetInstanceId}/skill`: mapping from a widget instance to its exposed skill.

Tools:

- `jute_skill_list`: list available Widget Skills.
- `jute_skill_read_context`: read current public context for a skill or widget instance.
- `jute_skill_invoke_action`: invoke a declared action through the hub.
- `jute_skill_prompt_get`: get hub-approved prompt guidance for a skill.

Hub-level MCP tools, such as display notification or focus, may still exist. Widget-owned behavior should be exposed through `jute_skill_invoke_action`.

## Initial Built-In Skills

Initial built-in widgets should expose these skills:

- `jute.date_time.current`: read date, time, timezone, locale, and display format.
- `jute.weather.current`: read weather condition, temperature, humidity, wind, sunrise, sunset, and freshness.
- `jute.chat_history.current`: read available agents, selected agent, recent conversation summaries, and conversation availability.

These skills should be instance-aware even if the first dashboard only has one instance of each widget.

## Permissions And Scopes

Widget permission:

- `agent:skill`: allow the widget to expose a Widget Skill to hub-approved agents.

MCP scopes:

- `skills:read`: list skills and read skill definitions.
- `skills:context_read`: read current public skill context.
- `skills:action_invoke`: invoke safe skill actions.
- `skills:prompt_read`: read hub-approved skill prompts.

The hub may map these scopes onto broader POC scopes while the implementation is small, but the public contract should use skill scopes.

## Validation Failure Behavior

Invalid Widget Skills are disabled, not partially exposed.

The hub should reject or disable a skill when:

- `skillId` is missing, unstable, or duplicated for the same widget instance;
- required permissions are not declared by the widget;
- the visibility policy is unsupported;
- context fields are missing required metadata;
- action schemas are invalid JSON Schema;
- action side-effect levels are unsupported;
- prompts contain raw secrets or policy-bypassing instructions;
- the widget is hidden or lacks the required permission grant.

When a skill is disabled, MCP resources omit it and A2A dashboard context does not mention it. The widget may still render visually if its visual manifest is valid.

## Safety Rules

- The hub validates and normalizes every skill before exposing it.
- The hub generates MCP tool descriptions from stable templates.
- `widget.yaml` summary and prompt text must not contain raw model instructions that override Jute policy or bypass permissions.
- Agents cannot access private widget state, hidden widget state, raw credentials, raw smart-home adapter payloads, camera frames, microphone audio, or browser storage.
- Widgets cannot grant themselves agent access; users or trusted config grant permissions.
- Agents should treat skills as available capabilities, not as user intent.

## Implementation Order

1. Document the Widget Skill contract.
2. Add `widget.yaml` manifests for `date-time`, `weather`, and `chat-history`.
3. Build the Hub Widget Skill Registry to read and validate all `widget.yaml` files at startup.
4. Build the MCP Bridge around generic skill resources and tools.
5. Add per-agent skill scopes.
6. Add approval-gated configure and home-action skills later.
