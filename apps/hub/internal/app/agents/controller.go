package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/registry"
)

var (
	errAgentHistoryUnsupported = errors.New("agent history is unavailable")
)

type ControllerOptions struct {
	Manager             *AgentManager
	Messages            a2aclient.MessageSender
	TurnRunner          *Runner
	GetDashboardContext func(ctx context.Context) map[string]any
}

type Controller struct {
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
			"agents": c.opts.Manager.List(r.Context(), true),
		})
	case http.MethodPost:
		var req struct {
			CardURL string `json:"cardUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		agent, err := c.opts.Manager.Add(r.Context(), req.CardURL)
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
			agent, err := c.opts.Manager.Patch(agentID, req.Enabled)
			if err != nil {
				c.writeAgentConfigError(w, err)
				return
			}
			c.writeJSON(w, http.StatusOK, agent)
		case http.MethodDelete:
			if err := c.opts.Manager.Delete(agentID); err != nil {
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
	enriched, err := c.opts.Manager.RefreshCard(r.Context(), agentID)
	if err != nil {
		c.writeAgentConfigError(w, err)
		return
	}
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

	agent, ok := c.opts.Manager.Find(req.AgentID)
	if !ok {
		c.writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if !agent.Enabled {
		c.writeError(w, http.StatusConflict, "agent is disabled")
		return
	}

	configuredAgent, ok := c.opts.Manager.ConfiguredAgent(req.AgentID)
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
		agent, ok := c.opts.Manager.Find(strings.TrimSpace(req.AgentID))
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
		var req ConversationTurnRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		c.handleConversationTurnStream(w, r, conversationID, req)
		return
	}
	c.writeError(w, http.StatusNotFound, "conversation route not found")
}

func (c *Controller) handleConversationTurnStream(
	w http.ResponseWriter,
	r *http.Request,
	conversationID string,
	req ConversationTurnRequest,
) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		c.writeError(w, http.StatusInternalServerError, "streaming is unsupported by server")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := make(chan Event, 100)
	done := make(chan struct{})

	go func() {
		defer close(done)
		_, err := c.opts.TurnRunner.Run(r.Context(), conversationID, req, func(ev Event) error {
			select {
			case <-r.Context().Done():
				return r.Context().Err()
			case ch <- ev:
				return nil
			}
		})
		if err != nil {
			select {
			case <-r.Context().Done():
			case ch <- Event{
				Kind:           EventTurnFailed,
				ConversationID: conversationID,
				AgentID:        req.AgentID,
				Message:        "Agent request failed",
			}:
			}
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-done:
			// Drain remaining events
			for len(ch) > 0 {
				event := <-ch
				c.writeSSE(w, flusher, event)
			}
			return
		case event := <-ch:
			c.writeSSE(w, flusher, event)
		}
	}
}

func (c *Controller) writeSSE(w http.ResponseWriter, flusher http.Flusher, event Event) {
	bytes, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\n", event.Kind)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
	flusher.Flush()
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
	agent, ok := c.opts.Manager.Find(agentID)
	if !ok {
		c.writeError(w, http.StatusNotFound, "agent not found")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	if !agent.Enabled {
		c.writeError(w, http.StatusConflict, "agent is disabled")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	configuredAgent, ok := c.opts.Manager.ConfiguredAgent(agentID)
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

func (c *Controller) selectedAgentInterface(ctx context.Context, agent registry.Agent) selectedAgentInterface {
	selected := selectedAgentInterface{
		EndpointURL:     agent.EndpointURL,
		ProtocolBinding: agent.ProtocolBinding,
		ProtocolVersion: a2aclient.ProtocolVersion10,
	}
	configured, _ := c.opts.Manager.ConfiguredAgent(agent.ID)
	cache := c.opts.Manager.cards.Current(ctx, agent, configured)
	if cache.CardStatus == "available" && cache.SelectedEndpointURL != "" {
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

func (c *Controller) AgentStatusSummary(ctx context.Context) AgentStatusSummary {
	return c.opts.Manager.StatusSummary(ctx)
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
		c.writeError(w, http.StatusInternalServerError, "agent request failed")
	}
}
