# Chat History Widget

A native Jute Dash widget that lists the recent chat interactions with active A2A agents in the household. It also displays the current connection status and protocols (e.g. JSONRPC) of the active agent, and features a one-click button to open a conversation dialogue.

## Widget Details

- **Kind (Identifier)**: `chat-history`
- **Default Size**: `medium` (2 columns, 2 rows)
- **Minimum Size**: 1x1
- **Overflow Policy**: `scroll`
- **Allow Multiple**: `false`

## Usage and Configuration

The `chat-history` widget does not require any custom settings. It dynamically tracks active household A2A agents and your conversation event streams managed by the Go hub.

### Example YAML Configuration (`jute.yaml`)

```yaml
dashboard:
  widgets:
    - id: "chat-history-1"
      type: "chat-history"
      title: "Assistant Chat"
      x: 0
      y: 1
      w: 2
      h: 2
      visible: true
```
