package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nlink-jp/lagent/internal/config"
	"github.com/nlink-jp/lagent/internal/llm"
)

// A file that drifted before an approved write is not re-pinned by it:
// pinIsCurrent answers false before the write, and the guard carries
// that answer to the after hook (verification B).
func TestApprovedWriteDoesNotRepinADriftedFile(t *testing.T) {
	_, pf, policyPath, proj := pinFixture(t)
	if err := repin(policyPath, proj, pf); err != nil {
		t.Fatal(err)
	}
	if !pinIsCurrent(pf, proj, "AGENTS.md") {
		t.Fatal("freshly pinned file reported as drifted")
	}
	// A name with neither pin nor file is current: the write creates it.
	if !pinIsCurrent(pf, proj, "CLAUDE.md") {
		t.Fatal("absent file with no pin reported as drifted")
	}
	// `! git pull` rewrites AGENTS.md; then the model edits it and the
	// operator approves that edit.
	if err := os.WriteFile(filepath.Join(proj, "AGENTS.md"), []byte("pulled\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tc := llm.ToolCall{ID: "c1", Name: "edit_file", Args: map[string]any{"path": "AGENTS.md"}}
	g := &writeGuard{}
	g.begin(tc, "AGENTS.md", pinIsCurrent(pf, proj, "AGENTS.md"))
	if err := os.WriteFile(filepath.Join(proj, "AGENTS.md"), []byte("pulled and edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if g.end(tc, "AGENTS.md") {
		t.Fatal("guard reported the drifted file as current")
	}
	// An end without a begin is not current either.
	if g.end(llm.ToolCall{ID: "c2"}, "AGENTS.md") {
		t.Fatal("unknown call reported as current")
	}
}

// mutatePins edits the set the file holds at that moment: a second
// session's re-pin between our load and our write is kept, not reverted
// (verification C).
func TestMutatePinsKeepsConcurrentChanges(t *testing.T) {
	_, pf, policyPath, proj := pinFixture(t)
	if err := repin(policyPath, proj, pf); err != nil {
		t.Fatal(err)
	}
	// Another session records a new pin for .mcp.json on disk; our
	// in-memory copy does not know.
	stale := *pf
	if _, err := config.MutatePolicyFile(policyPath, func(other *config.PolicyFile) {
		pins := other.PinsFor(proj)
		pins[".mcp.json"] = "sha256:theirs"
		other.SetPins(proj, pins)
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "AGENTS.md"), []byte("ours\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repinName(policyPath, proj, "AGENTS.md", &stale, nil); err != nil {
		t.Fatal(err)
	}
	disk, err := config.LoadPolicyFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if disk.PinsFor(proj)[".mcp.json"] != "sha256:theirs" {
		t.Errorf("the other session's pin was reverted: %v", disk.PinsFor(proj))
	}
	if disk.PinsFor(proj)["AGENTS.md"] == pf.PinsFor(proj)["AGENTS.md"] {
		t.Error("our re-pin was not recorded")
	}
	if stale.PinsFor(proj)[".mcp.json"] != "sha256:theirs" {
		t.Error("the in-memory copy did not follow the file")
	}
}
