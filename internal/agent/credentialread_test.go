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
	"github.com/nlink-jp/lagent/internal/policy"
	"github.com/nlink-jp/lagent/internal/risk"
	"github.com/nlink-jp/lagent/internal/tools"
)

// credentialProject writes a project with a .env, its template, a link
// named like notes that points at the .env, and an ordinary file, and
// returns a registry on it. The marker never leaves the fixture: a
// result that carries it is a leak.
func credentialProject(t *testing.T) *tools.Registry {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{
		".env":         "API_KEY=sk-test-marker-never-real\n",
		".env.example": "API_KEY=\n",
		"README.md":    "# readme\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(dir, ".env"), filepath.Join(dir, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	reg, err := tools.New(dir, func(ctx context.Context, c string) *exec.Cmd { return exec.CommandContext(ctx, "/bin/true") }, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func credCall(name string, args map[string]any) llm.ToolCall {
	return llm.ToolCall{ID: "c", Name: name, Args: args}
}

// ADR-0015 §1: a single-file read tool on credential material reaches
// the gate as a must-prompt question, judged on the real path (a link
// named notes.txt pointing at .env is a .env read); the template and
// an ordinary file never reach the gate; a denied read returns no
// content.
func TestCredentialReadIsMustPrompt(t *testing.T) {
	gate := &floorGate{}
	a := New(Options{Registry: credentialProject(t), Gate: gate})
	ctx := context.Background()
	steps := []struct {
		name  string
		args  map[string]any
		gated bool
	}{
		{"read_file", map[string]any{"path": ".env"}, true},
		{"read_file", map[string]any{"path": "notes.txt"}, true},
		{"file_info", map[string]any{"paths": []any{"README.md", ".env"}}, true},
		{"file_info", map[string]any{"paths": []any{"README.md", "notes.txt"}}, true}, // the link inside the batch is resolved too
		{"view_image", map[string]any{"path": ".env"}, true},
		{"read_file", map[string]any{"path": ".env.example"}, false},
		{"read_file", map[string]any{"path": "README.md"}, false},
		{"file_info", map[string]any{"path": "README.md"}, false},
	}
	for _, s := range steps {
		before := len(gate.calls)
		out, denied, _, _ := a.execCall(ctx, credCall(s.name, s.args))
		if s.gated {
			if len(gate.calls) != before+1 || !gate.calls[before] {
				t.Errorf("%s %v: gate calls %v, want one more with mustPrompt=true", s.name, s.args, gate.calls)
			}
			if !denied || strings.Contains(out, "sk-test-marker") {
				t.Errorf("%s %v: denied=%v out=%q — a denied read must return no content", s.name, s.args, denied, out)
			}
			continue
		}
		if len(gate.calls) != before || denied {
			t.Errorf("%s %v reached the gate (calls %v, denied=%v)", s.name, s.args, gate.calls, denied)
		}
	}
}

// ADR-0015 §1: no standing answer reads a credential file. Unattended
// (-p) the deny gate answers and the content never returns; a "never"
// policy — what --allow read_file becomes for one run — skips the gate
// for an ordinary read and still asks for .env, with mustPrompt.
func TestCredentialReadIsDeniedUnattendedAndNotByNever(t *testing.T) {
	ctx := context.Background()
	deny := &denyAll{}
	a := New(Options{Registry: credentialProject(t), Gate: deny, Unattended: true})
	out, denied, _, _ := a.execCall(ctx, credCall("read_file", map[string]any{"path": ".env"}))
	if !denied || len(deny.asked) != 1 || strings.Contains(out, "sk-test-marker") || !strings.Contains(out, "unattended") {
		t.Errorf("unattended read_file .env: denied=%v asked=%v out=%q", denied, deny.asked, out)
	}
	out, denied, _, _ = a.execCall(ctx, credCall("read_file", map[string]any{"path": "README.md"}))
	if denied || len(deny.asked) != 1 || !strings.Contains(out, "# readme") {
		t.Errorf("unattended read_file README.md: denied=%v asked=%v out=%q", denied, deny.asked, out)
	}

	pol, _, err := policy.Build(map[string]string{"read_file": "never"}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	gate := &floorGate{}
	b := New(Options{Registry: credentialProject(t), Gate: gate, Policy: pol})
	if _, denied, _, _ := b.execCall(ctx, credCall("read_file", map[string]any{"path": "README.md"})); denied || len(gate.calls) != 0 {
		t.Errorf("never policy: an ordinary read was gated: denied=%v calls=%v", denied, gate.calls)
	}
	out, denied, _, _ = b.execCall(ctx, credCall("read_file", map[string]any{"path": ".env"}))
	if !denied || len(gate.calls) != 1 || !gate.calls[0] || strings.Contains(out, "sk-test-marker") {
		t.Errorf("never policy: read_file .env: denied=%v calls=%v out=%q — a never row must not answer a credential read", denied, gate.calls, out)
	}
}

// ADR-0015 §1 under --auto: the ladder escalates a credential read
// (Review, operator-only — never approved by the rule tier) and the
// gate is asked with mustPrompt; an ordinary read runs unasked.
func TestCredentialReadEscalatesUnderAuto(t *testing.T) {
	gate := &floorGate{}
	var autos []AutoDecision
	a := New(Options{Registry: credentialProject(t), Gate: gate, AutoApprove: true,
		OnAutoDecision: func(tc llm.ToolCall, d AutoDecision) { autos = append(autos, d) }})
	ctx := context.Background()
	out, denied, _, _ := a.execCall(ctx, credCall("read_file", map[string]any{"path": ".env"}))
	if !denied || strings.Contains(out, "sk-test-marker") {
		t.Errorf("--auto read_file .env: denied=%v out=%q", denied, out)
	}
	if len(autos) != 1 || autos[0].Approved || autos[0].Tier != risk.Review {
		t.Errorf("auto decisions = %+v, want one escalated Review", autos)
	}
	if len(gate.calls) != 1 || !gate.calls[0] {
		t.Errorf("gate calls = %v, want one with mustPrompt=true", gate.calls)
	}
	out, denied, _, _ = a.execCall(ctx, credCall("read_file", map[string]any{"path": "README.md"}))
	if denied || !strings.Contains(out, "# readme") || len(gate.calls) != 1 {
		t.Errorf("--auto read_file README.md: denied=%v out=%q gate=%v", denied, out, gate.calls)
	}
}

// An operator-approved credential read is operator-only but writes
// nothing: the pin hooks around operator writes stay quiet for it and
// still fire for an operator-only write.
func TestApprovedCredentialReadIsNotAnOperatorWrite(t *testing.T) {
	var before, after []string
	a := New(Options{Registry: credentialProject(t), Gate: &approveAll{},
		BeforeOperatorWrite: func(tc llm.ToolCall) { before = append(before, tc.Name) },
		OnOperatorWrite:     func(tc llm.ToolCall) { after = append(after, tc.Name) }})
	ctx := context.Background()
	out, denied, _, _ := a.execCall(ctx, credCall("read_file", map[string]any{"path": ".env"}))
	if denied || !strings.Contains(out, "sk-test-marker") {
		t.Fatalf("approved read: denied=%v out=%q", denied, out)
	}
	if len(before) != 0 || len(after) != 0 {
		t.Errorf("an approved read fired the operator-write hooks: %v %v", before, after)
	}
	if _, denied, _, _ := a.execCall(ctx, credCall("write_file", map[string]any{"path": "AGENTS.md", "content": "# agents\n"})); denied {
		t.Fatal("approved operator-only write was denied")
	}
	if len(before) != 1 || before[0] != "write_file" || len(after) != 1 {
		t.Errorf("operator-write hooks after an operator-only write: before=%v after=%v", before, after)
	}
}

// ADR-0016 §2: the kernel is the boundary and the matcher is only the
// prompt-raiser. When the matcher misses, the child's refusal produces
// no bytes, the operator gets the same question, and on a yes the read
// runs in process. When the matcher hits, the approved call must not go
// to the child at all: the cage would refuse what the operator allowed.
func TestKernelRefusalBecomesTheOperatorsQuestion(t *testing.T) {
	ctx := context.Background()
	reg := credentialProject(t)
	var childCalls int
	reg.SetFileChild(func(context.Context, string, map[string]any) (string, error) {
		childCalls++
		return "", tools.ErrCredentialRead
	})
	gate := &floorGate{}
	a := New(Options{Registry: reg, Gate: gate})
	// README.md is ordinary: only the kernel's answer can raise a gate.
	out, denied, _, _ := a.execCall(ctx, credCall("read_file", map[string]any{"path": "README.md"}))
	if childCalls != 1 {
		t.Errorf("the read did not go through the child: %d calls", childCalls)
	}
	if len(gate.calls) != 1 || !gate.calls[0] {
		t.Errorf("the kernel's refusal must reach the operator as a must-prompt: %v", gate.calls)
	}
	if !denied || strings.Contains(out, "readme") {
		t.Errorf("a refused-and-declined read returned content: denied=%v out=%q", denied, out)
	}

	// The approved credential read bypasses the cage entirely.
	reg2 := credentialProject(t)
	reg2.SetFileChild(func(context.Context, string, map[string]any) (string, error) {
		t.Error("an approved credential read went to the cage that would refuse it")
		return "", tools.ErrCredentialRead
	})
	approve := &approveAll{}
	a2 := New(Options{Registry: reg2, Gate: approve})
	out, denied, _, _ = a2.execCall(ctx, credCall("read_file", map[string]any{"path": ".env"}))
	if denied || !strings.Contains(out, "sk-test-marker") {
		t.Errorf("the approved read did not run in process: denied=%v out=%q", denied, out)
	}
}
