package hooks

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func run(t *testing.T, h Hook, name string, args map[string]any) (bool, string, []string) {
	t.Helper()
	var notes []string
	r := New(Hooks{PreToolUse: []Hook{h}}, func(s string) { notes = append(notes, s) })
	deny, why := r.Pre(context.Background(), Session{ID: "sess-1", TranscriptPath: "/tmp/sess-1.jsonl", CWD: t.TempDir()}, name, args)
	return deny, why, notes
}

// The org's guard denies via stdout JSON with exit 0 — the measured
// contract, not the documented one (ADR-0012 context).
func TestDenyViaHookSpecificOutputJSON(t *testing.T) {
	h := Hook{Matcher: "shell_exec", Command: `echo '{"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"relative path"}}'`}
	deny, why, _ := run(t, h, "shell_exec", map[string]any{"command": "sed -i x y"})
	if !deny || !strings.Contains(why, "relative path") {
		t.Fatalf("deny=%v why=%q", deny, why)
	}
}

// The older top-level decision form also denies.
func TestDenyViaDecisionBlock(t *testing.T) {
	h := Hook{Matcher: "*", Command: `echo '{"decision":"block","reason":"nope"}'`}
	deny, why, _ := run(t, h, "write_file", nil)
	if !deny || why != "nope" {
		t.Fatalf("deny=%v why=%q", deny, why)
	}
}

// Exit code 2 denies with stderr as the reason.
func TestDenyViaExit2(t *testing.T) {
	h := Hook{Matcher: "shell_exec", Command: `echo "stop that" >&2; exit 2`}
	deny, why, _ := run(t, h, "shell_exec", nil)
	if !deny || why != "stop that" {
		t.Fatalf("deny=%v why=%q", deny, why)
	}
}

// Matchers speak both vocabularies: a hooks block copied from Claude
// Code settings ("Bash") matches lagent's shell_exec (ADR-0012 §4).
func TestMatcherAcceptsClaudeCodeNames(t *testing.T) {
	h := Hook{Matcher: "Bash", Command: `echo '{"decision":"block","reason":"x"}'`}
	if deny, _, _ := run(t, h, "shell_exec", nil); !deny {
		t.Error("Bash matcher did not cover shell_exec")
	}
	if deny, _, _ := run(t, h, "list_files", nil); deny {
		t.Error("Bash matcher covered an unrelated tool")
	}
	alt := Hook{Matcher: "Write|Edit", Command: `echo '{"decision":"block","reason":"x"}'`}
	if deny, _, _ := run(t, alt, "edit_file", nil); !deny {
		t.Error("alternation did not cover edit_file")
	}
}

// Everything that is not an explicit denial fails open, with a notice
// for real failures: hooks only ever tighten (ADR-0012 §3).
func TestFailOpenWithNotice(t *testing.T) {
	for name, h := range map[string]Hook{
		"nonzero exit": {Matcher: "*", Command: `echo boom >&2; exit 1`},
		"missing tool": {Matcher: "*", Command: `/no/such/binary-xyz`},
	} {
		deny, _, notes := run(t, h, "shell_exec", nil)
		if deny {
			t.Errorf("%s: denied instead of failing open", name)
		}
		if len(notes) != 1 {
			t.Errorf("%s: expected one warning, got %v", name, notes)
		}
	}
	// Plain informational output is not a failure and not a verdict.
	deny, _, notes := run(t, Hook{Matcher: "*", Command: `echo checked`}, "shell_exec", nil)
	if deny || len(notes) != 0 {
		t.Errorf("informational output: deny=%v notes=%v", deny, notes)
	}
	// An explicit allow decision is pass-through, not a bypass.
	deny, _, _ = run(t, Hook{Matcher: "*", Command: `echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}'`}, "shell_exec", nil)
	if deny {
		t.Error("allow denied")
	}
}

// A timeout fails open with a notice; the turn is not stalled forever.
func TestTimeoutFailsOpen(t *testing.T) {
	h := Hook{Matcher: "*", Command: `sleep 5`, Timeout: 200 * time.Millisecond}
	start := time.Now()
	deny, _, notes := run(t, h, "shell_exec", nil)
	if deny {
		t.Error("timeout denied")
	}
	if time.Since(start) > 3*time.Second {
		t.Error("timeout did not bound the run")
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "timed out") {
		t.Errorf("notes = %v", notes)
	}
}

// The payload is the Claude Code shape: the installed org guard reads
// tool_input.command from it verbatim, and a per-session hook reads
// session_id (ADR-0012 §5). shell_exec's lane rides in tool_input too.
func TestPayloadShape(t *testing.T) {
	h := Hook{Matcher: "*", Command: `python3 -c "
import json,sys
p=json.load(sys.stdin)
assert p['hook_event_name']=='PreToolUse', p
assert p['session_id']=='sess-1', p
assert p['transcript_path']=='/tmp/sess-1.jsonl', p
assert p['tool_name']=='shell_exec', p
assert p['tool_input']['command']=='gofmt -w .', p
assert p['tool_input']['access']=='write', p
assert p['cwd'], p
print(json.dumps({'decision':'block','reason':'shape ok'}))
"`}
	deny, why, notes := run(t, h, "shell_exec", map[string]any{"command": "gofmt -w .", "access": "write"})
	if !deny || why != "shape ok" {
		t.Fatalf("deny=%v why=%q notes=%v", deny, why, notes)
	}
}

// First denial wins; later hooks do not run after a deny.
func TestFirstDenyWins(t *testing.T) {
	r := New(Hooks{PreToolUse: []Hook{
		{Matcher: "*", Command: `echo '{"decision":"block","reason":"first"}'`},
		{Matcher: "*", Command: `echo '{"decision":"block","reason":"second"}'`},
	}}, nil)
	deny, why := r.Pre(context.Background(), Session{CWD: t.TempDir()}, "shell_exec", nil)
	if !deny || why != "first" {
		t.Fatalf("deny=%v why=%q", deny, why)
	}
	if New(Hooks{}, nil).HasPreToolUse() || !r.HasPreToolUse() {
		t.Error("HasPreToolUse misreports")
	}
}

// A hook whose background child keeps stdout open must not hold the
// session past the hook's own exit — WaitDelay bounds the wait for the
// inherited pipe.
func TestGrandchildHoldingStdoutDoesNotStallTheHook(t *testing.T) {
	h := Hook{Matcher: "*", Command: `sleep 20 & echo done`, Timeout: 500 * time.Millisecond}
	start := time.Now()
	deny, _, _ := run(t, h, "shell_exec", nil)
	if deny {
		t.Error("the hook denied")
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("the hook took %s — the grandchild's pipe held it", took)
	}
}

// The timeout kills the hook's process group, so a child the hook
// started does not survive it as an orphan.
func TestTimeoutKillsTheHooksChildren(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	h := Hook{Matcher: "*", Command: `sleep 30 & echo $! > ` + pidFile + `; wait`, Timeout: 300 * time.Millisecond}
	run(t, h, "shell_exec", nil)
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("the hook did not record its child's pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("the hook's child (pid %d) outlived the timeout", pid)
	}
}

// A hook's output is bounded as it arrives.
func TestHookOutputIsBounded(t *testing.T) {
	h := Hook{Matcher: "*", Command: `head -c 5000000 /dev/zero | tr '\0' 'a'; echo`, Timeout: 5 * time.Second}
	r := New(Hooks{PreToolUse: []Hook{h}}, nil)
	out, ran := r.exec(context.Background(), h, t.TempDir(), map[string]any{"x": 1})
	if !ran {
		t.Fatal("hook did not run")
	}
	// The kept bytes plus the one-line note that says they were cut.
	if len(out.stdout) > hookStdoutCap+120 || !strings.Contains(out.stdout, "[hook output truncated: ") {
		t.Fatalf("stdout held %d bytes without a cut note; the cap is %d", len(out.stdout), hookStdoutCap)
	}
}
