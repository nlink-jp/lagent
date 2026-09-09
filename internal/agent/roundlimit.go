package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nlink-jp/lagent/internal/llm"
)

// The round limit is an intervention ladder, not a guillotine. A
// deterministic loop detector escalates a suspected runaway
// immediately; reaching the limit is a checkpoint that asks the
// operator or stops fail-closed; and an absolute cap bounds the spend
// nothing can lift.
//
// gem-agent puts a model-tier progress review at the checkpoint, which
// lets an unattended run continue on its own verdict. There is no such
// review here (RFP §3, Phase 2): unattended, the checkpoint stops.
const (
	// loopThreshold: consecutive identical calls (name + canonical
	// args) before the intervention fires. Legitimate repetition
	// exists (polling), so detection escalates — it never kills.
	loopThreshold = 3
	// roundCapMultiplier × max_turns is the absolute ceiling: the
	// Block-floor principle applied to rounds.
	roundCapMultiplier = 3
	// turnCallsKept bounds the activity trace kept for the dialog.
	turnCallsKept = 40
)

// RoundLimitError is the plain hard stop (RoundReview off): the
// message teaches recovery. Wrappers detect it with errors.As rather
// than by matching the wording.
type RoundLimitError struct{ Rounds int }

func (e *RoundLimitError) Error() string {
	return fmt.Sprintf(roundStopFmt, "round limit", e.Rounds)
}

// roundExtension is one extension grant: half of max_turns, at least 1.
func roundExtension(maxTurns int) int {
	if e := maxTurns / 2; e > 1 {
		return e
	}
	return 1
}

// RoundLimitInfo is what the operator-facing dialog gets to show: the
// trigger and the numbers. Localization happens in cmd — the agent
// stays UI-free.
type RoundLimitInfo struct {
	// Trigger: "round-limit" or "loop".
	Trigger string
	// Detail carries the repeated call for a loop trigger.
	Detail string
	Rounds int
	Limit  int
	Cap    int
	// RecentCalls is the turn's activity trace, oldest first, so the
	// operator can judge what the review would have judged.
	RecentCalls []string
}

// continuedNotice tells the operator what was waved through.
func (a *Agent) continuedNotice(trigger, detail string, limit int) string {
	if trigger == "loop" {
		return fmt.Sprintf(a.msgs.RoundLoopContinuedFmt, clip(detail, 80))
	}
	return fmt.Sprintf(a.msgs.RoundLimitContinuedFmt, limit)
}

// roundIntervention is the decision at a checkpoint (limit reached, or
// the loop detector fired). Returns whether the turn continues.
// Fail-closed at every uncertain edge.
func (a *Agent) roundIntervention(ctx context.Context, trigger, detail string, round, limit, cap int) bool {
	// A cancelled turn gets no dialog — the operator interrupted, and a
	// prompt on behalf of a dead turn is the last thing they asked for.
	if ctx.Err() != nil {
		return false
	}
	info := RoundLimitInfo{
		Trigger: trigger, Detail: detail,
		Rounds: round, Limit: limit, Cap: cap,
		RecentCalls: append([]string(nil), a.turnCalls...),
	}
	decision, source := false, "stop"
	if a.onRoundLimit != nil {
		decision, source = a.onRoundLimit(ctx, info), "operator"
		if decision {
			a.notify(a.continuedNotice(trigger, detail, limit))
		}
	}
	a.logRecord("round_intervention", map[string]any{
		"trigger": trigger, "detail": clip(detail, 200), "round": round,
		"limit": limit, "cap": cap, "decision": decision, "source": source,
	})
	return decision
}

// canonicalCallSig builds the loop-detector signature: tool name plus
// canonical (sorted-key) argument JSON.
func canonicalCallSig(tc llm.ToolCall) string {
	args, err := json.Marshal(tc.Args) // map marshal sorts keys
	if err != nil {
		args = []byte("{}")
	}
	return tc.Name + "\x00" + string(args)
}
