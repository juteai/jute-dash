package markets

import (
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

func TestMarketsWidget_FetchData(t *testing.T) {
	sampleJSON := `{
  "chart": {
    "result": [
      {
        "meta": {
          "symbol": "AAPL",
          "shortName": "Apple Inc.",
          "longName": "Apple Inc.",
          "regularMarketPrice": 150.0,
          "previousClose": 148.0,
          "currency": "USD"
        }
      }
    ]
  }
}`

	client := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(sampleJSON)),
			}, nil
		}),
	}

	w := &MarketsWidget{
		client:   client,
		cache:    make(map[string]marketCacheEntry),
		cacheTTL: 5 * time.Minute,
	}

	settings := map[string]any{
		"tickers": []any{
			"AAPL",
		},
	}

	data, err := w.FetchData(context.Background(), settings)
	if err != nil {
		t.Fatalf("FetchData error: %v", err)
	}

	tickers, ok := data.([]MarketItemResult)
	if !ok {
		t.Fatalf("expected []MarketItemResult, got %T", data)
	}

	if len(tickers) != 1 {
		t.Fatalf("expected 1 ticker result, got %d", len(tickers))
	}

	ticker := tickers[0]
	if ticker.Symbol != "AAPL" || ticker.Name != "Apple Inc." || ticker.Price != 150.0 || ticker.Currency != "USD" {
		t.Errorf("unexpected ticker quote values: %+v", ticker)
	}
}

func TestMarketsWidget_InvokeAction_QueryTicker(t *testing.T) {
	sampleJSON := `{
  "chart": {
    "result": [
      {
        "meta": {
          "symbol": "GOOG",
          "shortName": "Alphabet Inc.",
          "longName": "Alphabet Inc.",
          "regularMarketPrice": 2800.0,
          "previousClose": 2750.0,
          "currency": "USD"
        }
      }
    ]
  }
}`

	client := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(sampleJSON)),
			}, nil
		}),
	}

	w := &MarketsWidget{
		client: client,
	}

	// 1. Test query_ticker action
	res, err := w.InvokeAction(context.Background(), widgetskills.Snapshot{}, "inst1", "query_ticker", map[string]any{
		"symbol": "GOOG",
	})
	if err != nil {
		t.Fatalf("InvokeAction error: %v", err)
	}

	if res["symbol"] != "GOOG" || res["price"] != 2800.0 || res["name"] != "Alphabet Inc." {
		t.Errorf("unexpected stock result values: %+v", res)
	}

	// 2. Test missing symbol parameter error
	_, err = w.InvokeAction(context.Background(), widgetskills.Snapshot{}, "inst1", "query_ticker", nil)
	if err == nil {
		t.Error("expected error for missing symbol parameter")
	}

	resStock, err := w.InvokeAction(
		context.Background(),
		widgetskills.Snapshot{},
		"inst1",
		"query_stock",
		map[string]any{
			"symbol": "GOOG",
		},
	)
	if err != nil {
		t.Fatalf("InvokeAction query_stock error: %v", err)
	}
	if resStock["symbol"] != "GOOG" || resStock["price"] != 2800.0 {
		t.Errorf("unexpected query_stock result values: %+v", resStock)
	}
}

func TestMarketsWidget_SkillDeclaresNaturalMarketActions(t *testing.T) {
	skill := (&MarketsWidget{}).Skill()
	actions := map[string]widgetskills.Action{}
	for _, action := range skill.Actions {
		actions[action.ID] = action
	}
	for _, id := range []string{"refresh", "query_ticker", "query_stock", "query_share", "query_crypto"} {
		action, ok := actions[id]
		if !ok {
			t.Fatalf("expected action %q in markets skill", id)
		}
		if action.InputSchema == nil {
			t.Fatalf("expected action %q to include input schema", id)
		}
	}
	if actions["query_share"].Title != "Query share price" {
		t.Fatalf("unexpected query_share action: %+v", actions["query_share"])
	}
}

func TestMarketsWidget_ParseSettings(t *testing.T) {
	// 1. Tickers as maps
	raw1 := map[string]any{
		"tickers": []any{
			map[string]any{"symbol": "aapl"},
			map[string]any{"symbol": ""}, // should be ignored
			"goog",                       // string format
		},
	}
	s1 := parseSettings(raw1)
	if len(s1.Tickers) != 2 || s1.Tickers[0] != "AAPL" || s1.Tickers[1] != "GOOG" {
		t.Errorf("parseSettings raw1 failed: %+v", s1.Tickers)
	}
}

func TestMarketsWidget_FetchData_ErrorsAndCache(t *testing.T) {
	// 1. Cache hit
	w := &MarketsWidget{
		client:   &http.Client{},
		cache:    make(map[string]marketCacheEntry),
		cacheTTL: 5 * time.Minute,
	}
	cachedData := []MarketItemResult{{Symbol: "AAPL", Price: 150.0}}
	w.cache["AAPL"] = marketCacheEntry{
		data:      cachedData,
		fetchedAt: time.Now(),
	}
	settings := map[string]any{
		"tickers": []any{"AAPL"},
	}
	res, err := w.FetchData(context.Background(), settings)
	if err != nil {
		t.Fatalf("FetchData cache hit error: %v", err)
	}
	results := res.([]MarketItemResult)
	if len(results) != 1 || results[0].Symbol != "AAPL" || results[0].Price != 150.0 {
		t.Errorf("expected cached AAPL data, got: %+v", results)
	}

	// 2. Empty tickers returns empty result
	w = &MarketsWidget{
		client: &http.Client{},
	}
	resEmpty, err := w.FetchData(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error for empty tickers: %v", err)
	}
	resultsEmpty := resEmpty.([]MarketItemResult)
	if len(resultsEmpty) != 0 {
		t.Errorf("expected 0 results, got %d", len(resultsEmpty))
	}

	// 3. Yahoo Finance HTTP non-200 response
	w = &MarketsWidget{
		client: &http.Client{
			Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}),
		},
		cache:    make(map[string]marketCacheEntry),
		cacheTTL: 5 * time.Minute,
	}
	_, err = w.FetchData(context.Background(), settings)
	if err == nil {
		t.Error("expected error when all tickers fail to fetch")
	}

	// 4. Yahoo Finance JSON decode error
	w = &MarketsWidget{
		client: &http.Client{
			Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("not-json")),
				}, nil
			}),
		},
		cache:    make(map[string]marketCacheEntry),
		cacheTTL: 5 * time.Minute,
	}
	_, err = w.FetchData(context.Background(), settings)
	if err == nil {
		t.Error("expected error when json decoding fails")
	}

	// 5. Yahoo Finance API error response
	apiErrJSON := `{
		"chart": {
			"result": null,
			"error": {
				"code": "404",
				"description": "Symbol not found"
			}
		}
	}`
	w = &MarketsWidget{
		client: &http.Client{
			Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(apiErrJSON)),
				}, nil
			}),
		},
		cache:    make(map[string]marketCacheEntry),
		cacheTTL: 5 * time.Minute,
	}
	_, err = w.FetchData(context.Background(), settings)
	if err == nil {
		t.Error("expected error when Yahoo Finance returns chart error")
	}

	// 6. Yahoo Finance empty results
	emptyResultJSON := `{
		"chart": {
			"result": [],
			"error": null
		}
	}`
	w = &MarketsWidget{
		client: &http.Client{
			Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(emptyResultJSON)),
				}, nil
			}),
		},
		cache:    make(map[string]marketCacheEntry),
		cacheTTL: 5 * time.Minute,
	}
	_, err = w.FetchData(context.Background(), settings)
	if err == nil {
		t.Error("expected error when Yahoo Finance returns empty result list")
	}
}

func TestMarketsWidget_InvokeAction_EdgeCases(t *testing.T) {
	// 1. Unknown action
	w := &MarketsWidget{}
	w.client = &http.Client{}
	w.cache = make(map[string]marketCacheEntry)
	w.cacheTTL = 5 * time.Minute

	_, err := w.InvokeAction(context.Background(), widgetskills.Snapshot{}, "inst1", "unknown_action", nil)
	if err == nil {
		t.Error("expected error for unknown action")
	}

	// 2. refresh action success
	sampleJSON := `{
		"chart": {
			"result": [
				{
					"meta": {
						"symbol": "AAPL",
						"regularMarketPrice": 150.0
					}
				}
			]
		}
	}`
	w = &MarketsWidget{
		client: &http.Client{
			Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(sampleJSON)),
				}, nil
			}),
		},
		cache:    make(map[string]marketCacheEntry),
		cacheTTL: 5 * time.Minute,
	}

	snapshot := widgetskills.Snapshot{
		Layout: widgetskills.WidgetLayout{
			Widgets: []widgetskills.WidgetInstance{
				{
					ID:   "inst1",
					Kind: "markets",
					Settings: map[string]any{
						"tickers": []any{"AAPL"},
					},
				},
			},
		},
	}
	resRefresh, err := w.InvokeAction(context.Background(), snapshot, "inst1", "refresh", nil)
	if err != nil {
		t.Fatalf("refresh action error: %v", err)
	}
	if resRefresh["status"] != "completed" {
		t.Errorf("expected refresh status completed, got %q", resRefresh["status"])
	}

	// 3. refresh action error (when FetchData fails)
	w = &MarketsWidget{
		client: &http.Client{
			Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
				return nil, io.EOF
			}),
		},
		cache:    make(map[string]marketCacheEntry),
		cacheTTL: 5 * time.Minute,
	}
	_, err = w.InvokeAction(context.Background(), snapshot, "inst1", "refresh", nil)
	if err == nil {
		t.Error("expected error during refresh when FetchData fails")
	}

	// 4. query_ticker fetch error
	w = &MarketsWidget{
		client: &http.Client{
			Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
				return nil, io.EOF
			}),
		},
		cache:    make(map[string]marketCacheEntry),
		cacheTTL: 5 * time.Minute,
	}
	_, err = w.InvokeAction(context.Background(), widgetskills.Snapshot{}, "inst1", "query_ticker", map[string]any{
		"symbol": "AAPL",
	})
	if err == nil {
		t.Error("expected query_ticker to fail when fetch fails")
	}

	// CatalogInfo and Skill verification
	info := w.CatalogInfo()
	if info.Kind != "markets" {
		t.Errorf("CatalogInfo kind: %q", info.Kind)
	}
	skill := w.Skill()
	if skill.WidgetKind != "markets" {
		t.Errorf("Skill WidgetKind: %q", skill.WidgetKind)
	}
}
