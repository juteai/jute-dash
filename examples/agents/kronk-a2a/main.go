// Command kronk-a2a exposes a local Kronk-backed ADK agent through A2A.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2asrv"
	krnk "github.com/ardanlabs/kronk/sdk/kronk"
	krnkmodel "github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/ardanlabs/kronk/sdk/tools/defaults"
	"github.com/ardanlabs/kronk/sdk/tools/libs"
	"github.com/ardanlabs/kronk/sdk/tools/models"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/remoteagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/server/adka2a"
	"google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"

	"jute-dash/internal/mcpclient"

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
		mode = "launcher"
	}
	a2aServerAddress, closeAgent, err := startKronkAgentServer(ctx)
	if err != nil {
		return err
	}
	defer closeAgent()

	if mode == "server" {
		log.Printf("Kronk A2A server mode; waiting for shutdown signal")
		<-ctx.Done()
		return nil
	}
	if mode != "launcher" {
		return fmt.Errorf("unsupported KRONK_A2A_MODE %q", mode)
	}

	remoteAgent, err := remoteagent.NewA2A(remoteagent.A2AConfig{
		Name:            "A2A Kronk assistant",
		Description:     "A remote ADK agent served over A2A and backed by a local Kronk model.",
		AgentCardSource: a2aServerAddress,
	})
	if err != nil {
		return fmt.Errorf("create remote agent: %w", err)
	}

	launcherCfg := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(remoteAgent),
	}
	l := full.NewLauncher()
	if err := l.Execute(ctx, launcherCfg, os.Args[1:]); err != nil {
		return fmt.Errorf("run failed: %w\n\n%s", err, l.CommandLineSyntax())
	}
	return nil
}

func startKronkAgentServer(ctx context.Context) (string, func(), error) {
	a, closeAgent, err := newKronkAgent(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("create Kronk agent: %w", err)
	}

	listenAddress := strings.TrimSpace(os.Getenv("KRONK_A2A_LISTEN"))
	if listenAddress == "" {
		listenAddress = defaultListenAddress
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", listenAddress)
	if err != nil {
		closeAgent()
		return "", nil, fmt.Errorf("bind A2A server: %w", err)
	}

	baseURL := &url.URL{Scheme: "http", Host: listener.Addr().String()}
	agentPath := "/invoke"
	agentCard := &a2a.AgentCard{
		Name:               a.Name(),
		Description:        a.Description(),
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills:             adka2a.BuildAgentSkills(a),
		PreferredTransport: a2a.TransportProtocolJSONRPC,
		URL:                baseURL.JoinPath(agentPath).String(),
		Capabilities:       a2a.AgentCapabilities{Streaming: true},
	}

	mux := http.NewServeMux()
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))
	executor := adka2a.NewExecutor(adka2a.ExecutorConfig{
		RunnerConfig: runner.Config{
			AppName:        a.Name(),
			Agent:          a,
			SessionService: session.InMemoryService(),
		},
	})
	requestHandler := a2asrv.NewHandler(executor)
	mux.Handle(agentPath, a2asrv.NewJSONRPCHandler(requestHandler))

	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Kronk A2A Agent Card: %s", baseURL.JoinPath(a2asrv.WellKnownAgentCardPath).String())
		log.Printf("Kronk A2A JSON-RPC endpoint: %s", baseURL.JoinPath(agentPath).String())
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
	return baseURL.String(), closeAll, nil
}

func newKronkAgent(ctx context.Context) (agent.Agent, func(), error) {
	modelID := strings.TrimSpace(os.Getenv("KRONK_MODEL_ID"))
	sourceURL := strings.TrimSpace(os.Getenv("KRONK_MODEL_URL"))
	if modelID == "" && sourceURL == "" {
		modelID = defaultModelID
		log.Printf("KRONK_MODEL_ID / KRONK_MODEL_URL unset, defaulting to catalog model %q", modelID)
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

	tools, err := juteMCPTools()
	if err != nil {
		closeAgent()
		return nil, nil, err
	}
	instruction := "You reply briefly and clearly using only the information the user provides."
	if len(tools) > 0 {
		instruction += " You may use the Jute MCP tools to inspect visible dashboard Widget Skills and safe public widget context. Do not infer hidden widgets, secrets, raw microphone audio, camera frames, or private household state."
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "kronk_a2a_assistant",
		Description: "A helpful assistant running on a local Kronk model and exposed over A2A.",
		Model:       llm,
		Instruction: instruction,
		Tools:       tools,
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

func juteMCPTools() ([]adktool.Tool, error) {
	client, configured, err := mcpclient.NewFromEnv()
	if !configured {
		log.Printf("JUTE_MCP_URL unset; Kronk agent will run without Jute MCP tools")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("configure Jute MCP tools: %w", err)
	}
	log.Printf("Jute MCP tools enabled via JUTE_MCP_URL")

	tools := []adktool.Tool{}
	add := func(t adktool.Tool, err error) error {
		if err != nil {
			return err
		}
		tools = append(tools, t)
		return nil
	}
	if err := add(functiontool.New[emptyArgs, map[string]any](functiontool.Config{
		Name:        "jute_dashboard_context_get",
		Description: "Get safe current Jute dashboard context and visible Widget Skills.",
	}, func(ctx adktool.Context, args emptyArgs) (map[string]any, error) {
		text, err := client.ReadResourceText(ctx, "jute://dashboard/current")
		return textResult(text, err)
	})); err != nil {
		return nil, err
	}
	if err := add(functiontool.New[emptyArgs, map[string]any](functiontool.Config{
		Name:        "jute_skill_list",
		Description: "List visible Jute Widget Skills and their public summaries.",
	}, func(ctx adktool.Context, args emptyArgs) (map[string]any, error) {
		text, err := client.ReadResourceText(ctx, "jute://skills")
		return textResult(text, err)
	})); err != nil {
		return nil, err
	}
	if err := add(functiontool.New[skillContextArgs, map[string]any](functiontool.Config{
		Name:        "jute_skill_read_context",
		Description: "Read public context for a visible Jute Widget Skill.",
	}, func(ctx adktool.Context, args skillContextArgs) (map[string]any, error) {
		result, err := client.CallTool(ctx, "jute_skill_read_context", map[string]any{
			"skillId":          args.SkillID,
			"widgetInstanceId": args.WidgetInstanceID,
		})
		return toolResult(result, err)
	})); err != nil {
		return nil, err
	}
	if err := add(functiontool.New[skillActionArgs, map[string]any](functiontool.Config{
		Name:        "jute_skill_invoke_action",
		Description: "Invoke a declared low-risk Jute Widget Skill action through the hub.",
	}, func(ctx adktool.Context, args skillActionArgs) (map[string]any, error) {
		result, err := client.CallTool(ctx, "jute_skill_invoke_action", map[string]any{
			"skillId":          args.SkillID,
			"widgetInstanceId": args.WidgetInstanceID,
			"actionId":         args.ActionID,
		})
		return toolResult(result, err)
	})); err != nil {
		return nil, err
	}
	if err := add(functiontool.New[skillPromptArgs, map[string]any](functiontool.Config{
		Name:        "jute_skill_prompt_get",
		Description: "Get hub-approved prompt guidance for a Jute Widget Skill.",
	}, func(ctx adktool.Context, args skillPromptArgs) (map[string]any, error) {
		result, err := client.CallTool(ctx, "jute_skill_prompt_get", map[string]any{
			"skillId":  args.SkillID,
			"promptId": args.PromptID,
		})
		return toolResult(result, err)
	})); err != nil {
		return nil, err
	}
	return tools, nil
}

type emptyArgs struct{}

type skillContextArgs struct {
	SkillID          string `json:"skillId" jsonschema:"Jute Widget Skill ID."`
	WidgetInstanceID string `json:"widgetInstanceId,omitempty" jsonschema:"Optional widget instance ID."`
}

type skillActionArgs struct {
	SkillID          string `json:"skillId" jsonschema:"Jute Widget Skill ID."`
	WidgetInstanceID string `json:"widgetInstanceId,omitempty" jsonschema:"Optional widget instance ID."`
	ActionID         string `json:"actionId" jsonschema:"Widget Skill action ID."`
}

type skillPromptArgs struct {
	SkillID  string `json:"skillId" jsonschema:"Jute Widget Skill ID."`
	PromptID string `json:"promptId" jsonschema:"Widget Skill prompt ID."`
}

func textResult(text string, err error) (map[string]any, error) {
	if err != nil {
		return nil, err
	}
	return map[string]any{"text": text}, nil
}

func toolResult(result mcpclient.ToolCallResult, err error) (map[string]any, error) {
	if err != nil {
		return nil, err
	}
	out := map[string]any{"isError": result.IsError}
	if len(result.StructuredContent) > 0 {
		var structured any
		if err := json.Unmarshal(result.StructuredContent, &structured); err == nil {
			out["structuredContent"] = structured
		} else {
			out["structuredContent"] = string(result.StructuredContent)
		}
	}
	if len(result.Content) > 0 {
		out["text"] = result.Content[0].Text
	}
	return out, nil
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
