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
}
