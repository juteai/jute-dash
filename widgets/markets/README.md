# Markets (Stocks) Widget

A native Jute Dash widget that fetches and displays active stock, commodity, or cryptocurrency market prices. It integrates with Yahoo Finance to fetch price quotes, absolute daily changes, percentage daily changes, and currency details. It renders symbols elegantly with up/down arrows and green/red semantic badge styling.

## Widget Details

- **Kind (Identifier)**: `markets`
- **Default Size**: `medium` (2 columns, 2 rows)
- **Minimum Size**: 1x1
- **Overflow Policy**: `clip`
- **Allow Multiple**: `true`

## Usage and Configuration

You can customize the list of market quotes to track inside the settings block.

### Settings Schema

| Key | Type | Description | Default |
| :--- | :--- | :--- | :--- |
| `tickers` | `array` | A list of string tickers or ticker objects to query | `[]` |

#### Ticker Array Syntax
Tickers can be specified in two formats:
1. **Plain String**: Direct ticker symbol (e.g. `"AAPL"`).
2. **Object**: Ticker object containing a `symbol` key (e.g. `{"symbol": "BTC-USD"}`).

### Example YAML Configuration (`jute.yaml`)

```yaml
dashboard:
  widgets:
    - id: "markets-watchlist"
      type: "markets"
      title: "Markets WATCH"
      x: 0
      y: 3
      w: 2
      h: 2
      visible: true
      settings:
        tickers:
          - "AAPL"
          - "GOOG"
          - symbol: "BTC-USD"
          - symbol: "GC=F" # Gold futures
```
