package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/policy"
	"github.com/nlink-jp/lagent/internal/session"
	"github.com/nlink-jp/lagent/internal/tools"
)

// Review round 4: a view_image / read_document call refused by a
// Review round 4: Restart (gem-agent ADR-0071 /clear) hands the agent a fresh
// transcript. A transcript that died in the previous session (a
// conversation write failed, gem-agent ADR-0021) must not keep the NEW one
// dead: the operator was told "recording stopped" once, about a file
// that no longer receives anything; the new file is a different file.
func TestRestartRevivesADeadTranscript(t *testing.T) {
	old := &recordingLog{fail: true}
	a := newLogAgent(t, old, func(string) {})
	a.AddContext("first") // conversation write fails → transcript dead
	if !a.logDead {
		t.Fatal("precondition: the old transcript should be dead")
	}
	fresh := &recordingLog{}
	a.Restart(fresh)
	a.AddContext("second")
	if len(fresh.kinds) != 1 || fresh.kinds[0] != session.KindMessage {
		t.Fatalf("the new transcript received %v, want one %q record", fresh.kinds, session.KindMessage)
	}
}

// Review round 4: data queued for the next turn belongs to the session
// that queued it. A piped attachment from the old session must not ride
// into the first turn of the session /clear started.
func TestRestartDropsPendingAttachments(t *testing.T) {
	a := newLogAgent(t, &recordingLog{}, nil)
	a.AttachData("-", "stdin", "old session's context")
	a.Restart(&recordingLog{})
	if n := len(a.pendingAtts); n != 0 {
		t.Fatalf("%d attachment(s) survived Restart", n)
	}
}

// Review round 4: a cancel that lands during the model-tier risk
// Review round 4: Restart drops the ended session's late-return notes
// beside the queued attachments.
func TestRestartDropsTheOldSessionsNotes(t *testing.T) {
	a := newLogAgent(t, &recordingLog{}, nil)
	a.lateNotices = []string{"note"}
	a.Restart(&recordingLog{})
	if len(a.lateNotices) != 0 {
		t.Fatalf("old-session state survived Restart: notes=%d", len(a.lateNotices))
	}
}

// Review round 4: the pre-tool hook payload carries the call as the
// Review after v0.68.0: an abandoned call that returns after /clear
// belongs to the session that made it — no note for the new
// conversation, no tool_late_return in the new transcript.
func TestLateReturnAfterRestartStaysWithTheOldSession(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	writer := &tools.Tool{
		Name: "slow_write", Description: "writes, slowly, ignoring ctx",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
		Mutating:   true,
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			started <- struct{}{}
			<-release
			return "written", nil
		},
	}
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "slow_write", Args: map[string]any{}}}},
		{Content: "done"},
	}}
	oldLog := &safeLog{}
	a := newFloorTestAgent(t, mb, writer, oldLog)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { _, _ = a.Run(ctx, "go", nil); close(done) }()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the turn never returned")
	}
	newLog := &safeLog{}
	a.Restart(newLog)
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for !oldLog.has("tool_late_return") {
		if time.Now().After(deadline) {
			t.Fatal("the old transcript never received the late-return record")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if newLog.has("tool_late_return") {
		t.Error("the new transcript received the old session's late-return record")
	}
	if notes := a.takeLateNotices(); len(notes) != 0 {
		t.Errorf("the new conversation received the old session's note: %v", notes)
	}
}

// allowlistGate answers from a session allowlist unless the call must
// prompt; it records which it did.
type allowlistGate struct{ prompted, allowlisted []string }

// A gate this test drives never faces the mode question; refusing
// keeps the ceiling where the test put it.
func (g *allowlistGate) ApproveLift(name, detail, purpose, reason string) (bool, string) {
	return false, ""
}

func (g *allowlistGate) Approve(name, detail, purpose, reason string, mustPrompt bool) (bool, bool, string) {
	if mustPrompt {
		g.prompted = append(g.prompted, name+" "+detail)
		return true, false, ""
	}
	g.allowlisted = append(g.allowlisted, name+" "+detail)
	return true, true, ""
}

// gem-agent ADR-0072 §4.5: OperatorOnly is a floor like Block — an earlier 'a'
// for write_file must not answer a write to AGENTS.md.
func TestOperatorOnlyIsNotAnsweredByTheAllowlist(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "write_file", Args: map[string]any{"path": "notes.md", "content": "x"}}}},
		{ToolCalls: []llm.ToolCall{{ID: "c2", Name: "write_file", Args: map[string]any{"path": "AGENTS.md", "content": "x"}}}},
		{Content: "done"},
	}}
	gate := &allowlistGate{}
	a, _ := newAgent(t, mb, gate, 5)
	if _, err := a.Run(context.Background(), "write", nil); err != nil {
		t.Fatal(err)
	}
	if len(gate.allowlisted) != 1 || !strings.Contains(gate.allowlisted[0], "notes.md") {
		t.Errorf("the ordinary write should have been the allowlist's: %v", gate.allowlisted)
	}
	if len(gate.prompted) != 1 || !strings.Contains(gate.prompted[0], "AGENTS.md") {
		t.Errorf("the instruction-file write must prompt: %v", gate.prompted)
	}
}

// gem-agent ADR-0072 §4.9: a `never` policy (and the one-shot --allow grant it
// stands for) lifts the ordinary gate, not the OperatorOnly floor —
// found live: `--allow write_file --auto` wrote AGENTS.md unattended.
func TestNeverPolicyKeepsTheOperatorOnlyFloor(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "write_file", Args: map[string]any{"path": "notes.md", "content": "x"}}}},
		{ToolCalls: []llm.ToolCall{{ID: "c2", Name: "write_file", Args: map[string]any{"path": "AGENTS.md", "content": "x"}}}},
		{Content: "done"},
	}}
	gate := &allowlistGate{}
	a, reg := newAgent(t, mb, gate, 5)
	pol, _, err := policy.Build(map[string]string{"write_file": "never"}, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	a.SetPolicy(pol)
	if _, err := a.Run(context.Background(), "write", nil); err != nil {
		t.Fatal(err)
	}
	if len(gate.allowlisted) != 0 {
		t.Errorf("the never policy consulted the allowlist: %v", gate.allowlisted)
	}
	if len(gate.prompted) != 1 || !strings.Contains(gate.prompted[0], "AGENTS.md") {
		t.Errorf("the instruction-file write must prompt under a never policy: %v", gate.prompted)
	}
	if _, err := os.Stat(filepath.Join(reg.ProjectDir(), "notes.md")); err != nil {
		t.Error("the ordinary write under a never policy did not run")
	}
}
