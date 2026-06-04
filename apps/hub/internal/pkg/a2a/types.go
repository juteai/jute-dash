package a2a

const (
	ProtocolJSONRPC  = "JSONRPC"
	ProtocolHTTPJSON = "HTTP+JSON"
	ProtocolGRPC     = "GRPC"

	ProtocolVersion10 = "1.0"

	DashboardContextExtensionURI = "https://jute.dev/a2a/extensions/dashboard-context/v1"
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
	Name                 string            `json:"name"`
	Description          string            `json:"description,omitempty"`
	URL                  string            `json:"url,omitempty"`
	Version              string            `json:"version,omitempty"`
	ProtocolVersion      string            `json:"protocolVersion,omitempty"`
	PreferredTransport   string            `json:"preferredTransport,omitempty"`
	SupportedInterfaces  []AgentInterface  `json:"supportedInterfaces,omitempty"`
	AdditionalInterfaces []AgentInterface  `json:"additionalInterfaces,omitempty"`
	Capabilities         AgentCapabilities `json:"capabilities,omitempty"`
	DefaultInputModes    []string          `json:"defaultInputModes,omitempty"`
	DefaultOutputModes   []string          `json:"defaultOutputModes,omitempty"`
	Skills               []AgentSkill      `json:"skills,omitempty"`
	SecuritySchemes      map[string]any    `json:"securitySchemes,omitempty"`
	Extensions           map[string]any    `json:"extensions,omitempty"`
}

type AgentInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
	ProtocolVersion string `json:"protocolVersion,omitempty"`
	Tenant          string `json:"tenant,omitempty"`
}

type AgentCapabilities struct {
	Streaming         bool             `json:"streaming,omitempty"`
	PushNotifications bool             `json:"pushNotifications,omitempty"`
	ExtendedAgentCard bool             `json:"extendedAgentCard,omitempty"`
	Extensions        []AgentExtension `json:"extensions,omitempty"`
}

type AgentExtension struct {
	URI         string         `json:"uri,omitempty"`
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
}

type AgentSkill struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

func IsSupportedProtocolBinding(binding string) bool {
	_, ok := SupportedProtocolBindings[binding]
	return ok
}

func SupportsDashboardContext(card AgentCard) bool {
	for _, extension := range card.Capabilities.Extensions {
		if extension.URI == DashboardContextExtensionURI {
			return true
		}
	}
	return false
}
