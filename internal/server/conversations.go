package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	a2aclient "jute-dash/internal/a2a"
	"jute-dash/internal/store"
)

type ConversationCreateRequest struct {
	AgentID     string `json:"agentId"`
	Title       string `json:"title,omitempty"`
	InitialText string `json:"initialText,omitempty"`
}

type ConversationTurnRequest struct {
	Text string `json:"text"`
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	if s.conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "conversation store is unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		conversations, err := s.conversations.ListConversations(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "conversations are unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"conversations": conversations})
	case http.MethodPost:
		var req ConversationCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		conversation, err := s.createConversation(r.Context(), req)
		if err != nil {
			writeConversationError(w, err)
			return
		}
		if strings.TrimSpace(req.InitialText) != "" {
			if err := s.startConversationTurn(r.Context(), conversation.ID, strings.TrimSpace(req.InitialText)); err != nil {
				writeConversationError(w, err)
				return
			}
		}
		detail, err := s.conversations.Conversation(r.Context(), conversation.ID)
		if err != nil {
			writeConversationError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, detail)
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (s *Server) handleConversationSubroutes(w http.ResponseWriter, r *http.Request) {
	if s.conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "conversation store is unavailable")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/conversations/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	conversationID := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			detail, err := s.conversations.Conversation(r.Context(), conversationID)
			if err != nil {
				writeConversationError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, detail)
		case http.MethodDelete:
			if err := s.conversations.DeleteConversation(r.Context(), conversationID); err != nil {
				writeConversationError(w, err)
				return
			}
			event := s.publishConversationEvent(r.Context(), store.ConversationEvent{
				Type:           "conversation.deleted",
				ConversationID: conversationID,
				Payload:        map[string]any{"conversationId": conversationID},
			})
			writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "eventId": event.ID})
		default:
			writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodDelete)
		}
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
		if err := s.startConversationTurn(r.Context(), conversationID, strings.TrimSpace(req.Text)); err != nil {
			writeConversationError(w, err)
			return
		}
		detail, err := s.conversations.Conversation(r.Context(), conversationID)
		if err != nil {
			writeConversationError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, detail)
		return
	}
	writeError(w, http.StatusNotFound, "conversation route not found")
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if s.conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "conversation store is unavailable")
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

	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	replay, err := s.conversations.ConversationEventsSince(r.Context(), since)
	if err == nil {
		for _, event := range replay {
			writeSSE(w, event)
		}
		flusher.Flush()
	}

	ch := s.events.Subscribe()
	defer s.events.Unsubscribe(ch)
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			writeSSE(w, event)
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()
		}
	}
}

func (s *Server) createConversation(ctx context.Context, req ConversationCreateRequest) (store.Conversation, error) {
	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		return store.Conversation{}, errors.New("agentId is required")
	}
	agent, ok := s.registry.Find(agentID)
	if !ok {
		return store.Conversation{}, errors.New("agent not found")
	}
	if !agent.Enabled {
		return store.Conversation{}, errors.New("agent is disabled")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = agent.Name
	}
	conversation, err := s.conversations.CreateConversation(ctx, store.Conversation{
		ID:      newLocalID("conv"),
		AgentID: agent.ID,
		Title:   title,
		Status:  "idle",
	})
	if err != nil {
		return store.Conversation{}, err
	}
	s.publishConversationEvent(ctx, store.ConversationEvent{
		Type:           "conversation.created",
		ConversationID: conversation.ID,
		Payload:        map[string]any{"conversation": conversation},
	})
	return conversation, nil
}

func (s *Server) startConversationTurn(ctx context.Context, conversationID, text string) error {
	if text == "" {
		return errors.New("text is required")
	}
	detail, err := s.conversations.Conversation(ctx, conversationID)
	if err != nil {
		return err
	}
	agent, ok := s.registry.Find(detail.Conversation.AgentID)
	if !ok || !agent.Enabled {
		return errors.New("agent is not available")
	}
	userMessage, err := s.conversations.AddConversationMessage(ctx, store.ConversationMessage{
		ID:             newLocalID("msg"),
		ConversationID: conversationID,
		AgentID:        agent.ID,
		Role:           "user",
		Content:        text,
		Status:         "sent",
	})
	if err != nil {
		return err
	}
	s.publishConversationEvent(ctx, store.ConversationEvent{
		Type:           "conversation.message.created",
		ConversationID: conversationID,
		MessageID:      userMessage.ID,
		Payload:        map[string]any{"message": userMessage},
	})
	assistantMessage, err := s.conversations.AddConversationMessage(ctx, store.ConversationMessage{
		ID:             newLocalID("msg"),
		ConversationID: conversationID,
		AgentID:        agent.ID,
		Role:           "assistant",
		Content:        "",
		Status:         "streaming",
	})
	if err != nil {
		return err
	}
	s.publishConversationEvent(ctx, store.ConversationEvent{
		Type:           "conversation.message.created",
		ConversationID: conversationID,
		MessageID:      assistantMessage.ID,
		Payload:        map[string]any{"message": assistantMessage},
	})
	if _, err := s.conversations.UpdateConversationState(ctx, conversationID, "streaming", "", ""); err != nil {
		return err
	}
	s.publishConversationEvent(ctx, store.ConversationEvent{
		Type:           "conversation.turn_started",
		ConversationID: conversationID,
		MessageID:      assistantMessage.ID,
		Payload:        map[string]any{"conversationId": conversationID, "messageId": assistantMessage.ID},
	})

	go s.runConversationTurn(conversationID, assistantMessage.ID, text)
	return nil
}

func (s *Server) runConversationTurn(conversationID, assistantMessageID, text string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	detail, err := s.conversations.Conversation(ctx, conversationID)
	if err != nil {
		return
	}
	agent, ok := s.registry.Find(detail.Conversation.AgentID)
	if !ok {
		s.failConversationTurn(ctx, conversationID, assistantMessageID)
		return
	}
	configuredAgent, ok := s.configuredAgent(agent.ID)
	if !ok {
		s.failConversationTurn(ctx, conversationID, assistantMessageID)
		return
	}
	selected := s.selectedAgentInterface(ctx, agent)
	if selected.ProtocolBinding != a2aclient.ProtocolJSONRPC {
		s.failConversationTurn(ctx, conversationID, assistantMessageID)
		return
	}
	bearerToken, ok := agentBearerToken(configuredAgent)
	if !ok {
		s.failConversationTurn(ctx, conversationID, assistantMessageID)
		return
	}
	req := a2aclient.SendMessageRequest{
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		ProtocolVersion: selected.ProtocolVersion,
		Text:            text,
		BearerToken:     bearerToken,
		ConversationID:  detail.Conversation.A2AContextID,
		TaskID:          detail.Conversation.LatestTaskID,
		Extensions:      selected.Extensions,
		Metadata:        selected.Metadata,
	}
	if selected.Streaming {
		if streamer, ok := s.messages.(a2aclient.StreamingMessageSender); ok {
			err := streamer.StreamMessage(ctx, req, func(event a2aclient.StreamEvent) error {
				s.applyStreamEvent(ctx, conversationID, assistantMessageID, event)
				return nil
			})
			if err == nil {
				return
			}
		}
	}
	result, err := s.messages.SendMessage(ctx, req)
	if err != nil {
		s.failConversationTurn(ctx, conversationID, assistantMessageID)
		return
	}
	s.applyStreamEvent(ctx, conversationID, assistantMessageID, a2aclient.StreamEvent{
		ConversationID: result.ConversationID,
		TaskID:         result.TaskID,
		Status:         result.Status,
		Text:           result.Text,
		Terminal:       true,
	})
}

func (s *Server) applyStreamEvent(ctx context.Context, conversationID, assistantMessageID string, event a2aclient.StreamEvent) {
	status := "streaming"
	if event.Terminal {
		if strings.EqualFold(event.Status, "failed") || strings.EqualFold(event.Status, "canceled") || strings.EqualFold(event.Status, "rejected") {
			status = "failed"
		} else {
			status = "sent"
		}
	}
	if event.ConversationID != "" || event.TaskID != "" || event.Status != "" {
		_, _ = s.conversations.UpdateConversationState(ctx, conversationID, conversationStatus(status), event.ConversationID, event.TaskID)
	}
	if event.Text != "" || event.Terminal {
		appendContent := event.Append || (event.Terminal && event.Text == "")
		message, err := s.conversations.UpdateConversationMessage(ctx, assistantMessageID, event.Text, status, event.TaskID, appendContent)
		if err == nil {
			s.publishConversationEvent(ctx, store.ConversationEvent{
				Type:           "conversation.message.updated",
				ConversationID: conversationID,
				MessageID:      assistantMessageID,
				Payload:        map[string]any{"message": message},
			})
		}
	}
	if event.Terminal {
		s.publishConversationEvent(ctx, store.ConversationEvent{
			Type:           "conversation.turn_completed",
			ConversationID: conversationID,
			MessageID:      assistantMessageID,
			Payload:        map[string]any{"conversationId": conversationID, "status": conversationStatus(status)},
		})
	}
}

func (s *Server) failConversationTurn(ctx context.Context, conversationID, assistantMessageID string) {
	message, err := s.conversations.UpdateConversationMessage(ctx, assistantMessageID, "Message not sent. Check that the local agent is running, then retry.", "failed", "", false)
	if err == nil {
		s.publishConversationEvent(ctx, store.ConversationEvent{
			Type:           "conversation.message.updated",
			ConversationID: conversationID,
			MessageID:      assistantMessageID,
			Payload:        map[string]any{"message": message},
		})
	}
	_, _ = s.conversations.UpdateConversationState(ctx, conversationID, "failed", "", "")
	s.publishConversationEvent(ctx, store.ConversationEvent{
		Type:           "conversation.turn_completed",
		ConversationID: conversationID,
		MessageID:      assistantMessageID,
		Payload:        map[string]any{"conversationId": conversationID, "status": "failed"},
	})
}

func (s *Server) publishConversationEvent(ctx context.Context, event store.ConversationEvent) store.ConversationEvent {
	saved, err := s.conversations.AddConversationEvent(ctx, event)
	if err != nil {
		return event
	}
	s.events.Publish(saved)
	return saved
}

func writeSSE(w http.ResponseWriter, event store.ConversationEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "id: %d\n", event.ID)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func conversationStatus(messageStatus string) string {
	switch messageStatus {
	case "failed":
		return "failed"
	case "sent":
		return "completed"
	default:
		return "streaming"
	}
}

func writeConversationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrConversationNotFound):
		writeError(w, http.StatusNotFound, "conversation not found")
	case strings.Contains(err.Error(), "agentId is required"), strings.Contains(err.Error(), "text is required"):
		writeError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "agent not found"):
		writeError(w, http.StatusNotFound, "agent not found")
	case strings.Contains(err.Error(), "agent is disabled"), strings.Contains(err.Error(), "agent is not available"):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "conversation request failed")
	}
}

func newLocalID(prefix string) string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return prefix + "-" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
