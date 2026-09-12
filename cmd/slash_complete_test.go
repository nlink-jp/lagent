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
	if !strings.Contains(joined, "/settings") {
		t.Errorf("/s missing /settings: %v", got)
	}
	for _, gone := range []string{"/compact", "/riskbook", "/memory"} {
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
