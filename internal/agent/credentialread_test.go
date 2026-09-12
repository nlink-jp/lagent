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
