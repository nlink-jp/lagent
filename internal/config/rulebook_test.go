package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRulebook(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	if text, clipped, err := LoadRulebook(cfg); err != nil || text != "" || clipped {
		t.Fatalf("a missing rulebook is normal: %q %v %v", text, clipped, err)
	}
	if err := os.WriteFile(RulebookPath(cfg), []byte("- `go test` は承認してよい\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if text, clipped, err := LoadRulebook(cfg); err != nil || !strings.Contains(text, "go test") || clipped {
		t.Fatalf("got %q %v %v", text, clipped, err)
	}
	long := strings.Repeat("規", RulebookCap+10)
	if err := os.WriteFile(RulebookPath(cfg), []byte(long), 0o600); err != nil {
		t.Fatal(err)
	}
	text, clipped, err := LoadRulebook(cfg)
	if err != nil || !clipped || len([]rune(text)) != RulebookCap {
		t.Fatalf("clip by runes: %d %v %v", len([]rune(text)), clipped, err)
	}
}

func TestModelTierIsValidatedAndOffByDefault(t *testing.T) {
	clearEnv(t)
	cfg, err := Load(writeConfig(t, "[llm]\nmodel = \"m\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Approval.ModelTier != "off" || cfg.LLM.RiskReasoningEffort != "none" || cfg.LLM.RiskModel != "" {
		t.Errorf("defaults: tier %q effort %q model %q", cfg.Approval.ModelTier, cfg.LLM.RiskReasoningEffort, cfg.LLM.RiskModel)
	}
	cfg, err = Load(writeConfig(t, "[llm]\nmodel = \"m\"\nrisk_model = \"j\"\nrisk_reasoning_effort = \"\"\n[approval]\nmodel_tier = \"shell\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Approval.ModelTier != "shell" || cfg.LLM.RiskModel != "j" || cfg.LLM.RiskReasoningEffort != "" {
		t.Errorf("set: %+v %+v", cfg.Approval, cfg.LLM)
	}
	if _, err := Load(writeConfig(t, "[llm]\nmodel = \"m\"\n[approval]\nmodel_tier = \"all\"\n")); err == nil {
		t.Error("model_tier = all accepted: MCP is not in the tier's scope (ADR-0032)")
	}
}

// A checked-out repository cannot turn the judge on: the project file
// has no such key, and unknown keys are refused (ADR-0032 §3).
func TestProjectFileCannotSetModelTier(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ProjectFileName), []byte("[approval]\nmodel_tier = \"shell\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProject(dir); err == nil {
		t.Fatal("a project file set [approval].model_tier")
	}
}
