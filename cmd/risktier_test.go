package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/llm"
)

type fakeWindow struct{ err error }

func (f fakeWindow) ContextWindow(context.Context) (int, error) { return 1, f.err }

func TestCheckRiskModel(t *testing.T) {
	notListed := fmt.Errorf("llm: mlx-serve does not list model \"x\": %w", llm.ErrModelNotListed)
	if err := checkRiskModel(context.Background(), fakeWindow{notListed}, "mlxserve", "x"); err == nil || !strings.Contains(err.Error(), "[llm].risk_model") {
		t.Errorf("an unlisted judge on mlx-serve must stop startup: %v", err)
	}
	if err := checkRiskModel(context.Background(), fakeWindow{errors.New("connection refused")}, "mlxserve", "x"); err != nil {
		t.Errorf("a server that cannot be asked is not a startup failure: %v", err)
	}
	if err := checkRiskModel(context.Background(), fakeWindow{notListed}, "mlxserve", ""); err != nil {
		t.Errorf("no risk_model, nothing to check: %v", err)
	}
	if err := checkRiskModel(context.Background(), fakeWindow{notListed}, "lmstudio", "x"); err != nil {
		t.Errorf("only mlx-serve answers unknown names: %v", err)
	}
}

// ADR-0032 §4: the judge's reasoning_effort and the tier switch must
// reach the agent, or the config keys are decoration.
func TestModelTierIsWired(t *testing.T) {
	src := wiringSource(t)
	for _, want := range []string{
		"rb.SetReasoningEffort(cfg.LLM.RiskReasoningEffort)",
		`if cfg.Approval.ModelTier == "shell"`,
		// Read through the models phase's result since ADR-0054.
		"ModelTier:        md.riskBackend != nil",
		"Rulebook:         md.rulebook",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the session wiring lacks %q", want)
		}
	}
}
