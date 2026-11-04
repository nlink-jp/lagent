package tools

// The file tools name the session work directory as shell commands do,
// $LAGENT_WORK_DIR/… (ADR-0039 §7.1): the saved-result note uses that
// name because the model mis-copies the long absolute one.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkDirRef(t *testing.T) {
	for _, tc := range []struct {
		in   string
		rest string
		ok   bool
	}{
		{"$LAGENT_WORK_DIR/a.json", "a.json", true},
		{"${LAGENT_WORK_DIR}/sub/a.json", "sub/a.json", true},
		{"$LAGENT_WORK_DIR", "", true},
		{"${LAGENT_WORK_DIR}", "", true},
		{"$LAGENT_WORK_DIR/../x", "../x", true}, // not cleaned: the resolver checks containment
		{"$LAGENT_WORK_DIRX/a", "", false},
		{"$LAGENT_WORK_DIR_OLD/a", "", false},
		{"./$LAGENT_WORK_DIR/a", "", false},
		{"x/$LAGENT_WORK_DIR/a", "", false},
		{"$HOME/a", "", false},
		{"/abs/a", "", false},
		{"", "", false},
	} {
		rest, ok := WorkDirRef(tc.in)
		if rest != tc.rest || ok != tc.ok {
			t.Errorf("WorkDirRef(%q) = %q, %v; want %q, %v", tc.in, rest, ok, tc.rest, tc.ok)
		}
	}
}

func TestWorkDirNameIsTheInverseForFilesInside(t *testing.T) {
	work := "/state/lagent/-Users-me-proj/work/s1"
	for _, tc := range []struct{ in, want string }{
		{work + "/a.json", "$LAGENT_WORK_DIR/a.json"},
		{work + "/sub/b.txt", "$LAGENT_WORK_DIR/sub/b.txt"},
		{work, work}, // the directory itself is not a saved file
		{work + "-other/a.json", work + "-other/a.json"}, // a sibling sharing the prefix
		{"/elsewhere/a.json", "/elsewhere/a.json"},
	} {
		if got := WorkDirName(work, tc.in); got != tc.want {
			t.Errorf("WorkDirName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := WorkDirName("", work+"/a.json"); got != work+"/a.json" {
		t.Errorf("no work directory: got %q", got)
	}
	if rest, ok := WorkDirRef(WorkDirName(work, work+"/sub/b.txt")); !ok || rest != "sub/b.txt" {
		t.Errorf("the name does not round-trip: %q, %v", rest, ok)
	}
}

// Every file tool goes through the one resolver, so the short name
// works for a read, a copy source and a destination alike.
func TestTheFileToolsResolveTheWorkDirVariable(t *testing.T) {
	r, work := newRegistryWithWork(t)
	body := []byte("{\"rows\":[1,2,3]}\n")
	if err := os.WriteFile(filepath.Join(work, "bq-query-x.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, r, "read_file", map[string]any{"path": "$LAGENT_WORK_DIR/bq-query-x.json"})
	if err != nil || !strings.Contains(out, "rows") {
		t.Fatalf("read_file = %q, %v", out, err)
	}
	if _, err := run(t, r, "write_file", map[string]any{
		"path": "evidence/rows.json", "from": "${LAGENT_WORK_DIR}/bq-query-x.json"}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(r.ProjectDir(), "evidence", "rows.json")); err != nil || string(got) != string(body) {
		t.Fatalf("copy = %q, %v", got, err)
	}
	if _, err := run(t, r, "write_file", map[string]any{
		"path": "$LAGENT_WORK_DIR/notes/n.md", "content": "note\n"}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(work, "notes", "n.md")); err != nil || string(got) != "note\n" {
		t.Fatalf("work-dir write = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(r.ProjectDir(), "$LAGENT_WORK_DIR")); !os.IsNotExist(err) {
		t.Errorf("the variable was taken as a project directory name: %v", err)
	}
	real, err := r.RealPath("$LAGENT_WORK_DIR/bq-query-x.json")
	if err != nil {
		t.Fatal(err)
	}
	wantReal, _ := filepath.EvalSymlinks(filepath.Join(work, "bq-query-x.json"))
	if real != wantReal {
		t.Errorf("RealPath = %q, want %q (the decision point judges this path)", real, wantReal)
	}
}

// The expansion happens before containment is checked, so climbing out
// is refused like any other escape; with no work directory the name is
// an error, never a project path — even when the project happens to
// hold a directory of that literal name, which ./ still reaches.
func TestTheWorkDirVariableKeepsContainment(t *testing.T) {
	r, work := newRegistryWithWork(t)
	outside := filepath.Join(filepath.Dir(work), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"$LAGENT_WORK_DIR/../outside.txt", "$LAGENT_WORK_DIR/../../../../../../etc/hosts"} {
		if _, err := r.resolvePath(p); err == nil || !strings.Contains(err.Error(), "escapes") {
			t.Errorf("%s: err = %v, want an escape refusal", p, err)
		}
		if out, err := run(t, r, "read_file", map[string]any{"path": p}); err == nil && strings.Contains(out, "secret") {
			t.Errorf("%s was read: %q", p, out)
		}
	}

	bare := newRegistry(t)
	lit := filepath.Join(bare.ProjectDir(), "$LAGENT_WORK_DIR")
	if err := os.MkdirAll(lit, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lit, "a.txt"), []byte("project file"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := bare.resolvePath("$LAGENT_WORK_DIR/a.txt")
	if err == nil || !strings.Contains(err.Error(), "no work directory") {
		t.Errorf("with no work directory: err = %v", err)
	}
	if out, err := run(t, bare, "read_file", map[string]any{"path": "./$LAGENT_WORK_DIR/a.txt"}); err != nil || !strings.Contains(out, "project file") {
		t.Errorf("./$LAGENT_WORK_DIR did not reach the project directory: %q, %v", out, err)
	}
}
