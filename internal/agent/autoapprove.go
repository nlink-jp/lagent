package agent

import (
	"context"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/risk"
)

// AutoDecision is the outcome of the auto-approve ladder for one call.
//
// The ladder here is the rule tier alone (RFP §3): Safe runs, anything
// else is asked. gem-agent's model tier — a second model call judging
// the proposed call — is a Phase 2 measurement, because on one local
// model every review costs a full prompt pass; the field it would fill
// is kept so the transcript record and the UI keep one shape.
type AutoDecision struct {
	Approved bool
	// Tier is the logical verdict that started the ladder.
	Tier risk.Tier
	// Reason is operator-facing: why it auto-ran, or why it is being
	// asked about.
	Reason string
	// ModelConsulted reports whether a model tier ran. Always false
	// here; the field stays so the record shape matches gem-agent's.
	ModelConsulted bool
}

// EscalationReason renders why auto mode is asking instead of running,
// naming the tier that objected.
func EscalationReason(d AutoDecision) string {
	if d.Tier == risk.Block {
		return "auto-approve blocked by rule (always asks): " + d.Reason
	}
	return "auto-approve escalated: " + d.Reason
}

// decideAuto runs the ladder for one tool call. Every uncertain path
// returns Approved=false: the human gate is the backstop, never
// bypassed on doubt.
func (a *Agent) decideAuto(_ context.Context, tc llm.ToolCall) AutoDecision {
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
	// the rule tier marks it as the operator's alone; every other Review
	// is the operator's too, with no model tier to answer in their place.
	if v.OperatorOnly {
		return AutoDecision{Tier: v.Tier, Reason: v.Reason}
	}
	return AutoDecision{Tier: v.Tier, Reason: v.Reason}
}
