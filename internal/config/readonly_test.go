package config

// The ceiling and its watcher are two settings, not one tri-state
// (ADR-0080 §1), so they load, default and override independently.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const cfgBase = "[llm]\nmodel = \"m\"\n"

func TestReadOnlyConfigDefaultsOff(t *testing.T) {
	cfg, err := LoadWithOverrides(writeCfg(t, cfgBase), Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.ReadOnly {
		t.Errorf("default = %v, want off", cfg.Agent.ReadOnly)
	}
}

// All four combinations load, including the one a tri-state could not
// express: watching while already read-only.
func TestReadOnlyConfigLoads(t *testing.T) {
	for _, ro := range []bool{false, true} {
		body := cfgBase + "[agent]\nread_only = " + boolLit(ro) + "\n"
		cfg, err := LoadWithOverrides(writeCfg(t, body), Overrides{})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Agent.ReadOnly != ro {
			t.Errorf("read_only=%v → %v", ro, cfg.Agent.ReadOnly)
		}
		if cfg.Sources["agent.read_only"] != FromFile {
			t.Errorf("provenance = %q, want %q", cfg.Sources["agent.read_only"], FromFile)
		}
	}
}

func boolLit(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// The ceiling override runs both ways — --writable is how a run steps
// out of a configured ceiling, which is what makes the loosening
// visible on the invocation — and it does not touch the watcher.
func TestReadOnlyOverrideRunsBothWays(t *testing.T) {
	body := cfgBase + "[agent]\nread_only = true\n"
	cfg, err := LoadWithOverrides(writeCfg(t, body), Overrides{ReadOnly: "off"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.ReadOnly {
		t.Error("the ceiling override did not lower the ceiling")
	}
	if cfg.Sources["agent.read_only"] != FromFlag {
		t.Errorf("ceiling provenance = %q", cfg.Sources["agent.read_only"])
	}
	cfg, err = LoadWithOverrides(writeCfg(t, cfgBase), Overrides{ReadOnly: "on"})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Agent.ReadOnly {
		t.Error("--read-only did not raise the ceiling")
	}
}

// Both overrides refuse a value they do not understand, and each says
// which one it was: the watcher's branch had no test, so a typo there
// could have silently armed nothing (independent review, pass 2).
func TestReadOnlyRejectsAnUnknownOverride(t *testing.T) {
	for _, tc := range []struct {
		name string
		ov   Overrides
		want string
	}{
		{"ceiling", Overrides{ReadOnly: "maybe"}, "read-only override"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadWithOverrides(writeCfg(t, cfgBase), tc.ov)
			if err == nil {
				t.Fatalf("accepted an unknown override: %+v", cfg.Agent)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error does not name the setting: %v", err)
			}
			if !strings.Contains(err.Error(), `"maybe"`) {
				t.Errorf("error does not quote what was passed: %v", err)
			}
		})
	}
}
