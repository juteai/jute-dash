package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/registry"
)

var (
	errYAMLConfigRequired      = errors.New("YAML config file is required")
	errAgentHistoryUnsupported = errors.New("agent history is unavailable")
)

type ControllerOptions struct {
	Registry            registry.Registry
	CardService         *CardService
	Messages            a2aclient.MessageSender
	TurnRunner          *Runner
	ConfigPath          string
	GetAgentsConfig     func() []AgentConfig
	SaveAgentsConfig    func([]AgentConfig) error
	GetDashboardContext func(ctx context.Context) map[string]any
	OnRegistryUpdated   func(r registry.Registry)
}

type Controller struct {
	mu   sync.Mutex
	opts ControllerOptions
}

func NewController(opts ControllerOptions) *Controller {
	return &Controller{opts: opts}
}

func (c *Controller) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/agents", c.handleAgents)
	mux.HandleFunc("/api/v1/agents/", c.handleAgentSubroutes)
	mux.HandleFunc("/api/v1/messages", c.handleMessages)
	mux.HandleFunc("/api/v1/conversations", c.handleConversations)
	mux.HandleFunc("/api/v1/conversations/", c.handleConversationSubroutes)
}

func (c *Controller) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.writeJSON(w, http.StatusOK, map[string]any{
			"agents": c.agentsWithDiscovery(r.Context(), true),
		})
	case http.MethodPost:
		var req struct {
			CardURL string `json:"cardUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		agent, err := c.addAgentFromCard(r.Context(), req.CardURL)
		if err != nil {
			c.writeAgentConfigError(w, err)
			return
		}
		c.writeJSON(w, http.StatusCreated, agent)
	default:
		c.writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (c *Controller) handleAgentSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		c.writeError(w, http.StatusNotFound, "agent route not found")
		return
	}
	agentID := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPatch:
			var req struct {
				Enabled *bool `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
				return
			}
			agent, err := c.patchAgent(agentID, req.Enabled)
			if err != nil {
				c.writeAgentConfigError(w, err)
				return
			}
			c.writeJSON(w, http.StatusOK, agent)
		case http.MethodDelete:
			if err := c.deleteAgent(agentID); err != nil {
				c.writeAgentConfigError(w, err)
				return
			}
			c.writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
		default:
			c.writeMethodNotAllowed(w, http.MethodPatch+", "+http.MethodDelete)
		}
		return
	}
	if len(parts) != 2 || parts[1] != "refresh-card" {
		c.writeError(w, http.StatusNotFound, "agent route not found")
		return
	}
	if r.Method != http.MethodPost {
		c.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	agent, ok := c.opts.Registry.Find(agentID)
	if !ok {
		c.writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	cache := c.refreshAgentCard(r.Context(), agent)
	enriched := c.agentWithDiscovery(agent, cache)
	c.writeJSON(w, http.StatusOK, enriched)
}

func (c *Controller) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		c.writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	req.AgentID = strings.TrimSpace(req.AgentID)
	req.Text = strings.TrimSpace(req.Text)
	if req.AgentID == "" {
		c.writeError(w, http.StatusBadRequest, "agentId is required")
		return
	}
	if req.Text == "" {
		c.writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	agent, ok := c.opts.Registry.Find(req.AgentID)
	if !ok {
		c.writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if !agent.Enabled {
		c.writeError(w, http.StatusConflict, "agent is disabled")
		return
	}

	configuredAgent, ok := c.configuredAgent(req.AgentID)
	if !ok {
		c.writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	selected := c.selectedAgentInterface(r.Context(), agent)
	if selected.ProtocolBinding != a2aclient.ProtocolJSONRPC {
		c.writeError(w, http.StatusNotImplemented, "agent protocol binding is not implemented yet")
		return
	}

	bearerToken, ok := AgentBearerToken(configuredAgent)
	if !ok {
		c.writeError(w, http.StatusServiceUnavailable, "agent credentials are not available")
		return
	}

	result, err := c.opts.Messages.SendMessage(r.Context(), a2aclient.SendMessageRequest{
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		ProtocolVersion: selected.ProtocolVersion,
		Text:            req.Text,
		BearerToken:     bearerToken,
		ConversationID:  strings.TrimSpace(req.ConversationID),
		Extensions:      selected.Extensions,
		Metadata:        selected.Metadata,
	})
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, a2aclient.ErrUnsupportedProtocol) {
			status = http.StatusNotImplemented
		}
		c.writeError(w, status, "agent request failed")
		return
	}

	c.writeJSON(w, http.StatusOK, MessageResponse{
		ConversationID: result.ConversationID,
		TaskID:         result.TaskID,
		AgentID:        agent.ID,
		Status:         result.Status,
		Message:        result.Text,
	})
}

func (c *Controller) handleConversations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		agent, selected, bearerToken, ok := c.agentForHistoryRequest(w, r)
		if !ok {
			return
		}
		conversations, err := c.listAgentConversations(r.Context(), agent, selected, bearerToken)
		if err != nil {
			c.writeConversationError(w, err)
			return
		}
		c.writeJSON(w, http.StatusOK, map[string]any{"conversations": conversations})
	case http.MethodPost:
		var req ConversationCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		agent, ok := c.opts.Registry.Find(strings.TrimSpace(req.AgentID))
		if !ok {
			c.writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		detail := ConversationDetail{
			Conversation: Conversation{
				ID:           "ctx-" + NewLocalID(),
				AgentID:      agent.ID,
				Title:        FirstNonEmpty(strings.TrimSpace(req.Title), agent.Name),
				Status:       "idle",
				A2AContextID: "ctx-" + NewLocalID(),
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			Messages: []ConversationMessage{},
		}
		detail.Conversation.ID = detail.Conversation.A2AContextID
		if strings.TrimSpace(req.InitialText) != "" {
			turn := ConversationTurnRequest{AgentID: agent.ID, Text: strings.TrimSpace(req.InitialText)}
			var err error
			detail, err = c.opts.TurnRunner.Run(r.Context(), detail.Conversation.ID, turn, nil)
			if err != nil {
				c.writeConversationError(w, err)
				return
			}
		}
		c.writeJSON(w, http.StatusCreated, detail)
	default:
		c.writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (c *Controller) handleConversationSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/conversations/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		c.writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	conversationID := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			c.writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		agent, selected, bearerToken, ok := c.agentForHistoryRequest(w, r)
		if !ok {
			return
		}
		detail, err := c.agentConversation(r.Context(), agent, selected, bearerToken, conversationID)
		if err != nil {
			c.writeConversationError(w, err)
			return
		}
		c.writeJSON(w, http.StatusOK, detail)
		return
	}
	if len(parts) == 2 && parts[1] == "turns" {
		if r.Method != http.MethodPost {
			c.writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		var req ConversationTurnRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		detail, err := c.opts.TurnRunner.Run(r.Context(), conversationID, req, nil)
		if err != nil {
			c.writeConversationError(w, err)
			return
		}
		c.writeJSON(w, http.StatusOK, detail)
		return
	}
	if len(parts) == 3 && parts[1] == "turns" && parts[2] == "stream" {
		if r.Method != http.MethodPost {
			c.writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		c.handleConversationTurnStream(w, r, conversationID)
		return
	}
	c.writeError(w, http.StatusNotFound, "conversation route not found")
}

func (c *Controller) handleConversationTurnStream(w http.ResponseWriter, r *http.Request, conversationID string) {
	var req ConversationTurnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		c.writeError(w, http.StatusInternalServerError, "streaming is unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	_, err := c.opts.TurnRunner.Run(r.Context(), conversationID, req, func(event Event) error {
		var sseEvent string
		var data any

		switch event.Kind {
		case EventTurnStarted:
			sseEvent = "turn_started"
			data = map[string]any{
				"conversationId": event.ConversationID,
				"agentId":        event.AgentID,
				"status":         event.Status,
			}
		case EventAssistantDelta:
			sseEvent = "assistant_delta"
			data = map[string]any{
				"conversationId": event.ConversationID,
				"agentId":        event.AgentID,
				"taskId":         event.TaskID,
				"text":           event.Text,
				"append":         event.Append,
			}
		case EventStatusChanged:
			sseEvent = "status_changed"
			data = map[string]any{
				"conversationId": event.ConversationID,
				"agentId":        event.AgentID,
				"taskId":         event.TaskID,
				"status":         event.Status,
				"terminal":       event.Terminal,
			}
		case EventTurnCompleted:
			sseEvent = "turn_completed"
			data = event.Detail
		case EventTurnFailed:
			sseEvent = "turn_failed"
			data = map[string]any{
				"conversationId": event.ConversationID,
				"agentId":        event.AgentID,
				"message":        event.Message,
			}
		}

		sendConversationSSE(w, flusher, sseEvent, data)
		return nil
	})
	if err != nil {
		if errors.Is(err, a2aclient.ErrUnsupportedProtocol) {
			sendConversationSSE(w, flusher, "turn_failed", map[string]any{
				"conversationId": conversationID,
				"agentId":        req.AgentID,
				"message":        "agent protocol binding is not implemented yet",
			})
		} else {
			sendConversationSSE(w, flusher, "turn_failed", map[string]any{
				"conversationId": conversationID,
				"agentId":        req.AgentID,
				"message":        "Agent request failed",
			})
		}
	}
}

func sendConversationSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
	flusher.Flush()
}

// Internal implementation helpers

func (c *Controller) addAgentFromCard(ctx context.Context, cardURL string) (registry.Agent, error) {
	if err := c.requireWritableYAMLConfig(); err != nil {
		return registry.Agent{}, err
	}
	cardURL = strings.TrimSpace(cardURL)
	if cardURL == "" {
		return registry.Agent{}, errors.New("cardUrl is required")
	}
	result, err := c.opts.CardService.cardFetcher.Fetch(ctx, cardURL, "")
	if err != nil {
		return registry.Agent{}, err
	}
	selected, err := a2aclient.SelectInterface(result.Card)
	if err != nil {
		return registry.Agent{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	agents := c.opts.GetAgentsConfig()
	for _, existing := range agents {
		if existing.CardURL == cardURL {
			return c.agentWithDiscovery(
				registry.New([]registry.AgentConfig{mapToRegistryAgentConfig(existing)}).List()[0],
				cardCacheFromCard(existing.ID, result, selected),
			), nil
		}
	}

	id := uniqueAgentID(agents, slug(result.Card.Name))
	agent := AgentConfig{
		ID:              id,
		Name:            result.Card.Name,
		Description:     result.Card.Description,
		CardURL:         cardURL,
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		Enabled:         true,
		Capabilities:    []string{"conversation"},
		MCPScopes:       DefaultMCPReadScopes(),
	}

	nextAgents := append(append([]AgentConfig(nil), agents...), agent)
	if err := c.opts.SaveAgentsConfig(nextAgents); err != nil {
		return registry.Agent{}, err
	}

	reg := registry.New(mapToRegistryAgentConfigs(nextAgents))
	if c.opts.OnRegistryUpdated != nil {
		c.opts.OnRegistryUpdated(reg)
	}

	cache := cardCacheFromCard(agent.ID, result, selected)
	c.opts.CardService.Save(cache)
	return c.agentWithDiscovery(
		registry.New([]registry.AgentConfig{mapToRegistryAgentConfig(agent)}).List()[0],
		cache,
	), nil
}

func (c *Controller) patchAgent(agentID string, enabled *bool) (registry.Agent, error) {
	if err := c.requireWritableYAMLConfig(); err != nil {
		return registry.Agent{}, err
	}
	if enabled == nil {
		return registry.Agent{}, errors.New("enabled is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	agents := c.opts.GetAgentsConfig()
	nextAgents := append([]AgentConfig(nil), agents...)
	for i := range nextAgents {
		if nextAgents[i].ID != agentID {
			continue
		}
		nextAgents[i].Enabled = *enabled
		if err := c.opts.SaveAgentsConfig(nextAgents); err != nil {
			return registry.Agent{}, err
		}

		reg := registry.New(mapToRegistryAgentConfigs(nextAgents))
		if c.opts.OnRegistryUpdated != nil {
			c.opts.OnRegistryUpdated(reg)
		}

		agent := registry.New([]registry.AgentConfig{mapToRegistryAgentConfig(nextAgents[i])}).List()[0]
		if cache, ok := c.opts.CardService.Load(agent.ID); ok {
			agent = c.agentWithDiscovery(agent, cache)
		}
		return agent, nil
	}
	return registry.Agent{}, errors.New("agent not found")
}

func (c *Controller) deleteAgent(agentID string) error {
	if err := c.requireWritableYAMLConfig(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	agents := c.opts.GetAgentsConfig()
	nextAgents := make([]AgentConfig, 0, len(agents))
	found := false
	for _, agent := range agents {
		if agent.ID == agentID {
			found = true
			continue
		}
		nextAgents = append(nextAgents, agent)
	}
	if !found {
		return errors.New("agent not found")
	}
	if err := c.opts.SaveAgentsConfig(nextAgents); err != nil {
		return err
	}

	reg := registry.New(mapToRegistryAgentConfigs(nextAgents))
	if c.opts.OnRegistryUpdated != nil {
		c.opts.OnRegistryUpdated(reg)
	}

	c.opts.CardService.Remove(agentID)
	return nil
}

func (c *Controller) requireWritableYAMLConfig() error {
	ext := strings.ToLower(filepath.Ext(c.opts.ConfigPath))
	if strings.TrimSpace(c.opts.ConfigPath) == "" || (ext != ".yaml" && ext != ".yml") {
		return errYAMLConfigRequired
	}
	return nil
}

func (c *Controller) agentForHistoryRequest(
	w http.ResponseWriter,
	r *http.Request,
) (registry.Agent, selectedAgentInterface, string, bool) {
	agentID := strings.TrimSpace(r.URL.Query().Get("agentId"))
	if agentID == "" {
		c.writeError(w, http.StatusBadRequest, "agentId is required")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	agent, ok := c.opts.Registry.Find(agentID)
	if !ok {
		c.writeError(w, http.StatusNotFound, "agent not found")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	if !agent.Enabled {
		c.writeError(w, http.StatusConflict, "agent is disabled")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	configuredAgent, ok := c.configuredAgent(agent.ID)
	if !ok {
		c.writeError(w, http.StatusNotFound, "agent not found")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	selected := c.selectedAgentInterface(r.Context(), agent)
	if selected.ProtocolBinding != a2aclient.ProtocolJSONRPC {
		c.writeError(w, http.StatusNotImplemented, "agent protocol binding is not implemented yet")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	bearerToken, ok := AgentBearerToken(configuredAgent)
	if !ok {
		c.writeError(w, http.StatusServiceUnavailable, "agent credentials are not available")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	return agent, selected, bearerToken, true
}

func (c *Controller) listAgentConversations(
	ctx context.Context,
	agent registry.Agent,
	selected selectedAgentInterface,
	bearerToken string,
) ([]Conversation, error) {
	history, ok := c.opts.Messages.(a2aclient.TaskHistoryClient)
	if !ok {
		return []Conversation{UnsupportedConversation(agent)}, nil
	}
	result, err := history.ListTasks(ctx, a2aclient.ListTasksRequest{
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		ProtocolVersion: selected.ProtocolVersion,
		BearerToken:     bearerToken,
		PageSize:        50,
	})
	if err != nil {
		if isUnsupportedHistory(err) {
			return []Conversation{UnsupportedConversation(agent)}, nil
		}
		return nil, err
	}
	byContext := map[string]Conversation{}
	for _, task := range result.Tasks {
		contextID := FirstNonEmpty(task.ContextID, task.ID)
		conversation := byContext[contextID]
		if conversation.ID == "" {
			conversation = Conversation{
				ID:           contextID,
				AgentID:      agent.ID,
				Title:        FirstNonEmpty(FirstUserText(task.Messages), task.Text, agent.Name),
				Status:       task.Status,
				A2AContextID: contextID,
				LatestTaskID: task.ID,
				CreatedAt:    task.UpdatedAt,
				UpdatedAt:    task.UpdatedAt,
			}
		}
		if task.UpdatedAt >= conversation.UpdatedAt {
			conversation.UpdatedAt = task.UpdatedAt
			conversation.LatestTaskID = task.ID
			conversation.Status = task.Status
		}
		byContext[contextID] = conversation
	}
	conversations := make([]Conversation, 0, len(byContext))
	for _, conversation := range byContext {
		conversations = append(conversations, conversation)
	}
	return conversations, nil
}

func (c *Controller) agentConversation(
	ctx context.Context,
	agent registry.Agent,
	selected selectedAgentInterface,
	bearerToken, conversationID string,
) (ConversationDetail, error) {
	history, ok := c.opts.Messages.(a2aclient.TaskHistoryClient)
	if !ok {
		return ConversationDetail{Conversation: UnsupportedConversation(agent)}, nil
	}
	result, err := history.ListTasks(ctx, a2aclient.ListTasksRequest{
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		ProtocolVersion: selected.ProtocolVersion,
		BearerToken:     bearerToken,
		ContextID:       conversationID,
		PageSize:        50,
	})
	if err != nil {
		if isUnsupportedHistory(err) {
			return ConversationDetail{Conversation: UnsupportedConversation(agent)}, nil
		}
		return ConversationDetail{}, err
	}
	detail := ConversationDetail{
		Conversation: Conversation{
			ID:           conversationID,
			AgentID:      agent.ID,
			Title:        agent.Name,
			Status:       "idle",
			A2AContextID: conversationID,
			CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		},
		Messages: []ConversationMessage{},
	}
	for _, task := range result.Tasks {
		record := task
		if task.ID != "" {
			if loaded, err := history.GetTask(ctx, a2aclient.GetTaskRequest{
				EndpointURL:     selected.EndpointURL,
				ProtocolBinding: selected.ProtocolBinding,
				ProtocolVersion: selected.ProtocolVersion,
				BearerToken:     bearerToken,
				TaskID:          task.ID,
				HistoryLength:   50,
			}); err == nil {
				record = loaded
			}
		}
		detail.Conversation.LatestTaskID = record.ID
		detail.Conversation.Status = record.Status
		detail.Conversation.UpdatedAt = record.UpdatedAt
		if title := FirstUserText(record.Messages); title != "" && detail.Conversation.Title == agent.Name {
			detail.Conversation.Title = title
		}
		detail.Messages = append(detail.Messages, ConversationMessages(agent.ID, conversationID, record)...)
	}
	return detail, nil
}

func (c *Controller) configuredAgent(agentID string) (AgentConfig, bool) {
	agents := c.opts.GetAgentsConfig()
	for _, a := range agents {
		if a.ID == agentID {
			return a, true
		}
	}
	return AgentConfig{}, false
}

func (c *Controller) selectedAgentInterface(ctx context.Context, agent registry.Agent) selectedAgentInterface {
	selected := selectedAgentInterface{
		EndpointURL:     agent.EndpointURL,
		ProtocolBinding: agent.ProtocolBinding,
		ProtocolVersion: a2aclient.ProtocolVersion10,
	}
	cache, ok := c.currentAgentCardCache(ctx, agent)
	if ok && cache.SelectedEndpointURL != "" {
		selected.EndpointURL = cache.SelectedEndpointURL
		selected.ProtocolBinding = cache.SelectedProtocolBinding
		selected.ProtocolVersion = cache.SelectedProtocolVersion
		selected.Streaming = cache.Streaming
		if cache.DashboardContextSupported {
			selected.Extensions = []string{a2aclient.DashboardContextExtensionURI}
			selected.Metadata = map[string]any{
				a2aclient.DashboardContextExtensionURI: c.opts.GetDashboardContext(ctx),
			}
		}
	}
	return selected
}

func (c *Controller) agentsWithDiscovery(ctx context.Context, refreshMissing bool) []registry.Agent {
	c.mu.Lock()
	agents := c.opts.Registry.List()
	c.mu.Unlock()
	for i := range agents {
		if configured, ok := c.configuredAgent(agents[i].ID); ok {
			agents[i].AuthConfigured = configured.Auth != nil
			agents[i].AuthAvailable = agentAuthAvailable(configured)
		}
		var cache AgentCardCacheEntry
		var ok bool
		if refreshMissing {
			cache, ok = c.currentAgentCardCache(ctx, agents[i])
		} else {
			cache, ok = c.loadAgentCardCache(ctx, agents[i].ID)
		}
		if ok {
			agents[i] = c.agentWithDiscovery(agents[i], cache)
		}
	}
	return agents
}

func (c *Controller) AgentStatusSummary(ctx context.Context) AgentStatusSummary {
	agents := c.agentsWithDiscovery(ctx, false)
	summary := AgentStatusSummary{Total: len(agents)}
	for _, agent := range agents {
		if agent.Enabled {
			summary.Enabled++
		} else {
			summary.Disabled++
		}
		if agent.Enabled && agent.CardStatus == "available" && agentAuthAvailableFromPublic(agent) {
			summary.Available++
		}
		if agent.Enabled && agent.CardStatus != "" && agent.CardStatus != "available" {
			summary.Unavailable++
		}
		if agent.DashboardContextSupported {
			summary.DashboardContextSupported++
		}
		if len(agent.MCPScopes) > 0 {
			summary.MCPScoped++
		}
	}
	return summary
}

func (c *Controller) agentWithDiscovery(agent registry.Agent, cache AgentCardCacheEntry) registry.Agent {
	agent.CardStatus = cache.CardStatus
	agent.CardFetchedAt = cache.FetchedAt
	agent.CardError = cache.CardError
	agent.SelectedEndpointURL = cache.SelectedEndpointURL
	agent.SelectedProtocolBinding = cache.SelectedProtocolBinding
	agent.SelectedProtocolVersion = cache.SelectedProtocolVersion
	agent.Skills = append([]a2aclient.AgentSkill(nil), cache.Skills...)
	agent.Streaming = cache.Streaming
	agent.DashboardContextSupported = cache.DashboardContextSupported
	if agent.SelectedEndpointURL != "" {
		agent.EndpointURL = agent.SelectedEndpointURL
	}
	if agent.SelectedProtocolBinding != "" {
		agent.ProtocolBinding = agent.SelectedProtocolBinding
	}
	return agent
}

func (c *Controller) currentAgentCardCache(ctx context.Context, agent registry.Agent) (AgentCardCacheEntry, bool) {
	configured, _ := c.configuredAgent(agent.ID)
	return c.opts.CardService.Current(ctx, agent, configured), true
}

func (c *Controller) loadAgentCardCache(_ context.Context, agentID string) (AgentCardCacheEntry, bool) {
	return c.opts.CardService.Load(agentID)
}

func (c *Controller) refreshAgentCard(ctx context.Context, agent registry.Agent) AgentCardCacheEntry {
	configured, _ := c.configuredAgent(agent.ID)
	return c.opts.CardService.Refresh(ctx, agent, configured)
}

// Helpers

func agentAuthAvailable(agent AgentConfig) bool {
	if agent.Auth == nil {
		return true
	}
	return strings.TrimSpace(osGetenv(agent.Auth.EnvToken)) != ""
}

func agentAuthAvailableFromPublic(agent registry.Agent) bool {
	return !agent.AuthConfigured || agent.AuthAvailable
}

func isUnsupportedHistory(err error) bool {
	var rpcErr *a2aclient.RPCError
	return errors.As(err, &rpcErr)
}

func (c *Controller) writeAgentConfigError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errYAMLConfigRequired):
		c.writeError(w, http.StatusConflict, "YAML config file is required to add agents")
	case errors.Is(err, a2aclient.ErrAgentCardUnavailable):
		c.writeError(w, http.StatusBadGateway, "agent card could not be fetched")
	case errors.Is(err, a2aclient.ErrNoSupportedInterface):
		c.writeError(w, http.StatusBadRequest, "agent card has no compatible A2A 1.0 JSON-RPC interface")
	case strings.Contains(err.Error(), "required"):
		c.writeError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "not found"):
		c.writeError(w, http.StatusNotFound, err.Error())
	default:
		c.writeError(w, http.StatusInternalServerError, "agent configuration could not be updated")
	}
}

func (c *Controller) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (c *Controller) writeError(w http.ResponseWriter, status int, message string) {
	c.writeJSON(w, status, map[string]string{"error": message})
}

func (c *Controller) writeMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	c.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func mapToRegistryAgentConfig(agent AgentConfig) registry.AgentConfig {
	return registry.AgentConfig{
		ID:              agent.ID,
		Name:            agent.Name,
		Description:     agent.Description,
		CardURL:         agent.CardURL,
		EndpointURL:     agent.EndpointURL,
		ProtocolBinding: agent.ProtocolBinding,
		Enabled:         agent.Enabled,
	}
}

func mapToRegistryAgentConfigs(agents []AgentConfig) []registry.AgentConfig {
	out := make([]registry.AgentConfig, len(agents))
	for i, a := range agents {
		out[i] = mapToRegistryAgentConfig(a)
	}
	return out
}

func uniqueAgentID(agents []AgentConfig, base string) string {
	if base == "" {
		base = "agent"
	}
	used := map[string]struct{}{}
	for _, agent := range agents {
		used[agent.ID] = struct{}{}
	}
	if _, ok := used[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = slugPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "agent"
	}
	return value
}

func cardCacheFromCard(
	agentID string,
	result a2aclient.AgentCardFetchResult,
	selected a2aclient.SelectedInterface,
) AgentCardCacheEntry {
	fetchedAt := result.FetchedAt
	if fetchedAt.IsZero() {
		fetchedAt = time.Now().UTC()
	}
	return AgentCardCacheEntry{
		AgentID:                   agentID,
		CardJSON:                  result.Raw,
		CardStatus:                "available",
		SelectedEndpointURL:       selected.EndpointURL,
		SelectedProtocolBinding:   selected.ProtocolBinding,
		SelectedProtocolVersion:   selected.ProtocolVersion,
		Streaming:                 result.Card.Capabilities.Streaming,
		DashboardContextSupported: a2aclient.SupportsDashboardContext(result.Card),
		Skills:                    result.Card.Skills,
		FetchedAt:                 fetchedAt.Format(time.RFC3339Nano),
		ExpiresAt:                 fetchedAt.Add(10 * time.Minute).Format(time.RFC3339Nano),
	}
}

func (c *Controller) writeConversationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, a2aclient.ErrUnsupportedProtocol):
		c.writeError(w, http.StatusNotImplemented, "agent protocol binding is not implemented yet")
	case errors.Is(err, errAgentHistoryUnsupported):
		c.writeError(w, http.StatusNotImplemented, "agent history is unavailable")
	case strings.Contains(err.Error(), "disabled"):
		c.writeError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "required"):
		c.writeError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "not found"):
		c.writeError(w, http.StatusNotFound, err.Error())
	case strings.Contains(err.Error(), "credentials"):
		c.writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		c.writeError(w, http.StatusBadGateway, "agent request failed")
	}
}
