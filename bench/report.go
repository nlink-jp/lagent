package main

import (
	"fmt"
	"sort"
	"strings"
)

// group is one task × configuration cell of the report.
type group struct {
	Task, Config string
	Runs         []result
}

// report folds runs into one Markdown table: per task and
// configuration, how many runs completed, how many answered without a
// single tool call, and the medians a Phase 2 ADR quotes.
func report(results []result) string {
	groups := map[string]*group{}
	var order []string
	for _, r := range results {
		key := r.Task + "\x00" + r.Config
		g, ok := groups[key]
		if !ok {
			g = &group{Task: r.Task, Config: r.Config}
			groups[key] = g
			order = append(order, key)
		}
		g.Runs = append(g.Runs, r)
	}
	sort.Strings(order)

	var b strings.Builder
	b.WriteString("| task | config | runs | completed | no-tool answers | rounds (med) | calls (med) | prompt tok (med) | wall s (med) |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	for _, key := range order {
		g := groups[key]
		n := len(g.Runs)
		completed, noTool := 0, 0
		var rounds, calls, prompt, wall []float64
		for _, r := range g.Runs {
			if r.Completed {
				completed++
			}
			if r.ToolCalls == 0 {
				noTool++
			}
			rounds = append(rounds, float64(r.Rounds))
			calls = append(calls, float64(r.ToolCalls))
			prompt = append(prompt, float64(r.Prompt))
			wall = append(wall, r.WallSec)
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %d/%d | %d/%d | %.0f | %.0f | %.0f | %.0f |\n",
			g.Task, g.Config, n, completed, n, noTool, n,
			median(rounds), median(calls), median(prompt), median(wall))
	}
	b.WriteString("\nno-tool answers: runs where the model replied without calling any tool.\n")
	b.WriteString("Failures per run are in runs.jsonl (`failures`); stdout/stderr and the transcript sit beside each run.\n")
	return b.String()
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}
