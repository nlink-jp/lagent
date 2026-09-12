package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// runner executes cells in isolation (ADR-0006 §3): every run gets its
// own HOME (config.toml, global mcp.json), its own state root, and a
// fresh copy of the task's fixture. The operator's configuration and
// sessions are never touched.
type runner struct {
	Runtime  runtimeProfile
	Bin      string
	MCPBin   string
	OutDir   string
	Timeout  time.Duration
	Progress io.Writer
}

// result is one run's measurement, appended to <out>/runs.jsonl.
type result struct {
	Task       string         `json:"task"`
	Config     string         `json:"config"`
	Rep        int            `json:"rep"`
	Runtime    string         `json:"runtime"`
	Started    time.Time      `json:"started"`
	WallSec    float64        `json:"wall_sec"`
	ExitCode   int            `json:"exit_code"`
	TimedOut   bool           `json:"timed_out"`
	Rounds     int            `json:"rounds"`
	ToolCalls  int            `json:"tool_calls"`
	ByTool     map[string]int `json:"by_tool"`
	Prompt     int            `json:"prompt_tokens"`
	Output     int            `json:"output_tokens"`
	Cached     int            `json:"cached_tokens"`
	Completed  bool           `json:"completed"`
	Failures   []string       `json:"failures"`
	Answer     string         `json:"answer"`
	Transcript string         `json:"transcript"`
	Dir        string         `json:"dir"`
}

func (r result) line() string {
	state := "ok"
	switch {
	case r.TimedOut:
		state = "TIMEOUT"
	case !r.Completed:
		state = "FAIL " + strings.Join(r.Failures, "; ")
	}
	return fmt.Sprintf("%s/%s#%d rounds=%d calls=%d prompt=%d wall=%.0fs exit=%d %s",
		r.Task, r.Config, r.Rep, r.Rounds, r.ToolCalls, r.Prompt, r.WallSec, r.ExitCode, state)
}

func (r *runner) runOne(c cell) (result, error) {
	dir := filepath.Join(r.OutDir, c.Task.Name, c.Config.Name, fmt.Sprintf("rep%d", c.Rep))
	home := filepath.Join(dir, "home")
	state := filepath.Join(dir, "state")
	// The project lives under the run's HOME: the runtime's
	// instruction-file walk stops there, so the bench repository's own
	// AGENTS.md above the results directory never reaches a run.
	project := filepath.Join(home, "project")
	for _, d := range []string{home, state, project} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return result{}, err
		}
	}
	if err := copyTree(c.Task.Fixture, project); err != nil {
		return result{}, err
	}
	cfgDir := filepath.Join(home, r.Runtime.ConfigDir)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return result{}, err
	}
	// Absolute: the runtime resolves --config from its own working
	// directory, which is the run's project, not the bench's (a smoke
	// run with a relative --out fell back to the defaults and failed on
	// the missing model).
	cfgPath, err := filepath.Abs(filepath.Join(cfgDir, "config.toml"))
	if err != nil {
		return result{}, err
	}
	if err := copyFile(c.Config.Path, cfgPath); err != nil {
		return result{}, fmt.Errorf("configuration %s: %w", c.Config.Name, err)
	}
	if c.Task.MCP {
		if err := writeMCPConfig(filepath.Join(cfgDir, "mcp.json"), r.MCPBin); err != nil {
			return result{}, err
		}
	}
	absProjectForTrust, err := filepath.Abs(project)
	if err != nil {
		return result{}, err
	}
	if c.Task.Trust {
		// The strong, hand-written grant both runtimes honour
		// ([approval].trusted_projects); a super-table after its
		// sub-table is valid TOML, so this appends to any configuration.
		if err := appendTrustedProject(cfgPath, absProjectForTrust); err != nil {
			return result{}, err
		}
	}
	if c.Task.SkillsDir != "" {
		if err := copyTree(c.Task.SkillsDir, filepath.Join(cfgDir, "skills")); err != nil {
			return result{}, fmt.Errorf("task %s skills: %w", c.Task.Name, err)
		}
	}

	absProject, err := filepath.Abs(project)
	if err != nil {
		return result{}, err
	}
	absHome, err := filepath.Abs(home)
	if err != nil {
		return result{}, err
	}
	absState, err := filepath.Abs(state)
	if err != nil {
		return result{}, err
	}
	res := result{Task: c.Task.Name, Config: c.Config.Name, Rep: c.Rep,
		Runtime: r.Runtime.Name, Dir: dir, Started: time.Now()}

	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()
	override := r.Runtime.passThroughEnv(os.Getenv("HOME"))
	override["HOME"] = absHome
	override[r.Runtime.StateEnv] = absState
	if c.Task.Trust {
		// Record the pins the way the operator would (`trust --accept`),
		// so a trusted project's files load under pin_trusted_files.
		pin := exec.CommandContext(ctx, r.Bin, "trust", "--accept", "--config", cfgPath)
		pin.Dir = absProject
		pin.Env = isolatedEnv(os.Environ(), override)
		pinOut, pinErr := pin.CombinedOutput()
		_ = os.WriteFile(filepath.Join(dir, "trust.txt"), pinOut, 0o644)
		if pinErr != nil {
			return result{}, fmt.Errorf("task %s: trust --accept: %w\n%s", c.Task.Name, pinErr, pinOut)
		}
	}
	cmd := exec.CommandContext(ctx, r.Bin, "-p", c.Task.Prompt, "--auto", "--config", cfgPath)
	cmd.Dir = absProject
	cmd.Env = isolatedEnv(os.Environ(), override)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	stdout, err := os.Create(filepath.Join(dir, "stdout.txt"))
	if err != nil {
		return result{}, err
	}
	stderr, err := os.Create(filepath.Join(dir, "stderr.txt"))
	if err != nil {
		_ = stdout.Close()
		return result{}, err
	}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	runErr := cmd.Run()
	res.WallSec = time.Since(res.Started).Seconds()
	if cerr := stdout.Close(); cerr != nil {
		return result{}, cerr
	}
	if cerr := stderr.Close(); cerr != nil {
		return result{}, cerr
	}
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
	case errors.As(runErr, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		return result{}, fmt.Errorf("start %s: %w", r.Bin, runErr)
	}
	res.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)

	transcript, err := findTranscript(absState)
	if err == nil {
		res.Transcript = transcript
		st, serr := transcriptStats(transcript)
		if serr != nil {
			return result{}, serr
		}
		res.Rounds, res.ToolCalls, res.ByTool = st.Rounds, st.ToolCalls, st.ByTool
		res.Prompt, res.Output, res.Cached = st.Prompt, st.Output, st.Cached
		res.Answer = st.Answer
	}
	if res.Answer == "" {
		if data, rerr := os.ReadFile(filepath.Join(dir, "stdout.txt")); rerr == nil {
			res.Answer = strings.TrimSpace(string(data))
		}
	}
	res.Failures = c.Task.check(absProject, res.Answer, res.ToolCalls)
	if res.Failures == nil {
		res.Failures = []string{}
	}
	res.Completed = len(res.Failures) == 0 && !res.TimedOut
	if err := appendResult(filepath.Join(r.OutDir, "runs.jsonl"), res); err != nil {
		return result{}, err
	}
	return res, nil
}

// isolatedEnv keeps the parent environment (PATH, TMPDIR, locale) and
// overrides the variables that would otherwise reach the operator's
// configuration or state.
func isolatedEnv(parent []string, override map[string]string) []string {
	var env []string
	for _, kv := range parent {
		key, _, _ := strings.Cut(kv, "=")
		if _, drop := override[key]; drop {
			continue
		}
		env = append(env, kv)
	}
	for k, v := range override {
		env = append(env, k+"="+v)
	}
	return env
}

// writeMCPConfig points the run's global mcp.json at the bench fixture
// server, as the server "geo".
func writeMCPConfig(path, mcpBin string) error {
	cfg := map[string]any{"mcpServers": map[string]any{
		"geo": map[string]any{"command": mcpBin},
	}}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// findTranscript locates the one session file a run wrote under its
// state root. Both runtimes lay sessions out as
// <root>/sessions/projects/<escaped>/<id>.jsonl.
func findTranscript(stateRoot string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(stateRoot, "sessions", "projects", "*", "*.jsonl"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", errors.New("no transcript written")
	}
	// The newest wins if the runtime wrote more than one.
	best, bestTime := "", time.Time{}
	for _, m := range matches {
		st, err := os.Stat(m)
		if err != nil {
			continue
		}
		if best == "" || st.ModTime().After(bestTime) {
			best, bestTime = m, st.ModTime()
		}
	}
	return best, nil
}

func appendResult(path string, res result) (err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	data, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

// loadResults reads <dir>/runs.jsonl back.
func loadResults(dir string) ([]result, error) {
	data, err := os.ReadFile(filepath.Join(dir, "runs.jsonl"))
	if err != nil {
		return nil, err
	}
	var out []result
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r result
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("runs.jsonl line %d: %w", i+1, err)
		}
		out = append(out, r)
	}
	return out, nil
}

// appendTrustedProject adds the run's project to the configuration's
// [approval].trusted_projects, the grant that lets the fixture's own
// instruction files and project skills load.
func appendTrustedProject(cfgPath, project string) error {
	f, err := os.OpenFile(cfgPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n[approval]\ntrusted_projects = [%q]\n", project)
	return err
}
