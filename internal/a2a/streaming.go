package a2a

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type streamResponse struct {
	Message        *message            `json:"message,omitempty"`
	Task           *task               `json:"task,omitempty"`
	StatusUpdate   *taskStatusUpdate   `json:"statusUpdate,omitempty"`
	ArtifactUpdate *taskArtifactUpdate `json:"artifactUpdate,omitempty"`
}

type taskStatusUpdate struct {
	TaskID    string     `json:"taskId,omitempty"`
	ContextID string     `json:"contextId,omitempty"`
	Status    taskStatus `json:"status,omitempty"`
	Final     bool       `json:"final,omitempty"`
}

type taskArtifactUpdate struct {
	TaskID    string   `json:"taskId,omitempty"`
	ContextID string   `json:"contextId,omitempty"`
	Artifact  artifact `json:"artifact,omitempty"`
	Append    bool     `json:"append,omitempty"`
	LastChunk bool     `json:"lastChunk,omitempty"`
}

func (c *JSONRPCClient) StreamMessage(ctx context.Context, req SendMessageRequest, handler StreamHandler) error {
	if req.ProtocolBinding != "" && req.ProtocolBinding != ProtocolJSONRPC {
		return ErrUnsupportedProtocol
	}
	if strings.TrimSpace(req.EndpointURL) == "" {
		return errors.New("a2a endpoint url is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return errors.New("a2a message text is required")
	}
	if handler == nil {
		return errors.New("a2a stream handler is required")
	}

	payload := newSendRequest(req, methodSendStreamingMessage)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode a2a streaming request: %w", err)
	}
	httpReq, err := newHTTPRequest(ctx, req, body)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "text/event-stream")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = NewJSONRPCClient().HTTPClient
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return ErrAgentTransport
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%w: status %d", ErrAgentTransport, resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var data strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := handleSSEData(data.String(), handler); err != nil {
				return err
			}
			data.Reset()
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read a2a stream: %w", err)
	}
	if strings.TrimSpace(data.String()) != "" {
		return handleSSEData(data.String(), handler)
	}
	return nil
}

func handleSSEData(data string, handler StreamHandler) error {
	data = strings.TrimSpace(data)
	if data == "" || data == "[DONE]" {
		return nil
	}
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal([]byte(data), &rpcResp); err != nil {
		return fmt.Errorf("decode a2a stream event: %w", err)
	}
	if rpcResp.Error != nil {
		return &RPCError{Code: rpcResp.Error.Code}
	}
	if len(rpcResp.Result) == 0 {
		return nil
	}
	event, ok, err := extractStreamEvent(rpcResp.Result)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return handler(event)
}

func extractStreamEvent(raw json.RawMessage) (StreamEvent, bool, error) {
	var response streamResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return StreamEvent{}, false, fmt.Errorf("decode a2a stream response: %w", err)
	}
	switch {
	case response.Message != nil:
		msg := *response.Message
		return StreamEvent{
			ConversationID: msg.ContextID,
			TaskID:         msg.TaskID,
			Status:         "completed",
			Text:           textFromMessage(msg),
			Terminal:       true,
		}, true, nil
	case response.Task != nil:
		task := *response.Task
		return StreamEvent{
			ConversationID: task.ContextID,
			TaskID:         task.ID,
			Status:         fallbackID(task.Status.State, "working"),
			Text:           textFromOptionalMessage(task.Status.Message),
			Terminal:       isTerminalTaskState(task.Status.State),
		}, true, nil
	case response.StatusUpdate != nil:
		update := *response.StatusUpdate
		return StreamEvent{
			ConversationID: update.ContextID,
			TaskID:         update.TaskID,
			Status:         fallbackID(update.Status.State, "working"),
			Text:           textFromOptionalMessage(update.Status.Message),
			Terminal:       update.Final || isTerminalTaskState(update.Status.State),
		}, true, nil
	case response.ArtifactUpdate != nil:
		update := *response.ArtifactUpdate
		return StreamEvent{
			ConversationID: update.ContextID,
			TaskID:         update.TaskID,
			Status:         "working",
			Text:           textFromParts(update.Artifact.Parts),
			Append:         update.Append,
			Terminal:       update.LastChunk,
		}, true, nil
	default:
		return StreamEvent{}, false, nil
	}
}

func isTerminalTaskState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "completed", "failed", "canceled", "cancelled", "rejected":
		return true
	default:
		return false
	}
}
