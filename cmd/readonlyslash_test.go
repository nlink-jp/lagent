package cmd

// /readonly shows the ceiling and moves it. Its text describes a state
// rather than a transition, because showing must not claim a change.

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/agent"
	"github.com/nlink-jp/lagent/internal/sandbox"
	"github.com/nlink-jp/lagent/internal/tools"
	"github.com/nlink-jp/lagent/internal/uitext"
)

func readOnlyAgent(t *testing.T, c sandbox.Ceiling) *agent.Agent {
	t.Helper()
	reg, err := tools.New(t.TempDir(),
		func(ctx context.Context, cmd string) *exec.Cmd { return exec.CommandContext(ctx, "/bin/echo", "x") },
		time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return agent.New(agent.Options{Registry: reg, System: "s", MaxTurns: 3, Ceiling: c})
}

func readOnlySlash(t *testing.T, a *agent.Agent, input string, lang uitext.Lang) string {
	t.Helper()
	out, isErr := readOnlySlashErr(t, a, input, lang)
	if isErr {
		t.Fatalf("%s reported an error: %q", input, out)
	}
	return out
}

func readOnlySlashErr(t *testing.T, a *agent.Agent, input string, lang uitext.Lang) (string, bool) {
	t.Helper()
	out, isErr, _ := slashOutput(input, a, nil, nil, slashReloads{}, nil, "", uitext.For(lang), nil)
	return out, isErr
}

// Showing must not claim a change, in either language, and must not
// move anything.
func TestReadonlyShowsWithoutClaimingAChange(t *testing.T) {
	transitions := []string{"戻りました", "again", "switched", "切り替えました"}
	for _, lang := range []uitext.Lang{uitext.JA, uitext.EN} {
		for _, c := range []sandbox.Ceiling{
			{}, {ReadOnly: true},
		} {
			a := readOnlyAgent(t, c)
			out := readOnlySlash(t, a, "/readonly", lang)
			want := "OFF"
			if c.ReadOnly {
				want = "ON"
			}
			if !strings.Contains(out, want) {
				t.Errorf("%v/%+v: the line does not name the ceiling: %q", lang, c, out)
			}
			for _, verb := range transitions {
				if strings.Contains(out, verb) {
					t.Errorf("%v/%+v: showing claims a change (%q): %q", lang, c, verb, out)
				}
			}
			if a.CeilingState() != c {
				t.Errorf("%v/%+v: showing moved the state to %+v", lang, c, a.CeilingState())
			}
		}
	}
}

// on and off move the ceiling.
func TestReadonlySetsTheCeiling(t *testing.T) {
	cases := []struct {
		input string
		start sandbox.Ceiling
		want  sandbox.Ceiling
	}{
		{"/readonly on", sandbox.Ceiling{}, sandbox.Ceiling{ReadOnly: true}},
		{"/readonly off", sandbox.Ceiling{ReadOnly: true}, sandbox.Ceiling{}},
	}
	for _, tc := range cases {
		a := readOnlyAgent(t, tc.start)
		readOnlySlash(t, a, tc.input, uitext.JA)
		if got := a.CeilingState(); got != tc.want {
			t.Errorf("%q from %+v → %+v, want %+v", tc.input, tc.start, got, tc.want)
		}
	}
}

// The ON line names the way back.
func TestReadonlyOnNamesTheWayBack(t *testing.T) {
	for _, lang := range []uitext.Lang{uitext.JA, uitext.EN} {
		a := readOnlyAgent(t, sandbox.Ceiling{ReadOnly: true})
		if out := readOnlySlash(t, a, "/readonly", lang); !strings.Contains(out, "/readonly off") {
			t.Errorf("%v: ON does not name the way back: %q", lang, out)
		}
	}
}

func TestReadonlyRejectsUnknownArgumentsWithoutChangingAnything(t *testing.T) {
	// Trailing words take one branch, unknown words the other. Both are
	// typos and both must answer like one.
	for _, input := range []string{"/readonly maybe", "/readonly on auto", "/readonly auto"} {
		start := sandbox.Ceiling{ReadOnly: true}
		a := readOnlyAgent(t, start)
		out, isErr := readOnlySlashErr(t, a, input, uitext.EN)
		if !strings.Contains(out, "/readonly on|off") {
			t.Errorf("%q: no usage line: %q", input, out)
		}
		// The same answer /auto gives a typo: the TUI dims the line.
		if !isErr {
			t.Errorf("%q was not reported as an error", input)
		}
		if a.CeilingState() != start {
			t.Errorf("%q moved the state to %+v", input, a.CeilingState())
		}
	}
}

// /auto through the shared slash handler — the plain REPL's path, which
// had no test at all. The TUI intercepts /auto, so nothing exercised
// this branch even though it is where the argument used to be ignored
// (independent review, 2026-09-09).
func TestAutoSlashSetsAndToggles(t *testing.T) {
	cases := []struct {
		input string
		start bool
		want  bool
		isErr bool
	}{
		{"/auto", false, true, false},
		{"/auto", true, false, false},
		{"/auto on", false, true, false},
		{"/auto on", true, true, false},
		{"/auto off", true, false, false},
		{"/auto off", false, false, false},
		{"/auto maybe", true, true, true},
	}
	for _, tc := range cases {
		a := readOnlyAgent(t, sandbox.Ceiling{})
		a.SetAutoApprove(tc.start)
		out, isErr, _ := slashOutput(tc.input, a, nil, nil, slashReloads{}, nil, "",
			uitext.For(uitext.EN), nil)
		if isErr != tc.isErr {
			t.Errorf("%q from %v: isErr = %v, want %v (%q)", tc.input, tc.start, isErr, tc.isErr, out)
		}
		if a.AutoApprove() != tc.want {
			t.Errorf("%q from %v: auto = %v, want %v", tc.input, tc.start, a.AutoApprove(), tc.want)
		}
		if tc.isErr {
			if !strings.Contains(out, "/auto on|off") {
				t.Errorf("%q: no usage line: %q", tc.input, out)
			}
			continue
		}
		want := "OFF"
		if tc.want {
			want = "ON"
		}
		if !strings.Contains(out, want) {
			t.Errorf("%q: the line does not confirm the state: %q", tc.input, out)
		}
	}
}

// The state line makes the same reservation the banner makes: with the
// lanes off the ceiling still refuses the file tools and a write- or
// operator-declaring shell call, but nothing bounds a command that
// declares the read lane, so "nothing outside the scratch changes" is
// not a sentence /readonly may print (independent review, pass 2).
func TestReadonlyOnDoesNotPromiseWhatTheLanesAreNotGiving(t *testing.T) {
	reg, err := tools.New(t.TempDir(), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	// --no-sandbox, as buildExecFn establishes it: a lane runner that
	// applies no cage and an empty Enforcement. A registry with no lane
	// runner at all reports *confined* (the unconfined floor is the
	// operator's explicit flag), so the earlier version of this test
	// could not reach the state it claimed to (second independent
	// review).
	reg.SetLaneExec(func(ctx context.Context, command string, lane sandbox.Lane) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/bash", "-c", command)
	}, sandbox.Enforcement{})
	if reg.Confined() {
		t.Fatal("fixture is still confined; it cannot exercise this")
	}
	a := readOnlyAgent(t, sandbox.Ceiling{ReadOnly: true})
	for _, lang := range []uitext.Lang{uitext.EN, uitext.JA} {
		out, isErr, _ := slashOutput("/readonly", a, reg, nil, slashReloads{}, nil, "", uitext.For(lang), nil)
		if isErr {
			t.Fatalf("%v: %q", lang, out)
		}
		if strings.Contains(out, "nothing outside") || strings.Contains(out, "スクラッチの外は変更しません") {
			t.Errorf("%v: claims the lanes' guarantee with the lanes off: %q", lang, out)
		}
		if !strings.Contains(out, "sandbox") {
			t.Errorf("%v: does not say why the guarantee is narrower: %q", lang, out)
		}
		// Still the way back, in both wordings.
		if !strings.Contains(out, "/readonly off") {
			t.Errorf("%v: no way back: %q", lang, out)
		}
	}
}
