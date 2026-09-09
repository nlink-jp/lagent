package cmd

import (
	"testing"

	"github.com/nlink-jp/lagent/internal/uitext"
)

// "/mcp reload" dispatches to the
// reload closures; any other trailing word is a typo and says so.
func TestSlashReloadSubcommands(t *testing.T) {
	en := uitext.For(uitext.EN)
	reloads := slashReloads{
		mcp: func() string { return "MCP-RELOADED" },
	}
	out, isErr, _ := slashOutput("/mcp reload", nil, nil, nil, reloads, nil, "", en, nil)
	if isErr || out != "MCP-RELOADED" {
		t.Errorf("/mcp reload: %q isErr=%v", out, isErr)
	}
	if _, isErr, _ = slashOutput("/mcp restart", nil, nil, nil, reloads, nil, "", en, nil); !isErr {
		t.Error("unknown subcommand accepted")
	}
	// Reload unavailable (nil closure) reads as unknown, not a panic.
}

// ADR-0039 §3: load_skill reads the live list through its getter, and
