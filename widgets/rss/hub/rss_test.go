package rss

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
)

type mockRoundTripper func(req *http.Request) (*http.Response, error)

func (f mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRSSWidget_FetchData(t *testing.T) {
	sampleXML := `<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Article 1</title>
      <link>https://example.com/art1</link>
      <pubDate>Mon, 08 Jun 2026 18:30:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

	client := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(sampleXML)),
			}, nil
		}),
	}

	w := &RSSWidget{
		client:   client,
		cache:    make(map[string]rssCacheEntry),
		cacheTTL: 5 * time.Minute,
	}

	settings := map[string]any{
		"feeds": []any{
			map[string]any{
				"name": "Test",
				"url":  "https://example.com/rss",
			},
		},
	}

	data, err := w.FetchData(context.Background(), settings)
	if err != nil {
		t.Fatalf("FetchData error: %v", err)
	}

	feeds, ok := data.([]RSSFeedResult)
	if !ok {
		t.Fatalf("expected []RSSFeedResult, got %T", data)
	}

	if len(feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(feeds))
	}

	if feeds[0].FeedName != "Test" {
		t.Errorf("expected feed name Test, got %q", feeds[0].FeedName)
	}

	if len(feeds[0].Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feeds[0].Items))
	}

	item := feeds[0].Items[0]
	if item.Title != "Article 1" ||
		item.Link != "https://example.com/art1" ||
		item.PubDate != "Mon, 08 Jun 2026 18:30:00 GMT" {
		t.Errorf("unexpected item values: %+v", item)
	}
}

func TestRSSWidget_InvokeAction_ReadArticle(t *testing.T) {
	sampleHTML := `<html>
<head><style>body { color: black; }</style></head>
<body>
  <script>console.log("hello");</script>
  <p>First paragraph of the article.</p>
  <p>Second paragraph with keyword match.</p>
</body>
</html>`

	client := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://example.com/article" {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewReader(nil)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(sampleHTML)),
			}, nil
		}),
	}

	w := &RSSWidget{
		client: client,
	}

	// 1. Test missing url arg
	_, err := w.InvokeAction(context.Background(), widgetskills.Snapshot{}, "inst1", "read_article", nil)
	if err == nil {
		t.Error("expected error for missing url")
	}

	// 2. Test reading full page (truncated to 6000)
	res, err := w.InvokeAction(
		context.Background(),
		widgetskills.Snapshot{},
		"inst1",
		"read_article",
		map[string]any{
			"url": "https://example.com/article",
		},
	)
	if err != nil {
		t.Fatalf("InvokeAction error: %v", err)
	}
	content := res["content"].(string)
	if !strings.Contains(content, "First paragraph of the article.") ||
		strings.Contains(content, "<script>") ||
		strings.Contains(content, "<style>") {
		t.Errorf("cleaned HTML not correct: %q", content)
	}

	// 3. Test reading with grep filter query
	resGrep, err := w.InvokeAction(
		context.Background(),
		widgetskills.Snapshot{},
		"inst1",
		"read_article",
		map[string]any{
			"url":   "https://example.com/article",
			"query": "keyword",
		},
	)
	if err != nil {
		t.Fatalf("InvokeAction grep error: %v", err)
	}
	contentGrep := resGrep["content"].(string)
	if !strings.Contains(contentGrep, "Second paragraph with keyword match.") {
		t.Errorf("expected grep keyword match, got: %q", contentGrep)
	}
	if !strings.Contains(contentGrep, "First paragraph of the article.") {
		t.Errorf("expected neighbor paragraph to be present, got: %q", contentGrep)
	}

	resExplicitGrep, err := w.InvokeAction(
		context.Background(),
		widgetskills.Snapshot{},
		"inst1",
		"grep_article",
		map[string]any{
			"url":   "https://example.com/article",
			"query": "keyword",
		},
	)
	if err != nil {
		t.Fatalf("InvokeAction explicit grep error: %v", err)
	}
	contentExplicitGrep := resExplicitGrep["content"].(string)
	if !strings.Contains(contentExplicitGrep, "Second paragraph with keyword match.") {
		t.Errorf("expected explicit grep keyword match, got: %q", contentExplicitGrep)
	}
}

func TestRSSWidget_InvokeAction_Refresh(t *testing.T) {
	sampleXML := `<rss version="2.0">
<channel>
  <title>Example Feed</title>
  <item><title>Article 1</title><link>https://example.com/art1</link></item>
</channel>
</rss>`
	w := &RSSWidget{
		cache: make(map[string]rssCacheEntry),
		client: &http.Client{
			Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(sampleXML)),
				}, nil
			}),
		},
	}
	snapshot := widgetskills.Snapshot{
		Layout: widgetskills.WidgetLayout{
			Widgets: []widgetskills.WidgetInstance{{
				ID:      "rss-1",
				Kind:    "rss",
				Visible: true,
				Settings: map[string]any{
					"feeds": []any{map[string]any{"url": "https://example.com/rss"}},
				},
			}},
		},
	}

	res, err := w.InvokeAction(context.Background(), snapshot, "rss-1", "refresh", nil)
	if err != nil {
		t.Fatalf("InvokeAction refresh error: %v", err)
	}
	if res["status"] != "completed" || res["data"] == nil {
		t.Fatalf("unexpected refresh result: %+v", res)
	}
}

func TestRSSWidget_ParseSettings(t *testing.T) {
	// 1. limit as int
	raw1 := map[string]any{
		"limit": 10,
		"feeds": []any{
			map[string]any{"url": "https://url1", "name": "feed1"},
		},
	}
	s1 := parseSettings(raw1)
	if s1.Limit != 10 || len(s1.Feeds) != 1 || s1.Feeds[0].URL != "https://url1" {
		t.Errorf("parseSettings raw1 failed: %+v", s1)
	}

	// 2. feeds as []map[string]any
	raw2 := map[string]any{
		"limit": float64(3),
		"feeds": []map[string]any{
			{"url": "https://url2", "name": "feed2"},
		},
	}
	s2 := parseSettings(raw2)
	if s2.Limit != 3 || len(s2.Feeds) != 1 || s2.Feeds[0].URL != "https://url2" {
		t.Errorf("parseSettings raw2 failed: %+v", s2)
	}
}

func TestRSSWidget_FetchData_EdgeCases(t *testing.T) {
	// 1. Empty feed URL
	w := &RSSWidget{
		cache: make(map[string]rssCacheEntry),
	}
	settings := map[string]any{
		"feeds": []any{
			map[string]any{"url": ""},
		},
	}
	res, err := w.FetchData(context.Background(), settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	feeds := res.([]RSSFeedResult)
	if len(feeds) != 0 {
		t.Errorf("expected 0 feeds, got %d", len(feeds))
	}

	// 2. Cache hit
	cachedResult := RSSFeedResult{FeedName: "CachedFeed"}
	w.cache["https://cached-url"] = rssCacheEntry{
		data:      cachedResult,
		fetchedAt: time.Now(),
	}
	w.cacheTTL = 5 * time.Minute
	settingsCache := map[string]any{
		"feeds": []any{
			map[string]any{"url": "https://cached-url"},
		},
	}
	resCache, err := w.FetchData(context.Background(), settingsCache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	feedsCache := resCache.([]RSSFeedResult)
	if len(feedsCache) != 1 || feedsCache[0].FeedName != "CachedFeed" {
		t.Errorf("expected cache hit feed name CachedFeed, got: %+v", feedsCache)
	}

	// 3. HTTP Client request creation error
	settingsBadURL := map[string]any{
		"feeds": []any{
			map[string]any{"url": "http://invalid path"},
		},
	}
	w.client = &http.Client{}
	_, err = w.FetchData(context.Background(), settingsBadURL)
	if err != nil {
		t.Fatalf("FetchData should not return error on failed feeds: %v", err)
	}

	// 4. HTTP client DO error
	w.client = &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			return nil, io.EOF
		}),
	}
	settingsDoErr := map[string]any{
		"feeds": []any{
			map[string]any{"url": "https://fail-do-url"},
		},
	}
	_, err = w.FetchData(context.Background(), settingsDoErr)
	if err != nil {
		t.Fatalf("FetchData should handle DO error gracefully: %v", err)
	}

	// 5. Malformed XML decode error
	w.client = &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("not-xml")),
			}, nil
		}),
	}
	settingsBadXML := map[string]any{
		"feeds": []any{
			map[string]any{"url": "https://bad-xml-url"},
		},
	}
	_, err = w.FetchData(context.Background(), settingsBadXML)
	if err != nil {
		t.Fatalf("FetchData should handle XML error gracefully: %v", err)
	}
}

func TestRSSWidget_InvokeAction_EdgeCases(t *testing.T) {
	w := &RSSWidget{}

	// 1. Unknown Action
	_, err := w.InvokeAction(context.Background(), widgetskills.Snapshot{}, "inst1", "unknown_action", nil)
	if err == nil {
		t.Error("expected error for unknown action")
	}

	// 2. HTTP Non-200 status code
	w.client = &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     "404 Not Found",
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}
	_, err = w.InvokeAction(context.Background(), widgetskills.Snapshot{}, "inst1", "read_article", map[string]any{
		"url": "https://example.com/notfound",
	})
	if err == nil {
		t.Error("expected error for non-200 HTTP status")
	}

	_, err = w.InvokeAction(context.Background(), widgetskills.Snapshot{}, "inst1", "grep_article", map[string]any{
		"url": "https://example.com/article",
	})
	if err == nil {
		t.Error("expected error for grep_article without query")
	}

	// 3. HTML cleaning case-insensitivity and entity unescaping
	dirtyHTML := `<html><body>
		<SCRIPT>console.log("bad");</SCRIPT>
		<STYLE>body { color: red; }</style>
		<p>Google &amp; Apple are tech giants.</p>
	</body></html>`
	cleaned := cleanHTML(dirtyHTML)
	if strings.Contains(cleaned, "console.log") || strings.Contains(cleaned, "color: red") {
		t.Errorf("failed to strip case-insensitive script/style tags: %q", cleaned)
	}
	if !strings.Contains(cleaned, "Google & Apple") {
		t.Errorf("failed to unescape HTML entities: %q", cleaned)
	}

	// 4. Grep matches truncation and no matches
	paragraphs := "Paragraph 1\n\nParagraph 2\n\nParagraph 3"
	noMatch := grepArticle(paragraphs, "nonexistent")
	if noMatch != "[No matches found for query: nonexistent]" {
		t.Errorf("expected no matches placeholder, got %q", noMatch)
	}

	// Create long content for truncation testing
	var longBuilder strings.Builder
	for range 2000 {
		longBuilder.WriteString("Paragraph containing query term.\n\n")
	}
	truncated := grepArticle(longBuilder.String(), "query")
	if !strings.Contains(truncated, "[Truncated due to grep matches limit...]") {
		t.Error("expected truncated result for long grep matches")
	}

	// 5. Rune-safe truncation on non-ASCII content
	utf8Text := strings.Repeat("こんにちは", 1500) // 5 * 1500 = 7500 runes
	truncatedUTF8, isTruncated := truncateString(utf8Text, 6000)
	if !isTruncated || len([]rune(truncatedUTF8)) != 6000 {
		t.Errorf("expected truncation to 6000 runes, got %d", len([]rune(truncatedUTF8)))
	}

	// Test CatalogInfo and Skill definitions
	info := w.CatalogInfo()
	if info.Kind != "rss" {
		t.Errorf("CatalogInfo kind: %q", info.Kind)
	}
	skill := w.Skill()
	if skill.WidgetKind != "rss" {
		t.Errorf("Skill WidgetKind: %q", skill.WidgetKind)
	}
}
