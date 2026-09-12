package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// credentialProject adds credential-named entries to the navigation
// fixture (ADR-0015 §2): a .env and a nested .env.local holding the
// needle, a template that is not a secret, a private key, and a
// directory the list names whole.
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

// splitFooter separates a tool result from its credential footer.
func splitFooter(t *testing.T, out string) (body, footer string) {
	t.Helper()
	i := strings.Index(out, "[credential files:")
	if i < 0 {
		t.Fatalf("no credential footer:\n%s", out)
	}
	return out[:i], out[i:]
}

// search_files never reads a credential file — no line of it is
// returned — and says how many it skipped, by name, with the route;
// include_ignored does not unlock them; the template is searched.
func TestSearchFilesSkipsCredentialFilesAndSaysSo(t *testing.T) {
	r := credentialProject(t)
	for _, args := range []map[string]any{
		{"pattern": "maxRetries"},
		{"pattern": "maxRetries", "include_ignored": true},
	} {
		out, err := run(t, r, "search_files", args)
		if err != nil {
			t.Fatal(err)
		}
		body, footer := splitFooter(t, out)
		for _, leaked := range []string{"sk-test-marker", ".env:1", ".env.local:1", "id_rsa:1", "credentials:1", "PRIVATE"} {
			if strings.Contains(body, leaked) {
				t.Errorf("%v: search read a credential file (%q):\n%s", args, leaked, out)
			}
		}
		if !strings.Contains(body, ".env.example:1") {
			t.Errorf("%v: the template was not searched:\n%s", args, out)
		}
		for _, want := range []string{"4 skipped", ".env", "config/.env.local", "keys/id_rsa", ".aws/", "read_file on one asks the operator"} {
			if !strings.Contains(footer, want) {
				t.Errorf("%v: footer lacks %q: %s", args, want, footer)
			}
		}
	}
	// A search rooted at a directory of credential files finds nothing
	// and says why, rather than reporting an empty scan.
	out, err := run(t, r, "search_files", map[string]any{"pattern": "maxRetries", "path": "keys"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "PRIVATE") || !strings.Contains(out, "no matches") || !strings.Contains(out, "1 skipped (keys/id_rsa)") {
		t.Errorf("search rooted at keys/:\n%s", out)
	}
}

// list_tree leaves credential entries out — a file, a nested file, and
// a directory the list names whole — and reports them; the template
// and the parent directories stay.
func TestListTreeSkipsCredentialFilesAndSaysSo(t *testing.T) {
	out, err := run(t, credentialProject(t), "list_tree", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	body, footer := splitFooter(t, out)
	for _, absent := range []string{`(?m)^\.env$`, `id_rsa`, `(?m)^\.aws/`, `credentials`, `(?m)^  \.env\.local$`} {
		if regexp.MustCompile(absent).MatchString(body) {
			t.Errorf("tree lists a credential entry %q:\n%s", absent, out)
		}
	}
	for _, present := range []string{`(?m)^\.env\.example$`, `(?m)^config/$`, `(?m)^keys/$`, `(?m)^main\.go$`} {
		if !regexp.MustCompile(present).MatchString(body) {
			t.Errorf("tree lacks %q:\n%s", present, out)
		}
	}
	for _, want := range []string{"4 skipped", ".env", "config/.env.local", "keys/id_rsa", ".aws/", "read_file on one asks the operator"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer lacks %q: %s", want, footer)
		}
	}
}

// list_files leaves credential entries out and reports them; a
// directory holding only credential files lists as its footer, never
// as empty.
func TestListFilesSkipsCredentialFilesAndSaysSo(t *testing.T) {
	r := credentialProject(t)
	out, err := run(t, r, "list_files", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	body, footer := splitFooter(t, out)
	if regexp.MustCompile(`(?m)^\.env$`).MatchString(body) || strings.Contains(body, ".aws/") {
		t.Errorf("list_files listed a credential entry:\n%s", out)
	}
	for _, want := range []string{".env.example", "config/", "keys/", "main.go"} {
		if !strings.Contains(body, want) {
			t.Errorf("list_files lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(footer, "2 skipped (.aws/, .env)") && !strings.Contains(footer, "2 skipped (.env, .aws/)") {
		t.Errorf("footer: %s", footer)
	}
	out, err = run(t, r, "list_files", map[string]any{"path": "keys"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "empty directory") || !strings.HasPrefix(out, "[credential files: 1 skipped (keys/id_rsa)") {
		t.Errorf("list_files keys/:\n%s", out)
	}
}
