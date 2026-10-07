package agent

import (
	"context"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/risk"
)

// AutoDecision is the outcome of the auto-approve ladder for one call.
//
// The ladder is the rule tier, and — with [approval].model_tier =
// "shell" — gem-agent's model tier for the write-lane shell commands the
// rule tier leaves at Review (ADR-0032). Every other call keeps the rule
// tier's answer: Safe runs, anything else is asked.
type AutoDecision struct {
	Approved bool
	// Tier is the logical verdict that started the ladder.
	Tier risk.Tier
	// Reason is operator-facing: why it auto-ran, or why it is being
	// asked about.
	Reason string
	// ModelConsulted reports whether the model tier ran.
	ModelConsulted bool
	// Confidence is the number the model tier returned, and
	// ConfidenceKnown says whether there is one: the tier can run and
	// still produce none (a transport error, an unparseable verdict).
	Confidence      float64
	ConfidenceKnown bool
}

// EscalationReason renders why auto mode is asking instead of running,
// naming the tier that objected.
func EscalationReason(d AutoDecision) string {
	switch {
	case d.Tier == risk.Block:
		return "auto-approve blocked by rule (always asks): " + d.Reason
	case d.ModelConsulted:
		return "auto-approve escalated by risk review: " + d.Reason
	}
	return "auto-approve escalated: " + d.Reason
}

// decideAuto runs the ladder for one tool call. Every uncertain path
// returns Approved=false: the human gate is the backstop, never
// bypassed on doubt.
func (a *Agent) decideAuto(ctx context.Context, tc llm.ToolCall) AutoDecision {
	d := a.decide(tc)
	if d.Tool == nil {
		return AutoDecision{Reason: "unknown tool"}
	}
	v := d.Verdict
	switch v.Tier {
	case risk.Safe:
		return AutoDecision{Approved: true, Tier: v.Tier, Reason: v.Reason}
	case risk.Block:
		// The deterministic floor.
		return AutoDecision{Tier: v.Tier, Reason: v.Reason}
	}
	// Review: a write into the instruction files or the runtime's
	// configuration persists into what every later session trusts, and
	// the rule tier marks it as the operator's alone.
	if v.OperatorOnly {
		return AutoDecision{Tier: v.Tier, Reason: v.Reason}
	}
	// A write-lane shell command is the model tier's when it is on
	// (ADR-0032); every other Review is the operator's.
	if a.modelTierJudges(tc, d) {
		return a.judge(ctx, tc, v)
	}
	return AutoDecision{Tier: v.Tier, Reason: v.Reason}
}
