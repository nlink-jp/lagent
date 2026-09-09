package agent

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/tools"
	"github.com/nlink-jp/lagent/internal/uitext"
)

// Nothing wired means English sentences, not empty format strings —
// every test in this package constructs an Agent without a catalog.
func TestNoCatalogMeansEnglish(t *testing.T) {
	reg, err := tools.New(t.TempDir(),
		func(ctx context.Context, command string) *exec.Cmd {
			return exec.CommandContext(ctx, "/bin/true")
		}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	a := New(Options{Registry: reg, Gate: &approveAll{}, MaxTurns: 1})
	if a.msgs == nil || a.msgs.TruncatedFmt == "" {
		t.Fatal("an agent with no catalog has no notice text")
	}
	if !strings.Contains(a.msgs.RemoteFaultFmt, "/mcp reload") {
		t.Errorf("the remote-fault notice lost its command: %q", a.msgs.RemoteFaultFmt)
	}
}

// The round limit and the loop guard are different events. A loop waved
// through, announced as a round count, told the operator nothing about
// the repeat that triggered it — and the number it printed was the hard
// cap, which reads as headroom the turn does not have.
func TestContinuedNoticeNamesItsTrigger(t *testing.T) {
	a := &Agent{msgs: uitext.For(uitext.EN)}

	limit := a.continuedNotice("round-limit", "", 20)
	if !strings.Contains(limit, "20") || strings.Contains(limit, "repeated") {
		t.Errorf("round-limit notice = %q", limit)
	}
	loop := a.continuedNotice("loop", "shell_exec: ls -la", 20)
	if !strings.Contains(loop, "repeated") || !strings.Contains(loop, "ls -la") {
		t.Errorf("loop notice = %q — it must name the call that repeated", loop)
	}
	if strings.Contains(loop, "20") {
		t.Errorf("loop notice quotes a round number that did not trigger it: %q", loop)
	}
}
