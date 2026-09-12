package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"LAGENT_PROVIDER", "LAGENT_BASE_URL", "LAGENT_MODEL", "LAGENT_API_KEY"} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

// ADR-0008 §1 (revised): the scratch-cache table ships with Go's row,
// a file adds rows to it and removes one with an empty value, and the
// table refuses what is not a cache — loader variables, the variables
// laneEnv decides, and directories that are not one name.
func TestScratchCachesTable(t *testing.T) {
	clearEnv(t)
	t.Setenv("LAGENT_MODEL", "m")
	cfg, err := Load(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Sandbox.Caches(); len(got) != 1 || got["GOCACHE"] != "go-build" {
		t.Errorf("default table = %v", got)
	}
	path := writeConfig(t, "[sandbox.scratch_caches]\nGOCACHE = \"\"\nPIP_CACHE_DIR = \"pip\"\nUV_CACHE_DIR = \"uv\"\n")
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Sandbox.Caches(); len(got) != 2 || got["PIP_CACHE_DIR"] != "pip" || got["UV_CACHE_DIR"] != "uv" {
		t.Errorf("table with Go removed = %v", got)
	}
	if cfg.Source("sandbox.scratch_caches") != FromFile {
		t.Errorf("provenance = %q", cfg.Source("sandbox.scratch_caches"))
	}
	for _, bad := range []string{
		"DYLD_INSERT_LIBRARIES = \"x\"", "LD_PRELOAD = \"x\"", "TMPDIR = \"t\"", "PATH = \"p\"",
		"\"not a name\" = \"x\"", "GOCACHE = \"../out\"", "GOCACHE = \"a/b\"", "GOCACHE = \".\"",
	} {
		if _, err := Load(writeConfig(t, "[sandbox.scratch_caches]\n"+bad+"\n")); err == nil || !strings.Contains(err.Error(), "sandbox.scratch_caches") {
			t.Errorf("%s: accepted (err=%v)", bad, err)
		}
	}
}

func TestLoadFileWithDefaults(t *testing.T) {
	clearEnv(t)
	path := writeConfig(t, `
[llm]
model = "example-model"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Model != "example-model" {
		t.Errorf("model = %q", cfg.LLM.Model)
	}
	if cfg.LLM.Provider != "lmstudio" || cfg.LLM.BaseURL != "http://localhost:1234/v1" {
		t.Errorf("llm defaults = %+v, want lmstudio at the LM Studio port", cfg.LLM)
	}
	if !cfg.Sandbox.Enabled {
		t.Error("sandbox should default to enabled")
	}
	if cfg.Agent.MaxTurns != 50 || cfg.Agent.ShellTimeoutSec != 120 {
		t.Errorf("agent defaults = %+v", cfg.Agent)
	}
	if !cfg.MCP.Enabled || cfg.MCP.CallTimeoutSec != 60 {
		t.Errorf("mcp defaults = %+v", cfg.MCP)
	}
	if cfg.TUI.Theme != "auto" {
		t.Errorf("tui theme default = %q, want auto", cfg.TUI.Theme)
	}
}

func TestInvalidThemeRejected(t *testing.T) {
	clearEnv(t)
	path := writeConfig(t, `
[llm]
model = "m"

[tui]
theme = "solarized"
`)
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "[tui].theme") {
		t.Fatalf("invalid theme should be rejected, got %v", err)
	}
}

func TestEnvPrecedence(t *testing.T) {
	clearEnv(t)
	path := writeConfig(t, `
[llm]
model = "file-model"
base_url = "http://file:1/v1"
`)
	// LAGENT_* beats file.
	t.Setenv("LAGENT_MODEL", "tool-model")
	t.Setenv("LAGENT_BASE_URL", "http://env:2/v1")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Model != "tool-model" {
		t.Errorf("LAGENT_MODEL should beat file: got %q", cfg.LLM.Model)
	}
	if cfg.LLM.BaseURL != "http://env:2/v1" {
		t.Errorf("LAGENT_BASE_URL should beat file: got %q", cfg.LLM.BaseURL)
	}
}

func TestMissingFileEnvOnly(t *testing.T) {
	clearEnv(t)
	t.Setenv("LAGENT_MODEL", "env-model")
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Model != "env-model" {
		t.Errorf("env-only config = %+v", cfg)
	}
}

func TestFlagOverrideBeatsEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("LAGENT_MODEL", "env-model")
	cfg, err := LoadWithOverrides(filepath.Join(t.TempDir(), "nonexistent.toml"), Overrides{Model: "flag-model"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Model != "flag-model" {
		t.Errorf("flag should beat env: got %q", cfg.LLM.Model)
	}
}

// gem-agent ADR-0039 §5: --mcp on|off overrides [mcp].enabled at the top of the
// precedence, with flag provenance; anything else is a loud error.
func TestMCPFlagOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("LAGENT_MODEL", "m")
	path := filepath.Join(t.TempDir(), "nonexistent.toml")

	cfg, err := LoadWithOverrides(path, Overrides{MCP: "off"})
	if err != nil || cfg.MCP.Enabled {
		t.Errorf("--mcp off: enabled=%v err=%v", cfg != nil && cfg.MCP.Enabled, err)
	}
	if cfg.Source("mcp.enabled") != FromFlag {
		t.Errorf("provenance = %v, want flag", cfg.Source("mcp.enabled"))
	}
	cfg, err = LoadWithOverrides(path, Overrides{MCP: "on"})
	if err != nil || !cfg.MCP.Enabled {
		t.Errorf("--mcp on: err=%v", err)
	}
	if _, err = LoadWithOverrides(path, Overrides{MCP: "maybe"}); err == nil ||
		!strings.Contains(err.Error(), "--mcp") {
		t.Errorf("invalid --mcp value: %v", err)
	}
}

// --auto arms auto-approve with flag provenance; not passing it leaves
// the config value alone (the flag is one-way — gem-agent ADR-0053).
func TestAutoFlagOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("LAGENT_MODEL", "m")
	path := filepath.Join(t.TempDir(), "nonexistent.toml")

	cfg, err := LoadWithOverrides(path, Overrides{Auto: true})
	if err != nil || !cfg.Agent.AutoApprove {
		t.Errorf("--auto: auto_approve=%v err=%v", cfg != nil && cfg.Agent.AutoApprove, err)
	}
	if cfg.Source("agent.auto_approve") != FromFlag {
		t.Errorf("provenance = %v, want flag", cfg.Source("agent.auto_approve"))
	}
	cfg, err = LoadWithOverrides(path, Overrides{})
	if err != nil || cfg.Agent.AutoApprove {
		t.Errorf("flag not given must leave the default: auto_approve=%v err=%v",
			cfg != nil && cfg.Agent.AutoApprove, err)
	}
	if cfg.Source("agent.auto_approve") == FromFlag {
		t.Error("flag provenance recorded without the flag")
	}
}

func TestUnknownKeyIsError(t *testing.T) {
	clearEnv(t)
	path := writeConfig(t, `
[llm]
model = "m"
base_ur = "typo"
`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("unknown key should be a strict-decode error, got: %v", err)
	}
}

func TestMissingRequiredFields(t *testing.T) {
	clearEnv(t)
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err == nil {
		t.Fatal("a missing model should be an error")
	}
	if !strings.Contains(err.Error(), "[llm].model") {
		t.Errorf("error should name the missing field: %v", err)
	}
}

func TestLLMSectionIsValidated(t *testing.T) {
	clearEnv(t)
	for name, body := range map[string]string{
		"unknown provider":      "[llm]\nmodel = \"m\"\nprovider = \"vertex\"\n",
		"relative base":         "[llm]\nmodel = \"m\"\nbase_url = \"localhost:1234/v1\"\n",
		"empty base":            "[llm]\nmodel = \"m\"\nbase_url = \"\"\n",
		"openai needs a window": "[llm]\nmodel = \"m\"\nprovider = \"openai\"\n",
	} {
		if _, err := Load(writeConfig(t, body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	cfg, err := Load(writeConfig(t, "[llm]\nmodel = \"m\"\nprovider = \"openai\"\napi_key = \"k\"\n[model]\ncontext_window = 8192\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Provider != "openai" || cfg.LLM.APIKey != "k" || cfg.Model.ContextWindow != 8192 {
		t.Errorf("config = %+v", cfg.LLM)
	}
}

func TestLoadProjectReadsApprovalPolicyOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ProjectFileName), []byte(`
[approval.tools]
"write_file" = "always"
"mcp__tor-exit-lookup__*" = "never"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Approval.Tools["write_file"] != "always" ||
		cfg.Approval.Tools["mcp__tor-exit-lookup__*"] != "never" {
		t.Errorf("tools = %v", cfg.Approval.Tools)
	}
}

// A project file must not be able to reach settings that belong to the
// operator — the model, credentials, the sandbox switch.
func TestProjectFileRejectsAnythingButPolicy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ProjectFileName), []byte(`
[sandbox]
enabled = false

[approval.tools]
"write_file" = "always"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProject(dir)
	if err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("a project file disabled the sandbox: err = %v", err)
	}
}

func TestLoadProjectMissingFileIsNotAnError(t *testing.T) {
	cfg, err := LoadProject(t.TempDir())
	if err != nil || len(cfg.Approval.Tools) != 0 {
		t.Errorf("LoadProject on a project with no file: %+v %v", cfg, err)
	}
}

func TestTrustedProjectsMatchesExactPaths(t *testing.T) {
	clearEnv(t)
	path := writeConfig(t, `
[llm]
model = "m"
[approval]
trusted_projects = ["/work/mine"]
[approval.tools]
"shell_exec" = "always"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TrustsProject("/work/mine") {
		t.Error("listed project not trusted")
	}
	// No prefix matching: /work/mine-evil must not inherit the trust of
	// /work/mine.
	for _, other := range []string{"/work/mine-evil", "/work/mine/sub", "/work", ""} {
		if cfg.TrustsProject(other) {
			t.Errorf("TrustsProject(%q) = true", other)
		}
	}
	if cfg.Approval.Tools["shell_exec"] != "always" {
		t.Errorf("approval tools = %v", cfg.Approval.Tools)
	}
}

// Four precedence layers with nothing on screen assumes the operator
// remembers them. /settings shows this instead (gem-agent ADR-0009).
func TestConfigRecordsWhereEachValueCameFrom(t *testing.T) {
	clearEnv(t)
	path := writeConfig(t, `
[llm]
model = "file-model"
[agent]
max_turns = 7
`)
	t.Setenv("LAGENT_BASE_URL", "http://env:2/v1")
	cfg, err := LoadWithOverrides(path, Overrides{Model: "flag-model"})
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"llm.model":               FromFlag,
		"llm.base_url":            FromEnv + ":LAGENT_BASE_URL",
		"agent.max_turns":         FromFile,
		"agent.shell_timeout_sec": FromDefault,
		"tui.theme":               FromDefault,
	} {
		if got := cfg.Source(key); got != want {
			t.Errorf("Source(%q) = %q, want %q", key, got, want)
		}
	}
}

// Hooks entries parse from [[hooks.pre_tool_use]] and are validated:
// gem-agent ADR-0072 §4.5: a project config is untrusted input read before the
// trust prompt; an oversized one is refused, never parsed.
func TestLoadProjectRefusesOversizeFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ProjectFileName), make([]byte, ProjectFileCap+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProject(dir); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("oversize project config accepted: %v", err)
	}
}

// [mcp].advertise is deferred by default, all is the baseline, anything
// else is refused; preload is a plain list.
func TestMCPAdvertiseAndPreload(t *testing.T) {
	clearEnv(t)
	cfg, err := Load(writeConfig(t, "[llm]\nmodel = \"m\"\n"))
	if err != nil || cfg.MCP.Advertise != "deferred" || len(cfg.MCP.Preload) != 0 {
		t.Fatalf("defaults: %+v err=%v", cfg.MCP, err)
	}
	cfg, err = Load(writeConfig(t, "[llm]\nmodel = \"m\"\n[mcp]\nadvertise = \"all\"\npreload = [\"tor-exit-lookup\", \"whois-lookup\"]\n"))
	if err != nil || cfg.MCP.Advertise != "all" || len(cfg.MCP.Preload) != 2 {
		t.Fatalf("explicit: %+v err=%v", cfg.MCP, err)
	}
	if cfg.Source("mcp.advertise") != FromFile || cfg.Source("mcp.preload") != FromFile {
		t.Errorf("provenance: %q %q", cfg.Source("mcp.advertise"), cfg.Source("mcp.preload"))
	}
	if _, err := Load(writeConfig(t, "[llm]\nmodel = \"m\"\n[mcp]\nadvertise = \"lazy\"\n")); err == nil || !strings.Contains(err.Error(), "[mcp].advertise") {
		t.Errorf("unknown advertise accepted: %v", err)
	}
}
