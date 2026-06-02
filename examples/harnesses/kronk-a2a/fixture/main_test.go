package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKronkInstructionGuidesMCPWidgetSkillUse(t *testing.T) {
	instruction := kronkInstruction(true)

	required := []string{
		"Widget Skills",
		"jute_skill_list",
		"jute.weather.current",
		"jute_skill_read_context",
		"jute_skill_invoke_action",
		"jute.date_time.current",
		"jute.chat_history.current",
		"Never answer that you lack weather",
		"Return only the final user-facing answer",
	}
	for _, want := range required {
		if !strings.Contains(instruction, want) {
			t.Fatalf("instruction missing %q:\n%s", want, instruction)
		}
	}
}

func TestKronkInstructionExplainsMissingMCP(t *testing.T) {
	instruction := kronkInstruction(false)

	if strings.Contains(instruction, "jute.weather.current") {
		t.Fatalf("plain instruction should not reference MCP-only skill IDs:\n%s", instruction)
	}
	if !strings.Contains(instruction, "Jute MCP tools are not configured") {
		t.Fatalf("plain instruction should explain missing MCP:\n%s", instruction)
	}
}

func TestProbeJuteMCPSucceedsWhenJuteToolsAreAvailable(t *testing.T) {
	var sawAgentID bool
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAgentID = r.Header.Get("X-Jute-Agent-ID") == "kronk-local"
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"jute_skill_read_context"}]}}`))
	}))
	defer mcp.Close()

	if err := probeJuteMCP(t.Context(), mcp.URL, "", "kronk-local"); err != nil {
		t.Fatalf("probeJuteMCP() error = %v", err)
	}
	if !sawAgentID {
		t.Fatal("probe did not send X-Jute-Agent-ID")
	}
}

func TestProbeJuteMCPFailsWhenJuteToolsAreMissing(t *testing.T) {
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"not_jute"}]}}`))
	}))
	defer mcp.Close()

	err := probeJuteMCP(t.Context(), mcp.URL, "", "kronk-local")
	if err == nil || !strings.Contains(err.Error(), "jute_skill_read_context") {
		t.Fatalf("expected missing tool error, got %v", err)
	}
}

func TestProbeJuteMCPFailsOnRPCError(t *testing.T) {
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32001,"message":"unauthorized"}}`))
	}))
	defer mcp.Close()

	err := probeJuteMCP(t.Context(), mcp.URL, "", "")
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected RPC error, got %v", err)
	}
}
