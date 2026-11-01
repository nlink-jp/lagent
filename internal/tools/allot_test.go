package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// --- the runtime's allotment (ADR-0034 §2–§3) ---

const testAllot = 4096

// footerSlack is what a tool may add past its allotment: the marker,
// the route, a listing's footer. The runtime's backstop allows more.
const footerSlack = 600

func runAllotted(t *testing.T, r *Registry, name string, n int, args map[string]any) string {
	t.Helper()
	tool, ok := r.Get(name)
	if !ok {
		t.Fatalf("tool %s not registered", name)
	}
	out, err := tool.Run(WithAllot(context.Background(), n), args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out
}

// numberedLines writes a file of n distinct, equally long lines.
func numberedLines(t *testing.T, dir, name string, n int) string {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %06d %s\n", i, strings.Repeat("x", 60))
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// Every built-in tool is either one that applies the allotment with its
// own route, or one whose result is short by construction and is left
// to the runtime's backstop. A new tool must be put in one of the two:
// the class is closed here rather than per instance.
func TestEveryToolIsClassifiedForTheAllotment(t *testing.T) {
	applies := map[string]bool{
		"read_file": true, "search_files": true, "list_tree": true,
		"list_files": true, "file_info": true, ShellExecName: true,
	}
	short := map[string]bool{ // a confirmation, an error, an image's caption
		"write_file": true, "edit_file": true, ViewImageName: true, ShowImageName: true,
	}
	r := newRegistry(t)
	seen := map[string]bool{}
	for _, tool := range r.List() {
		seen[tool.Name] = true
		if !applies[tool.Name] && !short[tool.Name] {
			t.Errorf("%s is not classified for the allotment (ADR-0034 §2)", tool.Name)
		}
	}
	for name := range applies {
		if !seen[name] {
			t.Errorf("%s is classified but not registered", name)
		}
	}
	for name := range short {
		if !seen[name] {
			t.Errorf("%s is classified but not registered", name)
		}
	}
}

func TestReadFileWholeFileAppliesTheAllotment(t *testing.T) {
	r := newRegistry(t)
	numberedLines(t, r.ProjectDir(), "big.txt", 5000)
	out := runAllotted(t, r, "read_file", testAllot, map[string]any{"path": "big.txt"})
	if len(out) > testAllot+footerSlack {
		t.Fatalf("returned %d bytes under a %d-byte allotment", len(out), testAllot)
	}
	if !regexp.MustCompile(`offset=\d+ reads on\]$`).MatchString(out) {
		t.Errorf("the cut names no route: %q", out[len(out)-120:])
	}
}

// A line window ends on a whole line and names the next one: before
// ADR-0034 it ended mid-line with no route at all.
func TestReadFileLineWindowEndsOnALineAndNamesTheNext(t *testing.T) {
	r := newRegistry(t)
	numberedLines(t, r.ProjectDir(), "big.txt", 5000)
	out := runAllotted(t, r, "read_file", testAllot, map[string]any{"path": "big.txt", "start_line": float64(100), "end_line": float64(4000)})
	if len(out) > testAllot+footerSlack {
		t.Fatalf("returned %d bytes under a %d-byte allotment", len(out), testAllot)
	}
	m := regexp.MustCompile(`\[showing lines 100–(\d+) of 5000\] \[start_line=(\d+) reads on\]`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no route in %q", out[len(out)-200:])
	}
	last, err1 := strconv.Atoi(m[1])
	next, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		t.Fatalf("route numbers: %v %v", err1, err2)
	}
	if next != last+1 {
		t.Errorf("shown through line %d, route reads on from %d", last, next)
	}
	body := out[:strings.Index(out, "\n[showing")]
	lines := strings.Split(body, "\n")
	if len(lines) != last-100+1 {
		t.Errorf("%d lines shown for lines 100–%d", len(lines), last)
	}
	whole := regexp.MustCompile(`^line \d{6} x{60}$`)
	for _, l := range lines {
		if !whole.MatchString(l) {
			t.Fatalf("a line was cut: %q", l)
		}
	}
	if !strings.Contains(out, "more line(s) of the window not shown") {
		t.Errorf("the dropped lines are not counted: %q", out[len(out)-200:])
	}
}

func TestReadFileByteWindowAppliesTheAllotment(t *testing.T) {
	r := newRegistry(t)
	numberedLines(t, r.ProjectDir(), "big.txt", 5000)
	out := runAllotted(t, r, "read_file", testAllot, map[string]any{"path": "big.txt", "offset": float64(0), "length": float64(100_000)})
	want := fmt.Sprintf("[bytes 0–%d of %d — offset=%d reads on]", testAllot, 5000*73, testAllot)
	if !strings.HasSuffix(out, want) {
		t.Errorf("byte window under the allotment ends %q, want %q", out[len(out)-80:], want)
	}
}

// Without an allotment nothing changes: the tool's own cap applies.
func TestNoAllotmentKeepsTheToolsOwnCap(t *testing.T) {
	r := newRegistry(t)
	numberedLines(t, r.ProjectDir(), "big.txt", 5000)
	out, err := run(t, r, "read_file", map[string]any{"path": "big.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < readCap {
		t.Errorf("a 365 KB file with no allotment returned %d bytes, under the %d-byte cap", len(out), readCap)
	}
	if len(out) > readCap+footerSlack {
		t.Errorf("returned %d bytes past the cap", len(out))
	}
}

func TestNavigationToolsApplyTheAllotmentAndSayHowToNarrow(t *testing.T) {
	r := newRegistry(t)
	dir := filepath.Join(r.ProjectDir(), "many")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 300 {
		name := fmt.Sprintf("file-with-a-fairly-long-name-%04d.txt", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("needle "+strings.Repeat("y", 80)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name string
		args map[string]any
	}{
		{"search_files", map[string]any{"pattern": "needle"}},
		{"list_tree", map[string]any{}},
		{"list_files", map[string]any{"path": "many"}},
	}
	for _, c := range cases {
		out := runAllotted(t, r, c.name, 1024, c.args)
		if len(out) > 1024+footerSlack {
			t.Errorf("%s returned %d bytes under a 1024-byte allotment", c.name, len(out))
		}
		if !strings.Contains(out, narrowRoute) {
			t.Errorf("%s cut names no route: %q", c.name, out[max(len(out)-160, 0):])
		}
	}
	var paths []any
	for i := range 10 {
		paths = append(paths, fmt.Sprintf("many/file-with-a-fairly-long-name-%04d.txt", i))
	}
	out := runAllotted(t, r, "file_info", 300, map[string]any{"paths": paths})
	if len(out) > 300+footerSlack || !strings.Contains(out, narrowRoute) {
		t.Errorf("file_info under a 300-byte allotment: %d bytes, %q", len(out), out[max(len(out)-160, 0):])
	}
}

func TestShellExecAppliesTheAllotment(t *testing.T) {
	r := newRegistry(t)
	out := runAllotted(t, r, ShellExecName, testAllot, map[string]any{"command": "seq 1 50000"})
	if len(out) > testAllot+footerSlack {
		t.Errorf("returned %d bytes under a %d-byte allotment", len(out), testAllot)
	}
	if !strings.Contains(out, "50000") {
		t.Errorf("the tail is not kept: %q", out[len(out)-200:])
	}
}

func TestCutUnkeptSaysTheRestWasNotKept(t *testing.T) {
	s := strings.Repeat("あ", 1000)
	out := CutUnkept(s, 100)
	if !strings.HasSuffix(out, "of 3000 bytes shown; the rest was not kept]") {
		t.Errorf("marker: %q", out)
	}
	if !strings.HasPrefix(out, strings.Repeat("あ", 33)+"\n") {
		t.Errorf("cut through a rune or past the limit: %q", out[:120])
	}
	if CutUnkept("short", 100) != "short" {
		t.Error("a result within its allotment was changed")
	}
}

func TestWithAllotClampsNegativeToZero(t *testing.T) {
	n, ok := Allot(WithAllot(context.Background(), -5))
	if !ok || n != 0 {
		t.Errorf("Allot = %d, %v", n, ok)
	}
	if _, ok := Allot(context.Background()); ok {
		t.Error("a context with no allotment reports one")
	}
}
