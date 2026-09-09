package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestADRCitationsResolve: an `ADR-NNNN` in a comment or a string of
// cmd/ and internal/ names one of this repository's records
// (docs/en/adr/NNNN-*.md) unless it is written `gem-agent ADR-NNNN`,
// the porting source's. The ported code keeps the source's design
// references (ADR-0001), and bare they pointed a maintainer at the
// wrong record: lagent's ADR-0004 is MCP loading, gem-agent's is
// auto-approve, and most of the source's numbers name records this
// repository does not have at all.
func TestADRCitationsResolve(t *testing.T) {
	root := repoRoot(t)
	local := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(root, "docs", "en", "adr"))
	if err != nil {
		t.Fatal(err)
	}
	adrFile := regexp.MustCompile(`^(\d{4})-.*\.md$`)
	for _, e := range entries {
		if m := adrFile.FindStringSubmatch(e.Name()); m != nil {
			local[m[1]] = true
		}
	}
	if len(local) == 0 {
		t.Fatal("no ADR files found under docs/en/adr")
	}
	// A qualified citation names the porting source; a bare one must
	// resolve here. "pre-ADR-NNNN" and the like count as bare.
	cite := regexp.MustCompile(`(gem-agent(?:'s)? )?ADR-(\d{4})`)
	check := func(pos token.Position, text string) {
		for _, m := range cite.FindAllStringSubmatch(text, -1) {
			if m[1] != "" || local[m[2]] {
				continue
			}
			t.Errorf("%s:%d: cites ADR-%s, which is not a record of this repository — write `gem-agent ADR-%s` for the porting source's, or add the ADR",
				pos.Filename, pos.Line, m[2], m[2])
		}
	}
	fset := token.NewFileSet()
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
			if !strings.HasSuffix(p, ".go") {
				return nil
			}
			f, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			for _, cg := range f.Comments {
				for _, c := range cg.List {
					check(fset.Position(c.Pos()), c.Text)
				}
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					check(fset.Position(lit.Pos()), lit.Value)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
