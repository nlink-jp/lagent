package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/sandbox"
)

// TestTUIOptionsWiring pins the runREPL → tui.New handoff at the AST
// level. gem-agent ADR-0029 shipped with the catalog resolved in cmd but never
// passed to the TUI — the field existed, the tests passed it directly,
// and the one production call site omitted it, so the entire chrome
// silently fell back to English (review round 2). A behavioral test
// cannot reach this literal without running the whole REPL, so the
// wiring itself is the thing under test: every field listed here must
// be set in the tui.Options composite literal in root.go.
func TestTUIOptionsWiring(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "root.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	required := map[string]bool{
		"Msgs":         false, // gem-agent ADR-0029: the field this test exists for
		"Theme":        false,
		"Banner":       false,
		"CompletePath": false,
		"Settings":     false,
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Options" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "tui" {
			return true
		}
		found = true
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok {
				if _, tracked := required[key.Name]; tracked {
					required[key.Name] = true
				}
			}
		}
		return true
	})
	if !found {
		t.Fatal("no tui.Options literal found in root.go")
	}
	for field, set := range required {
		if !set {
			t.Errorf("tui.Options in root.go does not set %s — the TUI silently loses that wiring", field)
		}
	}
}

// ADR-0004's predicate must reach agent.New, or every MCP tool is
// advertised and the catalog is decoration. Pinned on the source: the
// wiring is a closure over the session.
func TestAdvertiseIsWiredIntoTheAgent(t *testing.T) {
	src, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "Advertise:      adv.Advertise,") && !strings.Contains(string(src), "Advertise: adv.Advertise,") {
		t.Fatal("agent.New is not given adv.Advertise")
	}
}

// ADR-0008 §2: a one-shot run's agent must know it is unattended, or a
// denial keeps telling the model to ask a user who is not there.
func TestUnattendedIsWiredIntoTheAgent(t *testing.T) {
	src, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "Unattended:   oneShot,") && !strings.Contains(string(src), "Unattended: oneShot,") {
		t.Fatal("agent.New is not given Unattended: oneShot")
	}
}

// ADR-0009: the operator's reasoning_effort must reach the backend, or
// the config key is decoration.
func TestReasoningEffortIsWiredIntoTheBackend(t *testing.T) {
	src, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "backend.SetReasoningEffort(cfg.LLM.ReasoningEffort)") {
		t.Fatal("the backend is not given cfg.LLM.ReasoningEffort")
	}
}

// ADR-0008 §1: every lane's shell runs with the scratch-cache table
// pointed into the session scratch — the read lane at the table's
// directory, the approved lanes at a separate one — and the read lane
// additionally scrubs the operator's secrets and points its temporary
// directory at the scratch.
func TestLaneEnvRedirectsToolchainCachesInEveryLane(t *testing.T) {
	parent := []string{"PATH=/bin", "GOCACHE=/Users/someone/Library/Caches/go-build", "AWS_SECRET_ACCESS_KEY=x"}
	caches := map[string]string{"GOCACHE": "go-build", "PIP_CACHE_DIR": "pip"}
	for _, lane := range []sandbox.Lane{sandbox.LaneRead, sandbox.LaneWrite, sandbox.LaneOperator} {
		env := strings.Join(laneEnv(lane, "/scratch", caches, parent), "\n")
		// The unasked lane and the approved lanes never share a cache
		// directory: a shared content-addressed cache is a route from a
		// read-lane command to an approved build's output.
		suffix := "-approved"
		if lane == sandbox.LaneRead {
			suffix = ""
		}
		for _, want := range []string{"GOCACHE=/scratch/go-build" + suffix, "PIP_CACHE_DIR=/scratch/pip" + suffix} {
			if !strings.Contains(env+"\n", want+"\n") {
				t.Errorf("%s lane: %s not in the environment:\n%s", lane, want, env)
			}
		}
		if idx := strings.LastIndex(env, "GOCACHE="); !strings.HasPrefix(env[idx:], "GOCACHE=/scratch/go-build") {
			t.Errorf("%s lane: the redirected GOCACHE must come last so it wins:\n%s", lane, env)
		}
	}
	read := strings.Join(laneEnv(sandbox.LaneRead, "/scratch", caches, parent), "\n")
	if !strings.Contains(read, "TMPDIR=/scratch") || strings.Contains(read, "AWS_SECRET_ACCESS_KEY") {
		t.Errorf("read lane must scrub secrets and point TMPDIR at the scratch:\n%s", read)
	}
	write := strings.Join(laneEnv(sandbox.LaneWrite, "/scratch", caches, parent), "\n")
	if strings.Contains(write, "TMPDIR=/scratch") {
		t.Errorf("the write lane keeps its own TMPDIR:\n%s", write)
	}
	if env := laneEnv(sandbox.LaneWrite, "", caches, parent); len(env) != len(parent) {
		t.Errorf("no scratch, no redirect: %v", env)
	}
	// The table renders in name order, so the environment is stable
	// across runs; an empty table renders nothing.
	if got := strings.Join(toolchainCacheEnv("/s", sandbox.LaneRead, caches), " "); got != "GOCACHE=/s/go-build PIP_CACHE_DIR=/s/pip" {
		t.Errorf("toolchainCacheEnv = %q", got)
	}
	if got := toolchainCacheEnv("/s", sandbox.LaneWrite, nil); len(got) != 0 {
		t.Errorf("empty table rendered %v", got)
	}
	if scratchCachesLabel(caches) != "GOCACHE→go-build, PIP_CACHE_DIR→pip" || scratchCachesLabel(nil) != "(none)" {
		t.Errorf("label = %q / %q", scratchCachesLabel(caches), scratchCachesLabel(nil))
	}
}
