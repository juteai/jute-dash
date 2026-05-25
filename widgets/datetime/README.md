# Date & Time Widget

A native Jute Dash widget that displays the current local time, full formatted date, timezone name, and weekday. It dynamically updates every second and formats the display based on the household's global locale and timezone settings.

## Widget Details

- **Kind (Identifier)**: `date-time`
- **Default Size**: `wide` (2 columns, 1 row)
- **Minimum Size**: 1x1
- **Overflow Policy**: `clip` (No scrollbars, clean edge clipping)
- **Allow Multiple**: `false` (Only one clock widget per household profile)

## Usage and Configuration

The `date-time` widget does not require any custom settings. It reads and synchronizes directly with your Jute Hub's global home configuration (locale and timezone).

### Example YAML Configuration (`jute.yaml`)

```yaml
dashboard:
  widgets:
    - id: "datetime-1"
      type: "date-time"
      title: "Local Time"
      x: 0
      y: 0
      w: 2
      h: 1
      visible: true
```
