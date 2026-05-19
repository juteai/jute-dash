package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const dashboardContextExtensionURI = "https://jute.dev/a2a/extensions/dashboard-context/v1"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  sendParams      `json:"params"`
}

type sendParams struct {
	Message message `json:"message"`
}

type message struct {
	MessageID string         `json:"messageId,omitempty"`
	ContextID string         `json:"contextId,omitempty"`
	TaskID    string         `json:"taskId,omitempty"`
	Role      string         `json:"role,omitempty"`
	Parts     []part         `json:"parts,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type part struct {
	Text string `json:"text,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func main() {
	listen := strings.TrimSpace(os.Getenv("A2A_V1_DEV_LISTEN"))
	if listen == "" {
		listen = "127.0.0.1:9797"
	}
	baseURL := "http://" + listen

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"name":        "Jute A2A 1.0 Dev Agent",
			"description": "A tiny local A2A 1.0 fixture for testing Jute Dash chat and dashboard context.",
			"version":     "1.0.0",
			"supportedInterfaces": []map[string]string{
				{"url": baseURL + "/invoke", "protocolBinding": "JSONRPC", "protocolVersion": "1.0"},
			},
			"capabilities": map[string]any{
				"streaming": true,
				"extensions": []map[string]any{
					{"uri": dashboardContextExtensionURI, "description": "Receives redacted Jute dashboard context in message metadata."},
				},
			},
			"defaultInputModes":  []string{"text/plain"},
			"defaultOutputModes": []string{"text/plain"},
			"skills": []map[string]any{
				{"id": "chat", "name": "Local chat", "description": "Replies to short dashboard test prompts.", "tags": []string{"chat", "dev"}},
				{"id": "dashboard-context", "name": "Dashboard context", "description": "Reports whether Jute dashboard context was supplied.", "tags": []string{"jute", "context"}},
			},
		})
	})
	mux.HandleFunc("/invoke", handleInvoke)

	server := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("A2A 1.0 dev agent card: %s/.well-known/agent-card.json", baseURL)
	log.Printf("A2A 1.0 JSON-RPC endpoint: %s/invoke", baseURL)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if version := r.Header.Get("A2A-Version"); version != "1.0" {
		writeRPCError(w, nil, -32001, "A2A 1.0 is required")
		return
	}
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, nil, -32700, "invalid JSON")
		return
	}
	if req.Method != "SendMessage" && req.Method != "SendStreamingMessage" {
		writeRPCError(w, req.ID, -32601, "method not found")
		return
	}
	text := textFromParts(req.Params.Message.Parts)
	if text == "" {
		text = "empty prompt"
	}
	contextID := strings.TrimSpace(req.Params.Message.ContextID)
	if contextID == "" {
		contextID = "ctx-" + newID()
	}
	hasExtensionHeader := strings.Contains(r.Header.Get("A2A-Extensions"), dashboardContextExtensionURI)
	_, hasMetadata := req.Params.Message.Metadata[dashboardContextExtensionURI]
	contextStatus := "no dashboard context received"
	if hasExtensionHeader && hasMetadata {
		contextStatus = "dashboard context received"
	}
	answer := fmt.Sprintf("Dev A2A reply: %s. I saw %s.", text, contextStatus)
	if req.Method == "SendStreamingMessage" {
		writeStream(w, req.ID, contextID, answer)
		return
	}
	writeJSON(w, http.StatusOK, rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"message": message{
				MessageID: "msg-" + newID(),
				ContextID: contextID,
				Role:      "ROLE_AGENT",
				Parts:     []part{{Text: answer}},
			},
		},
	})
}

func writeStream(w http.ResponseWriter, id json.RawMessage, contextID, answer string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeRPCError(w, id, -32000, "streaming unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	taskID := "task-" + newID()
	writeSSE(w, id, map[string]any{
		"task": map[string]any{
			"id":        taskID,
			"contextId": contextID,
			"status":    map[string]any{"state": "working"},
		},
	})
	flusher.Flush()
	for _, chunk := range []string{answer[:min(len(answer), 22)], answer[min(len(answer), 22):]} {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		time.Sleep(250 * time.Millisecond)
		writeSSE(w, id, map[string]any{
			"artifactUpdate": map[string]any{
				"taskId":    taskID,
				"contextId": contextID,
				"append":    true,
				"artifact": map[string]any{
					"parts": []map[string]string{{"text": chunk}},
				},
			},
		})
		flusher.Flush()
	}
	time.Sleep(150 * time.Millisecond)
	writeSSE(w, id, map[string]any{
		"statusUpdate": map[string]any{
			"taskId":    taskID,
			"contextId": contextID,
			"final":     true,
			"status":    map[string]any{"state": "completed"},
		},
	})
	flusher.Flush()
}

func writeSSE(w http.ResponseWriter, id json.RawMessage, result any) {
	bytes, err := json.Marshal(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
}

func textFromParts(parts []part) string {
	chunks := []string{}
	for _, item := range parts {
		if text := strings.TrimSpace(item.Text); text != "" {
			chunks = append(chunks, text)
		}
	}
	return strings.Join(chunks, "\n\n")
}

func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	if len(id) == 0 {
		id = json.RawMessage(`null`)
	}
	writeJSON(w, http.StatusOK, rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func newID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
