# Hub

The Go service that is the authoritative runtime for Jute Dash. It owns all durable state, agent communication, and the MCP Bridge. The Display is a consumer of Hub APIs, never a peer.

## Language

### Configuration and persistence

**Bootstrap Config**:
YAML or JSON config loaded at startup to seed the Hub's initial state. Treated as import/export format, not runtime truth. Once the Hub has persisted state, SQLite is authoritative.
_Avoid_: config file, settings file, yaml config (when meaning the runtime source of truth)

**Runtime Store**:
The SQLite database that is the authoritative source of truth for all durable Hub state after first boot.
_Avoid_: database, DB, store (unqualified)

**Secret Reference**:
A pointer to a credential stored outside the config (e.g. an environment variable name). Raw secrets are never stored in YAML/JSON config or exposed through the Hub API.
_Avoid_: token, credential (when stored inline)

### Agent communication

**A2A**:
The protocol the Hub uses to communicate with Agents. A2A is the conversation and task protocol. The Hub is an A2A client; Agents are A2A servers.
_Avoid_: agent protocol, agent API

**Protocol Binding**:
The specific A2A transport variant selected for an Agent (e.g. `JSONRPC`, `HTTP+JSON`). The Hub selects the first binding supported by both the Agent Card and Jute.
_Avoid_: transport, interface

### MCP Bridge

**MCP Scope**:
A per-Agent permission that controls which parts of the MCP Bridge an Agent can access (e.g. `dashboard:read`, `skills:context_read`). Configured per Agent in the Hub config.
_Avoid_: MCP permission, MCP access level

**Widget Skill Registry**:
The Hub's internal index of all available Widget Skills, compiled from native Go widget registrations at startup. The MCP Bridge serves skills from this registry after applying policy and per-Agent scopes.
_Avoid_: skill store, capability registry
