package cmd

import (
	"context"
	"testing"

	"github.com/nlink-jp/lagent/internal/skills"
	"github.com/nlink-jp/lagent/internal/tools"
)

// ADR-0034 §2: the tools cmd registers return the operator's or the
// runtime's own words, bounded at their source; the round budget's
// backstop must never cut them (pre-release review: an operator's long
// ask_user answer and mcp_load's tool list were cut).
func TestCmdToolsAreKeptWhole(t *testing.T) {
	reg, err := tools.New(t.TempDir(), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := registerAskTool(reg, func(context.Context, string, []string) (int, error) { return 0, nil }); err != nil {
		t.Fatal(err)
	}
	if err := registerMemoryTools(reg, memoryStore{}); err != nil {
		t.Fatal(err)
	}
	if err := registerSkillTool(reg, func() []skills.Skill { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := registerMCPLoadTool(reg, newMCPAdvertiser(false, nil, nil), func() {}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ask_user", "save_memory", "delete_memory", skills.ToolName, MCPLoadName} {
		tool, ok := reg.Get(name)
		if !ok {
			t.Fatalf("%s not registered", name)
		}
		if !tool.KeepWhole {
			t.Errorf("%s is not KeepWhole", name)
		}
	}
}
