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
)

// A probe file that cannot be removed is named, with its directory, and
// a path already gone is not (ADR-0041 §2).
func TestRemoveProbesReportsLeftovers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes entries from a read-only directory")
	}
	dir := filepath.Join(t.TempDir(), ".lagent-lane-probe-x")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stuck := filepath.Join(dir, "plain")
	if err := os.WriteFile(stuck, []byte("probe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(dir, "AGENTS.md")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chmod(dir, 0o700)
		_ = os.Remove(stuck)
		_ = os.Remove(dir)
	}()
	left := removeProbes(stuck, gone, dir)
	if len(left) != 2 || left[0] != stuck || left[1] != dir {
		t.Fatalf("left = %v, want [%s %s]", left, stuck, dir)
	}
	if note := LeftoverNote(left); !strings.Contains(note, stuck) || !strings.Contains(note, "delete them yourself") {
		t.Errorf("note = %q", note)
	}
	if left := removeProbes(gone); len(left) != 0 {
		t.Errorf("a path already gone was reported: %v", left)
	}
}

// stuckProbes makes removeFile fail for any path under dir whose base
// starts with prefix — a probe the operator's role may create but not
// delete — and returns a cleanup that restores removeFile and removes
// what was left.
func stuckProbes(t *testing.T, dir, prefix string) func() []string {
	t.Helper()
	var stuck []string
	removeFile = func(p string) error {
		if within(dir, p) && strings.HasPrefix(filepath.Base(p), prefix) {
			stuck = append(stuck, p)
			return os.ErrPermission
		}
		return os.Remove(p)
	}
	return func() []string {
		removeFile = os.Remove
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
