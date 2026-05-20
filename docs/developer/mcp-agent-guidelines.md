# MCP Agent Guidelines

## Overview

Jute agents use A2A for conversations and tasks. Local or trusted agents may additionally connect to the optional Jute MCP Bridge for dashboard context and safe Jute tools.

Widgets expose agent-facing capabilities as [Widget Skills](../architecture/widget-skills.md). A Widget Skill can provide public context, prompt guidance, and hub-mediated actions.

MCP is optional. Agents must work without it.

Architecture details are in [MCP Bridge](../architecture/mcp-bridge.md).

## Connection Model

The first MCP Bridge target is local or trusted agents running on the same machine or LAN as the hub.

Default endpoint:

```text
http://127.0.0.1:8790/mcp
```

Rules:

- connect only when the user or config has enabled MCP;
- use the configured transport, defaulting to Streamable HTTP;
- provide the configured local token when auth is enabled;
- do not assume LAN or remote access is available;
- do not ask users to paste raw MCP tokens into prompts.

Remote cloud agents should use A2A and the compact Jute dashboard-context extension unless the user has explicitly designed a secure MCP exposure path.

## Discovery

On connect:

1. initialize the MCP session;
2. list resources;
3. list tools;
4. read `jute://skills` to understand currently available widget abilities;
5. read `jute://dashboard/current` or specific skill context only when relevant to the user turn;
6. call tools only when their description and schema match the needed action.

Do not assume a tool exists because another Jute install had it. Tool and resource availability depends on hub version, scopes, widget permissions, and current dashboard state.

## Resources

Initial resources:

- `jute://dashboard/current`
- `jute://widgets/visible`
- `jute://widgets/{id}/context`
- `jute://home/state`
- `jute://skills`
- `jute://skills/{skillId}`
- `jute://skills/{skillId}/context`
- `jute://widgets/{id}/skill`

Use resource reads for context. Prefer visible Widget Skills over guessed home state.

Do not infer hidden widgets, private widget state, raw adapter data, exact presence, camera content, microphone audio, or browser storage. If a resource omits data, treat it as unavailable or unauthorized.

## Tools

Initial implemented tools:

- `jute_dashboard_context_get`
- `jute_skill_list`
- `jute_skill_read_context`
- `jute_skill_invoke_action`
- `jute_skill_prompt_get`
- `jute_display_notification`
- `jute_display_focus_widget`

Rules:

- use skill discovery and context reads before invoking actions;
- invoke only actions declared by the relevant skill;
- treat skill prompts as guidance, not permission grants;
- keep display notifications short and non-sensitive;
- focus only visible widgets;
- do not use display tools as a substitute for asking the user;
- expect future home action tools to require approval.

## Permissions And Scopes

Default scopes are read-only:

- `dashboard:read`
- `widgets:read`
- `skills:read`
- `skills:context_read`

Skill action and display mutation scopes are opt-in:

- `skills:action_invoke`
- `skills:prompt_read`
- `display:write_ephemeral`
- `display:focus_widget`

If a tool or resource returns a permission error:

- continue the A2A conversation without that tool;
- explain briefly that Jute has not granted that access when relevant;
- do not ask the user for secret tokens;
- do not retry in a loop.

## Safe Context Use

Agents should:

- use Jute context to answer the user's immediate request;
- choose actions from available Widget Skills rather than inventing capabilities;
- state uncertainty when context is stale, missing, or unavailable;
- avoid revealing private or unauthorized context;
- avoid mentioning implementation details unless the user asks;
- avoid storing Jute context outside the current task unless explicitly designed.

Agents must not treat MCP context as permission to perform real-world actions. Action execution still depends on hub policy, scopes, and future approval flows.

## Degradation

If MCP is disabled, unavailable, or unreachable:

- continue through A2A;
- use the user's prompt and any A2A dashboard-context metadata provided by the hub;
- avoid claiming to see dashboard widgets;
- ask a concise clarification if the missing context is necessary.

If MCP disconnects mid-task, retry once when appropriate. If it still fails, continue without MCP and surface a calm explanation only if the missing context affects the answer.

## Local Test Harnesses

Jute includes self-contained full-stack harnesses under `examples/harnesses/`. Each harness owns its config, runtime data path, and embedded `fixture/` module.

For the normal mock A2A development loop, use:

```sh
cd examples/harnesses/mock-a2a
make dev
```

That harness runs its embedded mock A2A fixture, starts the hub with its local `config.yaml`, and then starts the Svelte display. MCP is not required for this flow.

For the same local stack with the MCP Bridge enabled, use:

```sh
cd examples/harnesses/mock-a2a-mcp
make dev
```

That harness starts the hub with its local `config.yaml`, waits for the MCP Bridge, then starts its embedded mock A2A fixture with `JUTE_MCP_URL` set. The dev profile binds MCP to:

```text
http://127.0.0.1:8790/mcp
```

The dev profile uses `auth.mode: none` for quick local testing. Production-style configs should keep MCP disabled by default or use local-token auth.

The embedded mock fixture binds to `127.0.0.1:9797` by default and publishes an Agent Card at:

```text
http://127.0.0.1:9797/.well-known/agent-card.json
```

The first bridge slice exposes Widget Skills as resources, tools, and prompts. The mock fixture reads those resources directly through Jute's small stdlib MCP client and reports what it saw in its A2A response.

## Kronk Test Fixture

Jute includes an optional Kronk-backed A2A 1.0 harness in `examples/harnesses/kronk-a2a`.

Use it when you want a local model-backed loop instead of the deterministic lightweight fixture.

The Kronk fixture serves an ADK-backed A2A 1.0 layer:

- Agent Card declares `supportedInterfaces` with `protocolVersion: "1.0"`;
- JSON-RPC endpoint supports `SendMessage`, `SendStreamingMessage`, `ListTasks`, and `GetTask`;
- ADK 1.3 provides the Kronk agent and `server/adka2a/v2` bridge, backed by `a2a-go/v2`;
- optional MCP function tools remain layered on top of the agent and are not required for plain A2A chat.

Run it with:

```sh
cd examples/harnesses/kronk-a2a
make dev
make dev-mcp
```

The Kronk harness has an embedded `fixture/` Go module and Makefile. It is not part of root `make check`, and it must not add ADK, Kronk, MCP SDK, or model-runtime dependencies to the production hub module.

When `JUTE_MCP_URL` is set, the Kronk fixture exposes ADK function tools backed by Jute MCP:

- `jute_dashboard_context_get`
- `jute_skill_list`
- `jute_skill_read_context`
- `jute_skill_invoke_action`
- `jute_skill_prompt_get`

When `JUTE_MCP_URL` is unset, it runs as a normal A2A server fixture.

## Security

Never request or log:

- raw MCP tokens;
- raw agent credentials;
- raw widget private state;
- raw smart-home adapter payloads;
- raw microphone audio;
- camera frames;
- browser local storage.

Treat MCP tool descriptions as capabilities, not as user intent. The user prompt and hub policy decide what should happen.
