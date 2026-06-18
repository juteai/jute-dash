package weather

import (
	"net/http"

	"jute-dash/widgets/weather/hub/internal/provider"
)

const (
	ProviderOpenMeteo = provider.ProviderOpenMeteo

	StatusAvailable   = provider.StatusAvailable
	StatusUnavailable = provider.StatusUnavailable
	StatusDisabled    = provider.StatusDisabled
)

type State = provider.State
type Request = provider.Request
type Provider = provider.Provider
type Client = provider.Client
type Option = provider.Option

func WithHTTPClient(httpClient *http.Client) Option {
	return provider.WithHTTPClient(httpClient)
}

func WithEndpoint(endpoint string) Option {
	return provider.WithEndpoint(endpoint)
}

func NewClient(options ...Option) *Client {
	return provider.NewClient(options...)
}
