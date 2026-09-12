package sandbox

import (
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/session"
	"github.com/nlink-jp/lagent/internal/workdir"
)

// The read lane keeps the runtime's own exports by name, not by
// prefix (system risk review R01): LAGENT_API_KEY is a real config
// variable, and the LAGENT_ prefix exemption carried it into every
// read-lane command's environment ahead of the secret-name rule.
func TestScrubEnvKeepsExportsByNameNotPrefix(t *testing.T) {
	env := ScrubEnv([]string{
		"PATH=/bin",
		"LAGENT_API_KEY=sk-not-for-the-model",
		"LAGENT_SESSION_ID=s",
		"LAGENT_WORK_DIR=/state/work/s",
		"LAGENT_PROJECT_DIR=/proj",
		"LAGENT_MODEL=m",               // an ordinary name: kept by the regex, not the list
		"LAGENT_STATE_DIR=/state",      // read by the runtime, not exported for commands: the regex decides
		"LAGENT_FUTURE_AUTH_SOCKET=/s", // a hypothetical later export with a secret-looking name: dropped until listed
	})
	got := strings.Join(env, "\n")
	for _, gone := range []string{"LAGENT_API_KEY=", "LAGENT_FUTURE_AUTH_SOCKET="} {
		if strings.Contains(got, gone) {
			t.Errorf("%s survived the scrub:\n%s", gone, got)
		}
	}
	for _, kept := range []string{"PATH=", "LAGENT_SESSION_ID=s", "LAGENT_WORK_DIR=/state/work/s", "LAGENT_PROJECT_DIR=/proj", "LAGENT_MODEL=m", "LAGENT_STATE_DIR=/state"} {
		if !strings.Contains(got, kept) {
			t.Errorf("%s was scrubbed:\n%s", kept, got)
		}
	}
}

// The allowlist spells the export sites' own constants: a renamed
// export must be renamed here too, or the read lane loses it.
func TestReadLaneExportsAreTheExportSites(t *testing.T) {
	want := map[string]bool{session.EnvVar: true, workdir.EnvVar: true, workdir.ProjectEnvVar: true}
	if len(readLaneExports) != len(want) {
		t.Fatalf("readLaneExports = %v, want exactly the three export constants %v", readLaneExports, want)
	}
	for name := range want {
		if !readLaneExports[name] {
			t.Errorf("readLaneExports lacks %s", name)
		}
	}
}
