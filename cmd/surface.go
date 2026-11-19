package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/tui"
	"github.com/nlink-jp/lagent/internal/uitext"
)

// surface is the one place that decides where operator-facing output
// goes (ADR-0054 §2.3): to the TUI program once it runs, to the stream
// (stderr, the startup tee) until then and in every session that has no
// TUI. What is said stays with the caller; the text each route prints is
// what the nine `if prog != nil` branches it replaces printed.
//
// One concrete type whose program field decides: the program is
// late-bound, so the same value is the stream before the TUI runs and
// again after it ends.
type surface struct {
	w        io.Writer
	msgs     *uitext.Messages
	oneShot  bool
	plainAsk askFunc
	// prog is set before the program runs and cleared when it returns:
	// a Send to a program that has ended is dropped without a word
	// (Bubble Tea selects on its cancelled context), so what the exit
	// says — a session_end hook's notice — went nowhere (ADR-0054,
	// Found 5). Atomic: a goroutine the session started may still be
	// reporting when the program ends.
	prog atomic.Pointer[tea.Program]
	// hookSink, while set, collects hook notices instead of showing them.
	hookSink *[]string
	// ended is set when the TUI has ended (endTUI). The exit's own
	// speakers — the session_end hooks — reach the stream then; what a
	// turn still running reports (one left behind by the third Ctrl+C on
	// a wedged tool, a late window lookup) is dropped, as the ended
	// program dropped it: it would land after the exit summary.
	ended atomic.Bool
}

// setProgram routes output to p.
func (s *surface) setProgram(p *tea.Program) { s.prog.Store(p) }

// endTUI is called when the program has returned: a Send to an ended
// program is dropped without a word, so the exit's hook notices go to
// the stream, and the rest is dropped on purpose (ADR-0054, Found 5).
func (s *surface) endTUI() {
	s.ended.Store(true)
	s.prog.Store(nil)
}

// notice puts a one-line warning in front of the operator wherever they
// are: the TUI's note strip, stderr otherwise.
func (s *surface) notice(msg string) {
	if s.ended.Load() {
		return
	}
	if p := s.prog.Load(); p != nil {
		p.Send(tui.Attached{Notes: []string{msg}})
		return
	}
	fmt.Fprintf(s.w, "[⚠ %s]\n", msg)
}

// hookNotice is a hook's notice. While a slash command runs the hooks
// (/clear) the notices are collected instead: the slash handler executes
// inside the TUI's Update, and Program.Send from there never returns —
// Bubble Tea's message channel is unbuffered and Update is its only
// consumer. A session_start hook that printed froze the TUI at /clear
// (ADR-0054, Found 1). Notes raised there ride back in the slash output.
func (s *surface) hookNotice(msg string) {
	if s.hookSink != nil {
		*s.hookSink = append(*s.hookSink, msg)
		return
	}
	if s.ended.Load() {
		// The exit's session_end hooks: the program has ended, the
		// stream is the operator's screen again.
		fmt.Fprintf(s.w, "[⚠ %s]\n", msg)
		return
	}
	s.notice(msg)
}

// collectHookNotices appends hook notices to into until stop is called.
// Only hook notices: notice itself is not collected.
func (s *surface) collectHookNotices(into *[]string) (stop func()) {
	s.hookSink = into
	return func() { s.hookSink = nil }
}

// toolCall shows a tool call. Headless runs (-p, piped stdin) get the
// declared purpose on its own line too (gem-agent ADR-0047 §5): the
// transcript of a scripted run is the only record it leaves behind.
func (s *surface) toolCall(name, detail, purpose string) {
	if s.ended.Load() {
		return
	}
	if p := s.prog.Load(); p != nil {
		p.Send(tui.ToolCall{Name: name, Detail: detail, Purpose: purpose})
		return
	}
	fmt.Fprintf(s.w, "\n[tool] %s %s\n", name, detail)
	if purpose != "" {
		fmt.Fprintf(s.w, "[tool] ↪ %s\n", purpose)
	}
}

// toolDone is the tool-finished signal; only the TUI has state to leave.
func (s *surface) toolDone(name string) {
	if s.ended.Load() {
		return
	}
	if p := s.prog.Load(); p != nil {
		p.Send(tui.ToolDone{Name: name})
	}
}

func (s *surface) attached(lines, notes []string) {
	if s.ended.Load() {
		return
	}
	if p := s.prog.Load(); p != nil {
		p.Send(tui.Attached{Lines: lines, Notes: notes})
		return
	}
	for _, l := range lines {
		fmt.Fprintln(s.w, "[📎 "+l+"]")
	}
	for _, n := range notes {
		fmt.Fprintln(s.w, "[⚠ "+n+"]")
	}
}

func (s *surface) autoApproved(tool, reason, tier string) {
	if s.ended.Load() {
		return
	}
	if p := s.prog.Load(); p != nil {
		p.Send(tui.AutoApproved{Tool: tool, Reason: reason, Tier: tier})
		return
	}
	fmt.Fprintf(s.w, "[%s]\n", fmt.Sprintf(strings.TrimSpace(s.msgs.AutoApprovedFmt), tier, tool+": "+reason))
}

// usage, stream and contextWindow feed the TUI's footer and status line;
// a session without one shows none of them.
func (s *surface) usage(u llm.Usage) {
	if s.ended.Load() {
		return
	}
	if p := s.prog.Load(); p != nil {
		p.Send(tui.Usage{Prompt: u.Prompt, Output: u.Output, Cached: u.Cached})
	}
}

func (s *surface) stream(ev llm.StreamEvent) {
	if s.ended.Load() {
		return
	}
	if p := s.prog.Load(); p != nil {
		p.Send(tui.StreamUpdate{Kind: ev.Kind, Thought: ev.Thought,
			Attempt: ev.Attempt, Max: ev.Max, Cause: ev.Cause, DelayMS: ev.DelayMS})
	}
}

func (s *surface) contextWindow(tokens int) {
	if s.ended.Load() {
		return
	}
	if p := s.prog.Load(); p != nil {
		p.Send(tui.ContextWindow{Tokens: tokens})
	}
}

// ask is a structured mid-turn choice (gem-agent ADR-0036): one-shot has
// nobody to ask, the TUI shows a dialog, the plain REPL reads a number
// from the SHARED stdin reader — a second bufio.Reader over the same fd
// strands typed-ahead input in one buffer while the other blocks
// (AGENTS.md gotcha; review round 3).
func (s *surface) ask(ctx context.Context, question string, options []string) (int, error) {
	if s.ended.Load() {
		return 0, errAskDeclined
	}
	if s.oneShot {
		return oneShotAsk(ctx, question, options)
	}
	if p := s.prog.Load(); p != nil {
		resp := make(chan int, 1)
		p.Send(tui.AskRequest{Question: question, Options: options, Resp: resp})
		idx := <-resp
		if idx < 0 {
			return 0, errAskDeclined
		}
		return idx, nil
	}
	return s.plainAsk(ctx, question, options)
}
