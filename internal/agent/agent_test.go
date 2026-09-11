package agent

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/nlk/guard"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/mention"
	"github.com/nlink-jp/lagent/internal/session"
	"github.com/nlink-jp/lagent/internal/tools"
)

// mockBackend replays scripted responses and archives every request.
type mockBackend struct {
	responses []*llm.Response
	calls     [][]llm.Message
	systems   []string
	toolDefs  [][]llm.ToolDef
}

func (m *mockBackend) ChatStream(ctx context.Context, system string, messages []llm.Message, defs []llm.ToolDef, onText func(string)) (*llm.Response, error) {
	m.calls = append(m.calls, append([]llm.Message(nil), messages...))
	m.systems = append(m.systems, system)
	m.toolDefs = append(m.toolDefs, defs)
	if len(m.responses) == 0 {
		return &llm.Response{Content: "(script exhausted)"}, nil
	}
	resp := m.responses[0]
	m.responses = m.responses[1:]
	if onText != nil && resp.Content != "" {
		onText(resp.Content)
	}
	return resp, nil
}

type approveAll struct {
	asked    []string
	purposes []string
}

func (a *approveAll) ApproveLift(name, detail, purpose, reason string) (bool, string) {
	return true, ""
}

func (a *approveAll) Approve(name, detail, purpose, reason string, mustPrompt bool) (bool, bool, string) {
	a.asked = append(a.asked, name+": "+detail)
	a.purposes = append(a.purposes, purpose)
	return true, false, ""
}

type denyAll struct{ asked []string }

func (d *denyAll) ApproveLift(name, detail, purpose, reason string) (bool, string) {
	return false, ""
}

func (d *denyAll) Approve(name, detail, purpose, reason string, mustPrompt bool) (bool, bool, string) {
	d.asked = append(d.asked, name+": "+detail)
	return false, false, ""
}

func newAgent(t *testing.T, backend llm.Backend, gate Approver, maxTurns int) (*Agent, *tools.Registry) {
	t.Helper()
	reg, err := tools.New(t.TempDir(),
		func(ctx context.Context, command string) *exec.Cmd {
			return exec.CommandContext(ctx, "/bin/bash", "-c", command)
		}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return New(Options{
		Backend: backend, Registry: reg, Gate: gate,
		System: "test system", MaxTurns: maxTurns,
	}), reg
}

func TestPlainAnswer(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "こんにちは"}}}
	a, _ := newAgent(t, mb, &approveAll{}, 5)

	var streamed bytes.Buffer
	out, err := a.Run(context.Background(), "hi", func(s string) { streamed.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if out != "こんにちは" || streamed.String() != "こんにちは" {
		t.Errorf("out=%q streamed=%q", out, streamed.String())
	}
	if a.HistoryLen() != 2 {
		t.Errorf("history = %d, want user+assistant", a.HistoryLen())
	}
	if mb.systems[0] != "test system" {
		t.Error("system prompt not passed through")
	}
	if len(mb.toolDefs[0]) != 9 { // the built-ins; ask_user, mcp_load and MCP tools register in cmd
		t.Errorf("tool defs = %d, want 9 built-ins", len(mb.toolDefs[0]))
	}
}

func TestToolCallLoopPreservesIDs(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{
			ToolCalls: []llm.ToolCall{{
				ID: "c1", Name: "write_file",
				Args: map[string]any{"path": "x.txt", "content": "data"},
			}},
		},
		{Content: "wrote it"},
	}}
	gate := &approveAll{}
	a, reg := newAgent(t, mb, gate, 5)

	out, err := a.Run(context.Background(), "write x.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "wrote it" {
		t.Errorf("out = %q", out)
	}

	// The tool really ran (approval + execution wired through).
	if data, err := os.ReadFile(filepath.Join(reg.ProjectDir(), "x.txt")); err != nil || string(data) != "data" {
		t.Errorf("tool did not run: %v %q", err, data)
	}
	if len(gate.asked) != 1 || !strings.Contains(gate.asked[0], "write_file") {
		t.Errorf("gate.asked = %v", gate.asked)
	}

	// Round 2 must see: user, assistant (with the call's id), tool result.
	second := mb.calls[1]
	if len(second) != 3 {
		t.Fatalf("round-2 history = %d messages", len(second))
	}
	asst := second[1]
	if len(asst.ToolCalls) != 1 || asst.ToolCalls[0].ID != "c1" {
		t.Error("tool-call id lost between rounds")
	}
	toolMsg := second[2]
	if toolMsg.Role != llm.RoleTool || toolMsg.ToolName != "write_file" || toolMsg.ToolCallID != "c1" {
		t.Errorf("tool message = %+v", toolMsg)
	}
	if !strings.Contains(toolMsg.Content, "wrote") {
		t.Errorf("tool result = %q", toolMsg.Content)
	}
}

func TestDeniedMutatingCall(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "write_file",
			Args: map[string]any{"path": "x.txt", "content": "data"}}}},
		{Content: "understood"},
	}}
	gate := &denyAll{}
	a, reg := newAgent(t, mb, gate, 5)

	if _, err := a.Run(context.Background(), "write x.txt", nil); err != nil {
		t.Fatal(err)
	}
	// Denied → file must not exist, and the model must see the denial.
	if _, err := os.Stat(filepath.Join(reg.ProjectDir(), "x.txt")); !os.IsNotExist(err) {
		t.Error("denied tool call still executed")
	}
	toolMsg := mb.calls[1][2]
	if !strings.Contains(toolMsg.Content, "denied") {
		t.Errorf("denial not surfaced to the model: %q", toolMsg.Content)
	}
}

// TestUnattendedDenialNamesTheRoute pins ADR-0008 §2: in a one-shot
// run a denial tells the model what it can still do, never to ask a
// user who is not there; the interactive text is unchanged.
func TestUnattendedDenialNamesTheRoute(t *testing.T) {
	call := llm.ToolCall{ID: "c1", Name: "write_file", Args: map[string]any{"path": "x.txt", "content": "data"}}
	for _, unattended := range []bool{false, true} {
		mb := &mockBackend{responses: []*llm.Response{{ToolCalls: []llm.ToolCall{call}}, {Content: "ok"}}}
		a, _ := newAgent(t, mb, &denyAll{}, 5)
		a.unattended = unattended
		if _, err := a.Run(context.Background(), "write x.txt", nil); err != nil {
			t.Fatal(err)
		}
		got := mb.calls[1][2].Content
		if unattended {
			if strings.Contains(got, "ask the user") || !strings.Contains(got, "unattended") || !strings.Contains(got, "read lane") {
				t.Errorf("unattended denial must name the route, not the user: %q", got)
			}
		} else if got != deniedResult {
			t.Errorf("interactive denial changed: %q", got)
		}
	}
}

// denyWithReasonGate denies every call with the operator's typed
// reason (gem-agent ADR-0060).
type denyWithReasonGate struct{ reason string }

func (d *denyWithReasonGate) ApproveLift(name, detail, purpose, reason string) (bool, string) {
	return false, d.reason
}

func (d *denyWithReasonGate) Approve(name, detail, purpose, reason string, mustPrompt bool) (bool, bool, string) {
	return false, false, d.reason
}

// gem-agent ADR-0060 §2/§3: the typed reason rides inside the denial function
// response, and the denial ships unwrapped — guidance the model may
// follow, not nonce-tagged data the system prompt forbids following.
func TestDeniedWithReasonReachesModelUnwrapped(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "write_file",
			Args: map[string]any{"path": "x.txt", "content": "data"}}}},
		{Content: "understood"},
	}}
	a, reg := newAgent(t, mb, &denyWithReasonGate{reason: "wrong file — put it in notes.md"}, 5)

	if _, err := a.Run(context.Background(), "write x.txt", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(reg.ProjectDir(), "x.txt")); !os.IsNotExist(err) {
		t.Error("denied tool call still executed")
	}
	toolMsg := mb.calls[1][2]
	if !strings.Contains(toolMsg.Content, "wrong file — put it in notes.md") {
		t.Errorf("operator reason not surfaced to the model: %q", toolMsg.Content)
	}
	// Unwrapped: the sent content starts with the denial text itself. A
	// wrapped result would open with the nonce tag instead.
	if !strings.HasPrefix(toolMsg.Content, "Tool execution denied by the user") {
		t.Errorf("reasoned denial reached the model wrapped: %q", toolMsg.Content)
	}
}

// gem-agent ADR-0060 §3: the wrap exemption keys on message provenance, never on
// content — a tool result that merely looks like a denial stays wrapped.
func TestWrapExemptsDenialByProvenanceOnly(t *testing.T) {
	tag := guard.NewTagWithPrefix("tool_output")
	history := []llm.Message{
		{Role: llm.RoleTool, ToolName: "write_file", Content: deniedWithReason("use notes.md"), Denial: true},
		{Role: llm.RoleTool, ToolName: "mcp__x__y", Content: deniedResult}, // forged shape, real tool output
	}
	out := wrapToolMessages(history, tag)
	if out[0].Content != history[0].Content {
		t.Errorf("gate denial was wrapped: %q", out[0].Content)
	}
	if out[1].Content == history[1].Content {
		t.Error("denial-shaped tool output shipped unwrapped — the exemption leaked from provenance to content")
	}
}

func TestReadOnlyToolSkipsApproval(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "list_files", Args: map[string]any{}}}},
		{Content: "done"},
	}}
	gate := &denyAll{} // would deny if asked
	a, _ := newAgent(t, mb, gate, 5)

	if _, err := a.Run(context.Background(), "list", nil); err != nil {
		t.Fatal(err)
	}
	if len(gate.asked) != 0 {
		t.Errorf("read-only tool should not prompt: %v", gate.asked)
	}
	if strings.Contains(mb.calls[1][2].Content, "denied") {
		t.Error("read-only tool was gated")
	}
}

func TestUnknownToolSurfacesError(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "no_such_tool", Args: map[string]any{}}}},
		{Content: "ok"},
	}}
	a, _ := newAgent(t, mb, &approveAll{}, 5)
	if _, err := a.Run(context.Background(), "x", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mb.calls[1][2].Content, "unknown tool") {
		t.Errorf("unknown tool not surfaced: %q", mb.calls[1][2].Content)
	}
}

func TestMaxTurnsCap(t *testing.T) {
	loop := &llm.Response{ToolCalls: []llm.ToolCall{{ID: "c", Name: "list_files", Args: map[string]any{}}}}
	mb := &mockBackend{responses: []*llm.Response{loop, loop, loop, loop, loop}}
	a, _ := newAgent(t, mb, &approveAll{}, 3)
	_, err := a.Run(context.Background(), "loop forever", nil)
	// gem-agent ADR-0040 §4: the stop message must teach recovery ("continue"),
	// and must NOT recommend /clear — the one action that destroys the
	// recoverable state.
	if err == nil || !strings.Contains(err.Error(), "round limit") ||
		!strings.Contains(err.Error(), "continue") || strings.Contains(err.Error(), "/clear") {
		t.Fatalf("expected recovery-teaching round-limit error, got %v", err)
	}
	if len(mb.calls) != 3 {
		t.Errorf("backend called %d times, want 3", len(mb.calls))
	}
}

// TestEmptyResponseDoesNotPoisonHistory is the field-reported bug: a
// model turn with neither text nor tool calls was stored, and every
// later request then carried an empty part — a 400 that repeated until
// the session was cleared. The turn must fail loudly and leave nothing
// behind, so the next message still works.
func TestEmptyResponseDoesNotPoisonHistory(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{}, {}, // empty twice: the one retry (ADR-0007) is spent
		{Content: "recovered"},
	}}
	a, _ := newAgent(t, mb, &approveAll{}, 5)

	_, err := a.Run(context.Background(), "first", nil)
	if err == nil {
		t.Fatal("a second empty response should be reported as an error")
	}
	if !strings.Contains(err.Error(), "no usable response") {
		t.Errorf("error should name the cause: %v", err)
	}
	for i, m := range a.history {
		if m.Role == llm.RoleAssistant && m.Content == "" && len(m.ToolCalls) == 0 {
			t.Fatalf("history[%d] stored an empty assistant turn", i)
		}
	}

	// The session stays usable.
	out, err := a.Run(context.Background(), "second", nil)
	if err != nil || out != "recovered" {
		t.Fatalf("next turn failed after an empty response: %q %v", out, err)
	}
}

// TestEmptyResponseIsAskedAgainOnce pins ADR-0007: an empty completion
// re-sends the identical request once, notes it, and records both
// attempts; the retry has the same history the failed request had.
func TestEmptyResponseIsAskedAgainOnce(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{FinishReason: "stop", OutputTokens: 4, ThoughtTokens: 1},
		{Content: "done"},
	}}
	var notices []string
	log := &capturingLog{}
	a := newLogAgent(t, log, func(m string) { notices = append(notices, m) })
	a.backend = mb
	a.maxTurns = 5

	out, err := a.Run(context.Background(), "fix it", nil)
	if err != nil || out != "done" {
		t.Fatalf("the retry should carry the turn: %q %v", out, err)
	}
	if len(mb.calls) != 2 || len(mb.calls[0]) != len(mb.calls[1]) {
		t.Fatalf("the re-sent request must be the failed one: %d calls, %d vs %d messages",
			len(mb.calls), len(mb.calls[0]), len(mb.calls[len(mb.calls)-1]))
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "empty") {
		t.Errorf("one notice expected: %v", notices)
	}
	var empties []map[string]any
	for i, kind := range log.kinds {
		if kind == "assistant_empty" {
			empties = append(empties, log.data[i].(map[string]any))
		}
	}
	if len(empties) != 1 || empties[0]["retried"] != true {
		t.Errorf("assistant_empty must record the retry: %v", empties)
	}
}

func TestResetClearsHistory(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "a"}, {Content: "b"}}}
	a, _ := newAgent(t, mb, &approveAll{}, 5)
	if _, err := a.Run(context.Background(), "one", nil); err != nil {
		t.Fatal(err)
	}
	a.Reset()
	if _, err := a.Run(context.Background(), "two", nil); err != nil {
		t.Fatal(err)
	}
	if len(mb.calls[1]) != 1 {
		t.Errorf("after Reset, second run should start fresh: %d messages", len(mb.calls[1]))
	}
}

// TestMentionAttachmentsAreNonceWrapped: the operator chose the file,
// but not what is inside it — attached content must reach the model as
// isolated data, exactly like tool output.
func TestMentionAttachmentsAreNonceWrapped(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "ok"}}}
	a, reg := newAgent(t, mb, &approveAll{}, 5)

	if err := os.WriteFile(filepath.Join(reg.ProjectDir(), "notes.md"), []byte("SECRET-MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	var attached []string
	var problems []string
	a.onAttach = func(atts []mention.Attachment, probs []mention.Problem) {
		for _, at := range atts {
			attached = append(attached, at.Ref)
		}
		for _, p := range probs {
			problems = append(problems, p.Ref+": "+p.Reason)
		}
	}

	if _, err := a.Run(context.Background(), "@notes.md と @missing.txt を見て", nil); err != nil {
		t.Fatal(err)
	}
	if len(attached) != 1 || attached[0] != "notes.md" {
		t.Errorf("attached = %v", attached)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "not found") {
		t.Errorf("problems = %v", problems)
	}

	sent := mb.calls[0][0].Content
	if !strings.Contains(sent, "SECRET-MARKER") {
		t.Fatal("attachment content never reached the model")
	}
	re := regexp.MustCompile(`<(tool_output_[0-9a-f]{32})>[^<]*SECRET-MARKER`)
	if !re.MatchString(sent) {
		t.Errorf("attachment not nonce-wrapped: %q", sent)
	}
	if !strings.Contains(sent, "@notes.md と @missing.txt を見て") {
		t.Error("the operator's own text must be sent unmodified")
	}
	// Stored history keeps the raw attachment (the tag is per-call).
	if a.history[0].Attachments[0].Content != "SECRET-MARKER" {
		t.Error("history should keep the raw attachment")
	}
}

func TestToolResultsNonceWrapped(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "list_files", Args: map[string]any{}}}},
		{Content: "done"},
	}}
	reg, err := tools.New(t.TempDir(),
		func(ctx context.Context, command string) *exec.Cmd {
			return exec.CommandContext(ctx, "/bin/bash", "-c", command)
		}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	a := New(Options{
		Backend: mb, Registry: reg, Gate: &approveAll{},
		System: "sys with tag <{{DATA_TAG}}>", MaxTurns: 5,
	})
	if _, err := a.Run(context.Background(), "list", nil); err != nil {
		t.Fatal(err)
	}

	// Round 2's tool message must be wrapped in this round's nonce tag,
	// and the same tag must appear expanded in the system prompt.
	re := regexp.MustCompile(`^<(tool_output_[0-9a-f]{32})>(?s).*</(tool_output_[0-9a-f]{32})>$`)
	toolMsg := mb.calls[1][2]
	m := re.FindStringSubmatch(toolMsg.Content)
	if m == nil {
		t.Fatalf("tool result not nonce-wrapped: %q", toolMsg.Content)
	}
	if m[1] != m[2] {
		t.Fatalf("open/close tags differ: %s vs %s", m[1], m[2])
	}
	if !strings.Contains(mb.systems[1], "<"+m[1]+">") {
		t.Error("system prompt does not reference the same turn tag")
	}
	if strings.Contains(mb.systems[1], "{{DATA_TAG}}") {
		t.Error("placeholder not expanded")
	}
	// gem-agent ADR-0018 inverted the per-call rule for the MAIN loop: the tag is
	// session-scoped so the request prefix stays byte-identical and
	// implicit caching can hit (guard.Wrap's collision refusal is what
	// makes reuse sound). Stability is pinned by
	// TestIsolationTagIsStableAcrossRoundsAndTurns.
	if mb.systems[0] != mb.systems[1] {
		t.Error("main-loop tag must be session-scoped (gem-agent ADR-0018)")
	}
}

// TestAddContextFeedsNextTurn: a !-shell note lands in history so the
// next backend call sees it before the new user input.
func TestAddContextFeedsNextTurn(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "ok"}}}
	a, _ := newAgent(t, mb, &approveAll{}, 5)

	a.AddContext("I ran this shell command myself:\n$ git status\n\nOutput:\nclean")
	if _, err := a.Run(context.Background(), "何か問題ある？", nil); err != nil {
		t.Fatal(err)
	}
	first := mb.calls[0]
	if len(first) != 2 {
		t.Fatalf("history = %d messages, want context note + user input", len(first))
	}
	if first[0].Role != llm.RoleUser || !strings.Contains(first[0].Content, "git status") {
		t.Errorf("context note missing or malformed: %+v", first[0])
	}
	if first[1].Content != "何か問題ある？" {
		t.Errorf("user input must follow the note: %+v", first[1])
	}
}

func TestOnUsageReportsEachRound(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{
			ToolCalls:    []llm.ToolCall{{ID: "c1", Name: "list_files", Args: map[string]any{}}},
			PromptTokens: 100, OutputTokens: 20,
		},
		{Content: "done", PromptTokens: 150, OutputTokens: 30},
	}}
	reg, err := tools.New(t.TempDir(),
		func(ctx context.Context, command string) *exec.Cmd {
			return exec.CommandContext(ctx, "/bin/bash", "-c", command)
		}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var got [][2]int
	a := New(Options{
		Backend: mb, Registry: reg, Gate: &approveAll{},
		System: "s", MaxTurns: 5,
		OnUsage: func(u llm.Usage) { got = append(got, [2]int{u.Prompt, u.Output}) },
	})
	if _, err := a.Run(context.Background(), "go", nil); err != nil {
		t.Fatal(err)
	}
	want := [][2]int{{100, 20}, {150, 30}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("usage reports = %v, want %v", got, want)
	}
}

func TestCallDetail(t *testing.T) {
	shell := llm.ToolCall{Name: "shell_exec", Args: map[string]any{"command": "make build"}}
	if got := CallDetail(shell); got != "make build" {
		t.Errorf("shell detail = %q", got)
	}
	write := llm.ToolCall{Name: "write_file", Args: map[string]any{"path": "a.txt", "content": strings.Repeat("x", 500)}}
	got := CallDetail(write)
	if !strings.Contains(got, "path=a.txt") || len(got) > 400 {
		t.Errorf("write detail = %q", got)
	}
}

// gem-agent ADR-0018: the isolation tag is session-scoped so the request prefix
// stays byte-identical across rounds AND turns — the shape implicit
// caching rewards. Reset rotates it.
func TestIsolationTagIsStableAcrossRoundsAndTurns(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c", Name: "list_files", Args: map[string]any{}}}},
		{Content: "turn one done"},
		{Content: "turn two done"},
	}}
	_, reg := newAgent(t, mb, &approveAll{}, 5)
	// The system prompt must carry the placeholder, or tag expansion is
	// a no-op and every assertion below passes vacuously.
	a := New(Options{Backend: mb, Registry: reg, Gate: &approveAll{},
		System: "sys <{{DATA_TAG}}>", MaxTurns: 5})
	if _, err := a.Run(context.Background(), "one", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Run(context.Background(), "two", nil); err != nil {
		t.Fatal(err)
	}
	if len(mb.systems) != 3 {
		t.Fatalf("%d calls", len(mb.systems))
	}
	// System instruction byte-identical across rounds and turns.
	if mb.systems[0] != mb.systems[1] || mb.systems[1] != mb.systems[2] {
		t.Error("system instruction changed between calls — the cache prefix is broken")
	}
	// The wrapped tool result is byte-identical when replayed.
	var wrapped []string
	for _, call := range mb.calls[1:] {
		for _, m := range call {
			if m.Role == llm.RoleTool {
				wrapped = append(wrapped, m.Content)
			}
		}
	}
	if len(wrapped) != 2 || wrapped[0] != wrapped[1] {
		t.Errorf("wrapped tool results differ across calls:\n%q\n%q", wrapped[0], wrapped[1])
	}

	// Reset rotates the tag: a fresh conversation, a fresh nonce.
	a.Reset()
	mb.responses = []*llm.Response{{Content: "after reset"}}
	if _, err := a.Run(context.Background(), "three", nil); err != nil {
		t.Fatal(err)
	}
	if mb.systems[3] == mb.systems[0] {
		t.Error("Reset did not rotate the isolation tag")
	}
}

// Cached tokens flow through the usage pipeline (gem-agent ADR-0018) — the
// measured answer to "is caching actually firing".
func TestUsageCarriesCachedTokens(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{Content: "hi", PromptTokens: 1000, OutputTokens: 20, CachedTokens: 800},
	}}
	_, reg := newAgent(t, mb, &approveAll{}, 5)
	log := &capturingLog{}
	var gotCached int
	a := New(Options{Backend: mb, Registry: reg, Gate: &approveAll{}, Log: log,
		System: "s", MaxTurns: 5,
		OnUsage: func(u llm.Usage) { gotCached = u.Cached }})
	if _, err := a.Run(context.Background(), "q", nil); err != nil {
		t.Fatal(err)
	}
	if gotCached != 800 {
		t.Errorf("OnUsage cached = %d", gotCached)
	}
	found := false
	for i, kind := range log.kinds {
		if kind == session.KindUsage {
			if r, ok := log.data[i].(session.UsageRecord); ok && r.Cached == 800 {
				found = true
			}
		}
	}
	if !found {
		t.Error("the usage record does not carry cached tokens")
	}
}

// gem-agent ADR-0019: side-calls (risk eval here) must accumulate into their own
// Review round 2: a tool call reached after the turn's context is
// cancelled must not open an approval prompt on behalf of a dead turn.
func TestExecCallRefusesAfterCancel(t *testing.T) {
	gate := &approveAll{}
	a, _ := newAgent(t, &mockBackend{}, gate, 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, _, _, _ := a.execCall(ctx, llm.ToolCall{Name: "write_file", Args: map[string]any{"path": "x", "content": "y"}})
	if !strings.HasPrefix(out, "error:") {
		t.Errorf("cancelled call ran: %q", out)
	}
	if len(gate.asked) != 0 {
		t.Errorf("gate prompted for a cancelled turn: %v", gate.asked)
	}
}

// The runtime's opening message names the untrusted-data tag and the
// session facts, unwrapped, as a user-role message the transcript keeps:
// the system prompt must stay byte-identical across sessions, so
// nothing per-session may live there.
func TestAnnounceSessionOpensTheConversation(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "read_file", Args: map[string]any{"path": "a.txt"}}}},
		{Content: "done"},
	}}
	a, reg := newAgent(t, mb, &approveAll{}, 5)
	if err := os.WriteFile(filepath.Join(reg.ProjectDir(), "a.txt"), []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	log := &capturingLog{}
	a.log = log
	a.AnnounceSession("- session work directory: /state/work/s1\n- session started: 2026-09-10")
	if a.HistoryLen() != 1 || a.history[0].Role != llm.RoleUser {
		t.Fatalf("history = %+v, want one user message", a.history)
	}
	opening := a.history[0].Content
	if !strings.HasPrefix(opening, session.FactsPrefix) {
		t.Errorf("opening message lacks the facts prefix: %q", opening)
	}
	if !strings.Contains(opening, a.tag.Name()) || !strings.Contains(opening, "/state/work/s1") {
		t.Errorf("opening message does not name the tag and the facts: %q", opening)
	}
	if len(log.kinds) != 1 || log.kinds[0] != session.KindMessage {
		t.Errorf("transcript records = %v, want one message", log.kinds)
	}
	if _, err := a.Run(context.Background(), "read a.txt", nil); err != nil {
		t.Fatal(err)
	}
	// On the wire the opening message rides as typed, and the tool
	// result that follows is wrapped in exactly the tag it announced.
	first := mb.calls[0][0]
	if first.Role != llm.RoleUser || first.Content != opening {
		t.Errorf("wire opening message = %+v", first)
	}
	last := mb.calls[1]
	tool := last[len(last)-1]
	if tool.Role != llm.RoleTool || !strings.Contains(tool.Content, "<"+a.tag.Name()+">") {
		t.Errorf("tool result is not wrapped in the announced tag: %q", tool.Content)
	}
}

// A registered tool the predicate hides is not declared, and a call to
// it is refused before any gate with the route to it; once the
// predicate admits it and the declarations are refreshed, it runs.
func TestAdvertiseHidesRegisteredToolsUntilRefreshed(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "mcp__srv__lookup", Args: map[string]any{}}}},
		{Content: "first"},
		{ToolCalls: []llm.ToolCall{{ID: "c2", Name: "mcp__srv__lookup", Args: map[string]any{}}}},
		{Content: "second"},
	}}
	reg, err := tools.New(t.TempDir(), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	if err := reg.Register(&tools.Tool{Name: "mcp__srv__lookup", Description: "d", Mutating: false,
		Parameters: map[string]any{"type": "object"},
		Run:        func(context.Context, map[string]any) (string, error) { ran++; return "RESULT", nil }}); err != nil {
		t.Fatal(err)
	}
	shown := false
	gate := &denyAll{}
	a := New(Options{Backend: mb, Registry: reg, Gate: gate, System: "s", MaxTurns: 5,
		Advertise: func(name string) bool { return shown || !strings.HasPrefix(name, "mcp__") }})
	for _, d := range mb.toolDefs {
		_ = d
	}
	if _, err := a.Run(context.Background(), "go", nil); err != nil {
		t.Fatal(err)
	}
	for _, d := range mb.toolDefs[0] {
		if d.Name == "mcp__srv__lookup" {
			t.Fatal("a hidden tool was declared to the model")
		}
	}
	if ran != 0 || len(gate.asked) != 0 {
		t.Fatalf("hidden tool ran (%d) or reached the gate (%v)", ran, gate.asked)
	}
	if res := a.history[2].Content; !strings.Contains(res, "not in your tool list") || !strings.Contains(res, "mcp_load") {
		t.Errorf("refusal does not name the route: %q", res)
	}
	shown = true
	a.RefreshTools()
	if _, err := a.Run(context.Background(), "again", nil); err != nil {
		t.Fatal(err)
	}
	declared := false
	for _, d := range mb.toolDefs[2] {
		if d.Name == "mcp__srv__lookup" {
			declared = true
		}
	}
	if !declared || ran != 1 {
		t.Errorf("after refresh: declared=%v ran=%d", declared, ran)
	}
}

// A reference the runtime could not attach is told to the model as
// well as to the operator (ADR-0005): the text still names the file,
// and a model not told the file is absent describes it from thin air.
func TestUnattachedReferenceIsToldToTheModel(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "ok"}}}
	a, _ := newAgent(t, mb, &approveAll{}, 5)
	var problems []mention.Problem
	a.onAttach = func(_ []mention.Attachment, p []mention.Problem) { problems = p }
	if _, err := a.Run(context.Background(), "what is in /nonexistent/dir/shot.png ?", nil); err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Fatalf("operator problems = %v", problems)
	}
	sent := mb.calls[0][len(mb.calls[0])-1]
	if !strings.Contains(sent.Content, "[not attached: /nonexistent/dir/shot.png") {
		t.Errorf("the model was not told: %q", sent.Content)
	}
	if len(sent.Attachments) != 0 {
		t.Errorf("a missing reference produced a wire attachment: %+v", sent.Attachments)
	}
	if !strings.Contains(a.history[len(a.history)-2].Content, "shot.png") {
		t.Error("the operator's text was rewritten")
	}
}
