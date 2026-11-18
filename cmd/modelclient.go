package cmd

import (
	"context"

	"github.com/nlink-jp/lagent/internal/config"
	"github.com/nlink-jp/lagent/internal/llm"
)

// modelClient is what a session needs of a model client: the chat call,
// the context window lookup, the stream observer and the reasoning
// effort (ADR-0054 §2.4). *llm.OpenAI is one as it stands; a test that
// runs the whole startup passes a scripted one.
type modelClient interface {
	llm.Backend
	ContextWindow(ctx context.Context) (int, error)
	SetObserver(fn func(llm.StreamEvent))
	SetReasoningEffort(effort string)
}

// newModelClient builds the client for one model name on the configured
// server: the main model's and, with the model tier on, the tier's. It is
// the one seam the startup has for a test: the characterization tests
// replace it with a scripted model and nothing else. On an error it
// returns a nil interface, never a typed nil inside one.
var newModelClient = func(cfg *config.Config, model string) (modelClient, error) {
	c, err := llm.NewOpenAI(cfg.LLM.BaseURL, model, cfg.LLM.APIKey, cfg.LLM.Provider)
	if err != nil {
		return nil, err
	}
	return c, nil
}
