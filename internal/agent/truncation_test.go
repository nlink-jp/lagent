package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/llm"
)

// scriptedBackend returns queued responses in order.
type scriptedBackend struct{ responses []*llm.Response }

func (b *scriptedBackend) ChatStream(ctx context.Context, system string, messages []llm.Message, tools []llm.ToolDef, onText func(string)) (*llm.Response, error) {
	r := b.responses[0]
	b.responses = b.responses[1:]
	if onText != nil && r.Content != "" {
		onText(r.Content)
	}
	return r, nil
}

// gem-agent ADR-0021 §8: a cut-off response with partial text is kept but
// reported — the empty guard alone let it pass as a complete answer.
func TestTruncatedResponseIsReported(t *testing.T) {
	backend := &scriptedBackend{responses: []*llm.Response{
		{Content: "partial answer that stops mid-", FinishReason: "length", PromptTokens: 10, OutputTokens: 5},
	}}
	var notices []string
	a := newLogAgent(t, nil, func(m string) { notices = append(notices, m) })
	a.backend = backend
	a.maxTurns = 3

	out, err := a.Run(context.Background(), "question", nil)
	if err != nil {
		t.Fatalf("Run: %v — the turn must still complete", err)
	}
	if !strings.Contains(out, "partial answer") {
		t.Errorf("partial content lost: %q", out)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "length") {
		t.Errorf("notices = %v — the cut-off must be reported with its reason", notices)
	}
}

// A normal STOP finish stays silent.
func TestCompleteResponseNotReported(t *testing.T) {
	backend := &scriptedBackend{responses: []*llm.Response{
		{Content: "complete", FinishReason: "stop", PromptTokens: 10, OutputTokens: 2},
	}}
	var notices []string
	a := newLogAgent(t, nil, func(m string) { notices = append(notices, m) })
	a.backend = backend
	a.maxTurns = 3
	if _, err := a.Run(context.Background(), "q", nil); err != nil {
		t.Fatal(err)
	}
	if len(notices) != 0 {
		t.Errorf("a STOP response produced notices: %v", notices)
	}
}

// gem-agent ADR-0021: an empty input is refused before it desyncs the transcript
// from the requests (buildContents would drop it from the request).
func TestEmptyInputRefused(t *testing.T) {
	log := &recordingLog{}
	a := newLogAgent(t, log, nil)
	if _, err := a.Run(context.Background(), "   \n ", nil); err == nil {
		t.Fatal("blank input accepted")
	}
	if len(log.kinds) != 0 {
		t.Errorf("blank input reached the transcript: %v", log.kinds)
	}
}

// A restored history arms first-round auto-compaction via the byte
