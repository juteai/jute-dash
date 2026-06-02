# ADR 0002: Contribution Model for Widgets and Themes

Widgets and Themes are contributed via fork and PR to the monorepo (`widgets/` and `themes/` respectively), not distributed as installable packs. There is no sandboxed iframe runtime and no Widget Pack format.

The earlier design specified Widget Packs — self-contained bundles rendered in sandboxed iframes, with a postMessage SDK for host/widget communication. That model was discarded in favour of native Svelte components committed directly to the repo. The trade-off: we lose runtime isolation and third-party distribution, but gain a simpler trust model, no postMessage contract to maintain, and full access to the Svelte component ecosystem without a sandbox boundary.

Widget Skills (the agent-facing capability declarations in `widget.yaml`) survive this change. The Hub reads them statically at startup and surfaces them through the MCP Bridge. The iframe/postMessage machinery that previously carried skill data is removed entirely.
