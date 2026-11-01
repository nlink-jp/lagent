package agent

import (
	"errors"

	"github.com/nlink-jp/lagent/internal/tools"
)

// One round is bounded by the window (ADR-0034). The results of one
// round of tool calls share a budget measured against the compaction
// threshold, re-judged before every call; each call is handed its part
// as an allotment its tool applies with its own route.
const (
	// allotBytesPerToken converts an allotment to bytes. It is below the
	// 10th percentile ADR-0033 measured (2.25), so the allotment bounds
	// what a result costs rather than estimating it.
	allotBytesPerToken = 2
	// minAllot is the floor of every allotment, outside the budget: a
	// short result — an edit's confirmation, an error — arrives whole,
	// and a byte window always advances.
	minAllot = 1024
	// allotSlack is what a tool adds past its allotment — the marker,
	// the route, a listing's footer — before the backstop cuts.
	allotSlack = 2048
)

// errWindowFull ends a turn whose round found no room (ADR-0034 §6):
// with no compaction able to run, every further result would be its
// route alone, and the model would call again into the same wall until
// the loop guard or the round limit stopped it.
var errWindowFull = errors.New("the conversation fills the context window and no compaction can run — progress so far is saved: /compact summarises the older part, /clear starts a fresh conversation")

// tailBound is T (ADR-0034 §1), in tokens: half of what a compaction can
// make room for under the threshold, with the system prompt and the tool
// definitions as they stand. A round that fits it can always be kept
// verbatim by a compaction that still gets under the threshold. 0 when
// the window is unknown — no bound; at least 1 otherwise, so a window the
// fixed part nearly fills still bounds the tail.
func (a *Agent) tailBound(window int) int {
	if window <= 0 {
		return 0
	}
	t := (a.compactAtPct*window/100 - a.estimateFixed() - summaryAllowance) / 2
	return max(t, 1)
}

// roundBudget is one round's account. window 0 means no budget: each
// tool's own cap applies, as before ADR-0034.
type roundBudget struct {
	window int
	used   int  // tokens this round's results have taken, at allotBytesPerToken
	full   bool // no compaction can run and the room reached zero
}

// allot is the allotment, in bytes, for the next call when remaining
// calls (this one included) are still to run (ADR-0034 §1–§2). With a
// compaction able to run before the next request, the room is what is
// left of T; without one, it is also held under the threshold by the
// live projection — this round's results so far included.
func (a *Agent) allot(b *roundBudget, remaining int) int {
	room := a.tailBound(b.window) - b.used
	if !a.AutoCompact() {
		room = min(room, a.compactAtPct*b.window/100-a.projectedPrompt())
		if room <= 0 {
			b.full = true
		}
	}
	share := max(room, 0) / max(remaining, 1) * allotBytesPerToken
	return max(share, minAllot)
}

// spend charges a result to the round, wrap included.
func (b *roundBudget) spend(result string, images int) {
	b.used += (len(result)+wrapBytes)/allotBytesPerToken + images*imageTokens
}

// bounded applies the backstop to a result: a tool that did not apply
// its allotment is cut past it, with the marker that says the rest was
// not kept. A tool marked KeepWhole (the operator's or the runtime's own
// words) and a gate denial are never cut; an image rides beside its
// text, which is short.
func (a *Agent) bounded(name, result string, allotted int, denied bool) string {
	if denied {
		return result
	}
	if t, ok := a.registry.Get(name); ok && t.KeepWhole {
		return result
	}
	return tools.CutUnkept(result, allotted+allotSlack)
}
