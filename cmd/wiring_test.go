package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
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
