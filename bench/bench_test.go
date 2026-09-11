package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestShippedTasksLoad: every task under bench/tasks parses, has a
// fixture, and its expectations are well-formed — the bench must not
// discover a broken task an hour into a run.
func TestShippedTasksLoad(t *testing.T) {
	tasks, err := loadTasks("tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) < 6 {
		t.Fatalf("expected the six ADR-0006 tasks, got %d", len(tasks))
	}
	for _, task := range tasks {
		if len(task.Expect.Files) == 0 && len(task.Expect.Answers) == 0 {
			t.Errorf("%s: no expectation decides completion", task.Name)
		}
	}
	if _, err := os.Stat(filepath.Join("configs", "lagent", "baseline.toml")); err != nil {
		t.Error("the lagent baseline configuration is missing")
	}
}

func TestLoadTasksSelectionAndErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a", "task.toml"), "prompt = \"do a\"\n[[expect.answer]]\nregex = 'x'\n")
	writeFile(t, filepath.Join(dir, "a", "testdata", "f.txt"), "")
	writeFile(t, filepath.Join(dir, "b", "task.toml"), "prompt = \"do b\"\n")
	writeFile(t, filepath.Join(dir, "b", "testdata", "f.txt"), "")

	tasks, err := loadTasks(dir, []string{"b"})
	if err != nil || len(tasks) != 1 || tasks[0].Name != "b" {
		t.Fatalf("selection: %v %v", tasks, err)
	}
	// Selecting the first directory must not let the later ones through
	// once the selection set has been satisfied (a smoke run selected two
	// tasks and got three).
	tasks, err = loadTasks(dir, []string{"a"})
	if err != nil || len(tasks) != 1 || tasks[0].Name != "a" {
		t.Fatalf("selection of the first task: %v %v", tasks, err)
	}
	if _, err := loadTasks(dir, []string{"zzz"}); err == nil || !strings.Contains(err.Error(), "zzz") {
		t.Errorf("unknown task should be named in the error, got %v", err)
	}
	writeFile(t, filepath.Join(dir, "c", "task.toml"), "prompt = \"c\"\nbogus = 1\n")
	writeFile(t, filepath.Join(dir, "c", "testdata", "f.txt"), "")
	if _, err := loadTasks(dir, []string{"c"}); err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Errorf("unknown key should fail, got %v", err)
	}
	writeFile(t, filepath.Join(dir, "d", "task.toml"), "prompt = \"d\"\n")
	if _, err := loadTasks(dir, []string{"d"}); err == nil || !strings.Contains(err.Error(), "fixture") {
		t.Errorf("missing fixture should fail, got %v", err)
	}
}

func TestCheckReportsEveryFailure(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "pager.go"), "for i := 0; i <= n; i++ {")
	task := &task{Name: "x", Expect: expect{
		MinToolCalls: 2,
		Files: []fileExpect{
			{Path: "pager.go", Contains: "i < n"},
			{Path: "pager.go", NotContains: "i <= n"},
			{Path: "missing.go", Contains: "x"},
		},
		Answers: []answerCheck{{Regex: `\b57\b`}},
	}}
	failures := task.check(project, "there are 570 rows", 1)
	if len(failures) != 5 {
		t.Fatalf("expected 5 failures, got %d: %v", len(failures), failures)
	}
	writeFile(t, filepath.Join(project, "pager.go"), "for i := 0; i < n; i++ {")
	task.Expect.Files = task.Expect.Files[:2]
	if f := task.check(project, "57", 3); len(f) != 0 {
		t.Errorf("expected pass, got %v", f)
	}
}

func TestPlanInterleavesConfigsInsideCases(t *testing.T) {
	tasks := []*task{{Name: "t1"}, {Name: "t2"}}
	configs := []configRef{{Name: "a"}, {Name: "b"}}
	cells := plan(tasks, configs, 2)
	var got []string
	for _, c := range cells {
		got = append(got, c.Task.Name+"/"+c.Config.Name+"#"+string(rune('0'+c.Rep)))
	}
	want := "t1/a#1 t1/b#1 t1/a#2 t1/b#2 t2/a#1 t2/b#1 t2/a#2 t2/b#2"
	if strings.Join(got, " ") != want {
		t.Errorf("order:\n got %s\nwant %s", strings.Join(got, " "), want)
	}
}

func TestParseConfigs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "configs", "lagent", "baseline.toml"), "[llm]\n")
	own := filepath.Join(dir, "own.toml")
	writeFile(t, own, "[llm]\n")
	refs, err := parseConfigs("baseline, mine="+own, dir, "lagent")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0].Name != "baseline" || refs[1].Name != "mine" || refs[1].Path != own {
		t.Errorf("refs: %+v", refs)
	}
	if _, err := parseConfigs("nope", dir, "lagent"); err == nil {
		t.Error("a missing configuration file must fail before any run")
	}
}

func TestTranscriptStats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	lines := []string{
		`{"ts":"2026-09-12T00:00:00Z","kind":"session","data":{"schema":2}}`,
		`{"ts":"2026-09-12T00:00:01Z","kind":"message","data":{"role":"user","content":"hi"}}`,
		`{"ts":"2026-09-12T00:00:02Z","kind":"usage","data":{"source":"main","prompt":100,"output":5,"cached":80}}`,
		`{"ts":"2026-09-12T00:00:02Z","kind":"message","data":{"role":"assistant","tool_calls":[{"id":"1","name":"read_file","args":{}},{"id":"2","name":"read_file","args":{}}]}}`,
		`{"ts":"2026-09-12T00:00:03Z","kind":"message","data":{"role":"tool","tool_name":"read_file","content":"x"}}`,
		`{"ts":"2026-09-12T00:00:04Z","kind":"usage","data":{"source":"main","prompt":150,"output":7}}`,
		`{"ts":"2026-09-12T00:00:04Z","kind":"message","data":{"role":"assistant","tool_calls":[{"id":"3","name":"edit_file","args":{}}]}}`,
		`{"ts":"2026-09-12T00:00:05Z","kind":"usage","data":{"source":"risk","prompt":999,"output":1}}`,
		`{"ts":"2026-09-12T00:00:06Z","kind":"message","data":{"role":"assistant","content":"done: 57"}}`,
		`not json`,
	}
	writeFile(t, path, strings.Join(lines, "\n")+"\n")
	st, err := transcriptStats(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Rounds != 2 || st.ToolCalls != 3 || st.ByTool["read_file"] != 2 || st.ByTool["edit_file"] != 1 {
		t.Errorf("rounds/calls: %+v", st)
	}
	if st.Prompt != 250 || st.Output != 12 || st.Cached != 80 {
		t.Errorf("tokens must sum main-loop usage only: %+v", st)
	}
	if st.Answer != "done: 57" {
		t.Errorf("answer: %q", st.Answer)
	}
}

func TestReportMediansAndRates(t *testing.T) {
	results := []result{
		{Task: "a", Config: "base", Completed: true, Rounds: 2, ToolCalls: 3, Prompt: 100, WallSec: 10},
		{Task: "a", Config: "base", Completed: false, Rounds: 0, ToolCalls: 0, Prompt: 50, WallSec: 4},
		{Task: "a", Config: "base", Completed: true, Rounds: 4, ToolCalls: 6, Prompt: 300, WallSec: 20},
		{Task: "a", Config: "alt", Completed: true, Rounds: 1, ToolCalls: 1, Prompt: 80, WallSec: 6},
	}
	out := report(results)
	if !strings.Contains(out, "| a | alt | 1 | 1/1 | 0/1 | 1 | 1 | 80 | 6 |") {
		t.Errorf("alt row:\n%s", out)
	}
	if !strings.Contains(out, "| a | base | 3 | 2/3 | 1/3 | 2 | 3 | 100 | 10 |") {
		t.Errorf("base row (medians of 3):\n%s", out)
	}
	if m := median([]float64{4, 1, 3, 2}); m != 2.5 {
		t.Errorf("even median: %v", m)
	}
}

func TestIsolatedEnvOverrides(t *testing.T) {
	env := isolatedEnv([]string{"HOME=/real", "PATH=/bin", "LAGENT_STATE_DIR=/real-state"},
		map[string]string{"HOME": "/iso", "LAGENT_STATE_DIR": "/iso-state"})
	joined := strings.Join(env, "\n")
	for _, bad := range []string{"HOME=/real", "LAGENT_STATE_DIR=/real-state"} {
		if strings.Contains(joined, bad) {
			t.Errorf("operator value leaked: %s", bad)
		}
	}
	for _, good := range []string{"HOME=/iso", "LAGENT_STATE_DIR=/iso-state", "PATH=/bin"} {
		if !strings.Contains(joined, good) {
			t.Errorf("missing %s", good)
		}
	}
}

// TestRunOneIsolatesAndMeasures drives the runner with a stand-in
// binary: a shell script that records the environment it was given,
// writes a transcript under the state root it was told, and edits the
// project. Pins the wiring the unit tests above cannot: the flags, the
// isolated HOME and state root, transcript discovery, and the checks.
func TestRunOneIsolatesAndMeasures(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-runtime")
	script := `#!/bin/sh
printf '%s\n' "$*" > "$HOME/argv.txt"
printf '%s\n' "$HOME" > "$LAGENT_STATE_DIR/home.txt"
mkdir -p "$LAGENT_STATE_DIR/sessions/projects/p"
cat > "$LAGENT_STATE_DIR/sessions/projects/p/s.jsonl" <<'EOF'
{"ts":"2026-09-12T00:00:00Z","kind":"session","data":{}}
{"ts":"2026-09-12T00:00:01Z","kind":"usage","data":{"source":"main","prompt":42,"output":3}}
{"ts":"2026-09-12T00:00:01Z","kind":"message","data":{"role":"assistant","tool_calls":[{"id":"1","name":"edit_file","args":{}}]}}
{"ts":"2026-09-12T00:00:02Z","kind":"message","data":{"role":"assistant","content":"fixed"}}
EOF
printf 'for i := 0; i < n; i++ {\n' > pager.go
echo fixed
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	taskDir := filepath.Join(dir, "tasks", "edit")
	writeFile(t, filepath.Join(taskDir, "task.toml"), "prompt = \"fix it\"\nmcp = true\n[expect]\nmin_tool_calls = 1\n[[expect.file]]\npath = \"pager.go\"\ncontains = \"i < n\"\n[[expect.answer]]\nregex = 'fixed'\n")
	writeFile(t, filepath.Join(taskDir, "testdata", "pager.go"), "for i := 0; i <= n; i++ {\n")
	tasks, err := loadTasks(filepath.Join(dir, "tasks"), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "baseline.toml")
	writeFile(t, cfg, "[llm]\nmodel = \"m\"\n")
	out := filepath.Join(dir, "results")
	r := &runner{Runtime: runtimes["lagent"], Bin: bin, MCPBin: "/nonexistent/bench-mcp",
		OutDir: out, Timeout: 30 * time.Second, Progress: &bytes.Buffer{}}
	res, err := r.runOne(cell{Task: tasks[0], Config: configRef{Name: "baseline", Path: cfg}, Rep: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Completed || res.Rounds != 1 || res.ToolCalls != 1 || res.Prompt != 42 || res.Answer != "fixed" {
		t.Errorf("result: %+v", res)
	}
	runDir := filepath.Join(out, "edit", "baseline", "rep1")
	argv, _ := os.ReadFile(filepath.Join(runDir, "home", "argv.txt"))
	if !strings.Contains(string(argv), "-p fix it --auto --config "+string(filepath.Separator)) {
		t.Errorf("argv must carry an absolute --config: %q", argv)
	}
	homeSeen, _ := os.ReadFile(filepath.Join(runDir, "state", "home.txt"))
	if !strings.HasSuffix(strings.TrimSpace(string(homeSeen)), filepath.Join("rep1", "home")) {
		t.Errorf("HOME was not the run's own: %q", homeSeen)
	}
	if _, err := os.Stat(filepath.Join(runDir, "home", ".config", "lagent", "mcp.json")); err != nil {
		t.Error("an mcp task must get a global mcp.json in its HOME")
	}
	if fixture, _ := os.ReadFile(filepath.Join(taskDir, "testdata", "pager.go")); !strings.Contains(string(fixture), "i <= n") {
		t.Error("the fixture itself was modified; runs must work on a copy")
	}
	loaded, err := loadResults(out)
	if err != nil || len(loaded) != 1 || loaded[0].Task != "edit" {
		t.Errorf("runs.jsonl: %v %v", loaded, err)
	}
	var raw map[string]any
	line, _ := os.ReadFile(filepath.Join(out, "runs.jsonl"))
	if err := json.Unmarshal(bytes.TrimSpace(line), &raw); err != nil || raw["by_tool"] == nil {
		t.Errorf("runs.jsonl shape: %v %s", err, line)
	}
}

func TestRunOneTimesOut(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "slow")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	taskDir := filepath.Join(dir, "tasks", "slow")
	writeFile(t, filepath.Join(taskDir, "task.toml"), "prompt = \"wait\"\n[[expect.answer]]\nregex = 'x'\n")
	writeFile(t, filepath.Join(taskDir, "testdata", "f"), "")
	tasks, _ := loadTasks(filepath.Join(dir, "tasks"), nil)
	cfg := filepath.Join(dir, "c.toml")
	writeFile(t, cfg, "")
	r := &runner{Runtime: runtimes["lagent"], Bin: bin, OutDir: filepath.Join(dir, "r"),
		Timeout: 300 * time.Millisecond, Progress: &bytes.Buffer{}}
	start := time.Now()
	res, err := r.runOne(cell{Task: tasks[0], Config: configRef{Name: "c", Path: cfg}, Rep: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut || res.Completed {
		t.Errorf("expected a timed-out, incomplete run: %+v", res)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("the deadline did not stop the run")
	}
}
