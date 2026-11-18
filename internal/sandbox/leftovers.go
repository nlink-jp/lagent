// Ported from gem-agent internal/sandbox/roots.go (LeftoverNote; the
// removal by path that came with it is gone, ADR-0051) at e42f89e95c054425e0f13b14cce242b9a3866fb8
// (v0.99.0), ADR-0001 / ADR-0041 §2.

package sandbox

import (
	"fmt"
	"strings"
)

// LeftoverNote is the operator's note for probe files that could not be
// removed: what is left, and that it is theirs to delete.
func LeftoverNote(left []string) string {
	return fmt.Sprintf("the sandbox check could not remove its test files: %s — delete them yourself", strings.Join(left, ", "))
}
