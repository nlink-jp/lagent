package cmd

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/tools"
	"github.com/nlink-jp/lagent/internal/uitext"
)

// loadHarness registers two stub servers' tools and returns the
// advertiser over that inventory.
func loadHarness(t *testing.T, all bool, preload, allow []string) (*mcpAdvertiser, *tools.Registry, mcpInventory) {
	t.Helper()
	reg, err := tools.New(t.TempDir(), func(ctx context.Context, c string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/true")
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	inv := mcpInventory{Servers: []string{"github", "tor-exit"}, Offered: map[string][]string{},
		Scopes:  map[string]string{"github": "global", "tor-exit": "global"},
		summary: map[string]string{}, registered: map[string][]string{}, instructions: map[string]string{}}
	for _, s := range []*stubServer{
		{stubCaller: stubCaller{name: "tor-exit"}, tools: []string{"check_ip", "update_list"}, instructions: "Reports whether an IP is a Tor exit. Call list_status first."},
		{stubCaller: stubCaller{name: "github"}, tools: []string{"get_me"}},
	} {
		if !attachMCPServer(context.Background(), s, reg, &strings.Builder{}, filterOf(t), &inv) {
			t.Fatalf("%s did not attach", s.name)
		}
	}
	adv := newMCPAdvertiser(all, preload, allow)
	adv.setInventory(inv)
	return adv, reg, inv
}

func TestAdvertiserHidesUntilLoaded(t *testing.T) {
	adv, reg, _ := loadHarness(t, false, nil, nil)
	if adv.Advertise("mcp__tor-exit__check_ip") || adv.Advertise("mcp__github__get_me") {
		t.Fatal("an unloaded server's tools are advertised")
	}
	if !adv.Advertise("read_file") || !adv.Advertise(MCPLoadName) {
		t.Fatal("a built-in is hidden")
	}
	out, err := adv.Load("tor-exit", reg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "mcp__tor-exit__check_ip") || !strings.Contains(out, "loaded") {
		t.Errorf("load result = %q", out)
	}
	if !adv.Advertise("mcp__tor-exit__check_ip") || adv.Advertise("mcp__github__get_me") {
		t.Error("loading one server advertised the wrong set")
	}
	if got := adv.LoadedServers(); len(got) != 1 || got[0] != "tor-exit" {
		t.Errorf("LoadedServers = %v", got)
	}
	// A second load says so instead of pretending it changed something.
	if out, _ := adv.Load("tor-exit", reg); !strings.Contains(out, "already") {
		t.Errorf("second load = %q", out)
	}
	// Reset is /clear's: back to nothing.
	adv.Reset()
	if adv.Advertise("mcp__tor-exit__check_ip") {
		t.Error("Reset kept a load")
	}
}

func TestAdvertiserRefusesUnknownWithTheNames(t *testing.T) {
	adv, reg, _ := loadHarness(t, false, nil, nil)
	_, err := adv.Load("tor_exit", reg)
	if err == nil || !strings.Contains(err.Error(), "tor-exit") || !strings.Contains(err.Error(), "github") {
		t.Fatalf("err = %v, want the known names", err)
	}
}

// Preloads, from the config and from a --allow grant naming the server
// (the sanitized spelling the pattern carries), are advertised from the
// start and survive Reset.
func TestAdvertiserPreloads(t *testing.T) {
	adv, _, _ := loadHarness(t, false, []string{"github"}, []string{"mcp__tor-exit__*", "write_file", "mcp__x__y"})
	for _, name := range []string{"mcp__github__get_me", "mcp__tor-exit__check_ip"} {
		if !adv.Advertise(name) {
			t.Errorf("%s not advertised from the start", name)
		}
	}
	adv.Reset()
	if !adv.Advertise("mcp__github__get_me") || !adv.Advertise("mcp__tor-exit__check_ip") {
		t.Error("Reset dropped a preload")
	}
	if got := serversFromAllow([]string{"mcp__a__*", "mcp__a__b", "mcp__*", "shell_exec"}); len(got) != 1 || got[0] != "a" {
		t.Errorf("serversFromAllow = %v", got)
	}
}

// advertise = "all" is the baseline: everything shown, nothing to load.
func TestAdvertiserAllShowsEverything(t *testing.T) {
	adv, reg, _ := loadHarness(t, true, nil, nil)
	if !adv.Advertise("mcp__github__get_me") || !adv.Loaded("github") {
		t.Fatal("all mode hid a tool")
	}
	if out, err := adv.Load("github", reg); err != nil || !strings.Contains(out, "already") {
		t.Errorf("load under all = %q, %v", out, err)
	}
}

func TestCatalogLinesNameServersToolsAndTrigger(t *testing.T) {
	adv, reg, inv := loadHarness(t, false, nil, nil)
	lines := inv.catalogLines(adv)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "mcp_load") {
		t.Errorf("the catalog does not say how to load: %q", joined)
	}
	if !strings.Contains(joined, "tor-exit (2 tools: check_ip, update_list) — Reports whether an IP is a Tor exit.") {
		t.Errorf("tor-exit line = %q", joined)
	}
	if !strings.Contains(joined, "github (1 tools: get_me)") {
		t.Errorf("github line = %q", joined)
	}
	if strings.Contains(joined, "[loaded") {
		t.Error("nothing is loaded yet")
	}
	if _, err := adv.Load("github", reg); err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(inv.catalogLines(adv), "\n"); !strings.Contains(joined, "github (1 tools: get_me) [loaded") {
		t.Errorf("loaded marker missing: %q", joined)
	}
	if got := inv.catalogLines(nil); len(got) != 3 {
		t.Errorf("catalog without an advertiser = %d lines, want header + 2", len(got))
	}
	if got := (mcpInventory{}).catalogLines(adv); got != nil {
		t.Errorf("an empty inventory produced a catalog: %v", got)
	}
}

func TestSummaryLinesCarryTheLoadedMarker(t *testing.T) {
	adv, reg, inv := loadHarness(t, false, nil, nil)
	_, _ = adv.Load("tor-exit", reg)
	joined := strings.Join(inv.summaryLinesWith(adv.Loaded), "\n")
	if !strings.Contains(joined, "tor-exit [global] (2 tools) — loaded") {
		t.Errorf("loaded marker missing: %q", joined)
	}
	if !strings.Contains(joined, "github [global] (1 tools) — not loaded (mcp_load, or /mcp load github)") {
		t.Errorf("not-loaded marker missing: %q", joined)
	}
	if plain := strings.Join(inv.summaryLines(), "\n"); strings.Contains(plain, "loaded") {
		t.Errorf("the plain summary carries a marker: %q", plain)
	}
}

// A resumed transcript's mcp_load calls are replayed; a server that is
// gone is skipped.
func TestReplayLoadsFollowsTheTranscript(t *testing.T) {
	adv, reg, _ := loadHarness(t, false, nil, nil)
	history := []llm.Message{
		{Role: llm.RoleUser, Content: "x"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
			{ID: "1", Name: MCPLoadName, Args: map[string]any{"server": "tor-exit"}},
			{ID: "2", Name: MCPLoadName, Args: map[string]any{"server": "gone"}},
			{ID: "3", Name: "read_file", Args: map[string]any{"path": "a"}},
		}},
	}
	if n := replayLoads(history, adv, reg); n != 1 {
		t.Errorf("replayed %d loads, want 1", n)
	}
	if !adv.Advertise("mcp__tor-exit__check_ip") || adv.Advertise("mcp__github__get_me") {
		t.Error("replay advertised the wrong set")
	}
}

// The mcp_load tool: read-only, refreshes the declarations, refuses
// blanks.
func TestMCPLoadToolRefreshesTheDeclarations(t *testing.T) {
	adv, reg, _ := loadHarness(t, false, nil, nil)
	refreshed := 0
	if err := registerMCPLoadTool(reg, adv, func() { refreshed++ }); err != nil {
		t.Fatal(err)
	}
	tool, ok := reg.Get(MCPLoadName)
	if !ok || tool.Mutating {
		t.Fatal("mcp_load missing or mutating")
	}
	if _, err := tool.Run(context.Background(), map[string]any{"server": " "}); err == nil {
		t.Error("a blank server name was accepted")
	}
	out, err := tool.Run(context.Background(), map[string]any{"server": "tor-exit"})
	if err != nil || !strings.Contains(out, "check_ip") || refreshed != 1 {
		t.Errorf("out=%q err=%v refreshed=%d", out, err, refreshed)
	}
}

// /mcp load <server> reaches the loader; the wrong shape is a typo.
func TestSlashMCPLoad(t *testing.T) {
	loaded := ""
	reloads := slashReloads{load: func(s string) (string, bool) { loaded = s; return "ok\n", false }}
	out, isErr, _ := slashOutput("/mcp load tor-exit", nil, nil, nil, nil, reloads, nil, "", uitext.For(uitext.EN), nil)
	if isErr || loaded != "tor-exit" || out != "ok\n" {
		t.Errorf("out=%q isErr=%v loaded=%q", out, isErr, loaded)
	}
	if _, isErr, _ := slashOutput("/mcp load", nil, nil, nil, nil, reloads, nil, "", uitext.For(uitext.EN), nil); !isErr {
		t.Error("/mcp load without a name was not an error")
	}
}
