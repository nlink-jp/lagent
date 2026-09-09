package cmd

// The optional half of the approval interface is optional so the test
// gates need not carry it — which means a production gate that forgets
// it silently offers "always" answers the ceiling refuses to honour.
// Nothing in the type system catches that, so this does (independent
// review, pass 2).

import (
	"testing"

	"github.com/nlink-jp/lagent/internal/agent"
	"github.com/nlink-jp/lagent/internal/approve"
	"github.com/nlink-jp/lagent/internal/tui"
)

// Compile-time first: a gate that loses the method is a build error,
// not a test failure, and the test below then only has to say which
// gates the list is meant to cover.
var (
	_ agent.OnceApprover = (*approve.Gate)(nil)
	_ agent.OnceApprover = (*tui.Gate)(nil)
	_ agent.OnceApprover = denyGate{}
)

func TestEveryGateTheBinaryUsesCanAskOnce(t *testing.T) {
	gates := map[string]agent.Approver{
		"plain REPL": approve.New(nil, nil),
		"TUI":        tui.NewGate(),
		"one-shot":   denyGate{},
	}
	for name, g := range gates {
		if _, ok := g.(agent.OnceApprover); !ok {
			t.Errorf("%s gate does not implement OnceApprover: a call the ceiling "+
				"cannot bound would be asked with 'a' and 'p' on offer", name)
		}
	}
}
