package agent

import (
	"testing"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/session"
)

// usageRecords picks the accounting records out of a captured transcript.
func usageRecords(t *testing.T, log *capturingLog) []session.UsageRecord {
	t.Helper()
	var out []session.UsageRecord
	for i, kind := range log.kinds {
		if kind != session.KindUsage {
			continue
		}
		r, ok := log.data[i].(session.UsageRecord)
		if !ok {
			t.Fatalf("record %d is %T, not a session.UsageRecord", i, log.data[i])
		}
		out = append(out, r)
	}
	return out
}

// ADR-0057: a model call that leaves no record cannot be priced later —
// Compaction is the other spend that used to die with the process.
// A call that spent nothing is not an accounting event — a mock or a
// failed call must not pad the transcript with zero rows.
func TestZeroSpendWritesNoRecord(t *testing.T) {
	mb := &mockBackend{}
	_, reg := newAgent(t, mb, &approveAll{}, 5)
	log := &capturingLog{}
	a := New(Options{Backend: mb, Registry: reg, Gate: &approveAll{}, Log: log, Model: "gemini-test"})
	a.logUsage(session.UsageMain, llm.Usage{})
	if len(usageRecords(t, log)) != 0 {
		t.Error("a zero-token call wrote an accounting record")
	}
}
