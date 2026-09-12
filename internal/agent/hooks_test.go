package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/tools"
)

func hookAgent(t *testing.T, mb *mockBackend, gate Approver,
	hook func(context.Context, string, map[string]any) (bool, string)) (*Agent, *tools.Registry) {
	t.Helper()
	reg, err := tools.New(t.TempDir(),
		func(ctx context.Context, command string) *exec.Cmd {
			return exec.CommandContext(ctx, "/bin/bash", "-c", command)
		}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return New(Options{
		Backend: mb, Registry: reg, Gate: gate,
		System: "test system", MaxTurns: 5, PreToolHook: hook,
	}), reg
}

// A hook deny is a deterministic floor (ADR-0012 §2): the tool never
// runs, the approval gate is never consulted — a deny is not a
// question — and the model receives the reason as the tool result,
// wrapped as data like any result.
func TestPreToolHookDenyIsAFloor(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "write_file",
			Args: map[string]any{"path": "x.txt", "content": "data"}}}},
		{Content: "understood"},
	}}
	gate := &approveAll{}
	var saw string
	log := &dataLog{}
	a, reg := hookAgent(t, mb, gate, func(_ context.Context, name string, _ map[string]any) (bool, string) {
		saw = name
		return true, "the org guard said no"
	})
	a.log = log
	if _, err := a.Run(context.Background(), "write x.txt", nil); err != nil {
		t.Fatal(err)
	}
	if saw != "write_file" {
		t.Fatalf("hook saw %q", saw)
	}
	if _, err := os.Stat(filepath.Join(reg.ProjectDir(), "x.txt")); err == nil {
		t.Error("denied tool ran anyway")
	}
	if len(gate.asked) != 0 {
		t.Errorf("approval gate consulted for a hook-denied call: %v", gate.asked)
	}
	toolMsg := mb.calls[1][2]
	if !strings.Contains(toolMsg.Content, "denied by a pre-tool hook") ||
		!strings.Contains(toolMsg.Content, "the org guard said no") {
		t.Errorf("the model was not told the reason: %q", toolMsg.Content)
	}
	if !strings.Contains(toolMsg.Content, "<"+a.tag.Name()+">") {
		t.Errorf("a hook's words must ship wrapped as data: %q", toolMsg.Content)
	}
	found := false
	for _, r := range log.records {
		if data, ok := r.data.(map[string]any); ok && r.kind == "hook_denied" && data["reason"] == "the org guard said no" {
			found = true
		}
	}
	if !found {
		t.Errorf("no hook_denied record with the reason: %+v", log.records)
	}
}

// A pass-through hook changes nothing: the call still goes through the
// normal ladder and runs (hooks only ever tighten, ADR-0012 §3).
func TestPreToolHookPassThroughStillGates(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "write_file",
			Args: map[string]any{"path": "y.txt", "content": "ok"}}}},
		{Content: "done"},
	}}
	gate := &approveAll{}
	a, reg := hookAgent(t, mb, gate, func(context.Context, string, map[string]any) (bool, string) {
		return false, ""
	})
	if _, err := a.Run(context.Background(), "write y.txt", nil); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(reg.ProjectDir(), "y.txt")); err != nil || string(data) != "ok" {
		t.Errorf("tool did not run after pass-through: %v %q", err, data)
	}
	if len(gate.asked) != 1 {
		t.Errorf("gate consulted %d times, want 1", len(gate.asked))
	}
}

// A cancel that lands while a pre-tool hook runs must not carry the
// call on to the gate.
func TestCancelDuringHookSkipsTheGate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c", Name: "write_file", Args: map[string]any{"path": "x.txt", "content": "y"}}}},
		{Content: "done"},
	}}
	gate := &approveAll{}
	a, reg := hookAgent(t, mb, gate, func(context.Context, string, map[string]any) (bool, string) {
		cancel()
		return false, ""
	})
	_, _ = a.Run(ctx, "write", nil)
	if len(gate.asked) != 0 {
		t.Fatalf("gate consulted after the interrupt: %v", gate.asked)
	}
	if _, err := os.Stat(filepath.Join(reg.ProjectDir(), "x.txt")); err == nil {
		t.Fatal("the tool ran after the interrupt")
	}
}

// The hook judges the call, not the proposer's justification: the
// declared purpose is stripped from what the hook sees.
func TestPreToolHookSeesNoPurpose(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "shell_exec",
			Args: map[string]any{"command": "true", "access": "read", "lagent_purpose": "just checking"}}}},
		{Content: "done"},
	}}
	var seen map[string]any
	a, _ := hookAgent(t, mb, &approveAll{}, func(_ context.Context, _ string, args map[string]any) (bool, string) {
		seen = args
		return false, ""
	})
	if _, err := a.Run(context.Background(), "run", nil); err != nil {
		t.Fatal(err)
	}
	if seen == nil || seen["command"] != "true" || seen["access"] != "read" {
		t.Fatalf("hook saw %v", seen)
	}
	for k := range seen {
		if strings.Contains(k, "purpose") {
			t.Errorf("the purpose reached the hook: %v", seen)
		}
	}
}
