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
	Expect expect `toml:"expect"`
	// Fixture is <Dir>/testdata: the name Go tooling ignores, so the
	// fixture's own .go files are never built, vetted or linted as
	// part of the module.
	Fixture string `toml:"-"`
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
	selecting := len(want) > 0
	var tasks []*task
	for _, e := range entries {
		if !e.IsDir() || (selecting && !want[e.Name()]) {
			continue
		}
		t, err := loadTask(filepath.Join(tasksDir, e.Name()))
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
		delete(want, e.Name())
	}
	if len(want) > 0 {
		missing := make([]string, 0, len(want))
		for n := range want {
			missing = append(missing, n)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("tasks not found: %s", strings.Join(missing, ", "))
	}
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
