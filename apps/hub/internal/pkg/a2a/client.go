package a2a

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	jsonRPCVersion             = "2.0"
	methodSendMessage          = "SendMessage"
	methodSendStreamingMessage = "SendStreamingMessage"
	methodGetTask              = "GetTask"
	methodListTasks            = "ListTasks"
	methodNotFoundCode         = -32601
)

var (
	ErrUnsupportedProtocol = errors.New("a2a protocol binding is not implemented")
	ErrAgentRPCFailure     = errors.New("agent returned an a2a json-rpc error")
	ErrAgentTransport      = errors.New("agent transport request failed")
)

type SendMessageRequest struct {
	EndpointURL     string
	ProtocolBinding string
	ProtocolVersion string
	Text            string
	BearerToken     string
	ConversationID  string
	TaskID          string
	Extensions      []string
	Metadata        map[string]any
}

type SendMessageResult struct {
	ConversationID string
	TaskID         string
	Status         string
	Text           string
}

type MessageSender interface {
	SendMessage(ctx context.Context, req SendMessageRequest) (SendMessageResult, error)
}

type StreamEvent struct {
	Kind           string
	ConversationID string
	TaskID         string
	Status         string
	Text           string
	Append         bool
	Terminal       bool
}

type StreamHandler func(StreamEvent) error

type StreamingMessageSender interface {
	SendMessage(ctx context.Context, req SendMessageRequest) (SendMessageResult, error)
	StreamMessage(ctx context.Context, req SendMessageRequest, handler StreamHandler) error
}

type TaskHistoryClient interface {
	ListTasks(ctx context.Context, req ListTasksRequest) (ListTasksResult, error)
	GetTask(ctx context.Context, req GetTaskRequest) (TaskRecord, error)
}

// Client consolidates all A2A client operations under a single port.
type Client interface {
	MessageSender
	StreamingMessageSender
	TaskHistoryClient
}

type JSONRPCClient struct {
	HTTPClient *http.Client
}

func NewJSONRPCClient() *JSONRPCClient {
	return &JSONRPCClient{
		HTTPClient: &http.Client{Timeout: 45 * time.Second},
	}
}

func (c *JSONRPCClient) SendMessage(ctx context.Context, req SendMessageRequest) (SendMessageResult, error) {
	if req.ProtocolBinding != "" && req.ProtocolBinding != ProtocolJSONRPC {
		return SendMessageResult{}, ErrUnsupportedProtocol
	}
	if strings.TrimSpace(req.EndpointURL) == "" {
		return SendMessageResult{}, errors.New("a2a endpoint url is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return SendMessageResult{}, errors.New("a2a message text is required")
	}

	return c.send(ctx, req)
}

type RPCError struct {
	Code int
}

func (e *RPCError) Error() string {
	if e.Code == methodNotFoundCode {
		return "agent does not support the requested a2a method"
	}
	return ErrAgentRPCFailure.Error()
}

func (c *JSONRPCClient) send(ctx context.Context, req SendMessageRequest) (SendMessageResult, error) {
	payload := newSendRequest(req, methodSendMessage)

	body, err := json.Marshal(payload)
	if err != nil {
		return SendMessageResult{}, fmt.Errorf("encode a2a request: %w", err)
	}

	httpReq, err := newHTTPRequest(ctx, req, body)
	if err != nil {
		return SendMessageResult{}, err
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = NewJSONRPCClient().HTTPClient
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return SendMessageResult{}, fmt.Errorf("%w", ErrAgentTransport)
	}
	defer resp.Body.Close()

	responseBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return SendMessageResult{}, fmt.Errorf("read a2a response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return SendMessageResult{}, fmt.Errorf("%w: status %d", ErrAgentTransport, resp.StatusCode)
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(responseBytes, &rpcResp); err != nil {
		return SendMessageResult{}, fmt.Errorf("decode a2a response: %w", err)
	}
	if rpcResp.Error != nil {
		return SendMessageResult{}, &RPCError{Code: rpcResp.Error.Code}
	}
	if len(rpcResp.Result) == 0 {
		return SendMessageResult{}, errors.New("a2a response did not include a result")
	}

	return extractResult(rpcResp.Result)
}

func newSendRequest(req SendMessageRequest, method string) jsonRPCRequest {
	return jsonRPCRequest{
		JSONRPC: jsonRPCVersion,
		ID:      newID(),
		Method:  method,
		Params: sendParams{
			Message: message{
				MessageID: newID(),
				ContextID: strings.TrimSpace(req.ConversationID),
				TaskID:    strings.TrimSpace(req.TaskID),
				Role:      "ROLE_USER",
				Parts: []part{
					{Text: req.Text},
				},
				Metadata: cleanMetadata(req.Metadata),
			},
			Configuration: sendConfiguration{
				ReturnImmediately:   boolPtr(false),
				AcceptedOutputModes: []string{"text/plain"},
			},
		},
	}
}

func newHTTPRequest(ctx context.Context, req SendMessageRequest, body []byte) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.EndpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build a2a request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("A2a-Version", fallbackID(req.ProtocolVersion, ProtocolVersion10))
	if len(req.Extensions) > 0 {
		httpReq.Header.Set("A2a-Extensions", strings.Join(req.Extensions, ","))
	}
	if strings.TrimSpace(req.BearerToken) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(req.BearerToken))
	}
	return httpReq, nil
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type sendParams struct {
	Message       message           `json:"message"`
	Configuration sendConfiguration `json:"configuration"`
}

type sendConfiguration struct {
	ReturnImmediately   *bool    `json:"returnImmediately,omitempty"`
	AcceptedOutputModes []string `json:"acceptedOutputModes,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonRPCError   `json:"error"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type resultKind struct {
	Kind string `json:"kind"`
}

type sendMessageResponse struct {
	Message *message `json:"message,omitempty"`
	Task    *task    `json:"task,omitempty"`
}

type message struct {
	Kind      string         `json:"kind,omitempty"`
	MessageID string         `json:"messageId,omitempty"`
	ContextID string         `json:"contextId,omitempty"`
	TaskID    string         `json:"taskId,omitempty"`
	Role      string         `json:"role,omitempty"`
	Parts     []part         `json:"parts,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type task struct {
	Kind      string     `json:"kind,omitempty"`
	ID        string     `json:"id,omitempty"`
	ContextID string     `json:"contextId,omitempty"`
	Status    taskStatus `json:"status,omitempty"`
	History   []message  `json:"history,omitempty"`
	Artifacts []artifact `json:"artifacts,omitempty"`
}

type taskStatus struct {
	State   string   `json:"state,omitempty"`
	Message *message `json:"message,omitempty"`
}

type artifact struct {
	Parts []part `json:"parts,omitempty"`
}

type part struct {
	Kind string `json:"kind,omitempty"`
	Text string `json:"text,omitempty"`
}

func extractResult(raw json.RawMessage) (SendMessageResult, error) {
	var wrapped sendMessageResponse
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		switch {
		case wrapped.Message != nil:
			return resultFromMessage(*wrapped.Message), nil
		case wrapped.Task != nil:
			return resultFromTask(*wrapped.Task), nil
		}
	}

	var kind resultKind
	if err := json.Unmarshal(raw, &kind); err != nil {
		return SendMessageResult{}, fmt.Errorf("decode a2a result kind: %w", err)
	}

	switch kind.Kind {
	case "message":
		var msg message
		if err := json.Unmarshal(raw, &msg); err != nil {
			return SendMessageResult{}, fmt.Errorf("decode a2a message result: %w", err)
		}
		return resultFromMessage(msg), nil
	case "task":
		var task task
		if err := json.Unmarshal(raw, &task); err != nil {
			return SendMessageResult{}, fmt.Errorf("decode a2a task result: %w", err)
		}
		return resultFromTask(task), nil
	default:
		return SendMessageResult{}, fmt.Errorf("unsupported a2a result kind %q", kind.Kind)
	}
}

func resultFromMessage(msg message) SendMessageResult {
	text := displayTextFromMessage(msg)
	if text == "" {
		text = "Agent returned an empty message."
	}
	return SendMessageResult{
		ConversationID: fallbackID(msg.ContextID, msg.MessageID, "local-a2a"),
		TaskID:         msg.TaskID,
		Status:         "completed",
		Text:           text,
	}
}

func resultFromTask(t task) SendMessageResult {
	status := normalizeTaskState(t.Status.State)
	if status == "" {
		status = "unknown"
	}
	if text := displayTextFromOptionalMessage(t.Status.Message); text != "" {
		return SendMessageResult{
			ConversationID: fallbackID(t.ContextID, t.ID, "local-a2a"),
			TaskID:         t.ID,
			Status:         status,
			Text:           text,
		}
	}
	for i := len(t.History) - 1; i >= 0; i-- {
		role := strings.TrimSpace(t.History[i].Role)
		if role != "agent" && role != "ROLE_AGENT" {
			continue
		}
		if text := displayTextFromMessage(t.History[i]); text != "" {
			return SendMessageResult{
				ConversationID: fallbackID(t.ContextID, t.ID, "local-a2a"),
				TaskID:         t.ID,
				Status:         status,
				Text:           text,
			}
		}
	}
	for i := len(t.History) - 1; i >= 0; i-- {
		if text := displayTextFromMessage(t.History[i]); text != "" {
			return SendMessageResult{
				ConversationID: fallbackID(t.ContextID, t.ID, "local-a2a"),
				TaskID:         t.ID,
				Status:         status,
				Text:           text,
			}
		}
	}
	for i := len(t.Artifacts) - 1; i >= 0; i-- {
		if text := displayTextFromParts(t.Artifacts[i].Parts); text != "" {
			return SendMessageResult{
				ConversationID: fallbackID(t.ContextID, t.ID, "local-a2a"),
				TaskID:         t.ID,
				Status:         status,
				Text:           text,
			}
		}
	}
	return SendMessageResult{
		ConversationID: fallbackID(t.ContextID, t.ID, "local-a2a"),
		TaskID:         t.ID,
		Status:         status,
		Text:           fmt.Sprintf("Agent task is %s.", status),
	}
}

func textFromParts(parts []part) string {
	var chunks []string
	for _, item := range parts {
		if item.Kind == "text" || item.Kind == "" {
			if text := strings.TrimSpace(item.Text); text != "" {
				chunks = append(chunks, text)
			}
		}
	}
	return strings.Join(chunks, "\n\n")
}

func fallbackID(candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return "local-a2a"
}

func cleanMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	cleaned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		if strings.TrimSpace(key) == "" || value == nil {
			continue
		}
		cleaned[key] = value
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func boolPtr(value bool) *bool {
	return &value
}

func newID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
