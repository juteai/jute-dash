package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Conversation struct {
	ID           string `json:"id"`
	AgentID      string `json:"agentId"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	A2AContextID string `json:"a2aContextId"`
	LatestTaskID string `json:"latestTaskId"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
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

type ConversationEvent struct {
	ID             int64          `json:"id"`
	Type           string         `json:"type"`
	ConversationID string         `json:"conversationId,omitempty"`
	MessageID      string         `json:"messageId,omitempty"`
	Payload        map[string]any `json:"payload"`
	CreatedAt      string         `json:"createdAt"`
}

type ConversationDetail struct {
	Conversation Conversation          `json:"conversation"`
	Messages     []ConversationMessage `json:"messages"`
}

var ErrConversationNotFound = errors.New("conversation not found")

func (s *Store) CreateConversation(ctx context.Context, conversation Conversation) (Conversation, error) {
	conversation.ID = strings.TrimSpace(conversation.ID)
	conversation.AgentID = strings.TrimSpace(conversation.AgentID)
	conversation.Title = strings.TrimSpace(conversation.Title)
	conversation.Status = firstNonEmpty(conversation.Status, "idle")
	if conversation.ID == "" || conversation.AgentID == "" {
		return Conversation{}, errors.New("conversation id and agent id are required")
	}
	if conversation.Title == "" {
		conversation.Title = "New conversation"
	}
	now := nowUTC()
	conversation.CreatedAt = firstNonEmpty(conversation.CreatedAt, now)
	conversation.UpdatedAt = firstNonEmpty(conversation.UpdatedAt, now)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO conversations (
  id, agent_id, title, status, a2a_context_id, latest_task_id, deleted_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, '', ?, ?)`,
		conversation.ID,
		conversation.AgentID,
		conversation.Title,
		conversation.Status,
		conversation.A2AContextID,
		conversation.LatestTaskID,
		conversation.CreatedAt,
		conversation.UpdatedAt,
	)
	if err != nil {
		return Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return conversation, nil
}

func (s *Store) ListConversations(ctx context.Context) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, agent_id, title, status, a2a_context_id, latest_task_id, created_at, updated_at
FROM conversations
WHERE deleted_at = ''
ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()
	conversations := []Conversation{}
	for rows.Next() {
		conversation, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversations: %w", err)
	}
	return conversations, nil
}

func (s *Store) Conversation(ctx context.Context, id string) (ConversationDetail, error) {
	conversation, err := s.conversation(ctx, id)
	if err != nil {
		return ConversationDetail{}, err
	}
	messages, err := s.ConversationMessages(ctx, id)
	if err != nil {
		return ConversationDetail{}, err
	}
	return ConversationDetail{Conversation: conversation, Messages: messages}, nil
}

func (s *Store) conversation(ctx context.Context, id string) (Conversation, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, agent_id, title, status, a2a_context_id, latest_task_id, created_at, updated_at
FROM conversations
WHERE id = ? AND deleted_at = ''`, strings.TrimSpace(id))
	conversation, err := scanConversation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, ErrConversationNotFound
	}
	return conversation, err
}

func (s *Store) ConversationMessages(ctx context.Context, conversationID string) ([]ConversationMessage, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, conversation_id, agent_id, role, content, status, a2a_message_id, a2a_task_id, created_at, updated_at
FROM conversation_messages
WHERE conversation_id = ?
ORDER BY created_at, id`, strings.TrimSpace(conversationID))
	if err != nil {
		return nil, fmt.Errorf("load conversation messages: %w", err)
	}
	defer rows.Close()
	messages := []ConversationMessage{}
	for rows.Next() {
		var message ConversationMessage
		if err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.AgentID,
			&message.Role,
			&message.Content,
			&message.Status,
			&message.A2AMessageID,
			&message.A2ATaskID,
			&message.CreatedAt,
			&message.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan conversation message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversation messages: %w", err)
	}
	return messages, nil
}

func (s *Store) AddConversationMessage(ctx context.Context, message ConversationMessage) (ConversationMessage, error) {
	message.ID = strings.TrimSpace(message.ID)
	message.ConversationID = strings.TrimSpace(message.ConversationID)
	message.AgentID = strings.TrimSpace(message.AgentID)
	message.Role = strings.TrimSpace(message.Role)
	message.Status = firstNonEmpty(message.Status, "sent")
	if message.ID == "" || message.ConversationID == "" || message.AgentID == "" || message.Role == "" {
		return ConversationMessage{}, errors.New("conversation message id, conversation id, agent id, and role are required")
	}
	now := nowUTC()
	message.CreatedAt = firstNonEmpty(message.CreatedAt, now)
	message.UpdatedAt = firstNonEmpty(message.UpdatedAt, now)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO conversation_messages (
  id, conversation_id, agent_id, role, content, status, a2a_message_id, a2a_task_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ID,
		message.ConversationID,
		message.AgentID,
		message.Role,
		message.Content,
		message.Status,
		message.A2AMessageID,
		message.A2ATaskID,
		message.CreatedAt,
		message.UpdatedAt,
	)
	if err != nil {
		return ConversationMessage{}, fmt.Errorf("add conversation message: %w", err)
	}
	if err := s.touchConversation(ctx, message.ConversationID, ""); err != nil {
		return ConversationMessage{}, err
	}
	return message, nil
}

func (s *Store) UpdateConversationMessage(ctx context.Context, messageID, content, status, taskID string, appendContent bool) (ConversationMessage, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return ConversationMessage{}, errors.New("message id is required")
	}
	now := nowUTC()
	if appendContent {
		_, err := s.db.ExecContext(ctx, `
UPDATE conversation_messages
SET content = content || ?, status = ?, a2a_task_id = CASE WHEN ? = '' THEN a2a_task_id ELSE ? END, updated_at = ?
WHERE id = ?`, content, status, taskID, taskID, now, messageID)
		if err != nil {
			return ConversationMessage{}, fmt.Errorf("append conversation message: %w", err)
		}
	} else {
		_, err := s.db.ExecContext(ctx, `
UPDATE conversation_messages
SET content = ?, status = ?, a2a_task_id = CASE WHEN ? = '' THEN a2a_task_id ELSE ? END, updated_at = ?
WHERE id = ?`, content, status, taskID, taskID, now, messageID)
		if err != nil {
			return ConversationMessage{}, fmt.Errorf("update conversation message: %w", err)
		}
	}
	message, err := s.message(ctx, messageID)
	if err != nil {
		return ConversationMessage{}, err
	}
	if err := s.touchConversation(ctx, message.ConversationID, ""); err != nil {
		return ConversationMessage{}, err
	}
	return message, nil
}

func (s *Store) UpdateConversationState(ctx context.Context, conversationID, status, a2aContextID, taskID string) (Conversation, error) {
	now := nowUTC()
	_, err := s.db.ExecContext(ctx, `
UPDATE conversations
SET status = CASE WHEN ? = '' THEN status ELSE ? END,
    a2a_context_id = CASE WHEN ? = '' THEN a2a_context_id ELSE ? END,
    latest_task_id = CASE WHEN ? = '' THEN latest_task_id ELSE ? END,
    updated_at = ?
WHERE id = ? AND deleted_at = ''`,
		status, status,
		a2aContextID, a2aContextID,
		taskID, taskID,
		now,
		conversationID,
	)
	if err != nil {
		return Conversation{}, fmt.Errorf("update conversation state: %w", err)
	}
	return s.conversation(ctx, conversationID)
}

func (s *Store) DeleteConversation(ctx context.Context, id string) error {
	now := nowUTC()
	result, err := s.db.ExecContext(ctx, `UPDATE conversations SET deleted_at = ?, updated_at = ? WHERE id = ?`, now, now, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ErrConversationNotFound
	}
	return nil
}

func (s *Store) AddConversationEvent(ctx context.Context, event ConversationEvent) (ConversationEvent, error) {
	event.Type = strings.TrimSpace(event.Type)
	if event.Type == "" {
		return ConversationEvent{}, errors.New("conversation event type is required")
	}
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return ConversationEvent{}, fmt.Errorf("encode conversation event payload: %w", err)
	}
	event.CreatedAt = firstNonEmpty(event.CreatedAt, nowUTC())
	result, err := s.db.ExecContext(ctx, `
INSERT INTO conversation_events (type, conversation_id, message_id, payload_json, created_at)
VALUES (?, ?, ?, ?, ?)`,
		event.Type,
		event.ConversationID,
		event.MessageID,
		string(payloadJSON),
		event.CreatedAt,
	)
	if err != nil {
		return ConversationEvent{}, fmt.Errorf("add conversation event: %w", err)
	}
	event.ID, _ = result.LastInsertId()
	return event, nil
}

func (s *Store) ConversationEventsSince(ctx context.Context, sinceID int64) ([]ConversationEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, type, conversation_id, message_id, payload_json, created_at
FROM conversation_events
WHERE id > ?
ORDER BY id`, sinceID)
	if err != nil {
		return nil, fmt.Errorf("load conversation events: %w", err)
	}
	defer rows.Close()
	events := []ConversationEvent{}
	for rows.Next() {
		event, err := scanConversationEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversation events: %w", err)
	}
	return events, nil
}

func (s *Store) message(ctx context.Context, messageID string) (ConversationMessage, error) {
	var message ConversationMessage
	err := s.db.QueryRowContext(ctx, `
SELECT id, conversation_id, agent_id, role, content, status, a2a_message_id, a2a_task_id, created_at, updated_at
FROM conversation_messages
WHERE id = ?`, messageID).Scan(
		&message.ID,
		&message.ConversationID,
		&message.AgentID,
		&message.Role,
		&message.Content,
		&message.Status,
		&message.A2AMessageID,
		&message.A2ATaskID,
		&message.CreatedAt,
		&message.UpdatedAt,
	)
	if err != nil {
		return ConversationMessage{}, fmt.Errorf("load conversation message: %w", err)
	}
	return message, nil
}

func (s *Store) touchConversation(ctx context.Context, conversationID, status string) error {
	now := nowUTC()
	if status == "" {
		_, err := s.db.ExecContext(ctx, `UPDATE conversations SET updated_at = ? WHERE id = ?`, now, conversationID)
		if err != nil {
			return fmt.Errorf("touch conversation: %w", err)
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE conversations SET status = ?, updated_at = ? WHERE id = ?`, status, now, conversationID)
	if err != nil {
		return fmt.Errorf("touch conversation: %w", err)
	}
	return nil
}

type conversationScanner interface {
	Scan(dest ...any) error
}

func scanConversation(scanner conversationScanner) (Conversation, error) {
	var conversation Conversation
	if err := scanner.Scan(
		&conversation.ID,
		&conversation.AgentID,
		&conversation.Title,
		&conversation.Status,
		&conversation.A2AContextID,
		&conversation.LatestTaskID,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Conversation{}, err
		}
		return Conversation{}, fmt.Errorf("scan conversation: %w", err)
	}
	return conversation, nil
}

func scanConversationEvent(scanner conversationScanner) (ConversationEvent, error) {
	var event ConversationEvent
	var payloadJSON string
	if err := scanner.Scan(
		&event.ID,
		&event.Type,
		&event.ConversationID,
		&event.MessageID,
		&payloadJSON,
		&event.CreatedAt,
	); err != nil {
		return ConversationEvent{}, fmt.Errorf("scan conversation event: %w", err)
	}
	event.Payload = map[string]any{}
	if strings.TrimSpace(payloadJSON) != "" {
		if err := json.Unmarshal([]byte(payloadJSON), &event.Payload); err != nil {
			return ConversationEvent{}, fmt.Errorf("decode conversation event payload: %w", err)
		}
	}
	return event, nil
}

func firstNonEmpty(candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
