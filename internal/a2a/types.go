package a2a

const (
	ProtocolJSONRPC  = "JSONRPC"
	ProtocolHTTPJSON = "HTTP+JSON"
	ProtocolGRPC     = "GRPC"
)

var SupportedProtocolBindings = map[string]struct{}{
	ProtocolJSONRPC:  {},
	ProtocolHTTPJSON: {},
	ProtocolGRPC:     {},
}

// AgentCard is the subset of the A2A Agent Card that Jute needs for discovery,
// display, and transport selection. Keep protocol-specific expansion isolated
// here so the rest of the hub can work with stable internal concepts.
type AgentCard struct {
	Name                 string                 `json:"name"`
	Description          string                 `json:"description,omitempty"`
	URL                  string                 `json:"url,omitempty"`
	Version              string                 `json:"version,omitempty"`
	ProtocolVersion      string                 `json:"protocolVersion,omitempty"`
	PreferredTransport   string                 `json:"preferredTransport,omitempty"`
	AdditionalInterfaces []AgentInterface       `json:"additionalInterfaces,omitempty"`
	Capabilities         map[string]any         `json:"capabilities,omitempty"`
	Skills               []AgentSkill           `json:"skills,omitempty"`
	SecuritySchemes      map[string]any         `json:"securitySchemes,omitempty"`
	Extensions           map[string]interface{} `json:"extensions,omitempty"`
}

type AgentInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
}

type AgentSkill struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func IsSupportedProtocolBinding(binding string) bool {
	_, ok := SupportedProtocolBindings[binding]
	return ok
}
