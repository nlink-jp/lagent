package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/tools"
)

// mcpAdvertiser decides which MCP tools the model is shown (ADR-0004).
// Every server's tools are registered; only a loaded server's are
// advertised. Loaded is what the model asked for through mcp_load, what
// the operator preloaded ([mcp].preload, a --allow grant naming the
// server, /mcp load), or everything when [mcp].advertise = "all".
type mcpAdvertiser struct {
	mu      sync.Mutex
	all     bool
	preload map[string]bool
	loaded  map[string]bool
	// owner maps a registered tool name to its server; rebuilt from the
	// inventory after every connect or reconnect.
	owner map[string]string
	// present is every server that registered tools.
	present map[string]bool
}

// MCPLoadName is the built-in that advertises one server's tools.
const MCPLoadName = "mcp_load"

func newMCPAdvertiser(all bool, preload []string, allow []string) *mcpAdvertiser {
	a := &mcpAdvertiser{all: all, preload: map[string]bool{}, loaded: map[string]bool{}, owner: map[string]string{}, present: map[string]bool{}}
	for _, name := range preload {
		if name = strings.TrimSpace(name); name != "" {
			a.preload[name] = true
		}
	}
	for _, name := range serversFromAllow(allow) {
		a.preload[name] = true
	}
	a.reset()
	return a
}

// serversFromAllow reads the servers a --allow grant names: an entry of
// the form mcp__<server>__* is the operator's declaration that this run
// needs that server, so it is advertised from the start — a pipeline
// must not depend on the model remembering to load (ADR-0004). The
// name is the sanitized one the prefix carries; matching happens
// against sanitized server names in setInventory.
func serversFromAllow(patterns []string) []string {
	var out []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		rest, ok := strings.CutPrefix(p, "mcp__")
		if !ok {
			continue
		}
		server, ok := strings.CutSuffix(rest, "__*")
		if !ok || server == "" || strings.Contains(server, "__") {
			continue
		}
		out = append(out, server)
	}
	return out
}

// reset returns to the starting state: the preloaded servers only.
func (a *mcpAdvertiser) reset() {
	a.loaded = map[string]bool{}
	for name := range a.preload {
		a.loaded[name] = true
	}
}

// Reset is /clear's: a cleared session starts unloaded, preloads aside.
func (a *mcpAdvertiser) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reset()
}

// setInventory rebuilds the tool→server map from what the servers
// registered. Preloads written with either spelling of a server (its
// name, or the sanitized name a --allow pattern carries) resolve here.
func (a *mcpAdvertiser) setInventory(inv mcpInventory) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.owner = map[string]string{}
	a.present = map[string]bool{}
	for server, names := range inv.registered {
		has := false
		for _, n := range names {
			if strings.HasPrefix(n, mcpToolPrefix(server)) {
				a.owner[n] = server
				has = true
			}
		}
		if has {
			a.present[server] = true
		}
	}
	for want := range a.preload {
		for server := range a.present {
			if want == server || want == sanitizeToolName(server) {
				a.loaded[server] = true
			}
		}
	}
}

// Advertise is the agent's predicate: a built-in is always shown, an
// MCP tool when its server is loaded.
func (a *mcpAdvertiser) Advertise(name string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.all {
		return true
	}
	server, ok := a.owner[name]
	if !ok {
		return !strings.HasPrefix(name, "mcp__")
	}
	return a.loaded[server]
}

// Loaded reports whether the model has the server's tools.
func (a *mcpAdvertiser) Loaded(server string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.all || a.loaded[server]
}

// LoadedServers lists what is loaded, sorted, for the transcript and /mcp.
func (a *mcpAdvertiser) LoadedServers() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []string
	for server := range a.present {
		if a.all || a.loaded[server] {
			out = append(out, server)
		}
	}
	sort.Strings(out)
	return out
}

// Load advertises one server's tools and returns the one-line list of
// what became available. An unknown or excluded name is refused with
// the names that exist, so the model's next call can use one of them.
func (a *mcpAdvertiser) Load(server string, registry *tools.Registry) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	server = strings.TrimSpace(server)
	if !a.present[server] {
		var known []string
		for s := range a.present {
			known = append(known, s)
		}
		sort.Strings(known)
		if len(known) == 0 {
			return "", errors.New("no MCP server registered tools this session")
		}
		return "", fmt.Errorf("no MCP server named %q this session; the servers are: %s", server, strings.Join(known, ", "))
	}
	already := a.all || a.loaded[server]
	a.loaded[server] = true
	var b strings.Builder
	if already {
		fmt.Fprintf(&b, "%s is already loaded. Its tools:\n", server)
	} else {
		fmt.Fprintf(&b, "%s loaded. Its tools are in your tool list now:\n", server)
	}
	var names []string
	for n, s := range a.owner {
		if s == server {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		desc := ""
		if t, ok := registry.Get(n); ok {
			desc = firstSentence(strings.TrimPrefix(t.Description, "[MCP:"+server+"] "))
		}
		fmt.Fprintf(&b, "  - %s: %s\n", n, clipRunes(desc, catalogSentenceCap))
	}
	return b.String(), nil
}

// registerMCPLoadTool registers mcp_load. refresh re-advertises the
// tool list after a load — the agent caches its declarations.
func registerMCPLoadTool(registry *tools.Registry, adv *mcpAdvertiser, refresh func()) error {
	return registry.Register(&tools.Tool{
		Name: MCPLoadName,
		Description: "Load one MCP server named in the runtime facts: its tools are added to your tool list for " +
			"the rest of the session and listed in the result. Call it before using any tool of a server " +
			"that is not loaded. Loading runs nothing on the server.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"server": map[string]any{"type": "string", "description": "the server name exactly as the runtime facts list it"},
			},
			"required": []string{"server"},
		},
		Mutating: false,
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			server, _ := args["server"].(string)
			if strings.TrimSpace(server) == "" {
				return "", errors.New("server is required")
			}
			out, err := adv.Load(server, registry)
			if err != nil {
				return "", err
			}
			if refresh != nil {
				refresh()
			}
			return out, nil
		},
	})
}

// replayLoads re-advertises the servers a restored transcript loaded:
// every mcp_load call the model made, so a resumed session sees what it
// saw. Servers that are gone are skipped without a word — the catalog in
// the new facts message says what is here now.
func replayLoads(history []llm.Message, adv *mcpAdvertiser, registry *tools.Registry) int {
	n := 0
	for _, m := range history {
		if m.Role != llm.RoleAssistant {
			continue
		}
		for _, tc := range m.ToolCalls {
			if tc.Name != MCPLoadName {
				continue
			}
			if server, _ := tc.Args["server"].(string); server != "" {
				if _, err := adv.Load(server, registry); err == nil {
					n++
				}
			}
		}
	}
	return n
}
