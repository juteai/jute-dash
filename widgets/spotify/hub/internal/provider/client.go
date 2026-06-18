package provider

type Client struct {
	settings Settings
}

func NewClient(settings Settings) Client {
	return Client{settings: settings}
}
