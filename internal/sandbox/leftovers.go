// Ported from gem-agent internal/sandbox/roots.go (removeFile,
// removeProbes, LeftoverNote) at e42f89e95c054425e0f13b14cce242b9a3866fb8
// (v0.99.0), ADR-0001 / ADR-0041 §2.

package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// removeFile is os.Remove; a test makes it fail to reach the leftover
// paths, which a real volume only takes with a sharing role.
var removeFile = os.Remove

// removeProbes removes a probe's files and then its directory, in the
// order given, and names each it could not remove (ADR-0041 §2). In a
// synced folder a leftover does not stay local: it is copied to the
// operator's other devices and to everyone who shares the folder, so it
// is reported, never left in silence. A path already gone is not one.
func removeProbes(paths ...string) []string {
	var left []string
	for _, p := range paths {
		if err := removeFile(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			left = append(left, p)
		}
	}
	return left
}

// LeftoverNote is the operator's note for probe files that could not be
// removed: what is left, and that it is theirs to delete.
func LeftoverNote(left []string) string {
	return fmt.Sprintf("the sandbox check could not remove its test files: %s — delete them yourself", strings.Join(left, ", "))
}
