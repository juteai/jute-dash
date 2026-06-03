package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"jute-dash/apps/hub/internal/pkg/displayactions"
	"jute-dash/apps/hub/internal/pkg/registry"
	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	_ "jute-dash/widgets/chathistory"
	_ "jute-dash/widgets/datetime"
	_ "jute-dash/widgets/markets"
	_ "jute-dash/widgets/rss"
	_ "jute-dash/widgets/weather"
	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
)





const cardCacheTTL = 10 * time.Minute

// agentCardService owns the in-memory agent card cache and A2A card fetching
// logic. It is the single place where agent card state lives; no other part of
// the Server holds or mutates card cache entries directly.
//
// Callers pass in the config and token they need — the service has no opinion
// about where those come from.
type agentCardService struct {
	mu          sync.Mutex
	cards       map[string]agentCardCache
	cardFetcher *a2aclient.AgentCardFetcher
}

func newAgentCardService() *agentCardService {
	return &agentCardService{
		cards:       map[string]agentCardCache{},
		cardFetcher: a2aclient.NewAgentCardFetcher(),
	}
}

// load returns the cached card for agentID if it exists.
func (svc *agentCardService) load(agentID string) (agentCardCache, bool) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	c, ok := svc.cards[agentID]
	return c, ok
}

// remove deletes a card cache entry for the given agent ID.
func (svc *agentCardService) remove(agentID string) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	delete(svc.cards, agentID)
}

// save writes a card cache entry.
func (svc *agentCardService) save(c agentCardCache) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.cards[c.AgentID] = c
}

// current returns a valid cache entry, refreshing it first if necessary.
func (svc *agentCardService) current(
	ctx context.Context,
	agent registry.Agent,
	configuredAgent AgentConfig,
) agentCardCache {
	if c, ok := svc.load(agent.ID); ok && c.CardStatus == "available" {
		return c
	}
	return svc.refresh(ctx, agent, configuredAgent)
}

// refresh fetches the agent card from the network, selects an interface, and
// stores the result. It always returns a cache entry — on failure the entry
// has CardStatus "unavailable".
func (svc *agentCardService) refresh(
	ctx context.Context,
	agent registry.Agent,
	configuredAgent AgentConfig,
) agentCardCache {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	c := agentCardCache{
		AgentID:                 agent.ID,
		CardStatus:              "unavailable",
		CardError:               "agent card is unavailable",
		SelectedEndpointURL:     agent.EndpointURL,
		SelectedProtocolBinding: agent.ProtocolBinding,
		SelectedProtocolVersion: a2aclient.ProtocolVersion10,
		FetchedAt:               now,
		ExpiresAt:               now,
	}
	bearerToken, _ := agentBearerToken(configuredAgent)
	result, err := svc.cardFetcher.Fetch(ctx, agent.CardURL, bearerToken)
	if err != nil {
		c.CardError = "agent card could not be fetched"
		svc.save(c)
		return c
	}
	selected, err := a2aclient.SelectInterface(result.Card)
	if err != nil {
		c.CardJSON = result.Raw
		c.CardError = "agent card has no compatible A2A 1.0 interface"
		c.FetchedAt = result.FetchedAt.Format(time.RFC3339Nano)
		c.ExpiresAt = result.FetchedAt.Add(cardCacheTTL).Format(time.RFC3339Nano)
		c.Skills = result.Card.Skills
		c.Streaming = result.Card.Capabilities.Streaming
		c.DashboardContextSupported = a2aclient.SupportsDashboardContext(result.Card)
		svc.save(c)
		return c
	}
	c.CardJSON = result.Raw
	c.CardStatus = "available"
	c.CardError = ""
	c.SelectedEndpointURL = selected.EndpointURL
	c.SelectedProtocolBinding = selected.ProtocolBinding
	c.SelectedProtocolVersion = selected.ProtocolVersion
	c.Streaming = result.Card.Capabilities.Streaming
	c.DashboardContextSupported = a2aclient.SupportsDashboardContext(result.Card)
	c.Skills = result.Card.Skills
	c.FetchedAt = result.FetchedAt.Format(time.RFC3339Nano)
	c.ExpiresAt = result.FetchedAt.Add(cardCacheTTL).Format(time.RFC3339Nano)
	svc.save(c)
	return c
}





var errYAMLConfigRequired = errors.New("YAML config file is required")

func (s *Server) addAgentFromCard(ctx context.Context, cardURL string) (registry.Agent, error) {
	if err := s.requireWritableYAMLConfig(); err != nil {
		return registry.Agent{}, err
	}
	cardURL = strings.TrimSpace(cardURL)
	if cardURL == "" {
		return registry.Agent{}, errors.New("cardUrl is required")
	}
	result, err := s.agentCards.cardFetcher.Fetch(ctx, cardURL, "")
	if err != nil {
		return registry.Agent{}, err
	}
	selected, err := a2aclient.SelectInterface(result.Card)
	if err != nil {
		return registry.Agent{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.cfg.Agents {
		if existing.CardURL == cardURL {
			return s.agentWithDiscovery(
				registry.New([]registry.AgentConfig{mapToRegistryAgentConfig(existing)}).List()[0],
				cardCacheFromCard(existing.ID, result, selected),
			), nil
		}
	}

	id := uniqueAgentID(s.cfg.Agents, slug(result.Card.Name))
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
	next := s.cfg
	next.Agents = append(append([]AgentConfig(nil), s.cfg.Agents...), agent)
	if err := SaveYAML(s.configPath, next); err != nil {
		return registry.Agent{}, err
	}
	s.cfg = next
	s.registry = registry.New(mapToRegistryAgentConfigs(s.cfg.Agents))
	cache := cardCacheFromCard(agent.ID, result, selected)
	s.agentCards.save(cache)
	return s.agentWithDiscovery(registry.New([]registry.AgentConfig{mapToRegistryAgentConfig(agent)}).List()[0], cache), nil
}

func (s *Server) patchAgent(agentID string, enabled *bool) (registry.Agent, error) {
	if err := s.requireWritableYAMLConfig(); err != nil {
		return registry.Agent{}, err
	}
	if enabled == nil {
		return registry.Agent{}, errors.New("enabled is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	next.Agents = append([]AgentConfig(nil), s.cfg.Agents...)
	for i := range next.Agents {
		if next.Agents[i].ID != agentID {
			continue
		}
		next.Agents[i].Enabled = *enabled
		if err := SaveYAML(s.configPath, next); err != nil {
			return registry.Agent{}, err
		}
		s.cfg = next
		s.registry = registry.New(mapToRegistryAgentConfigs(s.cfg.Agents))
		agent := registry.New([]registry.AgentConfig{mapToRegistryAgentConfig(next.Agents[i])}).List()[0]
		if cache, ok := s.agentCards.load(agent.ID); ok {
			agent = s.agentWithDiscovery(agent, cache)
		}
		return agent, nil
	}
	return registry.Agent{}, errors.New("agent not found")
}

func (s *Server) deleteAgent(agentID string) error {
	if err := s.requireWritableYAMLConfig(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	next.Agents = make([]AgentConfig, 0, len(s.cfg.Agents))
	found := false
	for _, agent := range s.cfg.Agents {
		if agent.ID == agentID {
			found = true
			continue
		}
		next.Agents = append(next.Agents, agent)
	}
	if !found {
		return errors.New("agent not found")
	}
	if err := SaveYAML(s.configPath, next); err != nil {
		return err
	}
	s.cfg = next
	s.registry = registry.New(mapToRegistryAgentConfigs(s.cfg.Agents))
	s.agentCards.remove(agentID)
	return nil
}

func (s *Server) requireWritableYAMLConfig() error {
	ext := strings.ToLower(filepath.Ext(s.configPath))
	if strings.TrimSpace(s.configPath) == "" || (ext != ".yaml" && ext != ".yml") {
		return errYAMLConfigRequired
	}
	return nil
}

func writeAgentConfigError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errYAMLConfigRequired):
		writeError(w, http.StatusConflict, "YAML config file is required to add agents")
	case errors.Is(err, a2aclient.ErrAgentCardUnavailable):
		writeError(w, http.StatusBadGateway, "agent card could not be fetched")
	case errors.Is(err, a2aclient.ErrNoSupportedInterface):
		writeError(w, http.StatusBadRequest, "agent card has no compatible A2A 1.0 JSON-RPC interface")
	case strings.Contains(err.Error(), "required"):
		writeError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "not found"):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "agent configuration could not be updated")
	}
}

func cardCacheFromCard(
	agentID string,
	result a2aclient.AgentCardFetchResult,
	selected a2aclient.SelectedInterface,
) agentCardCache {
	fetchedAt := result.FetchedAt
	if fetchedAt.IsZero() {
		fetchedAt = time.Now().UTC()
	}
	return agentCardCache{
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





var errAgentHistoryUnsupported = errors.New("agent history is unavailable")

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		agent, selected, bearerToken, ok := s.agentForHistoryRequest(w, r)
		if !ok {
			return
		}
		conversations, err := s.listAgentConversations(r.Context(), agent, selected, bearerToken)
		if err != nil {
			writeConversationError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"conversations": conversations})
	case http.MethodPost:
		var req ConversationCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		agent, ok := s.registry.Find(strings.TrimSpace(req.AgentID))
		if !ok {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		detail := ConversationDetail{
			Conversation: Conversation{
				ID:           "ctx-" + NewLocalID(),
				AgentID:      agent.ID,
				Title:        firstNonEmpty(strings.TrimSpace(req.Title), agent.Name),
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
			detail, err = s.sendConversationTurn(r.Context(), detail.Conversation.ID, turn)
			if err != nil {
				writeConversationError(w, err)
				return
			}
		}
		writeJSON(w, http.StatusCreated, detail)
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (s *Server) handleConversationSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/conversations/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	conversationID := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		agent, selected, bearerToken, ok := s.agentForHistoryRequest(w, r)
		if !ok {
			return
		}
		detail, err := s.agentConversation(r.Context(), agent, selected, bearerToken, conversationID)
		if err != nil {
			writeConversationError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
		return
	}
	if len(parts) == 2 && parts[1] == "turns" {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		var req ConversationTurnRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		detail, err := s.sendConversationTurn(r.Context(), conversationID, req)
		if err != nil {
			writeConversationError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
		return
	}
	if len(parts) == 3 && parts[1] == "turns" && parts[2] == "stream" {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		s.handleConversationTurnStream(w, r, conversationID)
		return
	}
	writeError(w, http.StatusNotFound, "conversation route not found")
}

func (s *Server) handleConversationTurnStream(w http.ResponseWriter, r *http.Request, conversationID string) {
	var req ConversationTurnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	_, err := s.turnRunner.Run(r.Context(), conversationID, req, func(event Event) error {
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

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "event stream is unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sendDisplaySSE(w, flusher, "hub.connected", map[string]any{
		"connectedAt": time.Now().UTC().Format(time.RFC3339Nano),
	})

	events := s.display.Subscribe(r.Context())
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			sendDisplayEventSSE(w, flusher, event)
		case <-heartbeat.C:
			_, _ = w.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		}
	}
}

func sendDisplayEventSSE(w http.ResponseWriter, flusher http.Flusher, event displayactions.Event) {
	sendDisplaySSE(w, flusher, event.Type, event.Data)
}

func sendDisplaySSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
	flusher.Flush()
}

func (s *Server) agentForHistoryRequest(
	w http.ResponseWriter,
	r *http.Request,
) (registry.Agent, selectedAgentInterface, string, bool) {
	agentID := strings.TrimSpace(r.URL.Query().Get("agentId"))
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agentId is required")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	agent, ok := s.registry.Find(agentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	if !agent.Enabled {
		writeError(w, http.StatusConflict, "agent is disabled")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	configuredAgent, ok := s.configuredAgent(agent.ID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	selected := s.selectedAgentInterface(r.Context(), agent)
	if selected.ProtocolBinding != a2aclient.ProtocolJSONRPC {
		writeError(w, http.StatusNotImplemented, "agent protocol binding is not implemented yet")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	bearerToken, ok := agentBearerToken(configuredAgent)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "agent credentials are not available")
		return registry.Agent{}, selectedAgentInterface{}, "", false
	}
	return agent, selected, bearerToken, true
}

func (s *Server) listAgentConversations(
	ctx context.Context,
	agent registry.Agent,
	selected selectedAgentInterface,
	bearerToken string,
) ([]Conversation, error) {
	history, ok := s.messages.(a2aclient.TaskHistoryClient)
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

func (s *Server) agentConversation(
	ctx context.Context,
	agent registry.Agent,
	selected selectedAgentInterface,
	bearerToken, conversationID string,
) (ConversationDetail, error) {
	history, ok := s.messages.(a2aclient.TaskHistoryClient)
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

func (s *Server) sendConversationTurn(
	ctx context.Context,
	conversationID string,
	req ConversationTurnRequest,
) (ConversationDetail, error) {
	return s.turnRunner.Run(ctx, conversationID, req, nil)
}

func isUnsupportedHistory(err error) bool {
	var rpcErr *a2aclient.RPCError
	return errors.As(err, &rpcErr)
}

func writeConversationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, a2aclient.ErrUnsupportedProtocol):
		writeError(w, http.StatusNotImplemented, "agent protocol binding is not implemented yet")
	case errors.Is(err, errAgentHistoryUnsupported):
		writeError(w, http.StatusNotImplemented, "agent history is unavailable")
	case strings.Contains(err.Error(), "disabled"):
		writeError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "required"):
		writeError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "not found"):
		writeError(w, http.StatusNotFound, err.Error())
	case strings.Contains(err.Error(), "credentials"):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		writeError(w, http.StatusBadGateway, "agent request failed")
	}
}






type Server struct {
	cfg         Config
	registry    registry.Registry
	messages    a2aclient.MessageSender
	agentCards  *agentCardService
	setup       SetupStatus
	layout      WidgetLayout
	layoutStore WidgetLayoutStore
	settings    HouseholdSettingsStore
	voice       VoiceConfig
	voiceStore  VoiceSettingsStore
	configPath  string
	display     *displayactions.Dispatcher
	turnRunner  *Runner
	mu          sync.Mutex
	started     time.Time
	version     string
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"startedAt"`
}

type StatusResponse struct {
	Status      string             `json:"status"`
	Version     string             `json:"version"`
	StartedAt   time.Time          `json:"startedAt"`
	Setup       SetupStatus  `json:"setup"`
	Config      ConfigStatus       `json:"config"`
	EventStream EventStreamStatus  `json:"eventStream"`
	MCP         MCPStatus          `json:"mcp"`
	Agents      AgentStatusSummary `json:"agents"`
	Voice       VoiceStatusSummary `json:"voice"`
}

type ConfigStatus struct {
	HasBootstrapConfig bool `json:"hasBootstrapConfig"`
	WritableYAML       bool `json:"writableYaml"`
}

type EventStreamStatus struct {
	Available bool `json:"available"`
}

type MCPStatus struct {
	Enabled       bool   `json:"enabled"`
	ServiceStatus string `json:"serviceStatus"`
	Transport     string `json:"transport"`
	ListenAddress string `json:"listenAddress"`
	Path          string `json:"path"`
	AuthMode      string `json:"authMode"`
	AllowLAN      bool   `json:"allowLAN"`
}

type AgentStatusSummary struct {
	Total                     int `json:"total"`
	Enabled                   int `json:"enabled"`
	Disabled                  int `json:"disabled"`
	Available                 int `json:"available"`
	Unavailable               int `json:"unavailable"`
	DashboardContextSupported int `json:"dashboardContextSupported"`
	MCPScoped                 int `json:"mcpScoped"`
}

type VoiceStatusSummary struct {
	Enabled       bool   `json:"enabled"`
	ServiceStatus string `json:"serviceStatus"`
	State         string `json:"state"`
}

var errInvalidHouseholdSettings = errors.New("invalid household settings")

type MessageRequest struct {
	AgentID        string `json:"agentId"`
	Text           string `json:"text"`
	ConversationID string `json:"conversationId,omitempty"`
}

type MessageResponse struct {
	ConversationID string `json:"conversationId"`
	TaskID         string `json:"taskId,omitempty"`
	AgentID        string `json:"agentId"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

type VoiceStatusResponse struct {
	Enabled                 bool   `json:"enabled"`
	Muted                   bool   `json:"muted"`
	State                   string `json:"state"`
	ServiceStatus           string `json:"serviceStatus"`
	DeviceProfileID         string `json:"deviceProfileId"`
	WakeWordModelID         string `json:"wakeWordModelId"`
	STTProviderID           string `json:"sttProviderId"`
	TTSProviderID           string `json:"ttsProviderId"`
	STTModelID              string `json:"sttModelId"`
	TTSModelID              string `json:"ttsModelId"`
	TTSVoiceID              string `json:"ttsVoiceId"`
	PreferredAgentID        string `json:"preferredAgentId"`
	CloudOptIn              bool   `json:"cloudOptIn"`
	CommandProvidersEnabled bool   `json:"commandProvidersEnabled"`
	FollowupWindowSeconds   int    `json:"followupWindowSeconds"`
	MicrophoneProfile       string `json:"microphoneProfile"`
	UpdatedAt               string `json:"updatedAt"`
}

type AgentStatusResponse struct {
	Agent registry.Agent `json:"agent"`
}

func New(cfg Config, version string) http.Handler {
	layout := DefaultWidgetLayout()
	return newServer(cfg, version, nil, SetupStatus{Complete: true}, layout, nil, "", nil)
}

func NewWithSetupStatus(
	cfg Config,
	version string,
	setup SetupStatus,
) http.Handler {
	return NewWithSetupStatusAndLayout(cfg, version, setup, DefaultWidgetLayout())
}

func NewWithSetupStatusAndLayout(
	cfg Config,
	version string,
	setup SetupStatus,
	layout WidgetLayout,
) http.Handler {
	return newServer(cfg, version, nil, setup, layout, nil, "", nil)
}

func NewWithMessageSender(
	cfg Config,
	version string,
	messageSender a2aclient.MessageSender,
) http.Handler {
	return newServer(
		cfg,
		version,
		messageSender,
		SetupStatus{Complete: true},
		DefaultWidgetLayout(),
		nil,
		"",
		nil,
	)
}

func NewWithSetupStatusAndLayoutStore(
	cfg Config,
	version string,
	setup SetupStatus,
	layoutStore WidgetLayoutStore,
) http.Handler {
	return NewWithSetupStatusAndLayoutStoreAndConfigPath(cfg, version, setup, layoutStore, "")
}

func NewWithSetupStatusAndLayoutStoreAndConfigPath(
	cfg Config,
	version string,
	setup SetupStatus,
	layoutStore WidgetLayoutStore,
	configPath string,
) http.Handler {
	return NewWithSetupStatusAndLayoutStoreAndConfigPathAndDisplayActions(
		cfg,
		version,
		setup,
		layoutStore,
		configPath,
		nil,
	)
}

func NewWithSetupStatusAndLayoutStoreAndConfigPathAndDisplayActions(
	cfg Config,
	version string,
	setup SetupStatus,
	layoutStore WidgetLayoutStore,
	configPath string,
	display *displayactions.Dispatcher,
) http.Handler {
	layout := DefaultWidgetLayout()
	if configPath != "" {
		yamlStore := NewYAMLSettingsStore(configPath)
		if loaded, err := yamlStore.WidgetLayout(context.Background(), ""); err == nil {
			layout = loaded
		}
	} else if layoutStore != nil {
		if loaded, err := layoutStore.WidgetLayout(context.Background(), ""); err == nil {
			layout = loaded
		}
	}
	return newServer(cfg, version, nil, setup, layout, layoutStore, configPath, display)
}

func newServer(
	cfg Config,
	version string,
	messageSender a2aclient.MessageSender,
	setup SetupStatus,
	layout WidgetLayout,
	layoutStore WidgetLayoutStore,
	configPath string,
	display *displayactions.Dispatcher,
) http.Handler {
	if messageSender == nil {
		messageSender = a2aclient.NewJSONRPCClient()
	}

	var activeLayoutStore WidgetLayoutStore
	var activeVoiceStore VoiceSettingsStore
	var activeSettingsStore HouseholdSettingsStore

	if configPath != "" {
		yamlStore := NewYAMLSettingsStore(configPath)
		activeLayoutStore = yamlStore
		activeVoiceStore = yamlStore
		activeSettingsStore = yamlStore
	} else if layoutStore != nil {
		activeLayoutStore = layoutStore
		if candidate, ok := layoutStore.(VoiceSettingsStore); ok {
			activeVoiceStore = candidate
		}
		if candidate, ok := layoutStore.(HouseholdSettingsStore); ok {
			activeSettingsStore = candidate
		}
	}

	// Fallback to memory if still nil
	if activeLayoutStore == nil {
		memStore := NewMemorySettingsStoreWithConfig(cfg, layout)
		activeLayoutStore = memStore
		activeVoiceStore = memStore
		activeSettingsStore = memStore
	}
	if activeVoiceStore == nil {
		activeVoiceStore = NewMemorySettingsStoreWithConfig(cfg, layout)
	}
	if activeSettingsStore == nil {
		activeSettingsStore = NewMemorySettingsStoreWithConfig(cfg, layout)
	}

	if display == nil {
		display = displayactions.NewDispatcher()
	}
	server := &Server{
		cfg:         cfg,
		registry:    registry.New(mapToRegistryAgentConfigs(cfg.Agents)),
		messages:    messageSender,
		agentCards:  newAgentCardService(),
		setup:       normalizeSetupStatus(setup),
		layout:      normalizeWidgetLayout(layout),
		layoutStore: activeLayoutStore,
		settings:    activeSettingsStore,
		voice:       cfg.Voice,
		voiceStore:  activeVoiceStore,
		configPath:  configPath,
		display:     display,
		started:     time.Now().UTC(),
		version:     version,
	}

	SetEnvReader(os.Getenv)
	server.turnRunner = NewRunner(RunnerOptions{
		Registry:       server.registry,
		GetAgentConfig: server.configuredAgent,
		GetAgentCardCache: func(ctx context.Context, agent registry.Agent) (AgentCardCache, bool) {
			cache, ok := server.currentAgentCardCache(ctx, agent)
			if !ok {
				return AgentCardCache{}, false
			}
			return AgentCardCache{
				SelectedEndpointURL:       cache.SelectedEndpointURL,
				SelectedProtocolBinding:   cache.SelectedProtocolBinding,
				SelectedProtocolVersion:   cache.SelectedProtocolVersion,
				Streaming:                 cache.Streaming,
				DashboardContextSupported: cache.DashboardContextSupported,
			}, true
		},
		GetDashboardContext: func(ctx context.Context) map[string]any {
			return server.dashboardContext(ctx)
		},
		Messages: server.messages,
	})

	if st, ok := activeLayoutStore.(interface {
		SetCatalog([]WidgetCatalogItem)
	}); ok {
		st.SetCatalog(widgetCatalogItems())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/api/v1/status", server.handleStatus)
	mux.HandleFunc("/api/v1/config", server.handleConfig)
	mux.HandleFunc("/api/v1/home", server.handleHome)
	mux.HandleFunc("/api/v1/agents", server.handleAgents)
	mux.HandleFunc("/api/v1/agents/", server.handleAgentSubroutes)
	mux.HandleFunc("/api/v1/messages", server.handleMessages)
	mux.HandleFunc("/api/v1/conversations", server.handleConversations)
	mux.HandleFunc("/api/v1/conversations/", server.handleConversationSubroutes)
	mux.HandleFunc("/api/v1/events", server.handleEvents)
	mux.HandleFunc("/api/v1/setup/status", server.handleSetupStatus)
	mux.HandleFunc("/api/v1/settings/household", server.handleHouseholdSettings)
	mux.HandleFunc("/api/v1/settings/rooms", server.handleRoomSettings)
	mux.HandleFunc("/api/v1/settings/tiles", server.handleTileSettings)
	mux.HandleFunc("/api/v1/widgets/catalog", server.handleWidgetCatalog)
	mux.HandleFunc("/api/v1/widgets/layout", server.handleWidgetLayout)
	mux.HandleFunc("/api/v1/widgets/layout/reset", server.handleWidgetLayoutReset)
	mux.HandleFunc("/api/v1/voice/status", server.handleVoiceStatus)
	mux.HandleFunc("/api/v1/voice/mute", server.handleVoiceMute)
	mux.HandleFunc("/api/v1/voice/unmute", server.handleVoiceUnmute)
	mux.HandleFunc("/api/v1/voice/cancel", server.handleVoiceCancel)
	mux.HandleFunc("/api/v1/voice/providers", server.handleVoiceProviders)

	return withCommonHeaders(mux)
}

func normalizeSetupStatus(setup SetupStatus) SetupStatus {
	if setup.Missing == nil {
		setup.Missing = []string{}
	}
	return setup
}

func normalizeWidgetLayout(layout WidgetLayout) WidgetLayout {
	if strings.TrimSpace(layout.ProfileID) == "" {
		layout.ProfileID = DefaultWidgetLayout().ProfileID
	}
	if layout.Widgets == nil {
		layout.Widgets = []WidgetInstance{}
	}
	for i := range layout.Widgets {
		if layout.Widgets[i].Settings == nil {
			layout.Widgets[i].Settings = map[string]any{}
		}
	}
	return layout
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		Version:   s.version,
		StartedAt: s.started,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	voiceStatus, err := s.currentVoiceStatus(r.Context())
	if err != nil {
		voiceStatus = voiceStatusFromConfig(s.voice, time.Now().UTC())
	}
	status := StatusResponse{
		Status:      s.overallStatus(r.Context()),
		Version:     s.version,
		StartedAt:   s.started,
		Setup:       s.setup,
		Config:      s.configStatus(),
		EventStream: EventStreamStatus{Available: true},
		MCP:         mcpStatusFromConfig(s.cfg.MCP),
		Agents:      s.agentStatusSummary(r.Context()),
		Voice: VoiceStatusSummary{
			Enabled:       voiceStatus.Enabled,
			ServiceStatus: voiceStatus.ServiceStatus,
			State:         voiceStatus.State,
		},
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) overallStatus(ctx context.Context) string {
	if !s.setup.Complete {
		return "degraded"
	}
	mcpStatus := mcpStatusFromConfig(s.cfg.MCP)
	if mcpStatus.Enabled && mcpStatus.ServiceStatus != "enabled" {
		return "degraded"
	}
	summary := s.agentStatusSummary(ctx)
	if summary.Enabled > 0 && summary.Unavailable >= summary.Enabled {
		return "degraded"
	}
	return "ok"
}

func (s *Server) configStatus() ConfigStatus {
	ext := strings.ToLower(filepath.Ext(s.configPath))
	return ConfigStatus{
		HasBootstrapConfig: strings.TrimSpace(s.configPath) != "",
		WritableYAML:       ext == ".yaml" || ext == ".yml",
	}
}

func mcpStatusFromConfig(cfg MCPConfig) MCPStatus {
	status := MCPStatus{
		Enabled:       cfg.Enabled,
		ServiceStatus: "disabled",
		Transport:     cfg.Transport,
		ListenAddress: cfg.ListenAddress,
		Path:          cfg.Path,
		AuthMode:      cfg.Auth.Mode,
		AllowLAN:      cfg.AllowLAN,
	}
	if !cfg.Enabled {
		return status
	}
	if strings.TrimSpace(cfg.Transport) == "" || strings.TrimSpace(cfg.ListenAddress) == "" ||
		strings.TrimSpace(cfg.Path) == "" {
		status.ServiceStatus = "misconfigured"
		return status
	}
	if strings.TrimSpace(cfg.Auth.Mode) == "" {
		status.ServiceStatus = "misconfigured"
		return status
	}
	status.ServiceStatus = "enabled"
	return status
}

func mergeHouseholdSettings(current, next HouseholdSettings) HouseholdSettings {
	if strings.TrimSpace(next.Home.Name) == "" {
		next.Home.Name = current.Home.Name
	}
	if strings.TrimSpace(next.Home.Timezone) == "" {
		next.Home.Timezone = current.Home.Timezone
	}
	if strings.TrimSpace(next.Home.Locale) == "" {
		next.Home.Locale = current.Home.Locale
	}
	if strings.TrimSpace(next.Display.Theme) == "" {
		next.Display.Theme = current.Display.Theme
	}
	if strings.TrimSpace(next.Display.ColorMode) == "" {
		next.Display.ColorMode = current.Display.ColorMode
	}
	if strings.TrimSpace(next.Display.ThemeID) == "" {
		next.Display.ThemeID = current.Display.ThemeID
	}
	if strings.TrimSpace(next.Display.Density) == "" {
		next.Display.Density = current.Display.Density
	}
	if strings.TrimSpace(next.Display.Motion) == "" {
		next.Display.Motion = current.Display.Motion
	}
	if strings.TrimSpace(next.Display.Background.Kind) == "" {
		next.Display.Background = current.Display.Background
	}
	if strings.TrimSpace(next.Display.WidgetChrome.Default) == "" {
		next.Display.WidgetChrome = current.Display.WidgetChrome
	}
	if strings.TrimSpace(next.Display.AccentColor) == "" {
		next.Display.AccentColor = current.Display.AccentColor
	}
	if strings.TrimSpace(next.Display.IdleMode) == "" {
		next.Display.IdleMode = current.Display.IdleMode
	}
	if strings.TrimSpace(next.Weather.Provider) == "" {
		next.Weather.Provider = current.Weather.Provider
	}
	if strings.TrimSpace(next.Weather.LocationName) == "" {
		next.Weather.LocationName = current.Weather.LocationName
	}
	if strings.TrimSpace(next.Weather.TemperatureUnit) == "" {
		next.Weather.TemperatureUnit = current.Weather.TemperatureUnit
	}
	if strings.TrimSpace(next.Weather.WindSpeedUnit) == "" {
		next.Weather.WindSpeedUnit = current.Weather.WindSpeedUnit
	}
	next.Setup = current.Setup
	return next
}

func validateHouseholdSettings(settings HouseholdSettings) error {
	if strings.TrimSpace(settings.Home.Name) == "" {
		return fmt.Errorf("%w: home.name is required", errInvalidHouseholdSettings)
	}
	if _, err := time.LoadLocation(settings.Home.Timezone); err != nil {
		return fmt.Errorf("%w: home.timezone is invalid", errInvalidHouseholdSettings)
	}
	if strings.TrimSpace(settings.Home.Locale) == "" {
		return fmt.Errorf("%w: home.locale is required", errInvalidHouseholdSettings)
	}
	cfg := DefaultConfig()
	cfg.Home = settings.Home
	cfg.Display = settings.Display
	cfg.Weather = settings.Weather
	if err := EnsureValidConfig(&cfg); err != nil {
		return fmt.Errorf("%w: %w", errInvalidHouseholdSettings, err)
	}
	return nil
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.cfg.Public())
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, HomeStateFromConfig(s.cfg, time.Now()))
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"agents": s.agentsWithDiscovery(r.Context(), true),
		})
	case http.MethodPost:
		var req struct {
			CardURL string `json:"cardUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		agent, err := s.addAgentFromCard(r.Context(), req.CardURL)
		if err != nil {
			writeAgentConfigError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, agent)
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (s *Server) handleAgentSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusNotFound, "agent route not found")
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
				writeError(w, http.StatusBadRequest, "invalid JSON request body")
				return
			}
			agent, err := s.patchAgent(agentID, req.Enabled)
			if err != nil {
				writeAgentConfigError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, agent)
		case http.MethodDelete:
			if err := s.deleteAgent(agentID); err != nil {
				writeAgentConfigError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
		default:
			writeMethodNotAllowed(w, http.MethodPatch+", "+http.MethodDelete)
		}
		return
	}
	if len(parts) != 2 || parts[1] != "refresh-card" {
		writeError(w, http.StatusNotFound, "agent route not found")
		return
	}
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	agent, ok := s.registry.Find(agentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	cache := s.refreshAgentCard(r.Context(), agent)
	enriched := s.agentWithDiscovery(agent, cache)
	writeJSON(w, http.StatusOK, enriched)
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.setup)
}

func (s *Server) handleHouseholdSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := s.currentHouseholdSettings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "household settings are unavailable")
			return
		}
		writeJSON(w, http.StatusOK, settings)
	case http.MethodPatch:
		var settings HouseholdSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		saved, err := s.saveHouseholdSettings(r.Context(), settings)
		if err != nil {
			if errors.Is(err, errInvalidHouseholdSettings) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "household settings could not be saved")
			return
		}
		writeJSON(w, http.StatusOK, saved)
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPatch)
	}
}

func (s *Server) handleRoomSettings(w http.ResponseWriter, r *http.Request) {
	handleConfigSliceSettings(w, r, configSliceSettings[RoomConfig]{
		key:       "rooms",
		load:      s.currentRooms,
		save:      s.saveRooms,
		loadError: "room settings are unavailable",
		saveError: "room settings could not be saved",
	})
}

func (s *Server) handleTileSettings(w http.ResponseWriter, r *http.Request) {
	handleConfigSliceSettings(w, r, configSliceSettings[TileConfig]{
		key:       "tiles",
		load:      s.currentTiles,
		save:      s.saveTiles,
		loadError: "tile settings are unavailable",
		saveError: "tile settings could not be saved",
	})
}

type configSliceSettings[T any] struct {
	key       string
	load      func(context.Context) ([]T, error)
	save      func(context.Context, []T) ([]T, error)
	loadError string
	saveError string
}

func handleConfigSliceSettings[T any](w http.ResponseWriter, r *http.Request, settings configSliceSettings[T]) {
	switch r.Method {
	case http.MethodGet:
		values, err := settings.load(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, settings.loadError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{settings.key: values})
	case http.MethodPut:
		var req map[string][]T
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		values, err := settings.save(r.Context(), req[settings.key])
		if err != nil {
			if errors.Is(err, ErrInvalidSettings) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, settings.saveError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{settings.key: values})
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (s *Server) handleWidgetCatalog(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	items := widgets.List()
	catalog := make([]widgets.WidgetCatalogItem, 0, len(items))
	for _, it := range items {
		catalog = append(catalog, it.CatalogInfo())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"widgets": catalog,
	})
}

func (s *Server) handleWidgetLayout(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		layout, err := s.currentWidgetLayout(r.Context(), r.URL.Query().Get("profileId"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "widget layout is unavailable")
			return
		}
		writeJSON(w, http.StatusOK, layout)
	case http.MethodPut:
		var layout WidgetLayout
		if err := json.NewDecoder(r.Body).Decode(&layout); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		if strings.TrimSpace(layout.ProfileID) == "" {
			layout.ProfileID = s.layout.ProfileID
		}
		saved, err := s.saveWidgetLayout(r.Context(), layout)
		if err != nil {
			if errors.Is(err, ErrInvalidLayout) {
				writeError(w, http.StatusBadRequest, "invalid widget layout")
				return
			}
			writeError(w, http.StatusInternalServerError, "widget layout could not be saved")
			return
		}
		writeJSON(w, http.StatusOK, saved)
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (s *Server) handleWidgetLayoutReset(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	profileID := strings.TrimSpace(r.URL.Query().Get("profileId"))
	if profileID == "" {
		profileID = s.layout.ProfileID
	}
	layout := DefaultWidgetLayout()
	layout.ProfileID = profileID

	var saved WidgetLayout
	var err error
	if s.layoutStore != nil {
		saved, err = s.layoutStore.ResetWidgetLayout(r.Context(), profileID)
	} else {
		saved, err = NormalizeWidgetLayout(layout, widgetCatalogMap())
		if err == nil {
			s.layout = saved
		}
	}
	if err != nil {
		if errors.Is(err, ErrInvalidLayout) {
			writeError(w, http.StatusBadRequest, "invalid widget layout")
			return
		}
		writeError(w, http.StatusInternalServerError, "widget layout could not be reset")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleVoiceStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	status, err := s.currentVoiceStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voice status is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleVoiceMute(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	status, err := s.setVoiceMuted(r.Context(), true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voice mute state could not be updated")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleVoiceUnmute(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	status, err := s.setVoiceMuted(r.Context(), false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voice mute state could not be updated")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleVoiceCancel(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	status, err := s.cancelVoice(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voice state could not be cancelled")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleVoiceProviders(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	providers, err := s.voiceProviders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voice providers are unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

func (s *Server) currentHouseholdSettings(ctx context.Context) (HouseholdSettings, error) {
	return s.settings.HouseholdSettings(ctx)
}

func (s *Server) saveHouseholdSettings(
	ctx context.Context,
	settings HouseholdSettings,
) (HouseholdSettings, error) {
	current, err := s.currentHouseholdSettings(ctx)
	if err != nil {
		current = HouseholdSettings{}
	}
	merged := mergeHouseholdSettings(current, settings)
	if err := validateHouseholdSettings(merged); err != nil {
		return HouseholdSettings{}, err
	}
	saved, err := s.settings.SaveHouseholdSettings(ctx, merged)
	if err != nil {
		return HouseholdSettings{}, err
	}
	s.mu.Lock()
	s.cfg.Home = saved.Home
	s.cfg.Display = saved.Display
	s.cfg.Weather = saved.Weather
	s.setup = saved.Setup
	s.mu.Unlock()
	return saved, nil
}

func (s *Server) currentRooms(ctx context.Context) ([]RoomConfig, error) {
	return s.settings.Rooms(ctx)
}

func (s *Server) saveRooms(ctx context.Context, rooms []RoomConfig) ([]RoomConfig, error) {
	saved, err := s.settings.SaveRooms(ctx, rooms)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cfg.Rooms = saved
	s.mu.Unlock()
	return saved, nil
}

func (s *Server) currentTiles(ctx context.Context) ([]TileConfig, error) {
	return s.settings.Tiles(ctx)
}

func (s *Server) saveTiles(ctx context.Context, tiles []TileConfig) ([]TileConfig, error) {
	saved, err := s.settings.SaveTiles(ctx, tiles)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cfg.Tiles = saved
	s.mu.Unlock()
	return saved, nil
}

func (s *Server) currentWidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error) {
	layout, err := s.layoutStore.WidgetLayout(ctx, profileID)
	if err != nil {
		return WidgetLayout{}, err
	}
	s.layout = hydrateServerWidgetLayout(ctx, layout)
	return s.layout, nil
}

func hydrateServerWidgetLayout(ctx context.Context, layout WidgetLayout) WidgetLayout {
	for i := range layout.Widgets {
		widget := &layout.Widgets[i]
		if !widget.Visible {
			continue
		}
		provider, ok := widgets.Get(widget.Kind)
		if !ok {
			continue
		}
		if widget.Overflow == "" {
			widget.Overflow = provider.CatalogInfo().Overflow
		}
		data, err := provider.FetchData(ctx, widget.Settings)
		if err == nil {
			widget.Data = data
		}
	}
	return layout
}

func (s *Server) saveWidgetLayout(ctx context.Context, layout WidgetLayout) (WidgetLayout, error) {
	saved, err := s.layoutStore.SaveWidgetLayout(ctx, layout)
	if err != nil {
		return WidgetLayout{}, err
	}
	s.layout = saved
	return s.layout, nil
}

func (s *Server) currentVoiceStatus(ctx context.Context) (VoiceStatusResponse, error) {
	settings, err := s.voiceStore.VoiceSettings(ctx, "")
	if err != nil {
		return VoiceStatusResponse{}, err
	}
	return voiceStatusFromSettings(settings), nil
}

func (s *Server) setVoiceMuted(ctx context.Context, muted bool) (VoiceStatusResponse, error) {
	settings, err := s.voiceStore.SetVoiceMuted(ctx, "", muted)
	if err != nil {
		return VoiceStatusResponse{}, err
	}
	status := voiceStatusFromSettings(settings)
	s.display.EmitVoiceStateChanged("default-display", displayactions.VoiceStatePayload{
		Enabled:       status.Enabled,
		Muted:         status.Muted,
		State:         status.State,
		ServiceStatus: status.ServiceStatus,
	})
	return status, nil
}

func (s *Server) cancelVoice(ctx context.Context) (VoiceStatusResponse, error) {
	settings, err := s.voiceStore.CancelVoice(ctx, "")
	if err != nil {
		return VoiceStatusResponse{}, err
	}
	status := voiceStatusFromSettings(settings)
	s.display.EmitVoiceStateChanged("default-display", displayactions.VoiceStatePayload{
		Enabled:       status.Enabled,
		Muted:         status.Muted,
		State:         status.State,
		ServiceStatus: status.ServiceStatus,
	})
	return status, nil
}

func (s *Server) voiceProviders(ctx context.Context) ([]VoiceProviderPack, error) {
	return s.voiceStore.VoiceProviders(ctx)
}

func voiceStatusFromSettings(settings VoiceSettings) VoiceStatusResponse {
	return VoiceStatusResponse{
		Enabled:                 settings.Enabled,
		Muted:                   settings.Muted,
		State:                   voiceState(settings.Enabled, settings.Muted),
		ServiceStatus:           voiceServiceStatus(settings.Enabled, settings.STTProviderID, settings.TTSProviderID),
		DeviceProfileID:         settings.DeviceProfileID,
		WakeWordModelID:         settings.WakeWordModelID,
		STTProviderID:           settings.STTProviderID,
		TTSProviderID:           settings.TTSProviderID,
		STTModelID:              settings.STTModelID,
		TTSModelID:              settings.TTSModelID,
		TTSVoiceID:              settings.TTSVoiceID,
		PreferredAgentID:        settings.PreferredAgentID,
		CloudOptIn:              settings.CloudOptIn,
		CommandProvidersEnabled: settings.CommandProvidersEnabled,
		FollowupWindowSeconds:   settings.FollowupWindowSeconds,
		MicrophoneProfile:       settings.MicrophoneProfile,
		UpdatedAt:               settings.UpdatedAt,
	}
}

func voiceStatusFromConfig(voice VoiceConfig, now time.Time) VoiceStatusResponse {
	return VoiceStatusResponse{
		Enabled:                 voice.Enabled,
		Muted:                   voice.MutedByDefault,
		State:                   voiceState(voice.Enabled, voice.MutedByDefault),
		ServiceStatus:           voiceServiceStatus(voice.Enabled, voice.STTProviderID, voice.TTSProviderID),
		DeviceProfileID:         "default-display",
		WakeWordModelID:         voice.WakeWordModelID,
		STTProviderID:           voice.STTProviderID,
		TTSProviderID:           voice.TTSProviderID,
		STTModelID:              voice.STTModelID,
		TTSModelID:              voice.TTSModelID,
		TTSVoiceID:              voice.TTSVoiceID,
		PreferredAgentID:        voice.PreferredAgentID,
		CloudOptIn:              voice.CloudOptIn,
		CommandProvidersEnabled: voice.CommandProvidersEnabled,
		FollowupWindowSeconds:   voice.FollowupWindowSeconds,
		MicrophoneProfile:       voice.MicrophoneProfile,
		UpdatedAt:               now.Format(time.RFC3339Nano),
	}
}

func voiceState(enabled, muted bool) string {
	if muted {
		return "muted"
	}
	if enabled {
		return "wake_listening"
	}
	return "idle"
}

func voiceServiceStatus(enabled bool, sttProviderID, _ string) string {
	if !enabled || strings.TrimSpace(sttProviderID) == "" {
		return "not_configured"
	}
	return "ready"
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	req.AgentID = strings.TrimSpace(req.AgentID)
	req.Text = strings.TrimSpace(req.Text)
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agentId is required")
		return
	}
	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	agent, ok := s.registry.Find(req.AgentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if !agent.Enabled {
		writeError(w, http.StatusConflict, "agent is disabled")
		return
	}

	configuredAgent, ok := s.configuredAgent(req.AgentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	selected := s.selectedAgentInterface(r.Context(), agent)
	if selected.ProtocolBinding != a2aclient.ProtocolJSONRPC {
		writeError(w, http.StatusNotImplemented, "agent protocol binding is not implemented yet")
		return
	}

	bearerToken, ok := agentBearerToken(configuredAgent)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "agent credentials are not available")
		return
	}

	result, err := s.messages.SendMessage(r.Context(), a2aclient.SendMessageRequest{
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
		writeError(w, status, "agent request failed")
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{
		ConversationID: result.ConversationID,
		TaskID:         result.TaskID,
		AgentID:        agent.ID,
		Status:         result.Status,
		Message:        result.Text,
	})
}

type agentCardCache struct {
	AgentID                   string
	CardJSON                  string
	CardStatus                string
	CardError                 string
	SelectedEndpointURL       string
	SelectedProtocolBinding   string
	SelectedProtocolVersion   string
	Streaming                 bool
	DashboardContextSupported bool
	Skills                    []a2aclient.AgentSkill
	FetchedAt                 string
	ExpiresAt                 string
}

func (s *Server) selectedAgentInterface(ctx context.Context, agent registry.Agent) selectedAgentInterface {
	selected := selectedAgentInterface{
		EndpointURL:     agent.EndpointURL,
		ProtocolBinding: agent.ProtocolBinding,
		ProtocolVersion: a2aclient.ProtocolVersion10,
	}
	cache, ok := s.currentAgentCardCache(ctx, agent)
	if ok && cache.SelectedEndpointURL != "" {
		selected.EndpointURL = cache.SelectedEndpointURL
		selected.ProtocolBinding = cache.SelectedProtocolBinding
		selected.ProtocolVersion = cache.SelectedProtocolVersion
		selected.Streaming = cache.Streaming
		if cache.DashboardContextSupported {
			selected.Extensions = []string{a2aclient.DashboardContextExtensionURI}
			selected.Metadata = map[string]any{
				a2aclient.DashboardContextExtensionURI: s.dashboardContext(ctx),
			}
		}
	}
	return selected
}

func (s *Server) agentsWithDiscovery(ctx context.Context, refreshMissing bool) []registry.Agent {
	s.mu.Lock()
	agents := s.registry.List()
	s.mu.Unlock()
	for i := range agents {
		if configured, ok := s.configuredAgent(agents[i].ID); ok {
			agents[i].AuthConfigured = configured.Auth != nil
			agents[i].AuthAvailable = agentAuthAvailable(configured)
		}
		var cache agentCardCache
		var ok bool
		if refreshMissing {
			cache, ok = s.currentAgentCardCache(ctx, agents[i])
		} else {
			cache, ok = s.loadAgentCardCache(ctx, agents[i].ID)
		}
		if ok {
			agents[i] = s.agentWithDiscovery(agents[i], cache)
		}
	}
	return agents
}

func (s *Server) agentStatusSummary(ctx context.Context) AgentStatusSummary {
	agents := s.agentsWithDiscovery(ctx, false)
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

func (s *Server) agentWithDiscovery(agent registry.Agent, cache agentCardCache) registry.Agent {
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

func (s *Server) currentAgentCardCache(ctx context.Context, agent registry.Agent) (agentCardCache, bool) {
	configured, _ := s.configuredAgent(agent.ID)
	return s.agentCards.current(ctx, agent, configured), true
}

func (s *Server) loadAgentCardCache(_ context.Context, agentID string) (agentCardCache, bool) {
	return s.agentCards.load(agentID)
}

func (s *Server) refreshAgentCard(ctx context.Context, agent registry.Agent) agentCardCache {
	configured, _ := s.configuredAgent(agent.ID)
	return s.agentCards.refresh(ctx, agent, configured)
}

func (s *Server) dashboardContext(ctx context.Context) map[string]any {
	layout, err := s.currentWidgetLayout(ctx, "")
	if err != nil {
		layout = s.layout
	}
	snap := Project(ctx, layout, s.cfg)

	widgetsList := []map[string]any{}
	for _, w := range snap.Widgets {
		widgetsList = append(widgetsList, map[string]any{
			"id":            w.ID,
			"kind":          w.Kind,
			"title":         w.Title,
			"size":          w.Size,
			"publicContext": w.PublicContext,
		})
	}

	return map[string]any{
		"schema": a2aclient.DashboardContextExtensionURI,
		"display": map[string]any{
			"deviceId":        snap.Display.DeviceID,
			"profile":         snap.Display.Profile,
			"locale":          snap.Display.Locale,
			"timezone":        snap.Display.Timezone,
			"interactionMode": snap.Display.InteractionMode,
		},
		"dashboard": map[string]any{
			"layoutId":         snap.Display.Profile,
			"visibleWidgetIds": snap.Dashboard.VisibleWidgetIDs,
		},
		"widgets": widgetsList,
	}
}

func (s *Server) configuredAgent(id string) (AgentConfig, bool) {
	for _, agent := range s.cfg.Agents {
		if agent.ID == id {
			return agent, true
		}
	}
	return AgentConfig{}, false
}

func mapToRegistryAgentConfig(cfg AgentConfig) registry.AgentConfig {
	return registry.AgentConfig{
		ID:              cfg.ID,
		Name:            cfg.Name,
		Description:     cfg.Description,
		CardURL:         cfg.CardURL,
		EndpointURL:     cfg.EndpointURL,
		ProtocolBinding: cfg.ProtocolBinding,
		Enabled:         cfg.Enabled,
		Capabilities:    cfg.Capabilities,
		MCPScopes:       cfg.MCPScopes,
		AuthConfigured:  cfg.Auth != nil && strings.TrimSpace(cfg.Auth.Type) != "",
	}
}

func mapToRegistryAgentConfigs(cfgs []AgentConfig) []registry.AgentConfig {
	res := make([]registry.AgentConfig, len(cfgs))
	for i, c := range cfgs {
		res[i] = mapToRegistryAgentConfig(c)
	}
	return res
}

func agentAuthAvailable(agent AgentConfig) bool {
	_, ok := agentBearerToken(agent)
	return ok
}

func agentAuthAvailableFromPublic(agent registry.Agent) bool {
	return !agent.AuthConfigured || agent.AuthAvailable
}

func withCommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Last-Event-ID")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	writeMethodNotAllowed(w, method)
	return false
}

func writeMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// widgetCatalogItems returns catalog items derived from all registered widgets.
func widgetCatalogItems() []WidgetCatalogItem {
	list := widgets.List()
	items := make([]WidgetCatalogItem, 0, len(list))
	for _, w := range list {
		ci := w.CatalogInfo()
		items = append(items, WidgetCatalogItem{
			Kind:          ci.Kind,
			Name:          ci.Name,
			Description:   ci.Description,
			DefaultTitle:  ci.DefaultTitle,
			DefaultW:      ci.DefaultW,
			DefaultH:      ci.DefaultH,
			MinW:          ci.MinW,
			MinH:          ci.MinH,
			DefaultSize:   ci.DefaultSize,
			Overflow:      ci.Overflow,
			AllowMultiple: ci.AllowMultiple,
		})
	}
	return items
}

// widgetCatalogMap returns a kind-keyed catalog map from all registered widgets.
func widgetCatalogMap() map[string]WidgetCatalogItem {
	items := widgetCatalogItems()
	m := make(map[string]WidgetCatalogItem, len(items))
	for _, item := range items {
		m[item.Kind] = item
	}
	return m
}

//nolint:revive // callback signatures require standard parameters



const (
	ProtocolVersion = "2025-11-25"
	jsonRPCVersion  = "2.0"

	callerAgentHeader = "X-Jute-Agent-Id"
)

type SnapshotProvider interface {
	Snapshot(context.Context) (widgetskills.Snapshot, error)
}

type DisplayActions interface {
	Notify(message, severity string) (displayactions.Notification, error)
	FocusWidget(widgetInstanceID, reason string) (displayactions.FocusWidget, error)
}

type Handler struct {
	cfg     MCPConfig
	version string

	provider SnapshotProvider
	display  DisplayActions
}

func NewHandler(
	cfg MCPConfig,
	version string,
	provider SnapshotProvider,
	display ...DisplayActions,
) http.Handler {
	var actionSink DisplayActions
	if len(display) > 0 {
		actionSink = display[0]
	}
	return &Handler{cfg: cfg, version: version, provider: provider, display: actionSink}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.validOrigin(r) {
		writeRPCError(w, http.StatusForbidden, nil, -32000, "origin is not allowed")
		return
	}
	if !h.authorized(r) {
		writeRPCError(w, http.StatusUnauthorized, nil, -32001, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "MCP SSE stream is not implemented", http.StatusMethodNotAllowed)
	case http.MethodPost:
		h.handlePost(w, r)
	default:
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePost(w http.ResponseWriter, r *http.Request) {
	var req rpcRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeRPCError(w, http.StatusBadRequest, nil, -32700, "invalid request")
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPCError(w, http.StatusBadRequest, nil, -32700, "parse error")
		return
	}
	if req.JSONRPC != jsonRPCVersion || strings.TrimSpace(req.Method) == "" {
		writeRPCError(w, http.StatusBadRequest, req.ID, -32600, "invalid request")
		return
	}
	if len(req.ID) == 0 {
		if strings.HasPrefix(req.Method, "notifications/") {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeRPCError(w, http.StatusBadRequest, nil, -32600, "request id is required")
		return
	}

	result, rpcErr := h.dispatch(r.Context(), r, req.Method, req.Params)
	if rpcErr != nil {
		writeRPCError(w, http.StatusOK, req.ID, rpcErr.Code, rpcErr.Message)
		return
	}
	writeRPCResult(w, req.ID, result)
}

func (h *Handler) dispatch(
	ctx context.Context,
	r *http.Request,
	method string,
	params json.RawMessage,
) (any, *rpcError) {
	switch method {
	case "initialize":
		return h.initializeResult(), nil
	case "resources/list":
		snapshot, err := h.snapshot(ctx)
		if err != nil {
			return nil, internalError()
		}
		caller, rpcErr := h.callerForRequest(snapshot, r)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"resources": resourcesList(snapshot, caller)}, nil
	case "resources/read":
		var req resourceReadParams
		if err := decodeParams(params, &req); err != nil {
			return nil, invalidParams(err)
		}
		return h.readResource(ctx, r, req.URI)
	case "tools/list":
		snapshot, err := h.snapshot(ctx)
		if err != nil {
			return nil, internalError()
		}
		caller, rpcErr := h.callerForRequest(snapshot, r)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"tools": toolsList(caller)}, nil
	case "tools/call":
		var req toolCallParams
		if err := decodeParams(params, &req); err != nil {
			return nil, invalidParams(err)
		}
		return h.callTool(ctx, r, req)
	case "prompts/list":
		snapshot, err := h.snapshot(ctx)
		if err != nil {
			return nil, internalError()
		}
		caller, rpcErr := h.callerForRequest(snapshot, r)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"prompts": promptsList(caller)}, nil
	case "prompts/get":
		var req promptGetParams
		if err := decodeParams(params, &req); err != nil {
			return nil, invalidParams(err)
		}
		return h.getPrompt(ctx, r, req.Name, req.Arguments)
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found"}
	}
}

func (h *Handler) initializeResult() map[string]any {
	return map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities": map[string]any{
			"resources": map[string]any{"listChanged": false},
			"tools":     map[string]any{"listChanged": false},
			"prompts":   map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":        "jute-dash",
			"title":       "Jute Dash MCP Bridge",
			"version":     h.version,
			"description": "Local MCP bridge exposing hub-approved Jute dashboard context and Widget Skills.",
		},
	}
}

func (h *Handler) readResource(ctx context.Context, r *http.Request, uri string) (any, *rpcError) {
	snapshot, err := h.snapshot(ctx)
	if err != nil {
		return nil, internalError()
	}
	caller, rpcErr := h.callerForRequest(snapshot, r)
	if rpcErr != nil {
		return nil, rpcErr
	}
	for _, router := range resourceRouters {
		if router.Match(uri) {
			if !caller.has(router.Scope) {
				return nil, missingScope(router.Scope)
			}
			val, err := router.Read(ctx, snapshot, uri)
			if err != nil {
				if errors.Is(err, widgetskills.ErrNotFound) {
					return nil, notFound("resource not found")
				}
				return nil, internalError()
			}
			text, err := jsonText(val)
			if err != nil {
				return nil, internalError()
			}
			return map[string]any{
				"contents": []map[string]any{
					{
						"uri":      uri,
						"mimeType": "application/json",
						"text":     text,
					},
				},
			}, nil
		}
	}
	return nil, notFound("resource not found")
}

func (h *Handler) callTool(ctx context.Context, r *http.Request, req toolCallParams) (any, *rpcError) {
	snapshot, err := h.snapshot(ctx)
	if err != nil {
		return nil, internalError()
	}
	caller, rpcErr := h.callerForRequest(snapshot, r)
	if rpcErr != nil {
		return nil, rpcErr
	}
	for _, router := range toolRouters {
		if router.Name == req.Name {
			if !caller.has(router.Scope) {
				return nil, missingScope(router.Scope)
			}
			val, err := router.Call(ctx, h, snapshot, req.Arguments)
			if err != nil {
				var rpcErr *rpcError
				if errors.As(err, &rpcErr) {
					return nil, rpcErr
				}
				if errors.Is(err, widgetskills.ErrNotFound) {
					return nil, notFound("skill or action not found")
				}
				return nil, invalidParams(err)
			}
			text, err := jsonText(val)
			if err != nil {
				return nil, internalError()
			}
			return map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": text},
				},
				"structuredContent": val,
				"isError":           false,
			}, nil
		}
	}
	return nil, notFound("tool not found")
}

func (h *Handler) getPrompt(
	ctx context.Context,
	r *http.Request,
	name string,
	arguments map[string]any,
) (any, *rpcError) {
	snapshot, err := h.snapshot(ctx)
	if err != nil {
		return nil, internalError()
	}
	caller, rpcErr := h.callerForRequest(snapshot, r)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !caller.has(MCPScopeSkillsPromptRead) {
		return nil, missingScope(MCPScopeSkillsPromptRead)
	}
	var text string
	switch name {
	case "jute_home_assistant_guidance":
		text = widgetskills.HomeAssistantGuidance()
	case "jute_widget_skill_guidance":
		text, err = widgetskills.PromptText(snapshot, stringArg(arguments, "skillId"), stringArg(arguments, "promptId"))
	default:
		return nil, notFound("prompt not found")
	}
	if err != nil {
		if errors.Is(err, widgetskills.ErrNotFound) {
			return nil, notFound("prompt not found")
		}
		return nil, invalidParams(err)
	}
	return map[string]any{
		"description": "Jute MCP prompt guidance",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": map[string]any{
					"type": "text",
					"text": text,
				},
			},
		},
	}, nil
}

func (h *Handler) snapshot(ctx context.Context) (widgetskills.Snapshot, error) {
	if h.provider == nil {
		return widgetskills.Snapshot{}, errors.New("mcp snapshot provider is not configured")
	}
	return h.provider.Snapshot(ctx)
}

type caller struct {
	AgentID   string
	Anonymous bool
	Scopes    map[string]struct{}
}

func (h *Handler) callerForRequest(snapshot widgetskills.Snapshot, r *http.Request) (caller, *rpcError) {
	agentID := strings.TrimSpace(r.Header.Get(callerAgentHeader))
	if agentID == "" {
		if h.cfg.Auth.Mode == "none" {
			return newCaller("", true, DefaultMCPReadScopes()), nil
		}
		return caller{}, unauthorized("mcp caller identity is required")
	}
	for _, agent := range snapshot.Agents {
		if agent.ID != agentID {
			continue
		}
		if !agent.Enabled {
			return caller{}, unauthorized("mcp caller is not enabled")
		}
		return newCaller(agent.ID, false, agent.MCPScopes), nil
	}
	return caller{}, unauthorized("mcp caller is not authorized")
}

func newCaller(agentID string, anonymous bool, scopes []string) caller {
	if len(scopes) == 0 {
		scopes = DefaultMCPReadScopes()
	}
	scopeSet := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			scopeSet[scope] = struct{}{}
		}
	}
	return caller{AgentID: agentID, Anonymous: anonymous, Scopes: scopeSet}
}

func (c caller) has(scope string) bool {
	_, ok := c.Scopes[scope]
	return ok
}

func (h *Handler) validOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if h.cfg.AllowLAN {
		return true
	}
	return isLoopbackHost(host)
}

func (h *Handler) authorized(r *http.Request) bool {
	if h.cfg.Auth.Mode == "none" {
		return true
	}
	if h.cfg.Auth.Mode != "local-token" {
		return false
	}
	token := strings.TrimSpace(os.Getenv(h.cfg.Auth.EnvToken))
	if token == "" {
		return false
	}
	return r.Header.Get("Authorization") == "Bearer "+token
}

type ResourceRouter struct {
	Scope string
	List  func(snapshot widgetskills.Snapshot) []map[string]any
	Match func(uri string) bool
	Read  func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error)
}

type ToolRouter struct {
	Name        string
	Title       string
	Description string
	Scope       string
	InputSchema map[string]any
	Call        func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error)
}

func extractActionIDs(skill widgetskills.Skill) []string {
	actionIDs := make([]string, 0, len(skill.Actions))
	for _, action := range skill.Actions {
		actionIDs = append(actionIDs, action.ID)
	}
	return actionIDs
}

func extractPromptIDs(skill widgetskills.Skill) []string {
	promptIDs := make([]string, 0, len(skill.Prompts))
	for _, prompt := range skill.Prompts {
		promptIDs = append(promptIDs, prompt.ID)
	}
	return promptIDs
}

//nolint:gochecknoglobals // static resource routers table
var resourceRouters = []ResourceRouter{
	{
		Scope: MCPScopeDashboardRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			return []map[string]any{
				resource(
					"jute://dashboard/current",
					"dashboard-current",
					"Current Dashboard Context",
					"Safe current dashboard context and visible Widget Skills.",
				),
			}
		},
		Match: func(uri string) bool { return uri == "jute://dashboard/current" },
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			return widgetskills.DashboardSnapshot(snapshot), nil
		},
	},
	{
		Scope: MCPScopeDashboardRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			return []map[string]any{
				resource("jute://home/state", "home-state", "Home State", "Normalized non-secret home state summary."),
			}
		},
		Match: func(uri string) bool { return uri == "jute://home/state" },
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			generatedAt := snapshot.GeneratedAt
			if generatedAt.IsZero() {
				generatedAt = time.Now().UTC()
			}
			return map[string]any{
				"schema":      "https://jute.dev/mcp/resources/home-state/v1",
				"generatedAt": generatedAt.UTC().Format(time.RFC3339Nano),
				"home":        snapshot.Config.Home,
				"rooms":       snapshot.Config.Rooms,
				"tiles":       snapshot.Config.Tiles,
			}, nil
		},
	},
	{
		Scope: MCPScopeWidgetsRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			return []map[string]any{
				resource(
					"jute://widgets/visible",
					"widgets-visible",
					"Visible Widgets",
					"Visible dashboard widgets and their Widget Skill mappings.",
				),
			}
		},
		Match: func(uri string) bool { return uri == "jute://widgets/visible" },
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			return widgetskills.VisibleWidgetsSnapshot(snapshot), nil
		},
	},
	{
		Scope: MCPScopeSkillsRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			return []map[string]any{
				resource(
					"jute://skills",
					"widget-skills",
					"Widget Skills",
					"Available Widget Skills for this display.",
				),
			}
		},
		Match: func(uri string) bool { return uri == "jute://skills" },
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			return widgetskills.SkillListSnapshot(snapshot), nil
		},
	},
	{
		Scope: MCPScopeSkillsRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			resources := []map[string]any{}
			for _, skill := range widgetskills.Available(snapshot) {
				resources = append(resources, resource(
					"jute://skills/"+skill.SkillID,
					"skill-"+skill.SkillID,
					skill.DisplayName+" Skill",
					skill.Summary,
				))
			}
			return resources
		},
		Match: func(uri string) bool {
			if !strings.HasPrefix(uri, "jute://skills/") {
				return false
			}
			rest := strings.TrimPrefix(uri, "jute://skills/")
			return !strings.Contains(rest, "/")
		},
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			skillID := strings.TrimPrefix(uri, "jute://skills/")
			skill, err := widgetskills.FindSkill(snapshot, skillID, "")
			if err != nil {
				return nil, err
			}
			generatedAt := snapshot.GeneratedAt
			if generatedAt.IsZero() {
				generatedAt = time.Now().UTC()
			}
			return map[string]any{
				"schema":      "https://jute.dev/mcp/resources/widget-skills/v1",
				"generatedAt": generatedAt.UTC().Format(time.RFC3339Nano),
				"skill":       skill,
				"contextUri":  "jute://skills/" + skill.SkillID + "/context",
				"actions":     skill.Actions,
				"prompts":     skill.Prompts,
			}, nil
		},
	},
	{
		Scope: MCPScopeSkillsRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			resources := []map[string]any{}
			for _, skill := range widgetskills.Available(snapshot) {
				resources = append(resources, resource(
					"jute://widgets/"+skill.WidgetInstanceID+"/skill",
					"widget-"+skill.WidgetInstanceID+"-skill",
					skill.WidgetTitle+" Skill",
					"Widget instance to Widget Skill mapping.",
				))
			}
			return resources
		},
		Match: func(uri string) bool {
			return strings.HasPrefix(uri, "jute://widgets/") && strings.HasSuffix(uri, "/skill")
		},
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			widgetID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://widgets/"), "/skill")
			skill, err := widgetskills.FindSkill(snapshot, "", widgetID)
			if err != nil {
				return nil, err
			}
			generatedAt := snapshot.GeneratedAt
			if generatedAt.IsZero() {
				generatedAt = time.Now().UTC()
			}
			return map[string]any{
				"schema":      "https://jute.dev/mcp/resources/widget-skills/v1",
				"generatedAt": generatedAt.UTC().Format(time.RFC3339Nano),
				"skill":       skill,
				"contextUri":  "jute://skills/" + skill.SkillID + "/context",
				"actions":     skill.Actions,
				"prompts":     skill.Prompts,
			}, nil
		},
	},
	{
		Scope: MCPScopeSkillsContextRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			resources := []map[string]any{}
			for _, skill := range widgetskills.Available(snapshot) {
				resources = append(resources, resource(
					"jute://skills/"+skill.SkillID+"/context",
					"skill-"+skill.SkillID+"-context",
					skill.DisplayName+" Context",
					"Current public context for "+skill.DisplayName+".",
				))
			}
			return resources
		},
		Match: func(uri string) bool {
			return strings.HasPrefix(uri, "jute://skills/") && strings.HasSuffix(uri, "/context")
		},
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			skillID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://skills/"), "/context")
			return widgetskills.SkillContext(snapshot, skillID, "")
		},
	},
	{
		Scope: MCPScopeSkillsContextRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			resources := []map[string]any{}
			for _, skill := range widgetskills.Available(snapshot) {
				resources = append(resources, resource(
					"jute://widgets/"+skill.WidgetInstanceID+"/context",
					"widget-"+skill.WidgetInstanceID+"-context",
					skill.WidgetTitle+" Context",
					"Current public Widget Skill context for "+skill.WidgetTitle+".",
				))
			}
			return resources
		},
		Match: func(uri string) bool {
			return strings.HasPrefix(uri, "jute://widgets/") && strings.HasSuffix(uri, "/context")
		},
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			widgetID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://widgets/"), "/context")
			return widgetskills.WidgetContext(snapshot, widgetID)
		},
	},
}

//nolint:gochecknoglobals // static tool routers table
var toolRouters = []ToolRouter{
	{
		Name:        "jute_dashboard_context_get",
		Title:       "Get Dashboard Context",
		Description: "Return safe current Jute dashboard context.",
		Scope:       MCPScopeDashboardRead,
		InputSchema: emptySchema(),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			for _, r := range resourceRouters {
				if r.Match("jute://dashboard/current") {
					return r.Read(ctx, snapshot, "jute://dashboard/current")
				}
			}
			return nil, errors.New("dashboard context resource not found")
		},
	},
	{
		Name:        "jute_skill_list",
		Title:       "List Widget Skills",
		Description: "List available Jute Widget Skills.",
		Scope:       MCPScopeSkillsRead,
		InputSchema: emptySchema(),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			for _, r := range resourceRouters {
				if r.Match("jute://skills") {
					return r.Read(ctx, snapshot, "jute://skills")
				}
			}
			return nil, errors.New("skills resource not found")
		},
	},
	{
		Name:        "jute_skill_read_context",
		Title:       "Read Widget Skill Context",
		Description: "Read public context for a Widget Skill.",
		Scope:       MCPScopeSkillsContextRead,
		InputSchema: objectSchema(map[string]any{
			"skillId":          map[string]any{"type": "string"},
			"widgetInstanceId": map[string]any{"type": "string"},
		}, []string{"skillId"}),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			skillID, widgetID := stringArg(args, "skillId"), stringArg(args, "widgetInstanceId")
			return widgetskills.SkillContext(snapshot, skillID, widgetID)
		},
	},
	{
		Name:        "jute_skill_invoke_action",
		Title:       "Invoke Widget Skill Action",
		Description: "Invoke a declared low-risk Widget Skill action through the hub.",
		Scope:       MCPScopeSkillsActionInvoke,
		InputSchema: objectSchema(map[string]any{
			"skillId":          map[string]any{"type": "string"},
			"widgetInstanceId": map[string]any{"type": "string"},
			"actionId":         map[string]any{"type": "string"},
		}, []string{"skillId", "actionId"}),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			return widgetskills.InvokeAction(
				snapshot,
				stringArg(args, "skillId"),
				stringArg(args, "widgetInstanceId"),
				stringArg(args, "actionId"),
				args,
			)
		},
	},
	{
		Name:        "jute_skill_prompt_get",
		Title:       "Get Widget Skill Prompt",
		Description: "Get hub-approved prompt guidance for a Widget Skill.",
		Scope:       MCPScopeSkillsPromptRead,
		InputSchema: objectSchema(map[string]any{
			"skillId":  map[string]any{"type": "string"},
			"promptId": map[string]any{"type": "string"},
		}, []string{"skillId", "promptId"}),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			text, err := widgetskills.PromptText(
				snapshot,
				stringArg(args, "skillId"),
				stringArg(args, "promptId"),
			)
			if err != nil {
				return nil, err
			}
			return map[string]any{"text": text}, nil
		},
	},
	{
		Name:        "jute_display_notification",
		Title:       "Display Notification",
		Description: "Show a short hub-sanitized notification on the Jute display.",
		Scope:       MCPScopeDisplayWrite,
		InputSchema: objectSchema(map[string]any{
			"message":  map[string]any{"type": "string"},
			"severity": map[string]any{"type": "string", "enum": []string{"info", "success", "warning", "error"}},
		}, []string{"message"}),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			if h.display == nil {
				return nil, &rpcError{Code: -32005, Message: "display actions are unavailable"}
			}
			notification, err := h.display.Notify(
				stringArg(args, "message"),
				stringArg(args, "severity"),
			)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"status":       "queued",
				"eventType":    displayactions.EventNotification,
				"notification": notification,
			}, nil
		},
	},
	{
		Name:        "jute_display_focus_widget",
		Title:       "Focus Widget",
		Description: "Ask the Jute display to highlight a visible widget instance.",
		Scope:       MCPScopeDisplayFocusWidget,
		InputSchema: objectSchema(map[string]any{
			"widgetInstanceId": map[string]any{"type": "string"},
			"reason":           map[string]any{"type": "string"},
		}, []string{"widgetInstanceId"}),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			if h.display == nil {
				return nil, &rpcError{Code: -32005, Message: "display actions are unavailable"}
			}
			widgetID := stringArg(args, "widgetInstanceId")
			if _, err := widgetskills.WidgetContext(snapshot, widgetID); err != nil {
				return nil, err
			}
			focus, err := h.display.FocusWidget(widgetID, stringArg(args, "reason"))
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"status":    "queued",
				"eventType": displayactions.EventFocusWidget,
				"focus":     focus,
			}, nil
		},
	},
}

func resourcesList(snapshot widgetskills.Snapshot, caller caller) []map[string]any {
	resources := []map[string]any{}
	for _, router := range resourceRouters {
		if caller.has(router.Scope) {
			resources = append(resources, router.List(snapshot)...)
		}
	}
	return resources
}

func resource(uri, name, title, description string) map[string]any {
	return map[string]any{
		"uri":         uri,
		"name":        name,
		"title":       title,
		"description": description,
		"mimeType":    "application/json",
	}
}

func toolsList(caller caller) []map[string]any {
	tools := []map[string]any{}
	for _, router := range toolRouters {
		if caller.has(router.Scope) {
			tools = append(tools, tool(
				router.Name,
				router.Title,
				router.Description,
				router.InputSchema,
			))
		}
	}
	return tools
}

func tool(name, title, description string, inputSchema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"title":       title,
		"description": description,
		"inputSchema": inputSchema,
	}
}

func promptsList(caller caller) []map[string]any {
	if !caller.has(MCPScopeSkillsPromptRead) {
		return []map[string]any{}
	}
	return []map[string]any{
		{
			"name":        "jute_home_assistant_guidance",
			"title":       "Jute Home Assistant Guidance",
			"description": "Guidance for using Jute dashboard context and Widget Skills safely.",
		},
		{
			"name":        "jute_widget_skill_guidance",
			"title":       "Jute Widget Skill Guidance",
			"description": "Guidance for using a specific Widget Skill prompt.",
			"arguments": []map[string]any{
				{"name": "skillId", "description": "Widget Skill ID.", "required": true},
				{"name": "promptId", "description": "Skill prompt ID.", "required": true},
			},
		},
	}
}

func emptySchema() map[string]any {
	return objectSchema(map[string]any{}, []string{})
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": true,
	}
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func stringArg(arguments map[string]any, key string) string {
	if arguments == nil {
		return ""
	}
	value, _ := arguments[key].(string)
	return strings.TrimSpace(value)
}

func jsonText(value any) (string, error) {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func writeRPCResult(w http.ResponseWriter, id json.RawMessage, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  result,
	})
}

func writeRPCError(w http.ResponseWriter, status int, id json.RawMessage, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

func invalidParams(err error) *rpcError {
	return &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)}
}

func unauthorized(message string) *rpcError {
	return &rpcError{Code: -32003, Message: message}
}

func missingScope(scope string) *rpcError {
	return unauthorized("missing MCP scope: " + scope)
}

func notFound(message string) *rpcError {
	return &rpcError{Code: -32004, Message: message}
}

func internalError() *rpcError {
	return &rpcError{Code: -32603, Message: "internal error"}
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("RPC Error %d: %s", e.Code, e.Message)
}

type resourceReadParams struct {
	URI string `json:"uri"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type promptGetParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}
