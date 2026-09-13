package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// credentialProject adds credential-named entries to the navigation
// fixture: a .env and a nested .env.local holding the needle, a
// template that is not a secret, a private key, and a directory the
// credential list names whole.
func credentialProject(t *testing.T) *Registry {
	t.Helper()
	r := navProject(t)
	dir := r.ProjectDir()
	for path, content := range map[string]string{
		".env":              "maxRetries=1\nAPI_KEY=sk-test-marker-never-real\n",
		"config/.env.local": "maxRetries=2\n",
		".env.example":      "maxRetries=0\n",
		"keys/id_rsa":       "maxRetries PRIVATE\n",
		".aws/credentials":  "maxRetries aws\n",
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// ADR-0016 §3: the walks list a credential-named entry like any other.
// The kernel lists names and refuses content, so a name was never the
// secret, and withholding it was a second rule over the same unbounded
// spelling domain. What a walk cannot read, it names.
func TestWalksListNamesAndNameWhatTheyCannotRead(t *testing.T) {
	r := credentialProject(t)
	dir := r.ProjectDir()

	out, err := run(t, r, "list_files", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".env", ".aws/", ".env.example"} {
		if !strings.Contains(out, want) {
			t.Errorf("list_files hid %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[credential files:") {
		t.Errorf("list_files still carries the withholding footer:\n%s", out)
	}

	out, err = run(t, r, "list_tree", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".env", "id_rsa", "credentials"} {
		if !strings.Contains(out, want) {
			t.Errorf("list_tree hid %q:\n%s", want, out)
		}
	}

	// A file the walk cannot open is named, not silently absent — the
	// shape `grep -r` has. In production the refusal comes from the
	// sandbox; here an unreadable mode produces the same error.
	blocked := filepath.Join(dir, "unreadable.txt")
	if err := os.WriteFile(blocked, []byte("maxRetries here\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	out, err = run(t, r, "search_files", map[string]any{"pattern": "maxRetries"})
	if err != nil {
		t.Fatal(err)
	}
	// The footer names every file the walk could not open, in walk
	// order: the credential-named ones this project holds are refused by
	// the list before the open (ADR-0016 §5, amended), so unreadable.txt
	// is one entry among several rather than the first.
	if !strings.Contains(out, "[not read:") || !strings.Contains(out, "unreadable.txt") ||
		!strings.Contains(out, "operator's approval") {
		t.Errorf("search_files did not name the file it could not read:\n%s", out)
	}
	// And the credential content never reached the result, even though
	// this registry has no file child (the degraded path).
	for _, leaked := range []string{"sk-test-marker-never-real", "maxRetries PRIVATE", "maxRetries aws"} {
		if strings.Contains(out, leaked) {
			t.Errorf("search_files read credential material:\n%s", out)
		}
	}
	// And the ordinary entries are searched as before.
	for _, want := range []string{".env.example:1", "docs/readme.md:2"} {
		if !strings.Contains(out, want) {
			t.Errorf("search_files lost %q:\n%s", want, out)
		}
	}
}

// The footers that survive the output cap are the ones that still
// exist: the caps and the ignore tally (ADR-0016 §3 removed the
// credential footer).
func TestFootersSurviveTheOutputCap(t *testing.T) {
	r := credentialProject(t)
	dir := r.ProjectDir()
	line := "needle " + strings.Repeat("x", 230) + "\n"
	for i := 0; i < 60; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("hay%02d.txt", i)), []byte(strings.Repeat(line, 6)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := run(t, r, "search_files", map[string]any{"pattern": "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[output truncated:") {
		t.Fatalf("the fixture did not pass the byte cap (%d bytes)", len(out))
	}
	for _, want := range []string{"matches in", "stopped at the 200-line cap"} {
		if !strings.Contains(out[strings.Index(out, "[output truncated:"):], want) {
			t.Errorf("search footer lost %q after the cap:\n…%s", want, out[len(out)-600:])
		}
	}
	for d := 0; d < 18; d++ {
		sub := filepath.Join(dir, fmt.Sprintf("bulk%02d", d))
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < 50; f++ {
			if err := os.WriteFile(filepath.Join(sub, fmt.Sprintf("%s-%02d.txt", strings.Repeat("n", 60), f)), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	out, err = run(t, r, "list_tree", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[output truncated:") {
		t.Fatalf("the tree fixture did not pass the byte cap (%d bytes)", len(out))
	}
	if !strings.Contains(out[strings.Index(out, "[output truncated:"):], "[stopped at 800 entries") {
		t.Errorf("tree footer lost its cap line:\n…%s", out[len(out)-600:])
	}
}
