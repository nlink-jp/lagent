package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSystemPromptShape(t *testing.T) {
	sys := buildSystemPrompt("/tmp/proj", "")

	// The defensive framing must stay at the very top, ahead of
	// anything a project could put in front of it.
	if !strings.HasPrefix(sys, "SECURITY, read first:") {
		t.Error("defensive instructions must lead the prompt")
	}
	// The tag is announced in the runtime-facts message, not templated
	// into the prompt (ADR-0003): a per-session nonce here would bust
	// the server's prefix cache every session.
	if strings.Contains(sys, "{{DATA_TAG}}") {
		t.Error("prompt must not carry the data-tag placeholder")
	}
	if !strings.Contains(sys, "/tmp/proj") {
		t.Error("prompt must name the project directory")
	}
}

func TestSystemPromptAppendsProjectContext(t *testing.T) {
	sys := buildSystemPrompt("/tmp/proj", "\n\nProject instructions:\n\n### AGENTS.md\n\nbuild with make")
	if !strings.Contains(sys, "build with make") {
		t.Error("project context not appended")
	}
	if strings.Index(sys, "SECURITY, read first:") > strings.Index(sys, "build with make") {
		t.Error("project context must come after the defensive framing")
	}
}

// TestLoadInstructionsReadsVendorFiles wires the loader to a real
// directory: the names and the ancestor walk are covered in
// internal/instructions, this checks the cmd-level seam.
func TestLoadInstructionsReadsVendorFiles(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory")
	}
	// Build the project under the real home so the ancestor walk (which
	// stops at home) reaches it.
	base, err := os.MkdirTemp(home, "lagent-test-")
	if err != nil {
		t.Skip("cannot create a temp dir under home")
	}
	defer func() { _ = os.RemoveAll(base) }()
	proj := filepath.Join(base, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "GEMINI.md"), []byte("gemini rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "CLAUDE.md"), []byte("workspace rules"), 0o644); err != nil {
		t.Fatal(err)
	}

	section, labels, notes := loadInstructions(proj, projectGrant{trusted: true})
	if len(notes) != 0 {
		t.Fatalf("notes = %v", notes)
	}
	if !strings.Contains(section, "gemini rules") || !strings.Contains(section, "workspace rules") {
		t.Errorf("section missing content: %q", section)
	}
	if len(labels) < 2 {
		t.Errorf("labels = %v, want the ancestor file and the project file", labels)
	}
	// Nearest last: the project's own file is the final section.
	if strings.Index(section, "workspace rules") > strings.Index(section, "gemini rules") {
		t.Error("project rules should come after ancestor rules")
	}
}

// The per-session facts ride the runtime's opening message, never the
// system prompt: the prompt must be byte-identical across sessions, or
// a local server re-processes every tool schema it renders after it
// (measured: 118 s against 2 s with 243 MCP tools).
func TestSystemPromptIsIdenticalAcrossSessions(t *testing.T) {
	a := buildSystemPrompt("/proj", "")
	b := buildSystemPrompt("/proj", "")
	if a != b {
		t.Fatal("two builds of the system prompt differ")
	}
	for _, volatile := range []string{"{{DATA_TAG}}", "Session work directory:", "Session started:", time.Now().Format("2006-01-02")} {
		if strings.Contains(a, volatile) {
			t.Errorf("system prompt carries per-session text %q", volatile)
		}
	}
	// The model is still told where those facts are.
	if !strings.Contains(a, "runtime-facts message") {
		t.Error("the prompt does not point at the runtime-facts message")
	}
}

// A capability nothing points at never gets used: without the facts the
// model has no way to learn the directory exists, and keeps putting
// intermediates in the project.
func TestSessionFactsNameTheWorkDirectory(t *testing.T) {
	work := "/state/lagent/proj/work/sess-1"
	got := sessionFacts(work, nil)
	if !strings.Contains(got, work) {
		t.Error("the work directory is not named in the facts")
	}
	// The literal path, not the variable: an MCP tool argument is JSON
	// the model writes, and nothing expands a variable on the way.
	if !strings.Contains(got, "$LAGENT_WORK_DIR") {
		t.Error("shell commands are not told the variable exists")
	}
	if i, j := strings.Index(got, work), strings.Index(got, "$LAGENT_WORK_DIR"); i < 0 || j < 0 || i > j {
		t.Error("the literal path should be given before the variable is mentioned")
	}
	if !strings.Contains(got, "session started: "+time.Now().Format("2006-01-02")) {
		t.Errorf("the start date is missing: %q", got)
	}
}

func TestSessionFactsOmitTheDirectoryWhenNone(t *testing.T) {
	got := sessionFacts("", nil)
	if strings.Contains(got, "work directory") {
		t.Error("a session with no work directory should not be told it has one")
	}
	if !strings.Contains(got, "session started:") {
		t.Error("the start date must still be given")
	}
}

// The prompt names only tools this runtime registers: a tool named to
// the model and absent from the roster sends it looking for a route
// that does not exist (the fork-era seam).
func TestSystemPromptNamesOnlyRegisteredTools(t *testing.T) {
	sys := buildSystemPrompt("/proj", "")
	for _, absent := range []string{"agentic_file_search", "summarize_file", "read_document", "datetime", "gem_agent_purpose", "Vertex", "Gemini"} {
		if strings.Contains(sys, absent) {
			t.Errorf("system prompt names %q, which this runtime does not have", absent)
		}
	}
	for _, present := range []string{"list_tree", "search_files", "read_file", "edit_file", "write_file", "shell_exec", "view_image", "lagent_purpose"} {
		if !strings.Contains(sys, present) {
			t.Errorf("system prompt lost %q", present)
		}
	}
}

// The system prompt says nothing about diagrams, on any surface
// (gem-agent ADR-0063): no tool to call, no format to prefer, no prohibition to
// over-generalize. Fence rendering is a view-layer concern, and the
// model's natural prior — a mermaid fence in Markdown — is already the
// wanted behavior. The measured failure this pins against: "do NOT
// write a mermaid fence" bred hand-drawn box art in replies and files.
func TestSystemPromptSaysNothingAboutDiagrams(t *testing.T) {
	sys := strings.ToLower(buildSystemPrompt("/proj", ""))
	for _, banned := range []string{"mermaid", "diagram", "ascii art", "box art"} {
		if strings.Contains(sys, banned) {
			t.Errorf("system prompt mentions %q — gem-agent ADR-0063 keeps diagrams out of the prompt", banned)
		}
	}
}

// The MCP catalog rides the facts message after the work directory and
// the date, line by line as the inventory rendered it.
func TestSessionFactsCarryTheCatalog(t *testing.T) {
	got := sessionFacts("", []string{"- MCP servers connected this session.", "  - tor-exit (2 tools: check_ip, update_list)"})
	if !strings.Contains(got, "tor-exit (2 tools") || strings.Index(got, "session started:") > strings.Index(got, "MCP servers") {
		t.Errorf("catalog missing or out of order:\n%s", got)
	}
}
