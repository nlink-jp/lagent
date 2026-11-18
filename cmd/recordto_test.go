package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/session"
)

// /clear reported the old session's persistent-file changes through
// curLog.Log; with no startup transcript curLog was nil and the call
// panicked (ADR-0054, found while reading). The callback it uses now
// records nothing when there is no transcript.
func TestRecordToAbsentTranscriptRecordsNothing(t *testing.T) {
	var none *session.Logger
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("recording to an absent transcript panicked: %v", r)
		}
	}()
	if err := recordTo(none)("persistent_changes", map[string]any{"reason": "clear"}); err != nil {
		t.Fatalf("absent transcript: want nil, got %v", err)
	}
}

// The panic the callback exists to prevent, shown on the type itself.
func TestNilLoggerMethodValuePanics(t *testing.T) {
	var none *session.Logger
	defer func() {
		if recover() == nil {
			t.Fatal("a nil *session.Logger's Log no longer panics — the guard may be obsolete; revisit ADR-0054's note")
		}
	}()
	_ = none.Log("persistent_changes", nil)
}

func TestRecordToPresentTranscriptWrites(t *testing.T) {
	lg, err := openSessionLog(t.TempDir(), "", t.TempDir(), "local-x", "v0.0.0-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := recordTo(lg)("persistent_changes", map[string]any{"reason": "clear"}); err != nil {
		t.Fatal(err)
	}
	path := lg.Path()
	if err := lg.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"persistent_changes"`) {
		t.Fatalf("record not written: %s", raw)
	}
}
