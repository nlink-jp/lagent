package cmd

import (
	"strings"
	"testing"
)

func TestSlashCompletionsSource(t *testing.T) {
	complete := slashCompletions()

	if got := complete("/us"); len(got) != 1 || got[0] != "/usage" {
		t.Errorf("/us → %v", got)
	}
	got := complete("/s")
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "/settings") {
		t.Errorf("/s missing /settings: %v", got)
	}
	for _, gone := range []string{"/skill", "/skills", "/compact", "/riskbook", "/memory"} {
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
