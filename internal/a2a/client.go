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
	"strings"
	"time"
)

const (
	jsonRPCVersion     = "2.0"
	methodMessageSend  = "message/send"
	methodSendMessage  = "SendMessage"
	methodNotFoundCode = -32601
)

var (
	ErrUnsupportedProtocol = errors.New("a2a protocol binding is not implemented")
	ErrAgentRPCFailure     = errors.New("agent returned an a2a json-rpc error")
	ErrAgentTransport      = errors.New("agent transport request failed")
)

type SendMessageRequest struct {
	EndpointURL     string
	ProtocolBinding string
	Text            string
	BearerToken     string
}

type SendMessageResult struct {
	ConversationID string
	Status         string
	Text           string
}

type MessageSender interface {
	SendMessage(ctx context.Context, req SendMessageRequest) (SendMessageResult, error)
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

	result, err := c.send(ctx, req, methodMessageSend)
	var rpcErr *RPCError
	if errors.As(err, &rpcErr) && rpcErr.Code == methodNotFoundCode {
		return c.send(ctx, req, methodSendMessage)
	}
	return result, err
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

func (c *JSONRPCClient) send(ctx context.Context, req SendMessageRequest, method string) (SendMessageResult, error) {
	payload := jsonRPCRequest{
		JSONRPC: jsonRPCVersion,
		ID:      newID(),
		Method:  method,
		Params: sendParams{
			Message: message{
				Kind:      "message",
				MessageID: newID(),
				Role:      "user",
				Parts: []part{
					{Kind: "text", Text: req.Text},
				},
			},
			Configuration: sendConfiguration{
				Blocking:            true,
				AcceptedOutputModes: []string{"text/plain"},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return SendMessageResult{}, fmt.Errorf("encode a2a request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.EndpointURL, bytes.NewReader(body))
	if err != nil {
		return SendMessageResult{}, fmt.Errorf("build a2a request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("A2A-Version", "0.3")
	if strings.TrimSpace(req.BearerToken) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(req.BearerToken))
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

type jsonRPCRequest struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      string     `json:"id"`
	Method  string     `json:"method"`
	Params  sendParams `json:"params"`
}

type sendParams struct {
	Message       message           `json:"message"`
	Configuration sendConfiguration `json:"configuration"`
}

type sendConfiguration struct {
	Blocking            bool     `json:"blocking"`
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

type message struct {
	Kind      string `json:"kind,omitempty"`
	MessageID string `json:"messageId,omitempty"`
	Role      string `json:"role,omitempty"`
	Parts     []part `json:"parts,omitempty"`
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
		text := textFromMessage(msg)
		if text == "" {
			text = "Agent returned an empty message."
		}
		return SendMessageResult{
			ConversationID: fallbackID(msg.MessageID, "local-a2a"),
			Status:         "completed",
			Text:           text,
		}, nil
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

func resultFromTask(t task) SendMessageResult {
	status := strings.TrimSpace(t.Status.State)
	if status == "" {
		status = "unknown"
	}
	if text := textFromOptionalMessage(t.Status.Message); text != "" {
		return SendMessageResult{ConversationID: fallbackID(t.ContextID, t.ID, "local-a2a"), Status: status, Text: text}
	}
	for i := len(t.History) - 1; i >= 0; i-- {
		if t.History[i].Role != "agent" {
			continue
		}
		if text := textFromMessage(t.History[i]); text != "" {
			return SendMessageResult{ConversationID: fallbackID(t.ContextID, t.ID, "local-a2a"), Status: status, Text: text}
		}
	}
	for i := len(t.History) - 1; i >= 0; i-- {
		if text := textFromMessage(t.History[i]); text != "" {
			return SendMessageResult{ConversationID: fallbackID(t.ContextID, t.ID, "local-a2a"), Status: status, Text: text}
		}
	}
	for i := len(t.Artifacts) - 1; i >= 0; i-- {
		if text := textFromParts(t.Artifacts[i].Parts); text != "" {
			return SendMessageResult{ConversationID: fallbackID(t.ContextID, t.ID, "local-a2a"), Status: status, Text: text}
		}
	}
	return SendMessageResult{
		ConversationID: fallbackID(t.ContextID, t.ID, "local-a2a"),
		Status:         status,
		Text:           fmt.Sprintf("Agent task is %s.", status),
	}
}

func textFromOptionalMessage(msg *message) string {
	if msg == nil {
		return ""
	}
	return textFromMessage(*msg)
}

func textFromMessage(msg message) string {
	return textFromParts(msg.Parts)
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

func newID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
