# RSS Feed Widget

A native Jute Dash widget that aggregates, parses, and displays active headlines from custom remote XML/RSS feeds with built-in feed caching to reduce server overhead.

## Widget Details

- **Kind (Identifier)**: `rss`
- **Default Size**: `medium` (2 columns, 2 rows)
- **Minimum Size**: 1x1
- **Overflow Policy**: `scroll`
- **Allow Multiple**: `true` (You can configure multiple separate RSS widgets for different topics)

## Usage and Configuration

You can customize the number of feed articles displayed and list custom feed sources inside the widget's settings.

### Settings Schema

| Key | Type | Description | Default |
| :--- | :--- | :--- | :--- |
| `limit` | `integer` | Number of news items to fetch and display per feed source | `5` |
| `feeds` | `array` | A list of feed objects containing `url` and optional `name` | `[]` |

#### Feed Object Properties
- `url` (`string`, required): The HTTP URL of the RSS feed.
- `name` (`string`, optional): A custom title to display for the feed section. If omitted, the widget extracts the channel name parsed from the RSS XML feed.

### Example YAML Configuration (`jute.yaml`)

```yaml
dashboard:
  widgets:
    - id: "rss-tech"
      type: "rss"
      title: "Tech News"
      x: 2
      y: 1
      w: 2
      h: 2
      visible: true
      settings:
        limit: 5
        feeds:
          - url: "https://news.ycombinator.com/rss"
            name: "Hacker News"
          - url: "https://feeds.feedburner.com/TechCrunch/"
            name: "TechCrunch"
```
