package rss

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

type RSSWidget struct {
	client   *http.Client
	cacheMu  sync.Mutex
	cache    map[string]rssCacheEntry
	cacheTTL time.Duration
}

type rssCacheEntry struct {
	data      any
	fetchedAt time.Time
}

type rssXML struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
}

type RSSItemResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	PubDate string `json:"pubDate,omitempty"`
}

type RSSFeedResult struct {
	FeedName string          `json:"feedName"`
	Items    []RSSItemResult `json:"items"`
}

// feedConfig holds a single feed URL and display name.
type feedConfig struct {
	URL  string
	Name string
}

// Settings holds the parsed per-instance configuration for the RSS widget.
type Settings struct {
	Limit int
	Feeds []feedConfig
}

func parseSettings(raw map[string]any) Settings {
	s := Settings{Limit: 5}
	// YAML numeric values decode as float64; JSON integers decode as int.
	if v, ok := raw["limit"].(float64); ok {
		s.Limit = int(v)
	} else if v, ok := raw["limit"].(int); ok {
		s.Limit = v
	}
	if rawFeeds, ok := raw["feeds"].([]any); ok {
		for _, item := range rawFeeds {
			if f, ok := item.(map[string]any); ok {
				s.Feeds = append(s.Feeds, feedConfig{
					URL:  stringVal(f["url"]),
					Name: stringVal(f["name"]),
				})
			}
		}
	} else if rawFeeds, ok := raw["feeds"].([]map[string]any); ok {
		for _, f := range rawFeeds {
			s.Feeds = append(s.Feeds, feedConfig{
				URL:  stringVal(f["url"]),
				Name: stringVal(f["name"]),
			})
		}
	}
	return s
}

func stringVal(v any) string {
	s, _ := v.(string)
	return s
}

func (w *RSSWidget) Kind() string {
	return "rss"
}

func (w *RSSWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          "rss",
		Name:          "RSS Feed",
		Description:   "Aggregates and displays headlines from custom RSS feeds.",
		DefaultTitle:  "Tech News",
		DefaultW:      6,
		DefaultH:      2,
		MinW:          3,
		MinH:          1,
		DefaultSize:   "medium",
		Overflow:      "scroll",
		AllowMultiple: true,
		SettingsSchema: []widgets.SettingField{
			{ID: "limit", Type: widgets.SettingNumber, Label: "Max headlines", Default: 5},
			{ID: "feeds", Type: widgets.SettingObjectList, Label: "Feeds", Fields: []widgets.SettingField{
				{ID: "name", Type: widgets.SettingString, Label: "Name"},
				{ID: "url", Type: widgets.SettingString, Label: "Feed URL"},
			}},
		},
	}
}

func (w *RSSWidget) FetchData(ctx context.Context, raw map[string]any) (any, error) {
	w.cacheMu.Lock()
	if w.client == nil {
		w.client = &http.Client{Timeout: 4 * time.Second}
		w.cache = make(map[string]rssCacheEntry)
		w.cacheTTL = 15 * time.Minute
	}
	w.cacheMu.Unlock()

	s := parseSettings(raw)

	results := []RSSFeedResult{}
	for _, feed := range s.Feeds {
		feedURL := feed.URL
		feedName := feed.Name
		if feedURL == "" {
			continue
		}

		w.cacheMu.Lock()
		cached, exists := w.cache[feedURL]
		if exists && time.Since(cached.fetchedAt) < w.cacheTTL {
			if result, ok := cached.data.(RSSFeedResult); ok {
				results = append(results, result)
			}
			w.cacheMu.Unlock()
			continue
		}
		w.cacheMu.Unlock()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
		if err != nil {
			continue
		}

		resp, err := w.client.Do(req)
		if err != nil {
			continue
		}

		var payload rssXML
		decodeErr := xml.NewDecoder(resp.Body).Decode(&payload)
		closeErr := resp.Body.Close()

		if decodeErr != nil || closeErr != nil {
			continue
		}

		if feedName == "" {
			feedName = payload.Channel.Title
		}
		if feedName == "" {
			feedName = "Feed"
		}

		items := []RSSItemResult{}
		for i, it := range payload.Channel.Items {
			if i >= s.Limit {
				break
			}
			items = append(items, RSSItemResult(it))
		}

		result := RSSFeedResult{
			FeedName: feedName,
			Items:    items,
		}

		w.cacheMu.Lock()
		w.cache[feedURL] = rssCacheEntry{
			data:      result,
			fetchedAt: time.Now(),
		}
		w.cacheMu.Unlock()

		results = append(results, result)
	}

	return results, nil
}

func (w *RSSWidget) Skill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             "jute.rss.current",
		WidgetKind:          "rss",
		DisplayName:         "RSS Feed",
		Summary:             "Read configured RSS feed headlines.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{Name: "data", Type: "array", Description: "Parsed RSS headlines and channels.", Sensitivity: "public"},
		},
		Actions: []widgetskills.Action{
			{
				ID:          "refresh",
				Title:       "Refresh RSS feeds",
				Description: "Query remote RSS servers for new articles.",
				SideEffect:  "read",
				InputSchema: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
				},
				OutputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status": map[string]any{"type": "string"},
					},
					"required": []string{"status"},
				},
			},
			rssArticleAction(
				"read_article",
				"Read article content",
				"Fetch the text content of a given article URL.",
				false,
			),
			rssArticleAction(
				"grep_article",
				"Search article content",
				"Fetch an article URL and return paragraphs matching a keyword query with surrounding context.",
				true,
			),
		},
		SupportedWidgetSizes: []string{"medium", "wide", "large"},
	}
}

func rssArticleAction(id, title, description string, requireQuery bool) widgetskills.Action {
	required := []string{"url"}
	if requireQuery {
		required = append(required, "query")
	}
	return widgetskills.Action{
		ID:          id,
		Title:       title,
		Description: description,
		SideEffect:  "read",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL of the article to read.",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Keyword query to filter matching paragraphs with surrounding context.",
				},
			},
			"required":             required,
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":  map[string]any{"type": "string"},
				"title":   map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			},
			"required": []string{"status", "title", "content"},
		},
	}
}

func (w *RSSWidget) InvokeAction(
	ctx context.Context,
	snapshot widgetskills.Snapshot,
	instanceID string,
	actionID string,
	arguments map[string]any,
) (map[string]any, error) {
	if actionID == "refresh" {
		return w.invokeRefresh(ctx, snapshot, instanceID)
	}
	if actionID != "read_article" && actionID != "grep_article" {
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}

	url, _ := arguments["url"].(string)
	if url == "" {
		return nil, errors.New("url parameter is required")
	}
	query, _ := arguments["query"].(string)
	if actionID == "grep_article" && strings.TrimSpace(query) == "" {
		return nil, errors.New("query parameter is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "JuteDash/1.0 (RSS Reader)")

	w.cacheMu.Lock()
	if w.client == nil {
		w.client = &http.Client{Timeout: 4 * time.Second}
	}
	client := w.client
	w.cacheMu.Unlock()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch article page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	cleanedText := cleanHTML(string(bodyBytes))

	if query != "" {
		cleanedText = grepArticle(cleanedText, query)
	} else {
		if truncatedText, truncated := truncateString(cleanedText, 6000); truncated {
			cleanedText = truncatedText + "\n\n[Truncated to first 6,000 characters. Specify a query parameter to search/grep for specific details.]"
		}
	}

	return map[string]any{
		"status":    "completed",
		"title":     "Article content from " + url,
		"content":   cleanedText,
		"updatedAt": time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func (w *RSSWidget) invokeRefresh(
	ctx context.Context,
	snapshot widgetskills.Snapshot,
	instanceID string,
) (map[string]any, error) {
	for _, widget := range snapshot.Layout.Widgets {
		if widget.ID != instanceID {
			continue
		}
		data, err := w.FetchData(ctx, widget.Settings)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"status":    "completed",
			"data":      data,
			"updatedAt": time.Now().UTC().Format(time.RFC3339Nano),
		}, nil
	}
	return nil, errors.New("widget instance not found")
}

var (
	reScript = regexp.MustCompile(`(?si)<script.*?>.*?</script>`)
	reStyle  = regexp.MustCompile(`(?si)<style.*?>.*?</style>`)
	reTags   = regexp.MustCompile(`<.*?>`)
)

func cleanHTML(rawHTML string) string {
	text := reScript.ReplaceAllString(rawHTML, "")
	text = reStyle.ReplaceAllString(text, "")
	text = reTags.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)

	lines := strings.Split(text, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}
	return strings.Join(cleanedLines, "\n\n")
}

func truncateString(s string, limit int) (string, bool) {
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit]), true
	}
	return s, false
}

func grepArticle(text string, query string) string {
	paragraphs := strings.Split(text, "\n\n")
	queryLower := strings.ToLower(query)

	matchedIndices := make(map[int]bool)
	for i, p := range paragraphs {
		if strings.Contains(strings.ToLower(p), queryLower) {
			matchedIndices[i] = true
		}
	}

	var matchedParagraphs []string
	for i := range paragraphs {
		if matchedIndices[i] || matchedIndices[i-1] || matchedIndices[i+1] {
			matchedParagraphs = append(matchedParagraphs, paragraphs[i])
		}
	}

	result := strings.Join(matchedParagraphs, "\n\n")
	if truncatedResult, truncated := truncateString(result, 12000); truncated {
		return truncatedResult + "\n\n[Truncated due to grep matches limit...]"
	}
	if len(result) == 0 {
		return "[No matches found for query: " + query + "]"
	}
	return result
}

func init() {
	widgets.RegisterWithSkill(&RSSWidget{}, nil)
}
