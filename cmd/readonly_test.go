package cmd

import (
	"testing"

	"github.com/nlink-jp/lagent/internal/config"
	"github.com/nlink-jp/lagent/internal/sandbox"
)

// --writable and --read-only are the two ends of one setting: one
// resolves, neither means "not given", both is a contradiction.
func TestReadOnlyOverrideResolvesOneSetting(t *testing.T) {
	for _, tc := range []struct {
		writable, readOnly bool
		want               string
		wantErr            bool
	}{
		{false, false, "", false},
		{true, false, "off", false},
		{false, true, "on", false},
		{true, true, "", true},
	} {
		got, err := readOnlyOverride(tc.writable, tc.readOnly)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("readOnlyOverride(%v, %v) = %q, %v; want %q, err=%v", tc.writable, tc.readOnly, got, err, tc.want, tc.wantErr)
		}
	}
}

// The ceiling the run starts with is the configured one, whatever the
// mode: one-shot has nobody to lift it, and that is the point.
func TestEffectiveCeiling(t *testing.T) {
	for _, ro := range []bool{false, true} {
		if got := effectiveCeiling(config.AgentConfig{ReadOnly: ro}); got != (sandbox.Ceiling{ReadOnly: ro}) {
			t.Errorf("effectiveCeiling(read_only=%v) = %+v", ro, got)
		}
	}
}
