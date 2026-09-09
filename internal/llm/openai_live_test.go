//go:build live

package llm

import (
	"context"
	"os"
	"strings"
	"testing"
)

// Measures the backend against a running local server: a text turn
// streams, a tool round trip pairs by id, and usage arrives on the
// stream. Run with the server up:
//
//	LAGENT_LIVE_BASE_URL=http://localhost:1234/v1 LAGENT_LIVE_MODEL=google/gemma-4-26b-a4b-qat \
//	  go test -tags live -run Live ./internal/llm/
func TestLiveToolRoundTrip(t *testing.T) {
	base := os.Getenv("LAGENT_LIVE_BASE_URL")
	model := os.Getenv("LAGENT_LIVE_MODEL")
	if base == "" || model == "" {
		t.Skip("LAGENT_LIVE_BASE_URL / LAGENT_LIVE_MODEL unset")
	}
	o, err := NewOpenAI(base, model, "", ProviderLMStudio)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if n, err := o.ContextWindow(ctx); err != nil || n <= 0 {
		t.Fatalf("ContextWindow: %d %v", n, err)
	}
	tools := []ToolDef{{Name: "read_file", Description: "Read a file", Parameters: map[string]any{
		"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}}}}
	hist := []Message{{Role: RoleUser, Content: "How many lines are in /etc/hosts? Read it first, then answer with just the number."}}
	first, err := o.ChatStream(ctx, "You are a coding agent. Use tools when needed.", hist, tools, nil)
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if len(first.ToolCalls) != 1 || first.ToolCalls[0].Name != "read_file" || first.FinishReason != "tool_calls" {
		t.Fatalf("turn 1 did not call read_file: %+v", first)
	}
	if first.PromptTokens == 0 || first.TotalTokens == 0 {
		t.Errorf("usage missing on the stream: %+v", first.Usage())
	}
	hist = append(hist, Message{Role: RoleAssistant, Content: first.Content, ToolCalls: first.ToolCalls})
	hist = append(hist, Message{Role: RoleTool, ToolName: "read_file", ToolCallID: first.ToolCalls[0].ID, Content: "127.0.0.1 localhost\n::1 localhost\n"})
	var streamed strings.Builder
	second, err := o.ChatStream(ctx, "You are a coding agent. Use tools when needed.", hist, tools, func(s string) { streamed.WriteString(s) })
	if err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	if !strings.Contains(second.Content, "2") || streamed.String() != second.Content {
		t.Fatalf("turn 2 content %q streamed %q", second.Content, streamed.String())
	}
	t.Logf("turn1 usage %+v, turn2 usage %+v", first.Usage(), second.Usage())
}
