package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// task is one bench case: a prompt, the fixture it runs in, and the
// expectations that decide "completed" mechanically (ADR-0006 §1).
type task struct {
	Name   string `toml:"-"`
	Dir    string `toml:"-"`
	Prompt string `toml:"prompt"`
	MCP    bool   `toml:"mcp"` // needs the bench MCP fixture server
	// Suite keeps a task out of the default selection. An empty suite
	// is the everyday set that `bench run` takes with no --tasks; a
	// named one runs only when --tasks names the task or the suite.
	// The injection family is the reason (ADR-0018 §5): those runs are
	// a deliberate measurement session, not part of every sweep, and
	// their result is not a rate the everyday table should carry.
	Suite string `toml:"suite"`
	// PayloadMarker is a string that must appear in a request body for
	// the run to mean anything. An injection task whose payload never
	// reached the model scores exactly like one where it arrived and
	// was ignored, and the second is the only one worth counting
	// (ADR-0018 §6). Measured on the family's first real sweep: 1 run
	// in 9 read the fixture without the injected sentence reaching a
	// request, and passed. Requires the runtime to have a trace switch.
	PayloadMarker string `toml:"payload_marker"`
	// Trust marks the run's project as trusted (the configuration's
	// [approval].trusted_projects names it, and the runtime's pins are
	// recorded before the run), so the fixture's own AGENTS.md and
	// project skills load. Off by default: an untrusted project is the
	// bench's baseline, and most tasks measure nothing about
	// instruction files.
	Trust  bool   `toml:"trust"`
	Expect expect `toml:"expect"`
	// Fixture is <Dir>/testdata: the name Go tooling ignores, so the
	// fixture's own .go files are never built, vetted or linted as
	// part of the module.
	Fixture string `toml:"-"`
	// SkillsDir is <Dir>/skills when it exists: skill directories the
	// runner installs into the run's global skills location
	// (~/.config/lagent/skills under the isolated HOME), so a task can
	// measure whether the model loads and follows a skill (ADR-0011).
	// Global, not project scope: the bench's project is untrusted.
	SkillsDir string `toml:"-"`
	// MemoryDir is <Dir>/memory when it exists: the memory root the
	// runner installs as the run's state-root memory (global/<name>.md
	// under it), so a task can measure whether the model acts on a
	// memory recalled in the facts message (ADR-0013).
	MemoryDir string `toml:"-"`
}

type expect struct {
	MinToolCalls int           `toml:"min_tool_calls"`
	Files        []fileExpect  `toml:"file"`
	Answers      []answerCheck `toml:"answer"`
}

type fileExpect struct {
	Path        string `toml:"path"`
	Contains    string `toml:"contains"`
	NotContains string `toml:"not_contains"`
}

type answerCheck struct {
	Regex string `toml:"regex"`
}

// loadTasks reads every bench/tasks/<name>/task.toml, or only the named
// ones, in name order.
func loadTasks(tasksDir string, only []string) ([]*task, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("tasks: %w", err)
	}
	want := map[string]bool{}
	for _, n := range only {
		want[n] = true
	}
	var tasks []*task
	if len(want) == 0 {
		// The everyday sweep: everything except the named suites, which
		// are deliberate measurement sessions (ADR-0018 §5).
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			t, err := loadTask(filepath.Join(tasksDir, e.Name()))
			if err != nil {
				return nil, err
			}
			if t.Suite == "" {
				tasks = append(tasks, t)
			}
		}
		if len(tasks) == 0 {
			return nil, fmt.Errorf("no tasks under %s", tasksDir)
		}
		return tasks, nil
	}
	// A selection names tasks. Load only those, so a malformed task
	// nobody asked for cannot break a run that does not include it.
	for _, e := range entries {
		if !e.IsDir() || !want[e.Name()] {
			continue
		}
		t, err := loadTask(filepath.Join(tasksDir, e.Name()))
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
		delete(want, e.Name())
	}
	// Whatever is left may name a suite rather than a task, which can
	// only be answered by reading the remaining tasks.
	if len(want) > 0 {
		claimed := map[string]bool{}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			t, err := loadTask(filepath.Join(tasksDir, e.Name()))
			if err != nil {
				continue // a task nobody selected; its error is not this run's
			}
			if t.Suite != "" && want[t.Suite] {
				tasks = append(tasks, t)
				claimed[t.Suite] = true
			}
		}
		for s := range claimed {
			delete(want, s)
		}
	}
	if len(want) > 0 {
		missing := make([]string, 0, len(want))
		for n := range want {
			missing = append(missing, n)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("tasks not found: %s", strings.Join(missing, ", "))
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Name < tasks[j].Name })
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks under %s", tasksDir)
	}
	return tasks, nil
}

func loadTask(dir string) (*task, error) {
	t := &task{Name: filepath.Base(dir), Dir: dir, Fixture: filepath.Join(dir, "testdata")}
	md, err := toml.DecodeFile(filepath.Join(dir, "task.toml"), t)
	if err != nil {
		return nil, fmt.Errorf("task %s: %w", t.Name, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("task %s: unknown key %s", t.Name, undecoded[0].String())
	}
	if strings.TrimSpace(t.Prompt) == "" {
		return nil, fmt.Errorf("task %s: prompt is empty", t.Name)
	}
	for _, a := range t.Expect.Answers {
		if _, err := regexp.Compile(a.Regex); err != nil {
			return nil, fmt.Errorf("task %s: answer regex: %w", t.Name, err)
		}
	}
	for _, f := range t.Expect.Files {
		if f.Path == "" || (f.Contains == "" && f.NotContains == "") {
			return nil, fmt.Errorf("task %s: a file expectation needs path and contains or not_contains", t.Name)
		}
	}
	if st, err := os.Stat(t.Fixture); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("task %s: fixture directory testdata/ missing", t.Name)
	}
	if st, err := os.Stat(filepath.Join(dir, "skills")); err == nil && st.IsDir() {
		t.SkillsDir = filepath.Join(dir, "skills")
	}
	if st, err := os.Stat(filepath.Join(dir, "memory")); err == nil && st.IsDir() {
		t.MemoryDir = filepath.Join(dir, "memory")
	}
	return t, nil
}

// check evaluates the expectations against the project directory the
// run worked in, the final answer, and the tool-call count. It returns
// every failure, never only the first, so a report can say which
// half of a task the run got right.
func (t *task) check(projectDir, answer string, toolCalls int) []string {
	var failures []string
	if t.Expect.MinToolCalls > 0 && toolCalls < t.Expect.MinToolCalls {
		failures = append(failures, fmt.Sprintf("tool calls %d < %d", toolCalls, t.Expect.MinToolCalls))
	}
	for _, f := range t.Expect.Files {
		data, err := os.ReadFile(filepath.Join(projectDir, f.Path))
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", f.Path, err))
			continue
		}
		if f.Contains != "" && !strings.Contains(string(data), f.Contains) {
			failures = append(failures, fmt.Sprintf("%s lacks %q", f.Path, f.Contains))
		}
		if f.NotContains != "" && strings.Contains(string(data), f.NotContains) {
			failures = append(failures, fmt.Sprintf("%s still has %q", f.Path, f.NotContains))
		}
	}
	for _, a := range t.Expect.Answers {
		if !regexp.MustCompile(a.Regex).MatchString(answer) {
			failures = append(failures, fmt.Sprintf("answer does not match %s", a.Regex))
		}
	}
	return failures
}

// copyTree copies a fixture into a fresh project directory. Fixtures
// hold regular files and directories only; anything else is refused so
// a symlink can never point a run outside its project.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			return copyFile(p, target)
		default:
			return fmt.Errorf("fixture %s: %s is not a regular file", src, rel)
		}
	})
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(out, in)
	return err
}
