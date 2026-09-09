package cmd

import (
	"testing"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/session"
)

// capturingLog records the transcript writes of the cmd-side tools.
type capturingLog struct {
	kinds []string
	data  []any
}

func (c *capturingLog) Log(kind string, data any) error {
	c.kinds = append(c.kinds, kind)
	c.data = append(c.data, data)
	return nil
}

func (c *capturingLog) usage(t *testing.T) []session.UsageRecord {
	t.Helper()
	var out []session.UsageRecord
	for i, kind := range c.kinds {
		if kind != session.KindUsage {
			continue
		}
		r, ok := c.data[i].(session.UsageRecord)
		if !ok {
			t.Fatalf("record %d is %T, not a session.UsageRecord", i, c.data[i])
		}
		out = append(out, r)
	}
	return out
}

// gem-agent ADR-0057: a side call is a model call, so it leaves the same
// The one accounting shape, end to end: what the agent writes and what
// the tools write must be the same record, or an aggregator has to know
// which code path spent the tokens.
func TestUsageRecordShapeIsShared(t *testing.T) {
	log := &capturingLog{}
	logUsage(log, session.UsageMain, "light-model",
		llm.Usage{Prompt: 10, Output: 2, Thoughts: 3, Cached: 4, ToolPrompt: 5, Total: 20})
	recs := log.usage(t)
	if len(recs) != 1 {
		t.Fatalf("got %d records", len(recs))
	}
	want := session.UsageRecord{Source: session.UsageMain, Model: "light-model",
		Prompt: 10, Output: 2, Thoughts: 3, Cached: 4, ToolPrompt: 5, Total: 20}
	if recs[0] != want {
		t.Errorf("record = %+v, want %+v", recs[0], want)
	}
	// A call that spent nothing is not an accounting event.
	logUsage(log, session.UsageRisk, "main-model", llm.Usage{})
	if len(log.usage(t)) != 1 {
		t.Error("a zero-token call wrote a record")
	}
}
