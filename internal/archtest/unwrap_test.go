package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnwrappedToolResultsAreOnlyTheSkillTool: every tool result
// enters the prompt nonce-wrapped as data, with one exemption —
// the skill tool, whose results are the operator's own installed
// instructions and are confined to a discovered skill's directory
// (ADR-0011). The exemption is the agent's `InstructionTools` option;
// this test pins every place that sets it (outside tests) to exactly
// `[]string{skills.ToolName}`. A second exemption needs a record of
// its own, and then this test, not a longer literal.
func TestUnwrappedToolResultsAreOnlyTheSkillTool(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	seen := 0
	for _, dir := range []string{"internal", "cmd"} {
		abs := filepath.Join(root, dir)
		err := filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if p != abs && (strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, p, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				kv, ok := n.(*ast.KeyValueExpr)
				if !ok {
					return true
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok || key.Name != "InstructionTools" {
					return true
				}
				seen++
				pos := fset.Position(kv.Pos())
				lit, ok := kv.Value.(*ast.CompositeLit)
				if !ok || len(lit.Elts) != 1 {
					t.Errorf("%s:%d: InstructionTools must be the one-element literal []string{skills.ToolName}", pos.Filename, pos.Line)
					return true
				}
				sel, ok := lit.Elts[0].(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "ToolName" {
					t.Errorf("%s:%d: the only unwrapped tool is skills.ToolName (ADR-0011); got %s", pos.Filename, pos.Line, types(lit.Elts[0]))
					return true
				}
				if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "skills" {
					t.Errorf("%s:%d: the only unwrapped tool is skills.ToolName (ADR-0011)", pos.Filename, pos.Line)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if seen != 1 {
		t.Errorf("InstructionTools is set in %d places outside tests; the wiring in cmd/root.go is the one", seen)
	}
}

func types(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		return v.Value
	case *ast.Ident:
		return v.Name
	default:
		return "an expression"
	}
}
