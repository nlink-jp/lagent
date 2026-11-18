package cmd

import (
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/skills"
)

func TestSlashCompletionsSource(t *testing.T) {
	complete := slashCompletions(func() []skills.Skill {
		return []skills.Skill{{Name: "meeting-notes"}, {Name: "mcp-tactics"}}
	})

	if got := complete("/us"); len(got) != 1 || got[0] != "/usage" {
		t.Errorf("/us → %v", got)
	}
	// "/skill " completes skill names (ADR-0011).
	if got := complete("/skill me"); len(got) != 1 || got[0] != "/skill meeting-notes" {
		t.Errorf("/skill me → %v", got)
	}
	got := complete("/s")
	joined := strings.Join(got, " ")
	for _, want := range []string{"/settings", "/show"} {
		if !strings.Contains(joined, want) {
			t.Errorf("/s missing %s: %v", want, got)
		}
	}
	// /compact arrived with ADR-0033; /riskbook is gem-agent's only.
	if got := complete("/comp"); len(got) != 1 || got[0] != "/compact" {
		t.Errorf("/comp → %v", got)
	}
	for _, gone := range []string{"/riskbook"} {
		if strings.Contains(strings.Join(complete("/"), " "), gone) {
			t.Errorf("completion offers %s, which this runtime does not have", gone)
		}
	}
	if got := complete("/v"); len(got) != 1 || got[0] != "/version" {
		t.Errorf("/v → %v", got)
	}
	if got := complete("/nonexistent"); len(got) != 0 {
		t.Errorf("unknown prefix → %v", got)
	}
}
