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
}
