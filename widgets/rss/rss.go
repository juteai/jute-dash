package rss

import (
	"context"
	"encoding/xml"
	"net/http"
	"sync"
	"time"

	"jute-dash/internal/widgetskills"
	"jute-dash/widgets"
)

type RSSWidget struct {
	client     *http.Client
	cacheMu    sync.Mutex
	cache      map[string]rssCacheEntry
	cacheTTL   time.Duration
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
	Title string `json:"title"`
	Link  string `json:"link"`
}

type RSSFeedResult struct {
	FeedName string          `json:"feedName"`
	Items    []RSSItemResult `json:"items"`
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
		DefaultW:      2,
		DefaultH:      2,
		MinW:          1,
		MinH:          1,
		DefaultSize:   "medium",
		Overflow:      "scroll",
		AllowMultiple: true,
	}
}

func (w *RSSWidget) FetchData(ctx context.Context, settings map[string]any) (any, error) {
	w.cacheMu.Lock()
	if w.client == nil {
		w.client = &http.Client{Timeout: 4 * time.Second}
		w.cache = make(map[string]rssCacheEntry)
		w.cacheTTL = 15 * time.Minute
	}
	w.cacheMu.Unlock()

	limit := 5
	if val, ok := settings["limit"].(float64); ok {
		limit = int(val)
	} else if val, ok := settings["limit"].(int); ok {
		limit = val
	}

	var feeds []map[string]any
	if rawFeeds, ok := settings["feeds"].([]any); ok {
		for _, raw := range rawFeeds {
			if f, ok := raw.(map[string]any); ok {
				feeds = append(feeds, f)
			}
		}
	} else if rawFeeds, ok := settings["feeds"].([]map[string]any); ok {
		feeds = rawFeeds
	}

	results := []RSSFeedResult{}
	for _, feed := range feeds {
		feedURL, _ := feed["url"].(string)
		feedName, _ := feed["name"].(string)
		if feedURL == "" {
			continue
		}

		w.cacheMu.Lock()
		cached, exists := w.cache[feedURL]
		if exists && time.Since(cached.fetchedAt) < w.cacheTTL {
			results = append(results, cached.data.(RSSFeedResult))
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
		resp.Body.Close()

		if decodeErr != nil {
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
			if i >= limit {
				break
			}
			items = append(items, RSSItemResult{
				Title: it.Title,
				Link:  it.Link,
			})
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
		},
		SupportedWidgetSizes: []string{"medium", "wide", "large"},
	}
}

func init() {
	w := &RSSWidget{}
	widgets.Register(w)
	widgetskills.Register(*w.Skill(), nil)
}
