package sandbox

import (
	"sort"
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/session"
	"github.com/nlink-jp/lagent/internal/workdir"
)

// ADR-0017: the runtime removes its own configuration variables from
// every child and touches nothing else. The withdrawn scrub did the
// reverse — it kept all of LAGENT_ and guessed, from a name, which of
// the operator's variables were secrets.
//
// R01 asked for a LAGENT_API_KEY that cannot reach a child. It cannot,
// and not because anything recognises the word "key": the key is a
// variable the runtime reads for itself, so it is in the removed half
// by what it is.
func TestChildEnvRemovesOnlyTheRuntimesOwn(t *testing.T) {
	env := ChildEnv([]string{
		// The runtime's own: removed.
		"LAGENT_API_KEY=sk-not-for-the-model", "LAGENT_STATE_DIR=/state",
		"LAGENT_PROVIDER=lmstudio", "LAGENT_BASE_URL=http://127.0.0.1:1234/v1",
		"LAGENT_MODEL=m", "LAGENT_REASONING_EFFORT=low",
		"LAGENT_LLM_TRACE=1", "LAGENT_MCP_STDERR=1",
		// The runtime's exports for children: kept.
		"LAGENT_SESSION_ID=s", "LAGENT_WORK_DIR=/state/work/s", "LAGENT_PROJECT_DIR=/proj",
		// The operator's world, untouched — including every name the
		// withdrawn scrub would have taken.
		"PATH=/bin", "HOME=/Users/x", "GOFLAGS=-mod=mod",
		"GITHUB_TOKEN=abc", "AWS_SECRET_ACCESS_KEY=k", "OPENAI_KEY=z", "GH_PAT=p",
	})
	got := strings.Join(env, "\n")
	for _, gone := range []string{
		"LAGENT_API_KEY=", "LAGENT_STATE_DIR=", "LAGENT_PROVIDER=", "LAGENT_BASE_URL=",
		"LAGENT_MODEL=", "LAGENT_REASONING_EFFORT=", "LAGENT_LLM_TRACE=", "LAGENT_MCP_STDERR=",
	} {
		if strings.Contains(got, gone) {
			t.Errorf("%s reached a child:\n%s", gone, got)
		}
	}
	for _, kept := range []string{
		"LAGENT_SESSION_ID=s", "LAGENT_WORK_DIR=/state/work/s", "LAGENT_PROJECT_DIR=/proj",
		"PATH=/bin", "HOME=/Users/x", "GOFLAGS=-mod=mod",
		"GITHUB_TOKEN=abc", "AWS_SECRET_ACCESS_KEY=k", "OPENAI_KEY=z", "GH_PAT=p",
	} {
		if !strings.Contains(got, kept) {
			t.Errorf("%s was removed:\n%s", kept, got)
		}
	}
	// The mechanism, shown on a name nothing else would catch: removed
	// because the list names it, not because it looks like anything.
	runtimeOwnEnv["LAGENT_QUIET_NAME"] = true
	defer delete(runtimeOwnEnv, "LAGENT_QUIET_NAME")
	if got := ChildEnv([]string{"LAGENT_QUIET_NAME=x", "QUIET_NAME=y"}); len(got) != 1 || got[0] != "QUIET_NAME=y" {
		t.Errorf("ChildEnv does not read runtimeOwnEnv: %v", got)
	}
}

// The exports spell the export sites' own constants: a renamed export
// must be renamed here too, or every child loses it.
func TestChildExportsAreTheExportSites(t *testing.T) {
	want := []string{session.EnvVar, workdir.EnvVar, workdir.ProjectEnvVar}
	sort.Strings(want)
	if got := ChildExportNames(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ChildExportNames() = %v, want the export constants %v", got, want)
	}
}
