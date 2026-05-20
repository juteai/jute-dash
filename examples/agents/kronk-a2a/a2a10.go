package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	a2av2 "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	adka2a "google.golang.org/adk/server/adka2a/v2"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

const dashboardContextExtensionURI = "https://jute.dev/a2a/extensions/dashboard-context/v1"

type kronkA2AServer struct {
	runner        *runner.Runner
	agentCard     *a2av2.AgentCard
	cardHandler   http.Handler
	invokeHandler http.Handler
}

func newKronkA2AServer(a agent.Agent, baseURL string) (*kronkA2AServer, error) {
	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:           a.Name(),
		Agent:             a,
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create ADK runner: %w", err)
	}

	capabilities := a2av2.AgentCapabilities{
		Streaming: true,
		Extensions: []a2av2.AgentExtension{
			{
				URI:         dashboardContextExtensionURI,
				Description: "Receives redacted Jute dashboard context in message metadata.",
			},
		},
	}
	card := &a2av2.AgentCard{
		Name:        a.Name(),
		Description: a.Description(),
		Version:     "1.0.0",
		SupportedInterfaces: []*a2av2.AgentInterface{
			a2av2.NewAgentInterface(strings.TrimRight(baseURL, "/")+"/invoke", a2av2.TransportProtocolJSONRPC),
		},
		Capabilities:       capabilities,
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills:             agentSkills(a),
	}

	executor := adka2a.NewExecutor(adka2a.ExecutorConfig{
		RunnerConfig: runner.Config{
			AppName:           a.Name(),
			Agent:             a,
			SessionService:    sessionService,
			AutoCreateSession: true,
		},
		RunConfig: agent.RunConfig{StreamingMode: agent.StreamingModeNone},
	})
	requestHandler := a2asrv.NewHandler(
		executor,
		a2asrv.WithCapabilityChecks(&capabilities),
		a2asrv.WithCallInterceptors(localDevUserInterceptor{}),
	)

	return &kronkA2AServer{
		runner:        r,
		agentCard:     card,
		cardHandler:   a2asrv.NewStaticAgentCardHandler(card),
		invokeHandler: a2asrv.NewJSONRPCHandler(requestHandler),
	}, nil
}

func agentSkills(a agent.Agent) []a2av2.AgentSkill {
	skills := adka2a.BuildAgentSkills(a)
	if len(skills) > 0 {
		return skills
	}
	return []a2av2.AgentSkill{
		{
			ID:          a.Name(),
			Name:        "Local Kronk chat",
			Description: "Replies with a local Kronk model.",
			Tags:        []string{"chat", "local", "kronk"},
			InputModes:  []string{"text/plain"},
			OutputModes: []string{"text/plain"},
		},
	}
}

type localDevUserInterceptor struct {
	a2asrv.PassthroughCallInterceptor
}

func (localDevUserInterceptor) Before(ctx context.Context, callCtx *a2asrv.CallContext, _ *a2asrv.Request) (context.Context, any, error) {
	callCtx.User = a2asrv.NewAuthenticatedUser("jute-local-dev", nil)
	return ctx, nil, nil
}

func (s *kronkA2AServer) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	s.cardHandler.ServeHTTP(w, r)
}

func (s *kronkA2AServer) handleInvoke(w http.ResponseWriter, r *http.Request) {
	s.invokeHandler.ServeHTTP(w, r)
}

func (s *kronkA2AServer) generateAnswer(ctx context.Context, contextID, text string) (string, error) {
	userMessage := genai.NewContentFromText(text, genai.RoleUser)
	finalText := ""
	allText := []string{}
	for event, err := range s.runner.Run(ctx, "jute-user", contextID, userMessage, agent.RunConfig{StreamingMode: agent.StreamingModeNone}) {
		if err != nil {
			return "", err
		}
		if event == nil || event.LLMResponse.Content == nil {
			continue
		}
		eventText := textFromGenAIParts(event.LLMResponse.Content.Parts)
		if strings.TrimSpace(eventText) == "" {
			continue
		}
		allText = append(allText, eventText)
		if event.IsFinalResponse() {
			finalText = eventText
		}
	}
	if strings.TrimSpace(finalText) != "" {
		return finalText, nil
	}
	if len(allText) > 0 {
		return allText[len(allText)-1], nil
	}
	return "Kronk returned an empty response.", nil
}

func textFromGenAIParts(parts []*genai.Part) string {
	chunks := []string{}
	for _, item := range parts {
		if item == nil {
			continue
		}
		if text := strings.TrimSpace(item.Text); text != "" {
			chunks = append(chunks, text)
		}
	}
	return strings.Join(chunks, "")
}

func newID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
