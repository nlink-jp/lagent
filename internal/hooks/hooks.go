// Package hooks runs operator-configured commands before a model tool
// call (ADR-0012). The stdin payload and the verdict contracts are
// Claude Code's PreToolUse, measured against the organization's
// installed guard rather than taken from documentation, so a hooks
// block written for Claude Code registers here unchanged.
//
// A deny is a deterministic floor: it stands before the approval
// ladder ever runs, and nothing downstream can overrule it. Every other
// outcome — pass, non-zero exit, unparseable output, timeout — fails
// open with a notice: hooks only ever tighten. The other Claude Code
// events are not here; the mechanism takes them when a consumer
// appears (ADR-0012 §1).
//
// Ported from gem-agent internal/hooks at 4f63349ac3bbaf5c4e1e3b4a955964088770cd1c (v0.77.0), ADR-0001,
// cut to the pre-tool event.
package hooks

import (
	"github.com/nlink-jp/lagent/internal/bounded"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// DefaultTimeout bounds one hook run. Short on purpose: a hook is a
// deterministic check, and the model's turn is stalled while it runs.
const DefaultTimeout = 10 * time.Second

// Hook is one configured command.
type Hook struct {
	// Matcher selects the tool names the hook covers: exact, "a|b"
	// alternation, or "*".
	Matcher string
	Command string // run via sh -c, payload on stdin
	Timeout time.Duration
}

// Hooks groups the operator's hooks by event. One event today.
type Hooks struct {
	PreToolUse []Hook
}

// Session identifies the running session to a hook (ADR-0012 §5) — the
// fields of Claude Code's payload that lagent can supply. An empty ID
// and TranscriptPath mean the session log is disabled; the fields are
// still sent, empty, so a script sees the same shape every time.
type Session struct {
	ID             string
	TranscriptPath string
	CWD            string
}

// aliases maps lagent tool names to their Claude Code names, so a
// hooks block copied from Claude Code settings works without renaming
// (ADR-0012 §4). The payload still carries lagent's real name — the
// org's guard measurably ignores it, and lying to scripts that do look
// would be worse.
var aliases = map[string]string{
	"shell_exec": "Bash",
	"write_file": "Write",
	"edit_file":  "Edit",
	"read_file":  "Read",
}

// Runner evaluates the configured hooks.
type Runner struct {
	hooks  Hooks
	notify func(string) // non-blocking failures surface here; never nil
}

// New builds a Runner. notify receives fail-open warnings (a broken or
// timed-out hook); pass nil to drop them.
func New(hs Hooks, notify func(string)) *Runner {
	if notify == nil {
		notify = func(string) {}
	}
	return &Runner{hooks: hs, notify: notify}
}

// HasPreToolUse reports whether any pre-tool hook is configured, so
// the caller can skip the work entirely in the common case.
func (r *Runner) HasPreToolUse() bool { return len(r.hooks.PreToolUse) > 0 }

// matches reports whether one matcher covers the name, in either
// vocabulary.
func matches(matcher, name string) bool {
	for _, m := range strings.Split(matcher, "|") {
		m = strings.TrimSpace(m)
		if m == "*" || m == name || m == aliases[name] {
			return true
		}
	}
	return false
}

// preToolPayload is the Claude Code PreToolUse stdin shape (measured
// contract), plus the session identity every Claude Code event carries:
// a hook that keeps per-session state needs to tie a call to its
// session.
type preToolPayload struct {
	HookEventName  string         `json:"hook_event_name"`
	SessionID      string         `json:"session_id"`
	TranscriptPath string         `json:"transcript_path"`
	ToolName       string         `json:"tool_name"`
	ToolInput      map[string]any `json:"tool_input"`
	CWD            string         `json:"cwd"`
}

// verdict covers the stdout JSON forms Claude Code scripts use: the
// hookSpecificOutput form (the org's guard, measured) and the older
// top-level decision form.
type verdict struct {
	Decision           string `json:"decision"` // "block" denies
	Reason             string `json:"reason"`
	HookSpecificOutput struct {
		PermissionDecision       string `json:"permissionDecision"` // "deny" denies
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// outcome is one finished hook process.
type outcome struct {
	stdout, stderr string
	timedOut       bool
	err            error // spawn failure or non-zero exit
}

// exec runs one hook with the payload on stdin. ok is false when the
// payload could not even be encoded (notified; the hook is skipped).
func (r *Runner) exec(ctx context.Context, h Hook, cwd string, payload any) (outcome, bool) {
	timeout := h.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	in, err := json.Marshal(payload)
	if err != nil {
		r.notify(fmt.Sprintf("hook %q skipped: cannot encode the payload: %v", h.Command, err))
		return outcome{}, false
	}
	cmd := exec.CommandContext(cctx, "/bin/sh", "-c", h.Command)
	// The hook runs in its own process group and the timeout kills the
	// group, so a child the hook started does not outlive it as an
	// orphan (shell_exec's hardening does the same). WaitDelay bounds
	// the wait for pipes a descendant still holds.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = hookWaitDelay
	cmd.Dir = cwd
	cmd.Stdin = bytes.NewReader(in)
	// Bounded as it arrives: a hook that printed without end would
	// exhaust memory before its timeout. The deny JSON is small by
	// contract; the caps are generous for it and a ceiling for
	// everything else.
	stdout, stderr := bounded.NewWriter(hookStdoutCap), bounded.NewWriter(hookStderrCap)
	cmd.Stdout, cmd.Stderr = stdout, stderr

	runErr := cmd.Run()
	out := outcome{stdout: capped(stdout), stderr: capped(stderr), err: runErr}
	if cctx.Err() == context.DeadlineExceeded {
		out.timedOut = true
		r.notify(fmt.Sprintf("hook %q timed out after %s", h.Command, timeout))
	}
	return out, true
}

// hookWaitDelay is how long exec waits, after the hook process ended,
// for pipes a descendant still holds.
const hookWaitDelay = time.Second

// hookStdoutCap / hookStderrCap bound what a hook's streams may leave
// in memory: the deny JSON fits many times over.
const (
	hookStdoutCap = 1 << 20
	hookStderrCap = 64 << 10
)

// capped renders a hook stream: the kept bytes, cut whole-rune, with
// the cut said.
func capped(w *bounded.Writer) string {
	data, more := w.Bytes()
	if !more {
		return string(data)
	}
	return string(data) + fmt.Sprintf("\n[hook output truncated: %d of %d bytes kept]", len(data), w.Total())
}

// exitCode2 reports whether the process ended with the deny exit code
// of the simple contract.
func exitCode2(err error) bool {
	var ee *exec.ExitError
	return errors.As(err, &ee) && ee.ExitCode() == 2
}

// Pre runs every matching pre-tool hook in order and reports the first
// denial. Anything that is not an explicit denial — pass, non-zero
// exit, unparseable output, timeout — lets the call proceed to the
// normal approval ladder: hooks only ever tighten (ADR-0012 §3). The
// hook runs in s.CWD and is told which session is calling.
func (r *Runner) Pre(ctx context.Context, s Session, name string, args map[string]any) (deny bool, reason string) {
	for _, h := range r.hooks.PreToolUse {
		if !matches(h.Matcher, name) {
			continue
		}
		out, ok := r.exec(ctx, h, s.CWD, preToolPayload{
			HookEventName: "PreToolUse", SessionID: s.ID, TranscriptPath: s.TranscriptPath,
			ToolName: name, ToolInput: args, CWD: s.CWD,
		})
		if !ok || out.timedOut {
			continue // the call proceeds
		}
		if out.err != nil {
			if exitCode2(out.err) {
				return true, orDefault(out.stderr, "blocked by a pre-tool hook")
			}
			r.notify(fmt.Sprintf("hook %q failed (%v) — the call proceeds", h.Command, out.err))
			continue
		}
		if v, isJSON := parseVerdict(out.stdout); isJSON {
			if blocked, why := v.denies(); blocked {
				return true, orDefault(why, "blocked by a pre-tool hook")
			}
		}
	}
	return false, ""
}

// parseVerdict reads stdout as a JSON verdict object. Plain text —
// informational output — is not JSON.
func parseVerdict(out string) (verdict, bool) {
	out = strings.TrimSpace(out)
	if !strings.HasPrefix(out, "{") {
		return verdict{}, false
	}
	var v verdict
	if json.Unmarshal([]byte(out), &v) != nil {
		return verdict{}, false
	}
	return v, true
}

// denies reports a block in either JSON contract, with its reason.
func (v verdict) denies() (bool, string) {
	if v.HookSpecificOutput.PermissionDecision == "deny" {
		return true, strings.TrimSpace(v.HookSpecificOutput.PermissionDecisionReason)
	}
	if v.Decision == "block" {
		return true, strings.TrimSpace(v.Reason)
	}
	return false, ""
}

func orDefault(s, def string) string {
	if s = strings.TrimSpace(s); s != "" {
		return s
	}
	return def
}
