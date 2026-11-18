// Tests ported from gem-agent internal/sandbox/roots_test.go
// (TestRemoveProbesReportsLeftovers, stuckProbes,
// TestProbesReturnLeftoversWhateverTheResult) at
// e42f89e95c054425e0f13b14cce242b9a3866fb8 (v0.99.0), without the added
// roots (lagent has none; ADR-0001).

package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/bounded"
)

// TestCleanupReportsLeftovers: a probe file that cannot be removed is
// named, with its directory, and a name already gone is not
// (ADR-0041 §2). The directory is made read-only so its entries cannot
// be unlinked.
func TestCleanupReportsLeftovers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes entries from a read-only directory")
	}
	root, err := openProbes(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir, err := root.subdir(".lagent-root-probe-")
	if err != nil {
		t.Fatal(err)
	}
	if err := dir.fileNamed(".env", "probe\n"); err != nil {
		t.Fatal(err)
	}
	dir.expect("created") // never made: already gone
	stuck := dir.at(".env")
	if err := os.Chmod(dir.path, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chmod(dir.path, 0o700)
		_ = os.Remove(stuck)
		_ = os.Remove(dir.path)
	}()
	left := root.cleanup()
	if len(left) != 2 || left[0] != stuck || left[1] != dir.path {
		t.Fatalf("left = %v, want [%s %s]", left, stuck, dir.path)
	}
	note := LeftoverNote(left)
	if !strings.Contains(note, stuck) || !strings.Contains(note, dir.path) || !strings.Contains(note, "delete them yourself") {
		t.Errorf("note does not name the leftovers and what to do: %q", note)
	}
}

// stuckProbes makes the removal fail for any name under dir that starts
// with prefix — a probe directory the operator's role may create but
// not delete — and returns a cleanup that restores the removal and
// removes what was left.
func stuckProbes(t *testing.T, dir, prefix string) func() []string {
	t.Helper()
	var stuck []string
	remove := removeProbe
	removeProbe = func(d *bounded.Dir, p string, isDir bool) error {
		if within(dir, p) && strings.HasPrefix(filepath.Base(p), prefix) {
			stuck = append(stuck, p)
			return os.ErrPermission
		}
		return remove(d, p, isDir)
	}
	return func() []string {
		removeProbe = remove
		for _, p := range stuck {
			_ = os.Remove(p)
		}
		return stuck
	}
}

// Both lane probes hand back the probe file they could not remove in
// the project, on success and on failure alike (ADR-0041 §2).
func TestProbesReturnLeftoversWhateverTheResult(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("sandbox-exec is macOS-only")
	}
	if err := Available(); err != nil {
		t.Skipf("sandbox-exec cannot apply a profile here: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	proj, err := os.MkdirTemp(home, ".lagent-leftover-test-")
	if err != nil {
		t.Skip("cannot create a directory under home")
	}
	defer func() { _ = os.RemoveAll(proj) }()
	proj, _ = ResolveWriteDir(proj)
	spec := Spec{ProjectDir: proj, Home: home}
	write, err := LaneProfile(LaneWrite, spec)
	if err != nil {
		t.Fatal(err)
	}
	read, err := LaneProfile(LaneRead, spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name    string
		profile string
		fails   bool
	}{{"passing", write, false}, {"failing", "(version 1)(allow default)", true}} {
		done := stuckProbes(t, proj, ".lagent-lane-probe-")
		left, err := VerifyWriteLane(c.profile, spec)
		stuck := done()
		if (err != nil) != c.fails || len(left) != 1 || len(stuck) != 1 || left[0] != stuck[0] {
			t.Errorf("VerifyWriteLane %s: left %v, stuck %v, err %v", c.name, left, stuck, err)
		}
	}
	// Whether this read profile passes here does not matter (it has no
	// scratch): the leftover is returned either way.
	done := stuckProbes(t, proj, ".lagent-lane-probe-")
	left, _ := VerifyReadLane(read, spec)
	if stuck := done(); len(left) != 1 || len(stuck) != 1 || left[0] != stuck[0] {
		t.Errorf("VerifyReadLane: left %v, stuck %v", left, stuck)
	}
}
