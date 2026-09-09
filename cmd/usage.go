package cmd

import (
	"fmt"
	"strings"

	"github.com/nlink-jp/lagent/internal/agent"
	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/session"

	"github.com/nlink-jp/lagent/internal/uitext"
)

// sessionLogger is the transcript sink the tool constructors already
// hold — narrow, so a test can capture the records.
type sessionLogger interface {
	Log(kind string, data any) error
}

// logUsage writes one accounting record for a side call (ADR-0057).
// Same shape and same kind as the agent's own: an aggregator reads one
// record type, sums by source, and prices by model.
func logUsage(log sessionLogger, source, model string, u llm.Usage) {
	if log == nil || u.Empty() {
		return
	}
	_ = log.Log(session.KindUsage, session.UsageRecord{
		Source: source, Model: model,
		Prompt: u.Prompt, Output: u.Output, Thoughts: u.Thoughts,
		Cached: u.Cached, ToolPrompt: u.ToolPrompt, Total: u.Total,
	})
}

// usageReport renders the /usage statement.
func usageReport(ag *agent.Agent, mainModel string) string {
	s := ag.Usage()
	var b strings.Builder

	fmt.Fprintf(&b, "main loop (%s):\n", mainModel)
	if s.Rounds == 0 {
		b.WriteString("  no requests yet\n")
	} else {
		fmt.Fprintf(&b, "  rounds %d · prompt %s · output %s · thoughts %s\n",
			s.Rounds, humanTok(s.Prompt), humanTok(s.Output), humanTok(s.Thoughts))
		cachePct := 0.0
		if s.Prompt > 0 {
			cachePct = 100 * float64(s.Cached) / float64(s.Prompt)
		}
		fmt.Fprintf(&b, "  cached %s of prompt (%.0f%%)\n",
			humanTok(s.Cached), cachePct)
		window := "unknown window"
		if s.Window > 0 {
			window = fmt.Sprintf("%s window (%.0f%%)", humanTok(s.Window), 100*float64(s.LastPrompt)/float64(s.Window))
		}
		fmt.Fprintf(&b, "  context now %s of %s\n", humanTok(s.LastPrompt), window)
	}
	if s.RiskCalls > 0 {
		fmt.Fprintf(&b, "risk & progress reviews (%s): %d calls · prompt %s · output %s\n",
			mainModel, s.RiskCalls, humanTok(s.RiskPrompt), humanTok(s.RiskOutput))
	}
	if s.CompactCalls > 0 {
		fmt.Fprintf(&b, "compaction (%s): %d calls · prompt %s · output %s\n",
			mainModel, s.CompactCalls, humanTok(s.CompactPrompt), humanTok(s.CompactOutput))
	}

	return b.String()
}

// humanTok renders a token count compactly.
func humanTok(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// exitSummary is printed once on the way out of an interactive session
// (every route: /quit, Ctrl+C, Ctrl+D). The last thing in the
// scrollback answers the two questions an operator actually has at
// exit — how do I get back, and what did this cost — in two lines,
// not a statement (/usage held that, while it was reachable). Empty
// when nothing happened: a resume hint for a session with no
// conversation would point at a file resume refuses.
func exitSummary(s agent.UsageStats, sessionID string, msgs *uitext.Messages) []string {
	if s.Rounds == 0 {
		return nil
	}
	var lines []string
	if sessionID != "" {
		lines = append(lines, fmt.Sprintf(msgs.ExitSessionFmt, sessionID, sessionID))
	}
	lines = append(lines, fmt.Sprintf(msgs.ExitUsageFmt,
		s.Rounds, humanTok(s.Prompt), humanTok(s.Output)))
	if s.AbandonedRunning > 0 {
		// ADR-0065 §2: a goroutine the floor left behind may still
		// write after the process is gone; the operator hears it.
		lines = append(lines, fmt.Sprintf(msgs.ExitAbandonedFmt, s.AbandonedRunning))
	}
	return lines
}
