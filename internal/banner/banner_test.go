package banner

import (
	"github.com/nlink-jp/lagent/internal/sandbox"

	"strings"
	"testing"
)

// gem-agent ADR-0078: a line earns a place at startup only if nothing else will
// say it. The enumerations become one row that names where the detail
// is — and the count survives because "did my toolset come up" is a
// question the operator has before typing.
func TestInventoryLineReplacesTheEnumerations(t *testing.T) {
	got := inventory(Facts{Servers: 24, Tools: 249})
	for _, want := range []string{"24 servers", "249 tools", "/mcp"} {
		if !strings.Contains(got, want) {
			t.Errorf("inventoryLine = %q, missing %q", got, want)
		}
	}
	if n := len(got); n > 80 {
		t.Errorf("the row that replaced eleven wraps at 80 columns: %d chars", n)
	}
}

// A part with nothing in it is not printed, and its command is not
// offered: an empty list is not a fact the operator can use.
func TestInventoryLineOmitsWhatIsEmpty(t *testing.T) {
	if got := inventory(Facts{}); got != "" {
		t.Errorf("inventoryLine = %q, want nothing for an empty inventory", got)
	}
	got := inventory(Facts{Servers: 1, Tools: 3})
	if !strings.Contains(got, "/mcp") {
		t.Errorf("inventoryLine = %q, want the command that expands it", got)
	}
	if inventory(Facts{}) != "" {
		t.Errorf("a session with nothing loaded still printed a row: %q", inventory(Facts{}))
	}
}

// The ordinary sandbox is silent; each abnormal state still prints,
// because the operator has no reason to go looking for it.
func TestSandboxLineIsAbnormalOnly(t *testing.T) {
	if got := SandboxLine(true, true, false); got != "" {
		t.Errorf("the normal case printed %q — it is a manual excerpt, identical every start", got)
	}
	for _, tc := range []struct {
		name                      string
		on, readLane, lanePrompts bool
		want                      string
	}{
		{"disabled", false, false, false, "DISABLED"},
		{"read lane unverified", true, false, false, "unverified"},
		{"read_lane_prompts", true, true, true, "read_lane_prompts"},
	} {
		got := SandboxLine(tc.on, tc.readLane, tc.lanePrompts)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: line = %q, want mention of %q", tc.name, got, tc.want)
		}
	}
}

// A disabled sandbox outranks every other abnormality: it is the one
// that changes what a shell command can reach.
func TestDisabledSandboxWinsOverTheOtherStates(t *testing.T) {
	if got := SandboxLine(false, true, true); !strings.Contains(got, "DISABLED") {
		t.Errorf("line = %q, want the disabled state", got)
	}
}

// The banner is composed by a rule, so the rule is what is tested: the
// ordinary session prints what nothing else says, and nothing more.
func TestOrdinarySessionIsThreeLines(t *testing.T) {
	got := Lines(Facts{
		Version: "v0.1.0", Model: "google/gemma-4-26b-a4b-qat",
		Instructions: []string{"AGENTS.md"},
		Servers:      24, Tools: 249,
		SandboxOn: true, ReadLane: true,
	})
	if len(got) != 3 {
		t.Fatalf("ordinary start printed %d lines:\n%s", len(got), strings.Join(got, "\n"))
	}
	joined := strings.Join(got, "\n")
	for _, gone := range []string{"project:", "session log:", "approval policy:", "risk rulebook:", "shell lanes"} {
		if strings.Contains(joined, gone) {
			t.Errorf("%q is back in the banner:\n%s", gone, joined)
		}
	}
}

// Every abnormal state still prints, and last — closest to the prompt.
func TestAbnormalStatesStillPrint(t *testing.T) {
	got := Lines(Facts{
		Version: "v", Model: "m",
		SandboxOn:   false,
		AutoApprove: true,
		Notes:       []string{"policy entry ignored"},
	})
	joined := strings.Join(got, "\n")
	for _, want := range []string{"DISABLED", "auto-approve: ON", "warning: policy entry ignored"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q:\n%s", want, joined)
		}
	}
	if !strings.HasPrefix(got[len(got)-1], "warning:") {
		t.Errorf("the warnings are not last, closest to the prompt: %v", got)
	}
}

// Starting in auto-approve is the only approval-regime fact with no
// other startup surface: the TUI footer carries it, the plain REPL has
// no footer, and gem-agent ADR-0078 removed the sandbox line that gestured at it.
func TestAutoApproveIsAnnouncedAtStart(t *testing.T) {
	on := strings.Join(Lines(Facts{Version: "v", Model: "m", SandboxOn: true, ReadLane: true, AutoApprove: true}), "\n")
	if !strings.Contains(on, "auto-approve: ON at start") || !strings.Contains(on, "/auto") {
		t.Errorf("auto-approve at start is silent: %q", on)
	}
	off := strings.Join(Lines(Facts{Version: "v", Model: "m", SandboxOn: true, ReadLane: true}), "\n")
	if strings.Contains(off, "auto-approve") {
		t.Errorf("a manual session announced auto-approve: %q", off)
	}
}

// One-shot prints this sentence without the rest of the banner, so it
// has to be true in one-shot too. The clause it used to carry — that
// every command still asks — is false there, where a gated call is
// denied rather than asked (interactively it held even under --auto), so
// the sentence states the confinement and the state to restore, and
// leaves the approval regime to the lines that own it.
func TestDisabledSandboxSentenceIsTrueInEveryMode(t *testing.T) {
	got := SandboxLine(false, false, false)
	if strings.Contains(got, "asks for your approval") {
		t.Errorf("the shared sentence claims an approval regime it cannot know: %q", got)
	}
	if !strings.Contains(got, "sandbox enabled") {
		t.Errorf("no state to restore named: %q", got)
	}
}

// Four states, one vocabulary. The panel row took two booleans while the
// runtime had four, so a session that had opted into read_lane_prompts
// was told its machine failed the probe (pre-release review).
func TestSandboxStatesAgreeAcrossSurfaces(t *testing.T) {
	for _, tc := range []struct {
		on, lane, prompts bool
		want              string
	}{
		{true, true, false, ""},
		{true, true, true, "read_lane_prompts"},
		{true, false, false, "unverified"},
		{false, false, false, "DISABLED"},
	} {
		line := SandboxLine(tc.on, tc.lane, tc.prompts)
		row := State(tc.on, tc.lane, tc.prompts)
		if tc.want == "" {
			if line != "" || row != "enabled" {
				t.Errorf("ordinary sandbox: line=%q row=%q", line, row)
			}
			continue
		}
		if !strings.Contains(line, tc.want) || !strings.Contains(row, tc.want) {
			t.Errorf("state %v: line=%q row=%q, both must name %q", tc, line, row, tc.want)
		}
	}
}

// Every abnormal sandbox line carries a command, which is what the
// interface reference promises of them.
func TestSandboxLinesCarryACommand(t *testing.T) {
	for _, line := range []string{
		SandboxLine(false, false, false),
		SandboxLine(true, true, true),
		SandboxLine(true, false, false),
	} {
		if !strings.Contains(line, "lagent") && !strings.Contains(line, "sandbox enabled") &&
			!strings.Contains(line, "config.toml") {
			t.Errorf("no next command in %q", line)
		}
	}
}

// One-shot has no REPL and no TUI, so its auto-approve line cannot name
// /auto or shift+tab.
func TestOneShotAutoApproveNamesTheFlag(t *testing.T) {
	got := AutoApproveOneShotLine()
	if strings.Contains(got, "/auto") || strings.Contains(got, "shift+tab") {
		t.Errorf("one-shot line names something it does not have: %q", got)
	}
	if !strings.Contains(got, "--auto") {
		t.Errorf("one-shot line has no next command: %q", got)
	}
}

// One line per setting, in /readonly's shape, and nothing for a default
// (the banner's test for a line).
func TestReadOnlyLines(t *testing.T) {
	for _, tc := range []struct {
		state sandbox.Ceiling
		want  []string
	}{
		{sandbox.Ceiling{}, nil},
		{sandbox.Ceiling{ReadOnly: true}, []string{"read-only: ON at start"}},
	} {
		got := ReadOnlyLines(tc.state, true)
		if len(got) != len(tc.want) {
			t.Errorf("state %+v → %d lines, want %d: %q", tc.state, len(got), len(tc.want), got)
			continue
		}
		for i, want := range tc.want {
			if !strings.Contains(got[i], want) {
				t.Errorf("state %+v line %d = %q, want it to contain %q", tc.state, i, got[i], want)
			}
		}
	}
	// One-shot has no footer and no /readonly, so its line names the
	// flag instead — the same shape as the auto-approve pair.
	if !strings.Contains(ReadOnlyOneShotLine(true), "--writable") {
		t.Errorf("the one-shot line does not name the escape: %q", ReadOnlyOneShotLine(true))
	}
}

// With the lanes off, the ceiling still refuses the file tools and a
// write- or operator-declaring shell call, but a read-lane declaration
// is bounded by nothing — so the sentence "changes nothing outside its
// scratch" is false, and the banner printed it directly beneath
// "sandbox: off" (independent review).
func TestReadOnlyLinesUnconfined(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
	}{
		{"banner", ReadOnlyLines(sandbox.Ceiling{ReadOnly: true}, false)[0]},
		{"one-shot", ReadOnlyOneShotLine(false)},
	} {
		if strings.Contains(tc.line, "changes nothing") {
			t.Errorf("%s claims the lanes' guarantee with the lanes off: %q", tc.name, tc.line)
		}
		if !strings.Contains(tc.line, "sandbox is off") {
			t.Errorf("%s does not say why the guarantee is narrower: %q", tc.name, tc.line)
		}
	}
}

// An unverified read lane is not an absent sandbox. read_lane_prompts
// is the operator asking for MORE gating, and a failed startup probe
// still leaves the read profile applied and every shell_exec gated — so
// in both the ceiling's sentence stands. The first version of this
// keyed on SandboxOn && ReadLane and printed "the sandbox is off"
// directly beneath "sandbox: enabled", to the most cautious
// configuration the tool has (second independent review).
func TestReadOnlyLineSurvivesAnUnverifiedReadLane(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    Facts
	}{
		{"read_lane_prompts", Facts{ReadOnly: sandbox.Ceiling{ReadOnly: true},
			SandboxOn: true, ReadLane: false, ReadLanePrompts: true}},
		{"probe failed", Facts{ReadOnly: sandbox.Ceiling{ReadOnly: true},
			SandboxOn: true, ReadLane: false}},
	} {
		lines := Lines(tc.f)
		var sandboxLine, roLine string
		for _, l := range lines {
			if strings.HasPrefix(l, "sandbox:") {
				sandboxLine = l
			}
			if strings.HasPrefix(l, "read-only:") {
				roLine = l
			}
		}
		if !strings.Contains(sandboxLine, "enabled") {
			t.Fatalf("%s: fixture is not a sandbox-on case: %q", tc.name, sandboxLine)
		}
		if strings.Contains(roLine, "sandbox is off") {
			t.Errorf("%s: the banner contradicts itself:\n  %s\n  %s", tc.name, sandboxLine, roLine)
		}
		if !strings.Contains(roLine, "changes nothing outside its scratch") {
			t.Errorf("%s: the guarantee was withdrawn where it holds: %q", tc.name, roLine)
		}
	}
}
