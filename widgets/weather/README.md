# Weather Widget

A native Jute Dash widget that provides real-time local weather reports. It integrates with Open-Meteo to display the current temperature, apparent temperature ("feels like"), humidity percentage, wind speed, wind speed unit, conditions, and dynamic weather condition icons.

## Widget Details

- **Kind (Identifier)**: `weather`
- **Default Size**: `wide` (2 columns, 1 row)
- **Minimum Size**: 1x1
- **Overflow Policy**: `clip`
- **Allow Multiple**: `false`

## Usage and Configuration

You can customize the location coordinates, names, and measurement units via settings in your YAML dashboard layout block.

### Settings Schema

| Key | Type | Description | Default |
| :--- | :--- | :--- | :--- |
| `location` | `string` | Human-readable location name | `"London"` |
| `latitude` | `float64` | Location latitude coordinate | `51.5072` |
| `longitude` | `float64` | Location longitude coordinate | `-0.1276` |
| `temperature-unit` | `string` | Temperature scale (`celsius` or `fahrenheit`) | `"celsius"` |
| `wind-speed-unit` | `string` | Wind speed scale (`kmh`, `mph`, `ms`, or `kn`) | `"kmh"` |

### Example YAML Configuration (`jute.yaml`)

```yaml
dashboard:
  widgets:
    - id: "weather-1"
      type: "weather"
      title: "Local Weather"
      x: 2
      y: 0
      w: 2
      h: 1
      visible: true
      settings:
        location: "New York"
        latitude: 40.7128
        longitude: -74.0060
        temperature-unit: "fahrenheit"
        wind-speed-unit: "mph"
```
