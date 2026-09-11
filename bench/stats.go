package main

import (
	"encoding/json"
	"time"

	"github.com/nlink-jp/lagent/internal/session"
)

// stats is what one transcript says about a run (ADR-0006 §5).
type stats struct {
	// Rounds counts assistant messages that carried tool calls — the
	// model went back to the tools that many times. Zero means the
	// model answered in a single round.
	Rounds    int
	ToolCalls int
	ByTool    map[string]int
	// Token totals over the main-loop usage records.
	Prompt, Output, Cached int
	// Answer is the last assistant text in the transcript.
	Answer string
}

// transcriptStats reads a session file with the runtime's own scanner.
// Only the envelope fields it needs are decoded; the record shapes are
// gem-usage-lens's (message, usage), shared by both runtimes.
func transcriptStats(path string) (stats, error) {
	st := stats{ByTool: map[string]int{}}
	err := session.Scan(path, func(kind string, _ time.Time, data json.RawMessage) error {
		switch kind {
		case session.KindMessage:
			var m struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					Name string `json:"name"`
				} `json:"tool_calls"`
			}
			if err := json.Unmarshal(data, &m); err != nil {
				return nil // a torn line is skipped, as Load does
			}
			if m.Role != "assistant" {
				return nil
			}
			if len(m.ToolCalls) > 0 {
				st.Rounds++
				for _, tc := range m.ToolCalls {
					st.ToolCalls++
					st.ByTool[tc.Name]++
				}
			}
			if m.Content != "" {
				st.Answer = m.Content
			}
		case session.KindUsage:
			var u struct {
				Source string `json:"source"`
				Prompt int    `json:"prompt"`
				Output int    `json:"output"`
				Cached int    `json:"cached"`
			}
			if err := json.Unmarshal(data, &u); err != nil {
				return nil
			}
			if u.Source == "" || u.Source == session.UsageMain {
				st.Prompt += u.Prompt
				st.Output += u.Output
				st.Cached += u.Cached
			}
		}
		return nil
	})
	return st, err
}
