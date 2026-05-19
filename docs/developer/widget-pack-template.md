# Widget Pack Template

Use this as the starting point for custom Jute widgets. A Widget Pack is static browser content plus a `widget.json` manifest. It can be built with any frontend framework as long as the final entrypoint speaks the Widget SDK message protocol.

## Directory

```text
com.example.energy-price/
  widget.json
  index.html
  README.md
  assets/
```

## widget.json

```json
{
  "id": "com.example.energy-price",
  "name": "Energy Price",
  "version": "0.1.0",
  "entry": "index.html",
  "permissions": ["home:read", "widget:state", "agent:skill"],
  "dataNeeds": ["energy.current_tariff", "home.locale"],
  "sizes": ["small", "medium", "wide"],
  "agentSkill": {
    "enabled": true,
    "skillId": "com.example.energy-price.current",
    "summary": "Read current energy tariff and identify cheaper upcoming usage windows.",
    "requiredPermissions": ["agent:skill", "home:read"],
    "visibilityPolicy": "visible_or_focused",
    "context": {
      "fields": [
        {
          "name": "tariffName",
          "type": "string",
          "description": "Current tariff display name.",
          "sensitivity": "public"
        },
        {
          "name": "currentPrice",
          "type": "number",
          "unit": "GBP/kWh",
          "description": "Current import electricity price.",
          "sensitivity": "public"
        },
        {
          "name": "nextCheapWindow",
          "type": "string",
          "description": "Next known cheaper usage window.",
          "sensitivity": "public"
        }
      ]
    },
    "actions": [
      {
        "id": "refresh",
        "title": "Refresh tariff data",
        "description": "Refresh tariff data through the hub-approved data source.",
        "sideEffect": "read",
        "requiresConfirmation": false,
        "inputSchema": {
          "type": "object",
          "additionalProperties": false
        },
        "outputSchema": {
          "type": "object",
          "properties": {
            "status": { "type": "string" },
            "updatedAt": { "type": "string" }
          },
          "required": ["status"]
        }
      }
    ],
    "prompts": [
      {
        "id": "energy_usage_advice",
        "title": "Energy usage advice",
        "purpose": "Guide an agent when answering questions about cheaper appliance usage times."
      }
    ]
  }
}
```

Remove `agentSkill` and `agent:skill` if the widget is visual-only and should not be visible to agents.

## index.html

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Energy Price Widget</title>
    <style>
      :root {
        color-scheme: light dark;
        font-family: system-ui, sans-serif;
      }

      body {
        margin: 0;
        padding: 12px;
        background: transparent;
        color: CanvasText;
      }

      .value {
        font-size: 1.75rem;
        font-weight: 700;
      }

      .muted {
        color: color-mix(in srgb, CanvasText 65%, transparent);
      }
    </style>
  </head>
  <body>
    <main aria-live="polite">
      <div class="muted">Energy price</div>
      <div class="value" id="value">Waiting for data</div>
      <div class="muted" id="detail">The widget is loading.</div>
    </main>

    <script type="module">
      const widgetId = 'com.example.energy-price';
      let permissions = [];

      function requestId() {
        return crypto.randomUUID();
      }

      function post(type, payload = {}) {
        window.parent.postMessage(
          {
            type,
            widgetId,
            requestId: requestId(),
            payload
          },
          '*'
        );
      }

      function renderTariff(payload) {
        const price = payload?.currentPrice;
        const tariff = payload?.tariffName ?? 'Unknown tariff';
        const nextWindow = payload?.nextCheapWindow ?? 'No cheaper window known';

        document.querySelector('#value').textContent =
          typeof price === 'number' ? `${price.toFixed(2)} GBP/kWh` : 'Unavailable';
        document.querySelector('#detail').textContent = `${tariff}. ${nextWindow}.`;
      }

      window.addEventListener('message', (event) => {
        const message = event.data;
        if (!message || typeof message.type !== 'string') return;

        switch (message.type) {
          case 'jute.host.permissions':
            permissions = message.payload?.permissions ?? [];
            if (permissions.includes('home:read')) {
              post('jute.widget.request_data', {
                topics: ['energy.current_tariff', 'home.locale']
              });
            }
            break;
          case 'jute.host.data':
            renderTariff(message.payload?.data?.energy?.current_tariff);
            post('jute.widget.update_state', {
              publicContext: {
                tariffName: message.payload?.data?.energy?.current_tariff?.tariffName,
                currentPrice: message.payload?.data?.energy?.current_tariff?.currentPrice,
                nextCheapWindow: message.payload?.data?.energy?.current_tariff?.nextCheapWindow
              }
            });
            break;
          case 'jute.host.error':
            document.querySelector('#value').textContent = 'Unavailable';
            document.querySelector('#detail').textContent =
              message.payload?.message ?? 'The widget could not load data.';
            break;
        }
      });

      post('jute.widget.ready', {
        supportedSizes: ['small', 'medium', 'wide']
      });
    </script>
  </body>
</html>
```

## README.md

```markdown
# Energy Price Widget

Shows the current electricity import tariff and the next known cheaper usage window.

## Permissions

- `home:read`: reads normalized energy tariff data from the hub.
- `widget:state`: stores non-secret widget display state.
- `agent:skill`: exposes safe public tariff context to agents.

## Agent Skill

Skill ID: `com.example.energy-price.current`

Public context:

- `tariffName`
- `currentPrice`
- `nextCheapWindow`

Actions:

- `refresh`: low-risk read action that refreshes tariff data.

No secrets, raw adapter payloads, precise presence data, camera frames, microphone audio, or browser storage are exposed.

## Supported Sizes

- `small`
- `medium`
- `wide`

## Verification

- Loads inside `WidgetFrame`.
- Handles missing tariff data.
- Handles permission denial.
- Respects reduced motion and high contrast.
- Does not call the hub API, MCP, or A2A directly.
```

## Contribution Notes

Before opening a PR, confirm:

- the manifest validates against the widget contract;
- requested permissions are minimal;
- all agent-visible fields are declared in `agentSkill.context.fields`;
- actions are safe, schema-defined, and use the correct side-effect level;
- no manifest, README, source, or asset contains raw secrets;
- the widget has useful empty, loading, unavailable, error, permission-required, and stale states.
