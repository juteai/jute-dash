package markets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"jute-dash/internal/widgetskills"
	"jute-dash/widgets"
)

type MarketsWidget struct {
	client     *http.Client
	cacheMu    sync.Mutex
	cache      map[string]marketCacheEntry
	cacheTTL   time.Duration
}

type marketCacheEntry struct {
	data      any
	fetchedAt time.Time
}

type yfResponse struct {
	QuoteResponse struct {
		Result []yfQuote `json:"result"`
	} `json:"quoteResponse"`
}

type yfQuote struct {
	Symbol            string  `json:"symbol"`
	ShortName         string  `json:"shortName"`
	LongName          string  `json:"longName"`
	RegularMarketPrice float64 `json:"regularMarketPrice"`
	RegularMarketChange float64 `json:"regularMarketChange"`
	RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
	Currency          string  `json:"currency"`
}

type MarketItemResult struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"changePercent"`
	Currency      string  `json:"currency"`
}

func (w *MarketsWidget) Kind() string {
	return "markets"
}

func (w *MarketsWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          "markets",
		Name:          "Markets (Stocks)",
		Description:   "Displays stock, commodity, or crypto market prices.",
		DefaultTitle:  "Markets",
		DefaultW:      2,
		DefaultH:      2,
		MinW:          1,
		MinH:          1,
		DefaultSize:   "medium",
		Overflow:      "clip",
		AllowMultiple: true,
	}
}

func (w *MarketsWidget) FetchData(ctx context.Context, settings map[string]any) (any, error) {
	w.cacheMu.Lock()
	if w.client == nil {
		w.client = &http.Client{Timeout: 4 * time.Second}
		w.cache = make(map[string]marketCacheEntry)
		w.cacheTTL = 5 * time.Minute
	}
	w.cacheMu.Unlock()

	var tickers []string
	if rawTickers, ok := settings["tickers"].([]any); ok {
		for _, raw := range rawTickers {
			if tMap, ok := raw.(map[string]any); ok {
				if sym, ok := tMap["symbol"].(string); ok && sym != "" {
					tickers = append(tickers, strings.ToUpper(sym))
				}
			} else if sym, ok := raw.(string); ok && sym != "" {
				tickers = append(tickers, strings.ToUpper(sym))
			}
		}
	}

	if len(tickers) == 0 {
		return []MarketItemResult{}, nil
	}

	cacheKey := strings.Join(tickers, ",")
	w.cacheMu.Lock()
	cached, exists := w.cache[cacheKey]
	if exists && time.Since(cached.fetchedAt) < w.cacheTTL {
		w.cacheMu.Unlock()
		return cached.data, nil
	}
	w.cacheMu.Unlock()

	apiURL := fmt.Sprintf("https://query1.finance.yahoo.com/v7/finance/quote?symbols=%s", strings.Join(tickers, ","))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return []MarketItemResult{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := w.client.Do(req)
	if err != nil {
		return []MarketItemResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []MarketItemResult{}, fmt.Errorf("YF API error: %d", resp.StatusCode)
	}

	var payload yfResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return []MarketItemResult{}, err
	}

	results := make([]MarketItemResult, 0, len(payload.QuoteResponse.Result))
	for _, q := range payload.QuoteResponse.Result {
		name := q.ShortName
		if name == "" {
			name = q.LongName
		}
		if name == "" {
			name = q.Symbol
		}
		results = append(results, MarketItemResult{
			Symbol:        q.Symbol,
			Name:          name,
			Price:         q.RegularMarketPrice,
			Change:        q.RegularMarketChange,
			ChangePercent: q.RegularMarketChangePercent,
			Currency:      q.Currency,
		})
	}

	w.cacheMu.Lock()
	w.cache[cacheKey] = marketCacheEntry{
		data:      results,
		fetchedAt: time.Now(),
	}
	w.cacheMu.Unlock()

	return results, nil
}

func (w *MarketsWidget) Skill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             "jute.markets.current",
		WidgetKind:          "markets",
		DisplayName:         "Markets (Stocks)",
		Summary:             "Read stock and cryptocurrency ticker prices.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{Name: "data", Type: "array", Description: "Yahoo Finance stock and crypto price quote details.", Sensitivity: "public"},
		},
		Actions: []widgetskills.Action{
			{
				ID:          "refresh",
				Title:       "Refresh market prices",
				Description: "Query Yahoo Finance for active ticker prices.",
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
	w := &MarketsWidget{}
	widgets.Register(w)
	widgetskills.Register(*w.Skill(), nil)
}
