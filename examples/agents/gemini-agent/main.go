// Command gemini-a2a exposes a Gemini-backed ADK agent through A2A 1.0.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model/gemini"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
	"google.golang.org/genai"
)

// headerRoundTripper injects a fixed set of HTTP headers on every request.
type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

const (
	defaultModelID = "gemini-2.5-flash"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY environment variable is required")
	}

	modelID := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if modelID == "" {
		modelID = defaultModelID
	}

	mcpURL := strings.TrimSpace(os.Getenv("JUTE_MCP_URL"))
	if mcpURL == "" {
		mcpURL = "http://127.0.0.1:8790/mcp"
	}

	// Initialize Gemini Model client using ADK's native Gemini model provider
	llm, err := gemini.NewModel(ctx, modelID, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return fmt.Errorf("failed to create Gemini model: %w", err)
	}

	agentID := strings.TrimSpace(os.Getenv("JUTE_MCP_AGENT_ID"))

	log.Printf("Connecting to Jute MCP bridge: %s (agent-id: %q)", mcpURL, agentID)
	mcpHTTPClient := &http.Client{}
	if agentID != "" {
		mcpHTTPClient.Transport = &headerRoundTripper{
			base:    http.DefaultTransport,
			headers: map[string]string{"X-Jute-Agent-ID": agentID},
		}
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint:             mcpURL,
		DisableStandaloneSSE: true,
		HTTPClient:           mcpHTTPClient,
	}
	mcpToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: transport,
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP toolset: %w", err)
	}

	// Instructions for Jute home assistant
	instruction := buildSystemInstruction()

	// Create ADK agent
	a, err := llmagent.New(llmagent.Config{
		Name:        "gemini_a2a_assistant",
		Description: "A helpful assistant running on Gemini and exposed over A2A.",
		Model:       llm,
		Instruction: instruction,
		Toolsets:    []adktool.Toolset{mcpToolset},
		GenerateContentConfig: &genai.GenerateContentConfig{
			MaxOutputTokens: 512,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create ADK agent: %w", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(a),
		A2AOptions: []a2asrv.RequestHandlerOption{
			a2asrv.WithTaskStore(taskstore.NewInMemory(&taskstore.InMemoryStoreConfig{
				Authenticator: func(ctx context.Context) (string, error) {
					return "local-user", nil
				},
			})),
		},
	}
	writeTimeout := strings.TrimSpace(os.Getenv("A2A_WRITE_TIMEOUT"))
	if writeTimeout == "" {
		writeTimeout = "10m"
	}
	readTimeout := strings.TrimSpace(os.Getenv("A2A_READ_TIMEOUT"))
	if readTimeout == "" {
		readTimeout = "10m"
	}

	l := full.NewLauncher()
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{
			"web",
			"--port", "9898",
			"--write-timeout", writeTimeout,
			"--read-timeout", readTimeout,
			"a2a",
			"--a2a_agent_url", "http://localhost:9898",
		}
	}
	if err := l.Execute(ctx, config, args); err != nil {
		return fmt.Errorf("launcher failed: %w", err)
	}
	return nil
}

func buildSystemInstruction() string {
	parts := []string{
		"You are a Jute Dash assistant for a home dashboard.",
		"Reply briefly, clearly, and conversationally.",
		"Return only the final user-facing answer.",
		"Never include private reasoning, scratchpad text, analysis, tool-selection notes, or function-call plans in your answer.",
		"Do not say whether you need or do not need to call tools.",
		"Use only information from the user and from tools you actually call.",
		"Jute MCP tools are available and expose the dashboard through Widget Skills.",
		"For questions about the current dashboard, visible widgets, weather, date, time, conversation history, or what Jute can do, inspect Jute MCP before answering.",
		"Start by listing available Widget Skills with jute_skill_list when you need to know what dashboard abilities exist.",
		"For weather questions, read the jute.weather.current skill context with jute_skill_read_context; if an action is needed, invoke only the declared refresh action through jute_skill_invoke_action.",
		"For date or time questions, read the jute.date_time.current skill context with jute_skill_read_context.",
		"For chat history or agent status questions, read the jute.chat_history.current skill context with jute_skill_read_context.",
		"For music playback requests, first call jute_skill_list, choose the exact visible Spotify or Apple Music skillId and widgetInstanceId, then invoke only one of that skill's declared action IDs.",
		"For RSS, headline, link, or article-reading requests, first call jute_skill_list, choose the visible RSS skillId and widgetInstanceId, then use read_article or grep_article with the article URL and optional query.",
		"For stocks, shares, crypto, commodities, or market-price requests, first call jute_skill_list, choose the visible Markets skillId and widgetInstanceId, then use query_stock, query_share, query_crypto, or query_ticker with the ticker symbol.",
		"Prefer specific Widget Skill context over broad dashboard context when the user asks about one widget.",
		"Never answer that you lack weather, time, dashboard, or widget data until you have checked the relevant Jute MCP tool and it is unavailable, unauthorized, or missing.",
		"If MCP context is unavailable or unauthorized, say that Jute dashboard context is unavailable and ask the user to check the local MCP connection.",
		"For simple greetings or ordinary chat, do not call tools.",
		"Do not invent capabilities, tools, widgets, actions, weather values, locations, or agent state that are not returned by Jute MCP. Never use generic music_player skill IDs or default_music_widget instance IDs.",
	}
	return strings.Join(parts, " ")
}
