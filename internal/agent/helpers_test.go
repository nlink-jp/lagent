package agent

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/tools"
)

// capturingLog records every transcript record kind with its payload.
type capturingLog struct {
	kinds []string
	data  []any
}

func (c *capturingLog) Log(kind string, data any) error {
	c.kinds = append(c.kinds, kind)
	c.data = append(c.data, data)
	return nil
}

// bashAgent is an agent over a registry whose shell runs real bash —
// the shape the purpose and loop-signature tests need.
func bashAgent(t *testing.T, mb *mockBackend, gate Approver) (*Agent, *tools.Registry) {
	t.Helper()
	reg, err := tools.New(t.TempDir(),
		func(ctx context.Context, command string) *exec.Cmd {
			return exec.CommandContext(ctx, "/bin/bash", "-c", command)
		}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return New(Options{
		Backend: mb, Registry: reg, Gate: gate,
		System: "test system", MaxTurns: 5,
	}), reg
}
