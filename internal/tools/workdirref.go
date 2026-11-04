package tools

import (
	"path/filepath"
	"strings"

	"github.com/nlink-jp/lagent/internal/pathid"
	"github.com/nlink-jp/lagent/internal/workdir"
)

// The file tools name the session work directory the way a shell command
// does: a path whose first segment is exactly $LAGENT_WORK_DIR (or
// ${LAGENT_WORK_DIR}) lies under it (ADR-0039 §7.1).
//
// It exists because the absolute name is a trap. The work directory sits
// under a state directory named after the project path with every `/`
// replaced by `-`, and the project path itself stands in the model's
// context: measured on the local model, 7 of 10 runs copied the saved
// file's name right and rewrote that state-directory name as the project
// path, so the copy failed or never happened. With the short name the
// only part the model reproduces is the file name.
//
// Only the first segment, and only in the built-in file tools: an MCP
// argument is still passed as written. A project directory literally
// named $LAGENT_WORK_DIR is reached as ./$LAGENT_WORK_DIR.
var workDirVars = []string{"$" + workdir.EnvVar, "${" + workdir.EnvVar + "}"}

// WorkDirRef reports whether p names a path in the session work
// directory by the variable, and returns the rest of it (possibly "",
// the directory itself). The rest is not cleaned: containment is the
// resolver's check, made after joining.
func WorkDirRef(p string) (rest string, ok bool) {
	for _, v := range workDirVars {
		if p == v {
			return "", true
		}
		if strings.HasPrefix(p, v+"/") {
			return p[len(v)+1:], true
		}
	}
	return "", false
}

// WorkDirName is the inverse for a file the runtime saved: the name the
// file tools accept for abs, $LAGENT_WORK_DIR/<rel>, when abs lies inside
// workDir; otherwise abs itself. The saved-result note names files this
// way so the model copies a short name it reproduces reliably.
func WorkDirName(workDir, abs string) string {
	if workDir == "" || abs == "" {
		return abs
	}
	if !pathid.Within(workDir, abs) || pathid.Same(workDir, abs) {
		return abs
	}
	rel, err := pathid.Rel(workDir, abs)
	if err != nil {
		return abs
	}
	return workDirVars[0] + "/" + filepath.ToSlash(rel)
}
