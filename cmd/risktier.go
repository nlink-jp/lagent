package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nlink-jp/lagent/internal/llm"
)

// windowAsker is the one call checkRiskModel needs from a backend.
type windowAsker interface {
	ContextWindow(ctx context.Context) (int, error)
}

// checkRiskModel refuses a [llm].risk_model mlx-serve does not list
// (ADR-0032 §4): mlx-serve answers any model name with the model it has
// loaded, so an unlisted judge would run on — and be recorded as — a
// model the operator did not name. Only an explicit risk_model on
// mlxserve is checked; a server that cannot be asked is not a startup
// failure, since every judgment then fails, escalates and says so.
func checkRiskModel(ctx context.Context, b windowAsker, provider, riskModel string) error {
	if provider != llm.ProviderMLXServe || riskModel == "" {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := b.ContextWindow(cctx); errors.Is(err, llm.ErrModelNotListed) {
		return fmt.Errorf("[llm].risk_model: %w", err)
	}
	return nil
}
