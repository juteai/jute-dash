// Command kronk-a2a exposes a local Kronk-backed ADK agent through A2A 1.0.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	krnk "github.com/ardanlabs/kronk/sdk/kronk"
	krnkmodel "github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/ardanlabs/kronk/sdk/tools/defaults"
	"github.com/ardanlabs/kronk/sdk/tools/libs"
	"github.com/ardanlabs/kronk/sdk/tools/models"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
	"google.golang.org/genai"

	kronkllm "github.com/craigh33/adk-go-kronk/kronk"
)

const (
	defaultModelID       = "Qwen/Qwen3-8B-Q8_0"
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

	modelID := strings.TrimSpace(os.Getenv("KRONK_MODEL_ID"))
	sourceURL := strings.TrimSpace(os.Getenv("KRONK_MODEL_URL"))
	if modelID == "" && sourceURL == "" {
		modelID = defaultModelID
		log.Printf("KRONK_MODEL_ID / KRONK_MODEL_URL unset, defaulting to catalog model %q", modelID)
	}

	mp, err := installSystem(ctx, modelID, sourceURL)
	if err != nil {
		return fmt.Errorf("install kronk runtime: %w", err)
	}

	cfg := kronkllm.Config{ModelFiles: mp.ModelFiles}
	if strings.TrimSpace(mp.ProjFile) != "" {
		cfg.ModelOptions = append(cfg.ModelOptions, krnkmodel.WithProjFile(mp.ProjFile))
	}

	llm, err := kronkllm.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build kronk llm provider: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if cerr := llm.Close(closeCtx); cerr != nil {
			log.Printf("close kronk llm: %v", cerr)
		}
	}()

	if mode == "selftest" {
		return runSelftest(ctx, llm)
	}

	mcpURL := strings.TrimSpace(os.Getenv("JUTE_MCP_URL"))
	if mcpURL == "" {
		mcpURL = "http://127.0.0.1:8790/mcp"
	}

	log.Printf("Connecting to Jute MCP bridge: %s", mcpURL)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             mcpURL,
		DisableStandaloneSSE: true,
	}
	mcpToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: transport,
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP toolset: %w", err)
	}

	instruction := kronkInstruction()

	a, err := llmagent.New(llmagent.Config{
		Name:        "kronk_a2a_assistant",
		Description: "A helpful assistant running on a local Kronk model and exposed over A2A.",
		Model:       llm,
		Instruction: instruction,
		Toolsets:    []adktool.Toolset{mcpToolset},
		GenerateContentConfig: &genai.GenerateContentConfig{
			MaxOutputTokens: 512,
		},
	})
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(a),
	}
	l := full.NewLauncher()
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"web", "a2a", "--port", "9797", "--a2a_agent_url", "http://localhost:9797"}
	}
	if err := l.Execute(ctx, config, args); err != nil {
		return fmt.Errorf("launcher: %w", err)
	}
	return nil
}

func runSelftest(ctx context.Context, llm model.LLM) error {
	processor := strings.TrimSpace(os.Getenv("KRONK_PROCESSOR"))
	if processor == "" {
		processor = "(default)"
	}
	log.Printf("Kronk selftest: running a single inference with processor=%s", processor)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText("Reply with one short word.", genai.RoleUser),
		},
	}
	var lastErr error
	for resp, err := range llm.GenerateContent(ctx, req, false) {
		if err != nil {
			lastErr = err
			break
		}
		_ = resp
	}
	if lastErr != nil {
		return fmt.Errorf("selftest inference failed: %w", lastErr)
	}
	log.Printf("Kronk selftest: success")
	return nil
}

func kronkInstruction() string {
	parts := []string{
		"You are a Jute Dash local test assistant for a home dashboard.",
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
		"Prefer specific Widget Skill context over broad dashboard context when the user asks about one widget.",
		"Never answer that you lack weather, time, dashboard, or widget data until you have checked the relevant Jute MCP tool and it is unavailable, unauthorized, or missing.",
		"If MCP context is unavailable or unauthorized, say that Jute dashboard context is unavailable and ask the user to check the local MCP connection.",
		"For simple greetings or ordinary chat, do not call tools.",
		"Do not invent capabilities, tools, widgets, actions, weather values, locations, or agent state that are not returned by Jute MCP.",
	}
	return strings.Join(parts, " ")
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
