// Package config loads lagent configuration following the org-standard
// Vertex AI schema: ~/.config/lagent/config.toml with env precedence
// LAGENT_* > GOOGLE_CLOUD_* > config file > built-in defaults.
//
// Ported from gem-agent internal/config at be7609980022e38314268c58ca94a6517e6f5d28 (v0.74.0), ADR-0001.
package config

import (
	"github.com/nlink-jp/lagent/internal/bounded"
	"net/url"

	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the lagent configuration.
type Config struct {
	LLM      LLMConfig      `toml:"llm"`
	Model    ModelConfig    `toml:"model"`
	Sandbox  SandboxConfig  `toml:"sandbox"`
	Agent    AgentConfig    `toml:"agent"`
	MCP      MCPConfig      `toml:"mcp"`
	TUI      TUIConfig      `toml:"tui"`
	Approval ApprovalConfig `toml:"approval"`

	// Sources records where each setting's effective value came from,
	// keyed by its TOML path ("model.name"). Four precedence layers with
	// nothing on screen is a design that assumes the operator remembers
	// them; /settings shows this instead (ADR-0009).
	Sources map[string]string `toml:"-"`
}

// Provenance values used in Sources.
const (
	FromFlag    = "flag"
	FromEnv     = "env"
	FromFile    = "config.toml"
	FromDefault = "default"
)

func (c *Config) note(key, source string) {
	if c.Sources == nil {
		c.Sources = map[string]string{}
	}
	c.Sources[key] = source
}

// Source returns where a setting came from, for display.
func (c *Config) Source(key string) string {
	if s, ok := c.Sources[key]; ok {
		return s
	}
	return FromDefault
}

// ApprovalConfig carries the per-tool approval policy (ADR-0008).
type ApprovalConfig struct {
	// Tools maps a tool name — or a trailing-wildcard prefix such as
	// "mcp__tor-exit-lookup__*" — to "always" or "never".
	Tools map[string]string `toml:"tools"`
	// TrustedProjects lists project directories whose own
	// .lagent.toml may REMOVE approvals. Everywhere else a project
	// file may only add them: a directory's contents are not necessarily
	// written by the operator, and cloning a repository must not be able
	// to switch the gate off (ADR-0008 §4).
	TrustedProjects []string `toml:"trusted_projects"`
	// PinTrustedFiles keys project trust on content (ADR-0074): the
	// agent-facing files are digested when trusted and a changed one
	// asks again before it is loaded. Default true; false restores the
	// ADR-0023 behaviour (a trusted directory stays trusted whatever
	// its files come to contain).
	PinTrustedFiles bool `toml:"pin_trusted_files"`
}

// ProjectConfig is <project>/.lagent.toml — the project-scoped half
// of ADR-0008. Deliberately tiny: it carries policy, nothing else. Model
// names, credentials and sandbox settings stay in the operator's own
// config, where a checked-out repository cannot reach them.
type ProjectConfig struct {
	Approval ProjectApproval `toml:"approval"`
	MCP      ProjectMCP      `toml:"mcp"`
}

// ProjectMCP is the project file's [mcp] table. It carries `exclude`
// and nothing else: a project may only REMOVE tools from the session
// (ADR-0077 §2), which is why it needs no trust condition — narrowing
// is the only composition available to it.
type ProjectMCP struct {
	Exclude []string `toml:"exclude"`
}

// ProjectApproval is the project file's [approval] table.
type ProjectApproval struct {
	Tools map[string]string `toml:"tools"`
}

// ProjectFileName is the project-scoped config file.
const ProjectFileName = ".lagent.toml"

// LoadProject reads <dir>/.lagent.toml. A missing file is not an
// error. Unknown keys are, for the same reason as in the main config:
// a policy that does not do what it says is worse than no policy.
func LoadProject(dir string) (*ProjectConfig, error) {
	path := filepath.Join(dir, ProjectFileName)
	var cfg ProjectConfig
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	// Untrusted input read before the trust prompt (ADR-0072 §4.5): a
	// bounded read, then the decode — never the file whole.
	raw, err := readCapped(path, ProjectFileCap)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	md, err := toml.Decode(string(raw), &cfg)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return nil, fmt.Errorf("unknown key(s) in %s: %s (this file carries [approval.tools] and [mcp].exclude only)",
			path, strings.Join(keys, ", "))
	}
	return &cfg, nil
}

// TrustsProject reports whether the operator listed dir as a project
// whose own policy file may remove approvals. Both sides are compared
// as resolved paths (ADR-0021): dir arrives symlink-resolved (/tmp is
// /private/tmp on macOS), so raw string equality made a trust entry
// under a symlinked path silently never match — and the startup note
// told the operator to add exactly the path that would not work.
func (c *Config) TrustsProject(dir string) bool {
	canon := canonicalPath(dir)
	for _, p := range c.Approval.TrustedProjects {
		if canonicalPath(expandHome(p)) == canon {
			return true
		}
	}
	return false
}

// canonicalPath cleans and symlink-resolves; a path that does not
// resolve (not yet existing) falls back to the cleaned form.
func canonicalPath(p string) string {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// expandHome resolves a leading ~ so trusted_projects entries may be
// written the way operators write paths.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// TUIConfig controls the interactive UI appearance.
type TUIConfig struct {
	// Theme: "auto" (detect background before startup), "dark", "light",
	// or "plain" (no colors at all — the escape hatch for terminal
	// themes that fight any styling).
	Theme string `toml:"theme"`
	// Language: "auto" (LC_ALL → LC_MESSAGES → LANG, POSIX-style),
	// "ja", or "en" — the language of the interactive chrome
	// (ADR-0029). Resolved once at startup.
	Language string `toml:"language"`
	// ShowThoughts streams the model's reasoning deltas, when the
	// server sends them, into the live area. Display-only — never
	// stored or replayed.
	ShowThoughts bool `toml:"show_thoughts"`
}

// MCPConfig controls the MCP client. Server definitions live in the
// project's .mcp.json (Claude Code format), not here — drop-in
// compatibility is the point.
type MCPConfig struct {
	Enabled        bool `toml:"enabled"`
	CallTimeoutSec int  `toml:"call_timeout_sec"`
	// Exclude names MCP servers, or single functions of them
	// ("obsidian/patch_vault_file"), that this session does not have
	// (ADR-0077). An excluded server is never started. Everything not
	// named here is declared, so an operator who sets nothing sees
	// today's behaviour and .mcp.json keeps the meaning it has.
	Exclude []string `toml:"exclude"`
}

// LLMConfig names the local server and the model. Every conversation
// goes through the OpenAI-compatible chat/completions endpoint at
// BaseURL; Provider selects only where the context length is detected
// from (LM Studio's /api/v0/models, Ollama's /api/show, or nowhere —
// "openai" needs [model].context_window).
type LLMConfig struct {
	Provider string `toml:"provider"`
	BaseURL  string `toml:"base_url"`
	// Model is the id the server lists. Always config-driven: there is
	// no built-in default, and hardcoding one anywhere else is a bug.
	Model string `toml:"model"`
	// APIKey is sent as a bearer token when set. Local servers need
	// none; the field exists for another OpenAI-compatible server.
	APIKey string `toml:"api_key"`
}

// ModelConfig holds what is known about the model itself.
type ModelConfig struct {
	// ContextWindow is the context window in tokens, shown in the TUI
	// footer. 0 (default) asks the provider at startup.
	ContextWindow int `toml:"context_window"`
}

// SandboxConfig controls the sandbox-exec wrapper for shell_exec.
type SandboxConfig struct {
	Enabled bool `toml:"enabled"`
	// ReadLaneDenyExec adds programs the read lane may not launch, by
	// name or absolute path, to sandbox.DefaultDenyExec (ADR-0073).
	ReadLaneDenyExec []string `toml:"read_lane_deny_exec"`
	// ReadLanePrompts keeps the approval prompt for read-lane commands
	// (ADR-0073 §5 — the opt-out from "runs unasked"): the kernel cage
	// still applies, the operator is asked as for any other call.
	ReadLanePrompts bool `toml:"read_lane_prompts"`
}

// AgentConfig holds agent-loop tunables.
type AgentConfig struct {
	MaxTurns        int `toml:"max_turns"`
	ShellTimeoutSec int `toml:"shell_timeout_sec"`
	// AutoApprove starts sessions in auto-approve mode (ADR-0004).
	// Default false: weakening the primary defense is opt-in.
	AutoApprove bool `toml:"auto_approve"`
	// ReadOnly starts the session with its lane ceiling in force
	// (ADR-0080 §1), on an axis of its own: auto-approve decides who
	// answers the gate, this decides what the session may reach at all.
	ReadOnly bool `toml:"read_only"`
}

// DefaultPath returns the org-standard per-tool config path.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "lagent", "config.toml"), nil
}

func defaults() Config {
	return Config{
		LLM:      LLMConfig{Provider: "lmstudio", BaseURL: "http://localhost:1234/v1"},
		Sandbox:  SandboxConfig{Enabled: true},
		Approval: ApprovalConfig{PinTrustedFiles: true},
		Agent:    AgentConfig{MaxTurns: 50, ShellTimeoutSec: 120},
		MCP:      MCPConfig{Enabled: true, CallTimeoutSec: 60},
		TUI:      TUIConfig{Theme: "auto", Language: "auto", ShowThoughts: true},
	}
}

// Overrides carries CLI-flag values, which sit at the top of the
// precedence order: flags > LAGENT_* > file > defaults.
type Overrides struct {
	Model string
	// MCP overrides [mcp].enabled for this run: "on" or "off"
	// (ADR-0039). "off" is the one-shot pipeline case — no server
	// child is spawned; "on" forces MCP against a config that
	// disables it. Empty means the flag was not given.
	MCP string
	// ReadOnly overrides [agent].read_only for this run (ADR-0080):
	// "on" or "off", empty when neither flag was given. Unlike Auto this
	// is not one-way — --writable exists precisely so a run can step out
	// of a configured ceiling, per invocation and visibly.
	ReadOnly string
	// Auto arms auto-approve for this run (ADR-0053). One-way: the
	// flag can only arm, so false simply means "flag not given" and
	// the config value stands.
	Auto bool
}

// Load reads the config with no CLI overrides.
func Load(path string) (*Config, error) {
	return LoadWithOverrides(path, Overrides{})
}

// LoadWithOverrides reads the config file at path (missing file is not an
// error — env vars alone can carry a complete config), applies env and
// flag overrides, and validates. Unknown keys in the file are an error
// (strict decode): a typo like [modle] silently ignored would surface as
// a confusing runtime failure far from its cause.
func LoadWithOverrides(path string, ov Overrides) (*Config, error) {
	cfg := defaults()

	if _, err := os.Stat(path); err == nil {
		md, err := toml.DecodeFile(path, &cfg)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		// IsDefined tells us exactly which keys the file set, which is
		// the only honest way to say "this came from the file" rather
		// than "this happens to differ from the default".
		for _, key := range trackedKeys {
			if md.IsDefined(strings.Split(key, ".")...) {
				cfg.note(key, FromFile)
			}
		}
		if undecoded := md.Undecoded(); len(undecoded) > 0 {
			keys := make([]string, len(undecoded))
			for i, k := range undecoded {
				keys[i] = k.String()
			}
			return nil, fmt.Errorf("unknown key(s) in %s: %s", path, strings.Join(keys, ", "))
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	applyEnv(&cfg)

	if ov.Model != "" {
		cfg.LLM.Model = ov.Model
		cfg.note("llm.model", FromFlag)
	}
	switch ov.MCP {
	case "":
	case "on", "off":
		cfg.MCP.Enabled = ov.MCP == "on"
		cfg.note("mcp.enabled", FromFlag)
	default:
		return nil, fmt.Errorf("--mcp must be on or off (got %q)", ov.MCP)
	}
	if ov.Auto {
		cfg.Agent.AutoApprove = true
		cfg.note("agent.auto_approve", FromFlag)
	}
	switch ov.ReadOnly {
	case "":
	case "on", "off":
		cfg.Agent.ReadOnly = ov.ReadOnly == "on"
		cfg.note("agent.read_only", FromFlag)
	default:
		return nil, fmt.Errorf(`read-only override must be "on" or "off" (got %q)`, ov.ReadOnly)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// applyEnv applies the org-standard precedence: LAGENT_<FIELD> beats
// the file value.
func applyEnv(cfg *Config) {
	for _, e := range []struct {
		env, key string
		set      func(string)
	}{
		{"LAGENT_PROVIDER", "llm.provider", func(v string) { cfg.LLM.Provider = v }},
		{"LAGENT_BASE_URL", "llm.base_url", func(v string) { cfg.LLM.BaseURL = v }},
		{"LAGENT_MODEL", "llm.model", func(v string) { cfg.LLM.Model = v }},
		{"LAGENT_API_KEY", "llm.api_key", func(v string) { cfg.LLM.APIKey = v }},
	} {
		if v := os.Getenv(e.env); v != "" {
			e.set(v)
			cfg.note(e.key, FromEnv+":"+e.env)
		}
	}
}

// trackedKeys are the settings /settings displays with provenance.
var trackedKeys = []string{
	"llm.provider", "llm.base_url", "llm.model", "llm.api_key",
	"model.context_window",
	"sandbox.enabled", "sandbox.read_lane_deny_exec", "sandbox.read_lane_prompts",
	"approval.pin_trusted_files",
	"agent.max_turns", "agent.shell_timeout_sec", "agent.auto_approve",
	"agent.read_only",
	"mcp.enabled", "mcp.call_timeout_sec",
	"tui.theme", "tui.language", "tui.show_thoughts",
}

func (c *Config) validate() error {
	var missing []string
	if c.LLM.Model == "" {
		missing = append(missing, "[llm].model (or LAGENT_MODEL)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, "; "))
	}
	switch c.LLM.Provider {
	case "lmstudio", "ollama", "openai":
	default:
		return fmt.Errorf("[llm].provider must be lmstudio, ollama, or openai (got %q)", c.LLM.Provider)
	}
	// An explicit `base_url = ""` in the file overwrites the default and
	// would surface as a dial error far from its cause — the failure the
	// strict decode exists to prevent.
	if u, err := url.Parse(c.LLM.BaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("[llm].base_url must be an absolute http(s) URL (got %q; delete the key for the default)", c.LLM.BaseURL)
	}
	if c.Agent.MaxTurns <= 0 {
		return fmt.Errorf("[agent].max_turns must be positive")
	}
	if c.Agent.ShellTimeoutSec <= 0 {
		return fmt.Errorf("[agent].shell_timeout_sec must be positive")
	}
	if c.MCP.CallTimeoutSec <= 0 {
		return fmt.Errorf("[mcp].call_timeout_sec must be positive")
	}
	switch c.TUI.Theme {
	case "auto", "dark", "light", "plain":
	default:
		return fmt.Errorf("[tui].theme must be auto, dark, light, or plain (got %q)", c.TUI.Theme)
	}
	switch c.TUI.Language {
	case "auto", "ja", "en":
	default:
		return fmt.Errorf("[tui].language must be auto, ja, or en (got %q)", c.TUI.Language)
	}
	if c.Model.ContextWindow < 0 {
		return fmt.Errorf("[model].context_window must not be negative")
	}
	if c.LLM.Provider == "openai" && c.Model.ContextWindow == 0 {
		return fmt.Errorf("[model].context_window is required with [llm].provider = \"openai\": that server has no endpoint to ask")
	}
	return nil
}

// ProjectFileCap bounds a project-supplied config file (.lagent.toml,
// .mcp.json): both are read before the operator has said whether to
// trust the directory, so their size is not the directory's to choose.
const ProjectFileCap = 1 << 20

// readCapped reads path up to cap bytes and refuses a longer file.
func readCapped(path string, cap int64) ([]byte, error) {
	// Through an os.Root at the file's directory: a link leaving the
	// directory is refused, as the instruction loader refuses it — the
	// pins digest the same view (ADR-0074, review F1).
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	f, err := root.Open(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, more, err := bounded.ReadAll(f, int(cap))
	if err != nil {
		return nil, err
	}
	if more {
		return nil, fmt.Errorf("larger than %d bytes", cap)
	}
	return data, nil
}
