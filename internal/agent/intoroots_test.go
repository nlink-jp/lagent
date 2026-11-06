// Tests ported from gem-agent internal/agent/intoroots_test.go at
// e42f89e95c054425e0f13b14cce242b9a3866fb8 (v0.99.0), with the work
// directory and the project as the roots (lagent has no added roots; ADR-0001).

package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/risk"
)

func realDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// A write through a link outside the roots lands in the project it
// leads to, auto-approved as an edit there, and every observer of the
// call sees the real path (ADR-0041 §1).
func TestWriteThroughAnOutsideLinkLandsInTheProject(t *testing.T) {
	outside := realDir(t)
	b := &autoBackend{}
	gate := &recordingGate{}
	a, reg, decisions := newAutoAgent(t, b, gate)
	link := filepath.Join(outside, "proj")
	if err := os.Symlink(reg.ProjectDir(), link); err != nil {
		t.Fatal(err)
	}
	b.responses = []*llm.Response{
		{ToolCalls: []llm.ToolCall{writeCall(filepath.Join(link, "hello.txt"))}},
		{Content: "done"},
	}
	var seen []string
	a.onToolCall = func(tc llm.ToolCall) { seen = append(seen, tc.Args["path"].(string)) }
	if _, err := a.Run(context.Background(), "write it", nil); err != nil {
		t.Fatal(err)
	}
	if len(gate.asked) != 0 {
		t.Errorf("the write reached the gate: %v", gate.asked)
	}
	if d := (*decisions)[0]; !d.Approved || d.Tier != risk.Safe {
		t.Errorf("decision = %+v, want a safe edit inside the project", d)
	}
	want := filepath.Join(reg.ProjectDir(), "hello.txt")
	if b, err := os.ReadFile(want); err != nil || string(b) != "x" {
		t.Errorf("the write did not land in the project: %q %v", b, err)
	}
	if len(seen) != 1 || seen[0] != want {
		t.Errorf("the call was shown as %v, want %q", seen, want)
	}
}

// A link to the project's AGENTS.md is judged as AGENTS.md — the
// operator's alone — and the approval does not show the link's spelling.
func TestALinkToTheProjectsInstructionsIsTheOperators(t *testing.T) {
	outside := realDir(t)
	b := &autoBackend{verdict: `{"approve": true, "confidence": 1.0, "reason": "fine"}`}
	gate := &recordingGate{}
	a, reg, _ := newAutoAgent(t, b, gate)
	link := filepath.Join(outside, "work")
	if err := os.Symlink(reg.ProjectDir(), link); err != nil {
		t.Fatal(err)
	}
	b.responses = []*llm.Response{
		{ToolCalls: []llm.ToolCall{writeCall(filepath.Join(link, "AGENTS.md"))}},
		{Content: "done"},
	}
	if _, err := a.Run(context.Background(), "edit", nil); err != nil {
		t.Fatal(err)
	}
	if len(gate.asked) != 1 || strings.Contains(gate.asked[0], outside) {
		t.Errorf("gate = %v, want the operator asked about the real AGENTS.md", gate.asked)
	}
	if _, err := os.Stat(filepath.Join(reg.ProjectDir(), "AGENTS.md")); err == nil {
		t.Error("AGENTS.md was written past a denied approval")
	}
}

// After the rewrite the outside link is never consulted: swapping it
// for another target does not move the write, and the verdict was
// taken on the string that runs.
func TestTheRewrittenCallDoesNotFollowASwappedLink(t *testing.T) {
	outside := realDir(t)
	a, reg, _ := newAutoAgent(t, &autoBackend{}, &recordingGate{})
	work := realDir(t)
	if err := reg.UseWorkDir(work); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(outside, "proj")
	if err := os.Symlink(reg.ProjectDir(), link); err != nil {
		t.Fatal(err)
	}
	tc := a.intoRoots(writeCall(filepath.Join(link, "x.txt")))
	if d := a.decide(tc); d.Verdict.Tier != risk.Safe {
		t.Fatalf("verdict on the rewritten call = %+v", d.Verdict)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(reg.WorkDir(), link); err != nil {
		t.Fatal(err)
	}
	tool, _ := reg.Get("write_file")
	if _, err := tool.Run(context.Background(), tc.Args); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(reg.ProjectDir(), "x.txt")); err != nil {
		t.Errorf("the write left the project the verdict judged: %v", err)
	}
	if _, err := os.Stat(filepath.Join(reg.WorkDir(), "x.txt")); err == nil {
		t.Error("the write followed the swapped link")
	}
}

// A link leading out of the roots is left as typed and refused with
// today's words, which do not name where it leads; only the file tools'
// path arguments are touched.
func TestIntoRootsLeavesEverythingElse(t *testing.T) {
	outside, escape := realDir(t), realDir(t)
	a, reg, _ := newAutoAgent(t, &autoBackend{}, &recordingGate{})
	if err := os.Symlink(escape, filepath.Join(outside, "elsewhere")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(reg.ProjectDir(), filepath.Join(outside, "proj")); err != nil {
		t.Fatal(err)
	}
	typed := filepath.Join(outside, "elsewhere", "x.txt")
	tc := a.intoRoots(writeCall(typed))
	if tc.Args["path"] != typed {
		t.Fatalf("a link leading out was rewritten to %v", tc.Args["path"])
	}
	tool, _ := reg.Get("write_file")
	if _, err := tool.Run(context.Background(), tc.Args); err == nil || strings.Contains(err.Error(), escape) {
		t.Errorf("err = %v, want a refusal that does not name %s", err, escape)
	}
	if _, err := os.Stat(filepath.Join(escape, "x.txt")); err == nil {
		t.Error("the write escaped")
	}
	in := filepath.Join(outside, "proj", "a.txt")
	real := filepath.Join(reg.ProjectDir(), "a.txt")
	got := a.intoRoots(llm.ToolCall{Name: "write_file", Args: map[string]any{"path": in, "from": in}})
	if got.Args["path"] != real || got.Args["from"] != real {
		t.Errorf("write_file = %v", got.Args)
	}
	got = a.intoRoots(llm.ToolCall{Name: "file_info", Args: map[string]any{"paths": []any{in, "rel.txt", 3}}})
	if ps := got.Args["paths"].([]any); ps[0] != real || ps[1] != "rel.txt" || ps[2] != 3 {
		t.Errorf("file_info paths = %v", ps)
	}
	if got := a.intoRoots(llm.ToolCall{Name: "list_files", Args: map[string]any{"path": filepath.Join(outside, "proj")}}); got.Args["path"] != reg.ProjectDir() {
		t.Errorf("the project by its link = %v", got.Args["path"])
	}
	for _, c := range []llm.ToolCall{
		{Name: "shell_exec", Args: map[string]any{"path": in}},
		{Name: "mcp__x__y", Args: map[string]any{"path": in}},
		{Name: "read_file", Args: map[string]any{"from": in}},
	} {
		if got := a.intoRoots(c); got.Args["path"] != c.Args["path"] || got.Args["from"] != c.Args["from"] {
			t.Errorf("%s was rewritten: %v", c.Name, got.Args)
		}
	}
}
