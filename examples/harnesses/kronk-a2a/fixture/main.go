// Command kronk-a2a exposes a local Kronk-backed ADK agent through A2A 1.0.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	krnk "github.com/ardanlabs/kronk/sdk/kronk"
	krnkmodel "github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/ardanlabs/kronk/sdk/tools/defaults"
	"github.com/ardanlabs/kronk/sdk/tools/libs"
	"github.com/ardanlabs/kronk/sdk/tools/models"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
	"google.golang.org/genai"

	kronkllm "github.com/craigh33/adk-go-kronk/kronk"
)

const (
	defaultModelID       = "Qwen3-0.6B-Q8_0"
	defaultListenAddress = "127.0.0.1:9797"
	installPhaseTimeout  = 25 * time.Minute
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("%v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mode := strings.TrimSpace(os.Getenv("KRONK_A2A_MODE"))
	if mode == "" {
		mode = "server"
	}
	a2aServer, closeAgent, err := startKronkAgentServer(ctx)
	if err != nil {
		return err
	}
	defer closeAgent()

	switch mode {
	case "server":
		log.Printf("Kronk A2A server mode; waiting for shutdown signal")
		<-ctx.Done()
		return nil
	case "selftest":
		return runSelftest(ctx, a2aServer)
	case "console", "launcher":
		return runConsole(ctx, a2aServer)
	default:
		return fmt.Errorf("unsupported KRONK_A2A_MODE %q", mode)
	}
}

// runSelftest loads the model and runs a single end-to-end inference so the
// caller can validate that the active processor (KRONK_PROCESSOR) actually
// completes a chat without aborting. The harness Makefile uses this to probe
// Metal vs CPU on first run. Any native abort() inside libllama terminates
// the process with a nonzero exit, which is exactly what the probe needs.
func runSelftest(ctx context.Context, server *kronkA2AServer) error {
	processor := strings.TrimSpace(os.Getenv("KRONK_PROCESSOR"))
	if processor == "" {
		processor = "(default)"
	}
	log.Printf("Kronk selftest: running a single inference with processor=%s", processor)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	answer, err := server.generateAnswer(ctx, "selftest-"+newID(), "Reply with one short word.")
	if err != nil {
		return fmt.Errorf("selftest inference failed: %w", err)
	}
	log.Printf("Kronk selftest: success (answer=%q)", strings.TrimSpace(answer))
	return nil
}

func startKronkAgentServer(ctx context.Context) (*kronkA2AServer, func(), error) {
	a, closeAgent, err := newKronkAgent(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create Kronk agent: %w", err)
	}

	listenAddress := strings.TrimSpace(os.Getenv("KRONK_A2A_LISTEN"))
	if listenAddress == "" {
		listenAddress = defaultListenAddress
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", listenAddress)
	if err != nil {
		closeAgent()
		return nil, nil, fmt.Errorf("bind A2A server: %w", err)
	}

	baseURL := &url.URL{Scheme: "http", Host: listener.Addr().String()}
	agentPath := "/invoke"
	a2aServer, err := newKronkA2AServer(a, baseURL.String())
	if err != nil {
		closeAgent()
		return nil, nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", a2aServer.handleAgentCard)
	mux.HandleFunc(agentPath, a2aServer.handleInvoke)

	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Kronk A2A 1.0 Agent Card: %s", baseURL.JoinPath(".well-known", "agent-card.json").String())
		log.Printf("Kronk A2A 1.0 JSON-RPC endpoint: %s", baseURL.JoinPath(agentPath).String())
		if serveErr := httpServer.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Printf("A2A server stopped unexpectedly: %v", serveErr)
		}
	}()

	closeAll := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown A2A server: %v", err)
		}
		closeAgent()
	}
	return a2aServer, closeAll, nil
}

func runConsole(ctx context.Context, server *kronkA2AServer) error {
	log.Printf("Kronk console mode; the A2A 1.0 server remains available while you chat locally")
	scanner := bufio.NewScanner(os.Stdin)
	sessionID := "console-" + newID()
	fmt.Print("\nUser -> ")
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			fmt.Print("\nUser -> ")
			continue
		}
		answer, err := server.generateAnswer(ctx, sessionID, text)
		if err != nil {
			fmt.Printf("\nAgent error: %v\n", err)
		} else {
			fmt.Printf("\nAgent -> %s\n", answer)
		}
		fmt.Print("\nUser -> ")
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read console input: %w", err)
	}
	return nil
}

func newKronkAgent(ctx context.Context) (agent.Agent, func(), error) {
	modelID := strings.TrimSpace(os.Getenv("KRONK_MODEL_ID"))
	sourceURL := strings.TrimSpace(os.Getenv("KRONK_MODEL_URL"))
	if modelID == "" && sourceURL == "" {
		modelID = defaultModelID
		log.Printf("KRONK_MODEL_ID / KRONK_MODEL_URL unset, defaulting to catalog model %q", modelID)
	}

	mcpURL := strings.TrimSpace(os.Getenv("JUTE_MCP_URL"))
	mcpToken := strings.TrimSpace(os.Getenv("JUTE_MCP_TOKEN"))
	mcpAgentID := strings.TrimSpace(os.Getenv("JUTE_MCP_AGENT_ID"))
	if mcpURL != "" {
		if err := probeJuteMCP(ctx, mcpURL, mcpToken, mcpAgentID); err != nil {
			return nil, nil, fmt.Errorf("JUTE_MCP_URL is set but Jute MCP tools are unavailable: %w", err)
		}
	}

	mp, err := installSystem(ctx, modelID, sourceURL)
	if err != nil {
		return nil, nil, fmt.Errorf("install kronk runtime: %w", err)
	}

	cfg := kronkllm.Config{ModelFiles: mp.ModelFiles}
	if strings.TrimSpace(mp.ProjFile) != "" {
		cfg.ModelOptions = append(cfg.ModelOptions, krnkmodel.WithProjFile(mp.ProjFile))
	}

	llm, err := kronkllm.New(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build kronk llm provider: %w", err)
	}

	closeAgent := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if cerr := llm.Close(closeCtx); cerr != nil {
			log.Printf("close kronk llm: %v", cerr)
		}
	}

	var toolsets []adktool.Toolset
	if mcpURL != "" {
		log.Printf("MCP tools enabled via JUTE_MCP_URL: %s", mcpURL)
		transport := &HTTPPostTransport{
			URL:         mcpURL,
			BearerToken: mcpToken,
			AgentID:     mcpAgentID,
		}
		mcpToolset, err := mcptoolset.New(mcptoolset.Config{
			Transport: transport,
		})
		if err != nil {
			closeAgent()
			return nil, nil, fmt.Errorf("failed to create MCP toolset: %w", err)
		}
		toolsets = append(toolsets, mcpToolset)
	} else {
		log.Printf("JUTE_MCP_URL unset; Kronk agent will run without MCP tools")
	}

	instruction := kronkInstruction(len(toolsets) > 0)

	a, err := llmagent.New(llmagent.Config{
		Name:        "kronk_a2a_assistant",
		Description: "A helpful assistant running on a local Kronk model and exposed over A2A.",
		Model:       llm,
		Instruction: instruction,
		Toolsets:    toolsets,
		GenerateContentConfig: &genai.GenerateContentConfig{
			MaxOutputTokens: 512,
		},
	})
	if err != nil {
		closeAgent()
		return nil, nil, fmt.Errorf("agent: %w", err)
	}
	return a, closeAgent, nil
}

func kronkInstruction(mcpEnabled bool) string {
	parts := []string{
		"You are a Jute Dash local test assistant for a home dashboard.",
		"Reply briefly, clearly, and conversationally.",
		"Return only the final user-facing answer.",
		"Never include private reasoning, scratchpad text, analysis, tool-selection notes, or function-call plans in your answer.",
		"Do not say whether you need or do not need to call tools.",
		"Use only information from the user and from tools you actually call.",
	}
	if mcpEnabled {
		parts = append(
			parts,
			"Jute MCP tools are available and expose the dashboard through Widget Skills.",
			"For questions about the current dashboard, visible widgets, weather, date, time, conversation history, or what Jute can do, inspect Jute MCP before answering.",
			"Start by listing available Widget Skills with jute_skill_list when you need to know what dashboard abilities exist.",
			"For weather questions, read the jute.weather.current skill context with jute_skill_read_context; if an action is needed, invoke only the declared refresh action through jute_skill_invoke_action.",
			"For date or time questions, read the jute.date_time.current skill context.",
			"For chat history or agent status questions, read the jute.chat_history.current skill context.",
			"Prefer specific Widget Skill context over broad dashboard context when the user asks about one widget.",
			"Never answer that you lack weather, time, dashboard, or widget data until you have checked the relevant Jute MCP tool and it is unavailable, unauthorized, or missing.",
			"If MCP context is unavailable or unauthorized, say that Jute dashboard context is unavailable and ask the user to check the local MCP connection.",
			"For simple greetings or ordinary chat, do not call tools.",
			"Do not invent capabilities, tools, widgets, actions, weather values, locations, or agent state that are not returned by Jute MCP.",
		)
	} else {
		parts = append(
			parts,
			"Jute MCP tools are not configured for this run.",
			"If the user asks for live dashboard, weather, widget, or time context, explain that the local Jute MCP connection is not enabled for this agent.",
		)
	}
	return strings.Join(parts, " ")
}

func probeJuteMCP(ctx context.Context, mcpURL, token, agentID string) error {
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := postMCPProbe(probeCtx, mcpURL, token, agentID, "tools/list", nil, &result); err != nil {
		return err
	}
	for _, tool := range result.Tools {
		if tool.Name == "jute_skill_read_context" {
			return nil
		}
	}
	return fmt.Errorf("tools/list did not include jute_skill_read_context")
}

func postMCPProbe(ctx context.Context, mcpURL, token, agentID, method string, params any, result any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return fmt.Errorf("encode probe request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create probe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if agentID != "" {
		req.Header.Set("X-Jute-Agent-ID", agentID)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("connect to Jute MCP: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Jute MCP returned HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode Jute MCP response: %w", err)
	}
	if envelope.Error != nil {
		return fmt.Errorf("Jute MCP %s failed: %s", method, envelope.Error.Message)
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return fmt.Errorf("Jute MCP %s returned no result", method)
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode Jute MCP %s result: %w", method, err)
	}
	return nil
}

// HTTPPostTransport implements a custom mcp.Transport for HTTP POST JSON-RPC.
type HTTPPostTransport struct {
	URL         string
	BearerToken string
	AgentID     string
	HTTPClient  *http.Client
}

// Connect creates a new mcp.Connection.
func (t *HTTPPostTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	client := t.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	return &httpConnection{
		url:         t.URL,
		bearerToken: t.BearerToken,
		agentID:     t.AgentID,
		httpClient:  client,
		incoming:    make(chan jsonrpc.Message, 16),
		closed:      make(chan struct{}),
	}, nil
}

type httpConnection struct {
	url         string
	bearerToken string
	agentID     string
	httpClient  *http.Client

	mu       sync.Mutex
	incoming chan jsonrpc.Message
	closed   chan struct{}
	isClosed bool
}

func (c *httpConnection) SessionID() string {
	return ""
}

func (c *httpConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.isClosed {
		c.isClosed = true
		close(c.closed)
	}
	return nil
}

func (c *httpConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, io.EOF
	case msg, ok := <-c.incoming:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	}
}

func (c *httpConnection) Write(ctx context.Context, msg jsonrpc.Message) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
		return io.ErrClosedPipe
	default:
	}

	reqBytes, err := jsonrpc.EncodeMessage(msg)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	var hasID bool
	if req, ok := msg.(*jsonrpc.Request); ok && req.ID.IsValid() {
		hasID = true
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(reqBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.bearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	if c.agentID != "" {
		httpReq.Header.Set("X-Jute-Agent-ID", c.agentID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http post failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	if !hasID {
		return nil
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	respMsg, err := jsonrpc.DecodeMessage(respBytes)
	if err != nil {
		return fmt.Errorf("decode response message: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
		return io.ErrClosedPipe
	case c.incoming <- respMsg:
		return nil
	}
}

func installSystem(ctx context.Context, modelID, sourceURL string) (models.Path, error) {
	ctx, cancel := context.WithTimeout(ctx, installPhaseTimeout)
	defer cancel()

	lib, err := libs.New(libs.WithVersion(defaults.LibVersion("")))
	if err != nil {
		return models.Path{}, err
	}
	if _, err := lib.Download(ctx, krnk.FmtLogger); err != nil {
		return models.Path{}, err
	}

	mdls, err := models.New()
	if err != nil {
		return models.Path{}, err
	}

	switch {
	case sourceURL != "":
		log.Printf("downloading model from URL: %q", sourceURL)
		return mdls.Download(ctx, krnk.FmtLogger, sourceURL)
	default:
		log.Printf("downloading model from catalog: %q", modelID)
		return mdls.Download(ctx, krnk.FmtLogger, modelID)
	}
}
