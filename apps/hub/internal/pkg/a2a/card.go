package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxAgentCardBytes = 2 << 20

var (
	ErrAgentCardUnavailable = errors.New("agent card is unavailable")
	ErrNoSupportedInterface = errors.New("agent card has no supported A2A 1.0 interface")
)

type AgentCardFetchResult struct {
	Card      AgentCard
	Raw       string
	FetchedAt time.Time
}

type SelectedInterface struct {
	EndpointURL     string
	ProtocolBinding string
	ProtocolVersion string
}

type AgentCardFetcher struct {
	HTTPClient *http.Client
}

func NewAgentCardFetcher() *AgentCardFetcher {
	return &AgentCardFetcher{
		HTTPClient: &http.Client{Timeout: 8 * time.Second},
	}
}

func (f *AgentCardFetcher) Fetch(
	ctx context.Context,
	cardURL AuthorizedAgentCardURL,
	bearerToken string,
) (AgentCardFetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cardURL.String(), nil)
	if err != nil {
		return AgentCardFetchResult{}, fmt.Errorf("build agent card request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(bearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))
	}

	client := f.HTTPClient
	if client == nil {
		client = NewAgentCardFetcher().HTTPClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return AgentCardFetchResult{}, ErrAgentCardUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return AgentCardFetchResult{}, fmt.Errorf("%w: status %d", ErrAgentCardUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAgentCardBytes))
	if err != nil {
		return AgentCardFetchResult{}, fmt.Errorf("read agent card: %w", err)
	}
	var card AgentCard
	if err := json.Unmarshal(body, &card); err != nil {
		return AgentCardFetchResult{}, fmt.Errorf("decode agent card: %w", err)
	}
	if strings.TrimSpace(card.Name) == "" {
		return AgentCardFetchResult{}, fmt.Errorf("%w: missing name", ErrAgentCardUnavailable)
	}
	return AgentCardFetchResult{
		Card:      card,
		Raw:       string(body),
		FetchedAt: time.Now().UTC(),
	}, nil
}

func SelectInterface(card AgentCard) (SelectedInterface, error) {
	interfaces := append([]AgentInterface(nil), card.SupportedInterfaces...)
	if len(interfaces) == 0 && card.URL != "" {
		binding := card.PreferredTransport
		if binding == "" {
			binding = ProtocolJSONRPC
		}
		interfaces = []AgentInterface{
			{
				URL:             card.URL,
				ProtocolBinding: binding,
				ProtocolVersion: firstNonEmpty(card.ProtocolVersion, ProtocolVersion10),
			},
		}
	}

	if len(interfaces) == 0 {
		return SelectedInterface{}, ErrNoSupportedInterface
	}

	for _, preferred := range []string{ProtocolJSONRPC, ProtocolHTTPJSON, ProtocolGRPC} {
		for _, item := range interfaces {
			if item.ProtocolBinding != preferred {
				continue
			}
			if version := strings.TrimSpace(item.ProtocolVersion); version != "" && version != ProtocolVersion10 {
				continue
			}
			if strings.TrimSpace(item.URL) == "" {
				continue
			}
			return SelectedInterface{
				EndpointURL:     item.URL,
				ProtocolBinding: item.ProtocolBinding,
				ProtocolVersion: firstNonEmpty(item.ProtocolVersion, ProtocolVersion10),
			}, nil
		}
	}
	return SelectedInterface{}, ErrNoSupportedInterface
}

func firstNonEmpty(candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
