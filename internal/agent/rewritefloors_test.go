package agent

// ADR-0051: the floors against summarizing overwrites that live at the
// agent layer — the approval detail carries write_file's replacement
// annotation, and the compaction stand-in warns that file contents are
// no longer verbatim.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/llm"
)

func TestDescribeCarriesTheWriteAnnotation(t *testing.T) {
	a, reg := newAgent(t, &mockBackend{}, &approveAll{}, 5)
	if err := os.WriteFile(filepath.Join(reg.ProjectDir(), "sop.md"),
		[]byte(strings.Repeat("x", 4096)), 0o644); err != nil {
		t.Fatal(err)
	}

	detail, _ := a.Describe(llm.ToolCall{Name: "write_file",
		Args: map[string]any{"path": "sop.md", "content": "short"}})
	if !strings.Contains(detail, "replaces existing file: 4KB → 5B") {
		t.Errorf("detail lacks the replacement annotation: %q", detail)
	}

	// A new file annotates nothing — the detail stays single-line.
	detail, _ = a.Describe(llm.ToolCall{Name: "write_file",
		Args: map[string]any{"path": "fresh.md", "content": "short"}})
	if strings.Contains(detail, "replaces existing file") {
		t.Errorf("new-file detail carries a replacement annotation: %q", detail)
	}
}
