package main

import (
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
