package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/tools"
)

// --- ADR-0034: one round is bounded by the window ---

// writeProjectFile puts n bytes of text in the agent's project.
func writeProjectFile(t *testing.T, a *Agent, name string, n int) {
	t.Helper()
	line := strings.Repeat("d", 79) + "\n"
	body := strings.Repeat(line, n/len(line)+1)[:n]
	if err := os.WriteFile(filepath.Join(a.registry.ProjectDir(), name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readCalls(paths ...string) *llm.Response {
	var calls []llm.ToolCall
	for i, p := range paths {
		calls = append(calls, llm.ToolCall{ID: fmt.Sprintf("c%d", i), Name: "read_file", Args: map[string]any{"path": p}})
	}
	return &llm.Response{ToolCalls: calls}
}

// toolBytes is what the round's results put in the history.
func toolBytes(h []llm.Message) (n int) {
	for _, m := range h {
		if m.Role == llm.RoleTool {
			n += len(m.Content)
		}
	}
	return n
}

// The reported round, scaled to a test window: four whole-file reads in
// parallel, two past read_file's own cap. They come back as one budget
// — the first cut with their routes, the small last one whole — and the
// next request needs no compaction at all.
func TestTheReportedRoundComesBackWithinItsBudget(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{readCalls("a.csv", "b.csv", "c.csv", "d.csv"), {Content: "done"}}}
	a := compactingAgent(t, mb, 16_000, 5, nil)
	for name, n := range map[string]int{"a.csv": 68_053, "b.csv": 300_000, "c.csv": 300_000, "d.csv": 900} {
		writeProjectFile(t, a, name, n)
	}
	bound := a.tailBound(16_000) * allotBytesPerToken
	if _, err := a.Run(context.Background(), "analyse the data", nil); err != nil {
		t.Fatal(err)
	}
	if len(mb.calls) != 2 {
		t.Fatalf("%d requests: the round should have needed no compaction", len(mb.calls))
	}
	got := toolBytes(a.history)
	if got > bound+4*allotSlack {
		t.Errorf("the round put %d bytes in the history; its budget was %d", got, bound)
	}
	results := 0
	for _, m := range a.history {
		if m.Role != llm.RoleTool {
			continue
		}
		results++
		if m.ToolCallID == "c3" {
			if strings.Contains(m.Content, "truncated") {
				t.Errorf("the small last read was cut: %q", m.Content[len(m.Content)-80:])
			}
			continue
		}
		if !strings.Contains(m.Content, "reads on]") {
			t.Errorf("%s was cut without a route: %q", m.ToolCallID, m.Content[len(m.Content)-80:])
		}
	}
	if results != 4 {
		t.Fatalf("%d results, want 4", results)
	}
	if p := a.projectedPrompt(); p*100 >= 80*16_000 {
		t.Errorf("the next request is projected at %d of 16000 — past the threshold", p)
	}
}

// What a small result does not use passes to the calls after it.
func TestTheBudgetCarriesForwardWhatASmallResultLeaves(t *testing.T) {
	a := compactingAgent(t, &mockBackend{}, 16_000, 5, nil)
	b := roundBudget{window: 16_000}
	first := a.allot(&b, 2)
	b.spend("tiny", 0)
	if second := a.allot(&b, 1); second < first*19/10 {
		t.Errorf("the second of two calls got %d after the first used almost nothing of %d", second, first)
	}
}

// Re-judged before every call: an mcp_load earlier in the round grows
// the tool definitions, and the next allotment sees it.
func TestTheBudgetIsReJudgedBeforeEveryCall(t *testing.T) {
	a := compactingAgent(t, &mockBackend{}, 16_000, 5, nil)
	b := roundBudget{window: 16_000}
	before := a.allot(&b, 2)
	a.toolDefs = []llm.ToolDef{{Name: "mcp__big__schema", Description: strings.Repeat("s", 12_000)}}
	if after := a.allot(&b, 2); after >= before {
		t.Errorf("allotment %d after the tool definitions grew, %d before", after, before)
	}
}

// Every allotment has a floor, outside the budget: an edit's
// confirmation or an error arrives whole, and a byte window advances.
func TestAnAllotmentNeverFallsBelowItsFloor(t *testing.T) {
	a := compactingAgent(t, &mockBackend{}, 16_000, 5, nil)
	b := roundBudget{window: 16_000, used: 1 << 30}
	if n := a.allot(&b, 3); n != minAllot {
		t.Errorf("allotment with the budget spent = %d, want the %d floor", n, minAllot)
	}
}

// With no compaction able to run, the round stays under the threshold on
// its own; when there is no room left at all, the turn ends after the
// round with the reason instead of letting the model call into the wall.
func TestAWindowWithNoRoomEndsTheTurn(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{readCalls("a.csv"), {Content: "never asked"}}}
	a := compactingAgent(t, mb, 16_000, 5, func(o *Options) { o.AutoCompact = false })
	writeProjectFile(t, a, "a.csv", 100_000)
	a.SetHistory([]llm.Message{user(strings.Repeat("h", 13_000*bytesPerToken)), assistant("ok")})
	_, err := a.Run(context.Background(), "read it", nil)
	if !errors.Is(err, errWindowFull) {
		t.Fatalf("err = %v, want the window-full stop", err)
	}
	if len(mb.calls) != 1 {
		t.Errorf("%d requests: the turn went on past a full window", len(mb.calls))
	}
	last := a.history[len(a.history)-1]
	if last.Role != llm.RoleTool || len(last.Content) > minAllot+allotSlack {
		t.Errorf("the round's result is %s with %d bytes", last.Role, len(last.Content))
	}
}

func TestWithoutCompactionTheRoundStaysUnderTheThreshold(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{readCalls("a.csv", "b.csv"), {Content: "done"}}}
	a := compactingAgent(t, mb, 16_000, 5, func(o *Options) { o.AutoCompact = false })
	writeProjectFile(t, a, "a.csv", 100_000)
	writeProjectFile(t, a, "b.csv", 100_000)
	a.SetHistory([]llm.Message{user(strings.Repeat("h", 9_000*bytesPerToken)), assistant("ok")})
	if _, err := a.Run(context.Background(), "read them", nil); err != nil {
		t.Fatal(err)
	}
	if p := a.projectedPrompt(); p*100 > 80*16_000+2*minAllot {
		t.Errorf("the request after the round is projected at %d of 16000, past the threshold", p)
	}
}

// A tool that does not apply its allotment is cut by the runtime, and the
// marker says the rest was not kept. An instruction file is never cut.
func TestTheBackstopCutsAToolThatIgnoresItsAllotment(t *testing.T) {
	huge := strings.Repeat("z", 60_000)
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "x", Name: "mcp__s__dump"}, {ID: "y", Name: "load_skill"}}},
		{Content: "done"},
	}}
	a := compactingAgent(t, mb, 16_000, 5, func(o *Options) { o.InstructionTools = []string{"load_skill"} })
	for _, name := range []string{"mcp__s__dump", "load_skill"} {
		if err := a.registry.Register(&tools.Tool{Name: name, Description: "d",
			Run: func(context.Context, map[string]any) (string, error) { return huge, nil }}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Run(context.Background(), "go", nil); err != nil {
		t.Fatal(err)
	}
	for _, m := range a.history {
		switch m.ToolCallID {
		case "x":
			if len(m.Content) > a.tailBound(16_000)*allotBytesPerToken+allotSlack+200 || !strings.HasSuffix(m.Content, "the rest was not kept]") {
				t.Errorf("the ignoring tool's result: %d bytes, ends %q", len(m.Content), m.Content[len(m.Content)-60:])
			}
		case "y":
			if m.Content != huge {
				t.Errorf("the instruction file was cut to %d bytes", len(m.Content))
			}
		}
	}
}

// The first round waits for the window lookup even with automatic
// compaction off: the budget judges against the window too (§7).
func TestTheFirstRoundWaitsForTheWindowWithoutAutoCompaction(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{readCalls("a.csv"), {Content: "done"}}}
	_, reg := newAgent(t, mb, &approveAll{}, 5)
	a := New(Options{Backend: mb, Registry: reg, Gate: &approveAll{}, System: "s", MaxTurns: 5,
		CompactAtPct: 80, WindowLookup: true})
	a.toolDefs = nil
	writeProjectFile(t, a, "a.csv", 100_000)
	go func() {
		time.Sleep(50 * time.Millisecond)
		a.SetContextWindow(16_000)
	}()
	if _, err := a.Run(context.Background(), "read it", nil); err != nil {
		t.Fatal(err)
	}
	if got := toolBytes(a.history); got > a.tailBound(16_000)*allotBytesPerToken+allotSlack {
		t.Errorf("the first round returned %d bytes: it did not wait for the window", got)
	}
}

// @-attachments share the bound (§5).
func TestAttachmentsShareTheBound(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "done"}}}
	a := compactingAgent(t, mb, 16_000, 5, nil)
	writeProjectFile(t, a, "big.txt", 60_000)
	if _, err := a.Run(context.Background(), "look at @big.txt", nil); err != nil {
		t.Fatal(err)
	}
	bound := a.tailBound(16_000) * allotBytesPerToken
	total := 0
	for _, att := range a.history[0].Attachments {
		total += len(att.Content)
	}
	if total == 0 || total > bound+200 {
		t.Errorf("attachments carry %d bytes; the bound is %d", total, bound)
	}
}

// --- §4: the retained tail is bounded by size ---

func TestTheRetainedTailIsBoundedBySize(t *testing.T) {
	huge := strings.Repeat("r", 30_000)
	// Inside a tool loop: the tail shrinks to the last round, never past
	// its assistant turn.
	loop := append(tenMessages(), toolCall("read_file"), toolResult("read_file", huge),
		toolCall("read_file"), toolResult("read_file", huge))
	if cut := compactCut(loop, keepRecent, 12_000); cut != len(loop)-2 {
		t.Errorf("inside a tool loop the cut is %d, want the last round's assistant turn at %d", cut, len(loop)-2)
	}
	// At the start of a turn: the operator's message alone.
	turn := append(append([]llm.Message{}, loop...), user("next"))
	if cut := compactCut(turn, keepRecent, 100); cut != len(turn)-1 {
		t.Errorf("at a turn's start the cut is %d, want the operator's message at %d", cut, len(turn)-1)
	}
	// A tail within the bound is the counted one, as before.
	if cut, old := compactCut(tenMessages(), keepRecent, 1<<20), compactCut(tenMessages(), keepRecent, 0); cut != old {
		t.Errorf("a small tail moved the cut: %d, was %d", cut, old)
	}
	// A short history can still hold an oversized round.
	short := []llm.Message{user("1"), assistant("2"), user("3"), toolCall("read_file"), toolResult("read_file", huge), user("go on")}
	if cut := compactCut(short, keepRecent, 100); cut != 5 {
		t.Errorf("a short history with an oversized round cut at %d, want 5", cut)
	}
}

// The session the report came from: an oversized round the count-based
// tail kept. Resumed — or once the operator writes again — the tail is
// that message, and the round is summarised.
func TestAStuckSessionIsRescuedOnceTheOperatorWritesAgain(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "SUMMARY of the reads"}, {Content: "done"}}}
	a := compactingAgent(t, mb, 16_000, 5, nil)
	huge := strings.Repeat("q", 40_000)
	stuck := []llm.Message{user("analyse"),
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "read_file"}, {ID: "2", Name: "read_file"}}},
		{Role: llm.RoleTool, ToolName: "read_file", ToolCallID: "1", Content: huge},
		{Role: llm.RoleTool, ToolName: "read_file", ToolCallID: "2", Content: huge},
		user("why did it stop?"),
	}
	a.SetHistory(stuck)
	if _, err := a.Run(context.Background(), "continue", nil); err != nil {
		t.Fatal(err)
	}
	if len(mb.calls) != 2 {
		t.Fatalf("%d requests, want the summariser and the turn", len(mb.calls))
	}
	if toolBytes(mb.calls[1]) != 0 {
		t.Errorf("the request after the rescue still carries %d bytes of tool results", toolBytes(mb.calls[1]))
	}
	if !isSummary(a.history[0]) {
		t.Errorf("the history does not begin with the summary: %+v", a.history[0].Role)
	}
}
