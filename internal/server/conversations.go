package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	a2aclient "jute-dash/internal/a2a"
	"jute-dash/internal/registry"
)

type ConversationCreateRequest struct {
	AgentID     string `json:"agentId"`
	Title       string `json:"title,omitempty"`
	InitialText string `json:"initialText,omitempty"`
}

type ConversationTurnRequest struct {
	AgentID string `json:"agentId"`
	Text    string `json:"text"`
}

type Conversation struct {
	ID                 string `json:"id"`
	AgentID            string `json:"agentId"`
	Title              string `json:"title"`
	Status             string `json:"status"`
	A2AContextID       string `json:"a2aContextId"`
	LatestTaskID       string `json:"latestTaskId"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
	HistoryUnsupported bool   `json:"historyUnsupported,omitempty"`
}

type ConversationMessage struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId"`
	AgentID        string `json:"agentId"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	Status         string `json:"status"`
	A2AMessageID   string `json:"a2aMessageId"`
	A2ATaskID      string `json:"a2aTaskId"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type ConversationDetail struct {
	Conversation Conversation          `json:"conversation"`
	Messages     []ConversationMessage `json:"messages"`
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
				ID:           "ctx-" + newLocalID(),
				AgentID:      agent.ID,
				Title:        firstNonEmpty(strings.TrimSpace(req.Title), agent.Name),
				Status:       "idle",
				A2AContextID: "ctx-" + newLocalID(),
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
	writeError(w, http.StatusNotFound, "conversation route not found")
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	if flusher, ok := w.(http.Flusher); ok {
		_, _ = w.Write([]byte(": no local event replay; conversations are agent-backed\n\n"))
		flusher.Flush()
	}
}

func (s *Server) agentForHistoryRequest(w http.ResponseWriter, r *http.Request) (registry.Agent, selectedAgentInterface, string, bool) {
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

func (s *Server) listAgentConversations(ctx context.Context, agent registry.Agent, selected selectedAgentInterface, bearerToken string) ([]Conversation, error) {
	history, ok := s.messages.(a2aclient.TaskHistoryClient)
	if !ok {
		return []Conversation{unsupportedConversation(agent)}, nil
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
			return []Conversation{unsupportedConversation(agent)}, nil
		}
		return nil, err
	}
	byContext := map[string]Conversation{}
	for _, task := range result.Tasks {
		contextID := firstNonEmpty(task.ContextID, task.ID)
		conversation := byContext[contextID]
		if conversation.ID == "" {
			conversation = Conversation{
				ID:           contextID,
				AgentID:      agent.ID,
				Title:        firstNonEmpty(firstUserText(task.Messages), task.Text, agent.Name),
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

func (s *Server) agentConversation(ctx context.Context, agent registry.Agent, selected selectedAgentInterface, bearerToken, conversationID string) (ConversationDetail, error) {
	history, ok := s.messages.(a2aclient.TaskHistoryClient)
	if !ok {
		return ConversationDetail{Conversation: unsupportedConversation(agent)}, nil
	}
	tasks, err := history.ListTasks(ctx, a2aclient.ListTasksRequest{
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		ProtocolVersion: selected.ProtocolVersion,
		BearerToken:     bearerToken,
		ContextID:       conversationID,
		PageSize:        50,
	})
	if err != nil {
		if isUnsupportedHistory(err) {
			return ConversationDetail{Conversation: unsupportedConversation(agent)}, nil
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
	for _, task := range tasks.Tasks {
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
		if title := firstUserText(record.Messages); title != "" && detail.Conversation.Title == agent.Name {
			detail.Conversation.Title = title
		}
		detail.Messages = append(detail.Messages, conversationMessages(agent.ID, conversationID, record)...)
	}
	return detail, nil
}

func (s *Server) sendConversationTurn(ctx context.Context, conversationID string, req ConversationTurnRequest) (ConversationDetail, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return ConversationDetail{}, errors.New("text is required")
	}
	agent, ok := s.registry.Find(strings.TrimSpace(req.AgentID))
	if !ok {
		return ConversationDetail{}, errors.New("agent not found")
	}
	configuredAgent, ok := s.configuredAgent(agent.ID)
	if !ok {
		return ConversationDetail{}, errors.New("agent not found")
	}
	selected := s.selectedAgentInterface(ctx, agent)
	if selected.ProtocolBinding != a2aclient.ProtocolJSONRPC {
		return ConversationDetail{}, a2aclient.ErrUnsupportedProtocol
	}
	bearerToken, ok := agentBearerToken(configuredAgent)
	if !ok {
		return ConversationDetail{}, errors.New("agent credentials are not available")
	}
	sendReq := a2aclient.SendMessageRequest{
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		ProtocolVersion: selected.ProtocolVersion,
		Text:            text,
		BearerToken:     bearerToken,
		ConversationID:  conversationID,
		Extensions:      selected.Extensions,
		Metadata:        selected.Metadata,
	}
	var streamText strings.Builder
	var taskID string
	var status string
	if selected.Streaming {
		if streamer, ok := s.messages.(a2aclient.StreamingMessageSender); ok {
			err := streamer.StreamMessage(ctx, sendReq, func(event a2aclient.StreamEvent) error {
				if event.Append {
					streamText.WriteString(event.Text)
				} else if event.Text != "" {
					streamText.Reset()
					streamText.WriteString(event.Text)
				}
				if event.TaskID != "" {
					taskID = event.TaskID
				}
				if event.Status != "" {
					status = event.Status
				}
				return nil
			})
			if err == nil {
				return s.detailAfterTurn(ctx, agent, selected, bearerToken, conversationID, taskID, text, streamText.String(), status), nil
			}
		}
	}
	result, err := s.messages.SendMessage(ctx, sendReq)
	if err != nil {
		return ConversationDetail{}, err
	}
	return s.detailAfterTurn(ctx, agent, selected, bearerToken, result.ConversationID, result.TaskID, text, result.Text, result.Status), nil
}

func (s *Server) detailAfterTurn(ctx context.Context, agent registry.Agent, selected selectedAgentInterface, bearerToken, conversationID, taskID, userText, assistantText, status string) ConversationDetail {
	if taskID != "" {
		if history, ok := s.messages.(a2aclient.TaskHistoryClient); ok {
			if task, err := history.GetTask(ctx, a2aclient.GetTaskRequest{
				EndpointURL:     selected.EndpointURL,
				ProtocolBinding: selected.ProtocolBinding,
				ProtocolVersion: selected.ProtocolVersion,
				BearerToken:     bearerToken,
				TaskID:          taskID,
				HistoryLength:   50,
			}); err == nil {
				return ConversationDetail{
					Conversation: conversationFromTask(agent, task),
					Messages:     conversationMessages(agent.ID, firstNonEmpty(task.ContextID, conversationID), task),
				}
			}
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	conversation := Conversation{
		ID:           conversationID,
		AgentID:      agent.ID,
		Title:        userText,
		Status:       firstNonEmpty(status, "completed"),
		A2AContextID: conversationID,
		LatestTaskID: taskID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return ConversationDetail{
		Conversation: conversation,
		Messages: []ConversationMessage{
			{ID: "local-user-" + newLocalID(), ConversationID: conversationID, AgentID: agent.ID, Role: "user", Content: userText, Status: "sent", CreatedAt: now, UpdatedAt: now},
			{ID: "local-agent-" + newLocalID(), ConversationID: conversationID, AgentID: agent.ID, Role: "assistant", Content: assistantText, Status: "sent", A2ATaskID: taskID, CreatedAt: now, UpdatedAt: now},
		},
	}
}

func conversationFromTask(agent registry.Agent, task a2aclient.TaskRecord) Conversation {
	return Conversation{
		ID:           firstNonEmpty(task.ContextID, task.ID),
		AgentID:      agent.ID,
		Title:        firstNonEmpty(firstUserText(task.Messages), task.Text, agent.Name),
		Status:       task.Status,
		A2AContextID: firstNonEmpty(task.ContextID, task.ID),
		LatestTaskID: task.ID,
		CreatedAt:    task.UpdatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

func conversationMessages(agentID, conversationID string, task a2aclient.TaskRecord) []ConversationMessage {
	messages := make([]ConversationMessage, 0, len(task.Messages))
	for i, message := range task.Messages {
		messages = append(messages, ConversationMessage{
			ID:             firstNonEmpty(message.ID, fmt.Sprintf("%s-%d", task.ID, i)),
			ConversationID: conversationID,
			AgentID:        agentID,
			Role:           message.Role,
			Content:        message.Text,
			Status:         "sent",
			A2ATaskID:      task.ID,
			CreatedAt:      firstNonEmpty(message.CreatedAt, task.UpdatedAt),
			UpdatedAt:      task.UpdatedAt,
		})
	}
	return messages
}

func unsupportedConversation(agent registry.Agent) Conversation {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return Conversation{
		ID:                 "history-unsupported-" + agent.ID,
		AgentID:            agent.ID,
		Title:              "History unavailable for this agent",
		Status:             "unsupported",
		CreatedAt:          now,
		UpdatedAt:          now,
		HistoryUnsupported: true,
	}
}

func firstUserText(messages []a2aclient.TaskMessage) string {
	for _, message := range messages {
		if message.Role == "user" && strings.TrimSpace(message.Text) != "" {
			return strings.TrimSpace(message.Text)
		}
	}
	return ""
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

func firstNonEmpty(candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func newLocalID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
