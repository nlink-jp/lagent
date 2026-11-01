package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/session"
)

// Tests ported from gem-agent internal/agent/compact_test.go at c06187c
// (telemetry and the content-filter case dropped: lagent has neither),
// then lagent's own (ADR-0033 §2–§5 and the session facts).

func user(s string) llm.Message      { return llm.Message{Role: llm.RoleUser, Content: s} }
func assistant(s string) llm.Message { return llm.Message{Role: llm.RoleAssistant, Content: s} }
func toolCall(name string) llm.Message {
	return llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "c", Name: name}}}
}
func toolResult(name, s string) llm.Message {
	return llm.Message{Role: llm.RoleTool, ToolName: name, ToolCallID: "c", Content: s}
}

func tenMessages() []llm.Message {
	return []llm.Message{
		user("1"), assistant("2"), user("3"), assistant("4"), user("5"),
		assistant("6"), user("7"), assistant("8"), user("9"), assistant("10"),
	}
}

// listCall is a round that asks for one read-only tool call.
func listRound(prompt int) *llm.Response {
	return &llm.Response{ToolCalls: []llm.ToolCall{{ID: "c", Name: "list_files", Args: map[string]any{}}}, PromptTokens: prompt}
}

// compactingAgent is an auto-compacting agent over a real registry with
// a known window. The tool block is dropped from the estimate's view so
// the tests measure the history, not the registry's schema size.
func compactingAgent(t *testing.T, mb *mockBackend, window, maxTurns int, opts func(*Options)) *Agent {
	t.Helper()
	_, reg := newAgent(t, mb, &approveAll{}, maxTurns)
	o := Options{
		Backend: mb, Registry: reg, Gate: &approveAll{}, System: "s", MaxTurns: maxTurns,
		AutoCompact: true, CompactAtPct: 80,
	}
	if opts != nil {
		opts(&o)
	}
	a := New(o)
	a.toolDefs = nil
	a.SetContextWindow(window)
	return a
}

// The cut must never land on a tool result: a retained tail starting
// with a result whose call was summarised away is a refused request.
func TestCompactCutNeverLandsOnAToolResult(t *testing.T) {
	histories := map[string][]llm.Message{
		"tool round straddling the cut": {
			user("1"), assistant("2"), user("3"), toolCall("read_file"), toolResult("read_file", "x"),
			assistant("6"), user("7"), toolCall("shell_exec"), toolResult("shell_exec", "y"), assistant("10"),
		},
		"long tool loop with one early user message": {
			user("1"), toolCall("a"), toolResult("a", "x"), toolCall("b"), toolResult("b", "y"),
			toolCall("c"), toolResult("c", "z"), toolCall("d"), toolResult("d", "w"), assistant("done"),
		},
		"nothing but a short exchange": {user("1"), assistant("2")},
	}
	for name, h := range histories {
		t.Run(name, func(t *testing.T) {
			cut := compactCut(h, keepRecent)
			if cut == 0 {
				return // "no safe boundary" is a valid answer
			}
			if cut < 0 || cut >= len(h) {
				t.Fatalf("cut = %d out of range for %d messages", cut, len(h))
			}
			if h[cut].Role == llm.RoleTool {
				t.Errorf("cut at %d lands on a tool result", cut)
			}
		})
	}
}

// The case compaction exists for: one long agent loop, whose only user
// message is the first. A user-only boundary rule could never cut it.
func TestCompactCutHandlesALoopWithNoLaterUserMessage(t *testing.T) {
	h := []llm.Message{user("go")}
	for range 8 {
		h = append(h, toolCall("read_file"), toolResult("read_file", "x"))
	}
	cut := compactCut(h, keepRecent)
	if cut == 0 {
		t.Fatal("a long tool loop could not be compacted at all")
	}
	if h[cut].Role != llm.RoleAssistant {
		t.Errorf("cut lands on %q, want the assistant turn that owns the following results", h[cut].Role)
	}
}

func TestCompactReplacesHeadWithASummaryQuotedAsData(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "SUMMARY TEXT"}}}
	a, _ := newAgent(t, mb, &approveAll{}, 5)
	a.SetHistory([]llm.Message{
		user("first"), assistant("a1"), user("second"), toolCall("read_file"),
		toolResult("read_file", "file body"), assistant("a2"), user("third"),
		assistant("a3"), user("fourth"), assistant("a4"),
	})

	res, err := a.Compact(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Before != 10 || res.After != 10-res.Replaced+1 {
		t.Errorf("result = %+v", res)
	}
	if a.history[0].Role != llm.RoleUser || len(a.history[0].Attachments) != 1 {
		t.Fatalf("head message = %+v, want a user message carrying the summary as an attachment", a.history[0])
	}
	if a.history[0].Attachments[0].Content != "SUMMARY TEXT" {
		t.Errorf("summary = %q", a.history[0].Attachments[0].Content)
	}
	if a.history[len(a.history)-1].Content != "a4" {
		t.Errorf("tail lost: %+v", a.history)
	}

	// The summariser is offered no tools and sees the transcript as
	// isolated data, not as instructions.
	if len(mb.toolDefs[0]) != 0 {
		t.Errorf("summariser was offered %d tools", len(mb.toolDefs[0]))
	}
	sent := mb.calls[0][0].Content
	if !strings.Contains(sent, "file body") {
		t.Error("transcript did not reach the summariser")
	}
	if !strings.HasPrefix(strings.TrimSpace(sent), "<transcript") {
		t.Errorf("transcript was not nonce-wrapped: %.60q", sent)
	}
	if !strings.Contains(mb.systems[0], "UNTRUSTED DATA") {
		t.Error("summariser prompt lost its defensive framing")
	}

	// And on the next real request the summary itself is wrapped as data.
	mb.responses = []*llm.Response{{Content: "ok"}}
	if _, err := a.Run(context.Background(), "carry on", nil); err != nil {
		t.Fatal(err)
	}
	head := mb.calls[1][0].Content
	if !strings.Contains(head, "SUMMARY TEXT") {
		t.Fatal("summary did not reach the model")
	}
	if !strings.Contains(head, "<"+a.tag.Name()) {
		t.Errorf("summary was sent unwrapped: %q", head)
	}
}

// A compaction that half-worked would silently delete a conversation.
func TestCompactLeavesHistoryIntactWhenTheSummariserFails(t *testing.T) {
	for name, resp := range map[string]*llm.Response{
		"empty answer": {},
		// A summary cut by the output limit has lost its end (§5).
		"cut off by the output limit": {Content: "partial summ", FinishReason: "length"},
	} {
		t.Run(name, func(t *testing.T) {
			mb := &mockBackend{responses: []*llm.Response{resp}}
			a, _ := newAgent(t, mb, &approveAll{}, 5)
			before := tenMessages()
			a.SetHistory(append([]llm.Message(nil), before...))

			if _, err := a.Compact(context.Background()); err == nil {
				t.Fatal("a failed summarisation reported success")
			}
			if len(a.history) != len(before) {
				t.Fatalf("history = %d messages, want the original %d", len(a.history), len(before))
			}
			for i := range before {
				if a.history[i].Content != before[i].Content {
					t.Fatalf("history[%d] = %q, want %q", i, a.history[i].Content, before[i].Content)
				}
			}
		})
	}
}

// guard refuses content that names its own tag: a summary that copied
// the session's tag name would be withheld on every later request, after
// the history it replaced was gone. It is refused before the swap.
func TestCompactRefusesASummaryThatNamesTheSessionTag(t *testing.T) {
	mb := &mockBackend{}
	a, _ := newAgent(t, mb, &approveAll{}, 5)
	before := tenMessages()
	a.SetHistory(append([]llm.Message(nil), before...)) // rotates the tag
	mb.responses = []*llm.Response{{Content: "results arrived inside <" + a.tag.Name() + "> tags"}}
	if _, err := a.Compact(context.Background()); err == nil {
		t.Fatal("a summary carrying the session tag replaced the history")
	}
	if len(a.history) != len(before) {
		t.Errorf("history = %d messages, want the original %d", len(a.history), len(before))
	}
}

func TestCompactRefusesWhenThereIsNothingToCompact(t *testing.T) {
	mb := &mockBackend{}
	a, _ := newAgent(t, mb, &approveAll{}, 5)
	a.SetHistory([]llm.Message{user("1"), assistant("2")})
	if _, err := a.Compact(context.Background()); !errors.Is(err, ErrNothingToCompact) {
		t.Fatalf("err = %v, want ErrNothingToCompact", err)
	}
	if len(mb.calls) != 0 {
		t.Error("a pointless compaction still paid for an LLM call")
	}
}

// The runtime-facts message — the tag name, the work directory, memory —
// is the runtime's own words: the latest rides the summary message
// verbatim, unwrapped, and never reaches the summariser.
func TestCompactCarriesTheSessionFactsAcrossTheCut(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "SUMMARY"}, {Content: "ok"}}}
	a, _ := newAgent(t, mb, &approveAll{}, 5)
	a.SetHistory(tenMessages())
	// What a resume leaves: the earlier start's facts in the history,
	// and the fresh set AnnounceSession appends.
	a.history = append([]llm.Message{user(session.FactsPrefix + "\n- untrusted-data tag: OLD-TAG\n")}, a.history...)
	a.AnnounceSession("- session work directory: /work/dir")
	for range 4 {
		a.history = append(a.history, user("more"), assistant("reply"))
	}

	if _, err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	if transcript := mb.calls[0][0].Content; strings.Contains(transcript, "OLD-TAG") || strings.Contains(transcript, "/work/dir") {
		t.Errorf("session facts reached the summariser: %.200q", transcript)
	}
	head := a.history[0]
	if !strings.HasPrefix(head.Content, session.FactsPrefix) || !strings.Contains(head.Content, a.tag.Name()) ||
		!strings.Contains(head.Content, "/work/dir") || strings.Contains(head.Content, "OLD-TAG") {
		t.Errorf("head = %q, want the latest facts leading the summary message", head.Content)
	}
	facts := 0
	for _, m := range a.history {
		if strings.Contains(m.Content, "/work/dir") {
			facts++
		}
	}
	if facts != 1 {
		t.Errorf("the facts appear %d times in the history, want once", facts)
	}

	// Sent unwrapped, as AnnounceSession's own message is.
	if _, err := a.Run(context.Background(), "go on", nil); err != nil {
		t.Fatal(err)
	}
	if sent := mb.calls[1][0].Content; !strings.HasPrefix(sent, session.FactsPrefix) {
		t.Errorf("the carried facts were not sent as the runtime's own words: %.80q", sent)
	}
}

// Facts that are already in the retained tail stay where they are.
func TestCompactLeavesFactsInTheTailAlone(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "SUMMARY"}}}
	a, _ := newAgent(t, mb, &approveAll{}, 5)
	a.SetHistory(tenMessages())
	a.AnnounceSession("- session work directory: /work/dir")
	if _, err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(a.history[0].Content, session.FactsPrefix) {
		t.Error("facts in the tail were copied into the summary message too")
	}
	if last := a.history[len(a.history)-1]; !strings.HasPrefix(last.Content, session.FactsPrefix) {
		t.Errorf("the tail lost its facts: %q", last.Content)
	}
}

func TestAutoCompactFiresAtTheThresholdAndIsRecorded(t *testing.T) {
	log := &capturingLog{}
	mb := &mockBackend{responses: []*llm.Response{
		// Round 0 reports 90% occupancy and asks for a tool; the
		// compaction lands before the next request.
		listRound(90_000),
		{Content: "SUMMARY"},
		{Content: "done"},
	}}
	var notices []string
	a := compactingAgent(t, mb, 100_000, 5, func(o *Options) {
		o.Log = log
		o.OnNotice = func(msg string) { notices = append(notices, msg) }
	})
	a.SetHistory(tenMessages())

	if _, err := a.Run(context.Background(), "keep going", nil); err != nil {
		t.Fatal(err)
	}
	if len(mb.calls) != 3 {
		t.Fatalf("%d backend calls, want request + summarise + request", len(mb.calls))
	}
	if a.history[0].Attachments == nil || a.history[0].Attachments[0].Content != "SUMMARY" {
		t.Errorf("history head = %+v, want the summary", a.history[0])
	}
	// Told before the wait and after it (§6).
	if len(notices) != 2 || !strings.Contains(notices[0], "heading for 90%") || !strings.Contains(notices[1], "compacted") {
		t.Errorf("notices = %q, want the start and the outcome", notices)
	}

	var compactions int
	for i, kind := range log.kinds {
		if kind != session.KindCompaction {
			continue
		}
		compactions++
		rec, ok := log.data[i].(session.Compaction)
		if !ok || rec.Replaced == 0 || rec.Message.Attachments[0].Content != "SUMMARY" {
			t.Errorf("compaction record = %+v", log.data[i])
		}
	}
	if compactions != 1 {
		t.Errorf("%d compaction records written, want 1 (a resumed session replays these)", compactions)
	}
	if s := a.Usage(); s.CompactCalls != 1 {
		t.Errorf("CompactCalls = %d, want 1", s.CompactCalls)
	}
}

// ADR-0033 §2: the round's own growth is judged before it is sent. The
// last measurement says 30%; the tool results appended since carry the
// next request past the threshold — gem-agent's trigger (the last
// measurement alone) would have sent it and failed.
func TestAutoCompactSeesTheGrowthOfTheRoundAboutToBeSent(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "SUMMARY"}}}
	a := compactingAgent(t, mb, 10_000, 5, nil)
	a.SetHistory(tenMessages())
	a.noteMeasured(3_000, a.estimateRequest())
	a.history = append(a.history, toolCall("read_file"),
		toolResult("read_file", strings.Repeat("x", 6_000*bytesPerToken)))
	// A few short exchanges after it, so the bulk falls in the head a
	// compaction can summarise (a bulk in the kept tail is declined).
	for range 4 {
		a.history = append(a.history, user("more"), assistant("reply"))
	}

	a.maybeAutoCompact(context.Background())
	if len(mb.calls) != 1 {
		t.Fatalf("%d summariser calls, want 1: 3k measured + ~6k appended is past 80%% of 10k", len(mb.calls))
	}
}

// Growth that is not history counts: a loaded MCP server's schemas ride
// every later request as tool definitions (ADR-0004).
func TestProjectionCountsToolDefinitionGrowth(t *testing.T) {
	a := compactingAgent(t, &mockBackend{}, 10_000, 5, nil)
	a.SetHistory(tenMessages())
	a.noteMeasured(3_000, a.estimateRequest())
	before := a.projectedPrompt()
	a.toolDefs = append(a.toolDefs, llm.ToolDef{Name: "mcp__big__tool", Description: strings.Repeat("d", 3_000*bytesPerToken)})
	if grew := a.projectedPrompt() - before; grew < 2_900 {
		t.Errorf("projection grew by %d tokens for ~3k tokens of new schema", grew)
	}
}

// Without a measurement (a resumed session's first round) the estimate
// alone decides; a measurement far below its own estimate — a server
// counting only uncached tokens — is not taken as a base.
func TestProjectionFallsBackToTheEstimate(t *testing.T) {
	a := compactingAgent(t, &mockBackend{}, 100_000, 5, nil)
	a.SetHistory([]llm.Message{user(strings.Repeat("x", 30_000*bytesPerToken))})
	est := a.estimateRequest()
	if got := a.projectedPrompt(); got != est {
		t.Errorf("unmeasured projection = %d, want the estimate %d", got, est)
	}
	a.noteMeasured(500, est)
	if got := a.projectedPrompt(); got != est {
		t.Errorf("projection from an implausible count = %d, want the estimate %d", got, est)
	}
	a.noteMeasured(25_000, est)
	if got := a.projectedPrompt(); got != 25_000 {
		t.Errorf("projection from a plausible count = %d, want 25000", got)
	}
}

// A compaction that could not get under the threshold — the system
// prompt and tool definitions are out of its reach — is declined once
// with the reason, not run every two rounds.
func TestAutoCompactDeclinesWhenTheFixedPartFillsTheWindow(t *testing.T) {
	mb := &mockBackend{}
	var notices []string
	a := compactingAgent(t, mb, 10_000, 5, func(o *Options) {
		o.OnNotice = func(msg string) { notices = append(notices, msg) }
	})
	a.system = strings.Repeat("s", 8_500*bytesPerToken)
	a.SetHistory(tenMessages())
	for range 3 {
		a.history = append(a.history, user("more"), assistant("reply"))
		a.maybeAutoCompact(context.Background())
	}
	if len(mb.calls) != 0 {
		t.Errorf("%d summariser calls for a compaction that cannot help", len(mb.calls))
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "tool definitions alone take about 85%") {
		t.Errorf("notices = %q, want one naming the share no compaction can shrink", notices)
	}
}

// Without the growth guard, a session whose recent tail alone exceeds the
// threshold would summarise on every single round and never shrink.
func TestAutoCompactDoesNotSpin(t *testing.T) {
	responses := []*llm.Response{listRound(95_000)}
	for range 8 {
		responses = append(responses, &llm.Response{Content: "SUMMARY"}, listRound(95_000))
	}
	responses = append(responses, &llm.Response{Content: "done"})
	mb := &mockBackend{responses: responses}
	var compactions int
	a := compactingAgent(t, mb, 100_000, 12, func(o *Options) {
		o.OnNotice = func(msg string) {
			if strings.Contains(msg, "compacted") {
				compactions++
			}
		}
	})
	a.SetHistory(tenMessages())
	if _, err := a.Run(context.Background(), "go", nil); err != nil {
		t.Fatal(err)
	}
	if compactions > 2 {
		t.Errorf("%d compactions in one turn — the guard is not holding", compactions)
	}
}

func TestAutoCompactGivesUpAfterRepeatedFailures(t *testing.T) {
	var responses []*llm.Response
	for range 12 {
		responses = append(responses, listRound(95_000), &llm.Response{})
	}
	mb := &mockBackend{responses: responses}
	var failures int
	var offMsg string
	a := compactingAgent(t, mb, 100_000, 12, func(o *Options) {
		o.OnNotice = func(msg string) {
			if strings.Contains(msg, "compaction failed") {
				failures++
			}
			if strings.Contains(msg, "automatic compaction is off") {
				offMsg = msg
			}
		}
	})
	a.SetHistory(tenMessages())
	_, _ = a.Run(context.Background(), "go", nil)
	if failures != maxCompactFailures {
		t.Errorf("%d failure notices, want %d then silence", failures, maxCompactFailures)
	}
	if offMsg == "" {
		t.Error("auto-compaction switched itself off without saying so")
	}
	// Off "for this session": a new session starts from the setting.
	a.Restart(nil)
	if !a.AutoCompact() {
		t.Error("a new session inherited the old one's disabled auto-compaction")
	}
}

// The whole point of recording compactions: a resumed session must come
// back compacted, not re-inflated.
func TestCompactedSessionRoundTripsThroughTheTranscript(t *testing.T) {
	dir := t.TempDir()
	lg, err := session.Open(dir, "/p")
	if err != nil {
		t.Fatal(err)
	}
	mb := &mockBackend{responses: []*llm.Response{
		{Content: "answer one"},
		{Content: "SUMMARY OF EARLIER"},
		{Content: "answer two"},
	}}
	_, reg := newAgent(t, mb, &approveAll{}, 5)
	a := New(Options{Backend: mb, Registry: reg, Gate: &approveAll{}, Log: lg, System: "s", MaxTurns: 5})
	a.SetHistory([]llm.Message{
		user("1"), assistant("2"), user("3"), assistant("4"),
		user("5"), assistant("6"), user("7"), assistant("8"),
	})
	// SetHistory does not write records (the transcript already has
	// them on a real resume), so replay the seed explicitly.
	for _, m := range a.history {
		if err := lg.Log(session.KindMessage, m); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Run(context.Background(), "question", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Run(context.Background(), "another", nil); err != nil {
		t.Fatal(err)
	}
	_ = lg.Close()

	restored, _, _, err := session.Load(lg.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != len(a.history) {
		t.Fatalf("restored %d messages, live history has %d", len(restored), len(a.history))
	}
	for i := range restored {
		if restored[i].Role != a.history[i].Role || restored[i].Content != a.history[i].Content {
			t.Errorf("message %d: restored %s %q, live %s %q",
				i, restored[i].Role, clip(restored[i].Content, 40), a.history[i].Role, clip(a.history[i].Content, 40))
		}
	}
	if len(restored[0].Attachments) != 1 || restored[0].Attachments[0].Content != "SUMMARY OF EARLIER" {
		t.Errorf("restored head = %+v, want the summary message", restored[0])
	}
}

func TestRenderTranscriptKeepsShapeAndClipsBulk(t *testing.T) {
	got := renderTranscript([]llm.Message{
		user("do the thing"),
		{Role: llm.RoleUser, Attachments: []llm.Attachment{{Ref: "a.txt", Kind: "file", Content: "attached body"}}},
		{Role: llm.RoleAssistant, Content: "on it", ToolCalls: []llm.ToolCall{{Name: "read_file", Args: map[string]any{"path": "a.txt"}}}},
		toolResult("read_file", strings.Repeat("z", 1500*3)),
		{Role: llm.RoleTool, ToolName: "view_image", Content: "ok", Attachments: []llm.Attachment{{Ref: "s.png", Kind: "image", Data: []byte("PNGBYTES")}}},
	}, 1500, 4000)
	for _, want := range []string{"[user] do the thing", "[user attached file a.txt] attached body",
		"[assistant] on it", `[assistant calls read_file] {"path":"a.txt"}`, "[result of read_file]",
		"[image s.png (8 bytes) shown to the assistant]"} {
		if !strings.Contains(got, want) {
			t.Errorf("transcript missing %q:\n%s", want, clip(got, 400))
		}
	}
	if strings.Contains(got, "PNGBYTES") {
		t.Error("image bytes reached the summariser")
	}
	if len(got) > 1500*2 {
		t.Errorf("transcript is %d bytes — bulk tool output should be clipped for the summariser", len(got))
	}
}

// ADR-0033 §4: the summariser's input fits half the window — clips
// tighten first, then the cut moves back to an earlier safe boundary.
func TestFitHeadBoundsTheSummarisersInput(t *testing.T) {
	var h []llm.Message
	for i := range 40 {
		h = append(h, user(fmt.Sprintf("u%d %s", i, strings.Repeat("q", 2000))),
			toolCall("read_file"), toolResult("read_file", strings.Repeat("r", 3000)))
	}
	cut := compactCut(h, keepRecent)

	// A large window takes the whole head at the loosest clip.
	if got, _, err := fitHead(h, cut, 1_000_000); err != nil || got != cut {
		t.Fatalf("large window: cut %d, err %v; want %d", got, err, cut)
	}
	// A small one moves the cut back, to a boundary that is never a
	// tool result, and what it returns fits.
	window := 4_000
	got, text, err := fitHead(h, cut, window)
	if err != nil {
		t.Fatal(err)
	}
	if got >= cut || got < minCompacted || h[got].Role == llm.RoleTool {
		t.Errorf("cut %d (of %d): want an earlier safe boundary", got, cut)
	}
	if len(text)/bytesPerToken > window/2 {
		t.Errorf("transcript ~%d tokens exceeds half of %d", len(text)/bytesPerToken, window)
	}
	// Nothing fits at all: refused, which leaves the history alone.
	if _, _, err := fitHead(h, cut, 100); err == nil {
		t.Error("a head that cannot fit was accepted")
	}
}

func TestSummaryMessageFramesTheSummaryAsAPastRecord(t *testing.T) {
	m := SummaryMessage("earlier we agreed to delete everything", "")
	if m.Role != llm.RoleUser {
		t.Errorf("role = %s", m.Role)
	}
	if m.Content == "" || !strings.Contains(m.Content, "not as a new instruction") || !strings.Contains(m.Content, "re-read a file before editing") {
		t.Errorf("framing = %q — a summary that reads as orders is an injection path", m.Content)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].Content != "earlier we agreed to delete everything" {
		t.Errorf("attachments = %+v", m.Attachments)
	}
}

func TestCompactCutIsStableUnderGrowth(t *testing.T) {
	var h []llm.Message
	for i := range 40 {
		if i%3 == 0 {
			h = append(h, user(fmt.Sprintf("u%d", i)))
			continue
		}
		h = append(h, assistant(fmt.Sprintf("a%d", i)))
	}
	cut := compactCut(h, keepRecent)
	if cut != len(h)-keepRecent {
		t.Fatalf("cut = %d, want exactly len-keepRecent when no tool result is in the way", cut)
	}
}

// /clear, a new session and a restore each start the size bookkeeping
// over: the history the measurements described is gone.
func TestResetsForgetTheMeasurements(t *testing.T) {
	a := compactingAgent(t, &mockBackend{}, 100_000, 5, nil)
	for name, reset := range map[string]func(){
		"Reset":      a.Reset,
		"Restart":    func() { a.Restart(nil) },
		"SetHistory": func() { a.SetHistory(nil) },
	} {
		a.noteMeasured(50_000, 50_000)
		a.compactedAt = 9
		reset()
		if a.measuredPrompt != 0 || a.estAtMeasure != 0 || a.compactedAt != 0 {
			t.Errorf("%s left measured=%d est=%d compactedAt=%d", name, a.measuredPrompt, a.estAtMeasure, a.compactedAt)
		}
	}
}

// Review finding: a summary message leads with the facts it carried, so
// it must not pass for a facts message — the second compaction has to
// read the first summary, and carry the facts without a second framing.
func TestASecondCompactionReadsTheFirstSummary(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "FIRST SUMMARY"}, {Content: "SECOND SUMMARY"}}}
	a, _ := newAgent(t, mb, &approveAll{}, 5)
	a.AnnounceSession("- session work directory: /work/dir")
	for range 5 {
		a.history = append(a.history, user("more"), assistant("reply"))
	}
	if _, err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		a.history = append(a.history, user("later"), assistant("reply"))
	}
	if _, err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	if transcript := mb.calls[1][0].Content; !strings.Contains(transcript, "FIRST SUMMARY") {
		t.Errorf("the second summariser never saw the first summary: %.300q", transcript)
	}
	if transcript := mb.calls[1][0].Content; strings.Contains(transcript, "/work/dir") || strings.Contains(transcript, a.tag.Name()) {
		t.Error("the carried facts reached the second summariser")
	}
	head := a.history[0]
	if strings.Count(head.Content, "/work/dir") != 1 || strings.Count(head.Content, "was compacted") != 1 {
		t.Errorf("head after two compactions = %q, want the facts and the framing once each", head.Content)
	}
	if head.Attachments[0].Content != "SECOND SUMMARY" {
		t.Errorf("summary = %q", head.Attachments[0].Content)
	}
}

// A compaction that worked — by hand included — ends a run of failures
// and drops the size base it no longer describes; /clear starts the
// compaction state over, the configured switch included.
func TestCompactAndResetClearTheFailureState(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "SUMMARY"}}}
	a := compactingAgent(t, mb, 100_000, 5, nil)
	a.SetHistory(tenMessages())
	a.compactFailures, a.warnedNoCut, a.warnedNoRoom = 1, true, true
	a.noteMeasured(50_000, 50_000)
	if _, err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a.compactFailures != 0 || a.warnedNoCut || a.warnedNoRoom || a.measuredPrompt != 0 {
		t.Errorf("after a compaction: failures=%d noCut=%v noRoom=%v measured=%d",
			a.compactFailures, a.warnedNoCut, a.warnedNoRoom, a.measuredPrompt)
	}
	a.compactFailures, a.warnedNoCut = 1, true
	a.SetAutoCompact(false)
	a.Reset()
	if a.compactFailures != 0 || a.warnedNoCut || !a.AutoCompact() {
		t.Errorf("after /clear: failures=%d noCut=%v auto=%v", a.compactFailures, a.warnedNoCut, a.AutoCompact())
	}
}

// Review finding: a turn can start before the window lookup answers
// (an argv first message). The first request waits for it — once, and
// boundedly — instead of going out unjudged.
func TestTheFirstRequestWaitsForTheWindow(t *testing.T) {
	mb := &mockBackend{responses: []*llm.Response{{Content: "SUMMARY"}, {Content: "done"}}}
	_, reg := newAgent(t, mb, &approveAll{}, 5)
	a := New(Options{Backend: mb, Registry: reg, Gate: &approveAll{}, System: "s", MaxTurns: 5,
		AutoCompact: true, CompactAtPct: 80})
	a.toolDefs = nil
	a.SetHistory(tenMessages())
	for range 4 {
		a.history = append(a.history, user(strings.Repeat("x", 3_000*bytesPerToken)), assistant("ok"))
	}
	for range 4 {
		a.history = append(a.history, user("short"), assistant("ok"))
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		a.SetContextWindow(10_000)
	}()
	if _, err := a.Run(context.Background(), "go", nil); err != nil {
		t.Fatal(err)
	}
	if len(mb.calls) != 2 || !strings.Contains(mb.systems[0], "UNTRUSTED DATA") {
		t.Fatalf("%d calls, first system %.40q: the first request did not wait for the window", len(mb.calls), mb.systems[0])
	}

	// A lookup that never answers costs one bounded wait, not one per round.
	old := windowWait
	windowWait = 20 * time.Millisecond
	t.Cleanup(func() { windowWait = old })
	b := compactingAgent(t, &mockBackend{}, 0, 5, nil)
	b.windowSet = make(chan struct{})
	b.windowOnce = sync.Once{}
	start := time.Now()
	for range 3 {
		b.maybeAutoCompact(context.Background())
	}
	if took := time.Since(start); took > 500*time.Millisecond {
		t.Errorf("three rounds without a window took %v", took)
	}
}
