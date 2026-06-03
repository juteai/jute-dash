package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const jsonRPCVersion = "2.0"

var (
	ErrNotConfigured = errors.New("jute mcp url is not configured")
	ErrRPCFailure    = errors.New("jute mcp rpc failure")
	ErrTransport     = errors.New("jute mcp transport failure")
)

type Config struct {
	URL         string
	BearerToken string
	AgentID     string
	Timeout     time.Duration
	HTTPClient  *http.Client
}

type Client struct {
	url         string
	bearerToken string
	agentID     string
	httpClient  *http.Client
}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

type Tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type Prompt struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type ToolCallResult struct {
	Content           []ToolContent   `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type PromptMessage struct {
	Role    string        `json:"role"`
	Content PromptContent `json:"content"`
}

type PromptContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type PromptResult struct {
	Description string          `json:"description"`
	Messages    []PromptMessage `json:"messages"`
}

func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, ErrNotConfigured
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{
		url:         strings.TrimSpace(cfg.URL),
		bearerToken: strings.TrimSpace(cfg.BearerToken),
		agentID:     strings.TrimSpace(cfg.AgentID),
		httpClient:  httpClient,
	}, nil
}

func (c *Client) Initialize(ctx context.Context) error {
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"clientInfo": map[string]any{
			"name":    "jute-dev-agent",
			"version": "dev",
		},
	}, &result); err != nil {
		return err
	}
	return nil
}

func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	var result struct {
		Resources []Resource `json:"resources"`
	}
	if err := c.call(ctx, "resources/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Resources, nil
}

func (c *Client) ReadResource(ctx context.Context, uri string) ([]ResourceContent, error) {
	var result struct {
		Contents []ResourceContent `json:"contents"`
	}
	if err := c.call(ctx, "resources/read", map[string]any{"uri": uri}, &result); err != nil {
		return nil, err
	}
	return result.Contents, nil
}

func (c *Client) ReadResourceText(ctx context.Context, uri string) (string, error) {
	contents, err := c.ReadResource(ctx, uri)
	if err != nil {
		return "", err
	}
	for _, content := range contents {
		if strings.TrimSpace(content.Text) != "" {
			return content.Text, nil
		}
	}
	return "", errors.New("mcp resource did not contain text")
}

func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (ToolCallResult, error) {
	var result ToolCallResult
	if arguments == nil {
		arguments = map[string]any{}
	}
	if err := c.call(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	}, &result); err != nil {
		return ToolCallResult{}, err
	}
	return result, nil
}

func (c *Client) ListPrompts(ctx context.Context) ([]Prompt, error) {
	var result struct {
		Prompts []Prompt `json:"prompts"`
	}
	if err := c.call(ctx, "prompts/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Prompts, nil
}

func (c *Client) GetPrompt(ctx context.Context, name string, arguments map[string]any) (PromptResult, error) {
	var result PromptResult
	if arguments == nil {
		arguments = map[string]any{}
	}
	if err := c.call(ctx, "prompts/get", map[string]any{
		"name":      name,
		"arguments": arguments,
	}, &result); err != nil {
		return PromptResult{}, err
	}
	return result, nil
}

func (c *Client) call(ctx context.Context, method string, params any, target any) error {
	payload := rpcRequest{
		JSONRPC: jsonRPCVersion,
		ID:      strconv.FormatInt(time.Now().UnixNano(), 10),
		Method:  method,
		Params:  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode mcp request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build mcp request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	if c.agentID != "" {
		req.Header.Set("X-Jute-Agent-Id", c.agentID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w", ErrTransport)
	}
	defer resp.Body.Close()

	responseBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read mcp response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%w: status %d", ErrTransport, resp.StatusCode)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(responseBytes, &rpcResp); err != nil {
		return fmt.Errorf("decode mcp response: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("%w: code %d", ErrRPCFailure, rpcResp.Error.Code)
	}
	if len(rpcResp.Result) == 0 || target == nil {
		return nil
	}
	if err := json.Unmarshal(rpcResp.Result, target); err != nil {
		return fmt.Errorf("decode mcp result: %w", err)
	}
	return nil
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code int `json:"code"`
}
