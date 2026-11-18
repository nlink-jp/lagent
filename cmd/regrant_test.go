package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/skills"
	"github.com/nlink-jp/lagent/internal/trustpin"
)

// regrantProject builds a home with a project under it, so the
// instruction walk and the global skill directory are the test's own
// and not the machine's.
func regrantProject(t *testing.T) (home, proj string) {
	t.Helper()
	home = t.TempDir()
	// The loader compares resolved paths; /var is a link on macOS.
	if real, err := filepath.EvalSymlinks(home); err == nil {
		home = real
	}
	t.Setenv("HOME", home)
	proj = filepath.Join(home, "work", "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	return home, proj
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSkill(t *testing.T, root, name string) {
	t.Helper()
	writeFile(t, filepath.Join(root, name, "SKILL.md"),
		"---\nname: "+name+"\ndescription: Does "+name+".\n---\n# "+name+"\n\nFollow the steps.\n")
}

func skillNames(list []skills.Skill) string {
	names := make([]string, 0, len(list))
	for _, s := range list {
		names = append(names, s.Scope+":"+s.Name)
	}
	return strings.Join(names, " ")
}

// /clear re-read the instruction files and then kept the system prompt
// it had built at startup: the prompt must come from what was re-read.
// Files that did not change give the same bytes, which is what lets the
// server's prefix cache survive a /clear (ADR-0003).
func TestRegrantRebuildsTheInstructionsFromTheFilesAsTheyAre(t *testing.T) {
	_, proj := regrantProject(t)
	agents := filepath.Join(proj, "AGENTS.md")
	writeFile(t, agents, "build with make\n")
	grant := projectGrant{trusted: true}

	first, list, notes := regrant(proj, grant, nil)
	if len(notes) != 0 || len(list) != 0 {
		t.Fatalf("a plain project: skills %v, notes %v", list, notes)
	}
	if !strings.Contains(buildSystemPrompt(proj, first), "build with make") {
		t.Fatalf("the project's file is not in the prompt: %q", first)
	}

	again, _, _ := regrant(proj, grant, nil)
	if buildSystemPrompt(proj, again) != buildSystemPrompt(proj, first) {
		t.Error("unchanged files gave a different system prompt: the prefix cache would not survive /clear")
	}

	writeFile(t, agents, "build with just\n")
	edited, _, _ := regrant(proj, grant, nil)
	sys := buildSystemPrompt(proj, edited)
	if !strings.Contains(sys, "build with just") || strings.Contains(sys, "build with make") {
		t.Errorf("the prompt does not follow the edited file: %q", edited)
	}
}

// A project file that changed since it was trusted is left out of the
// new session — the note /clear prints says so, and the prompt has to
// agree with it. The operator's own files are not the project's to
// switch off.
func TestRegrantLeavesOutWhatTheGrantExcludes(t *testing.T) {
	home, proj := regrantProject(t)
	writeFile(t, filepath.Join(proj, "AGENTS.md"), "project rule\n")
	writeFile(t, filepath.Join(home, ".config", "lagent", "AGENTS.md"), "my own rule\n")

	section, _, _ := regrant(proj, projectGrant{trusted: true}, nil)
	if !strings.Contains(section, "project rule") || !strings.Contains(section, "my own rule") {
		t.Fatalf("both files should load while the grant allows them: %q", section)
	}

	section, _, _ = regrant(proj, projectGrant{trusted: true, excluded: map[string]bool{"AGENTS.md": true}}, nil)
	if strings.Contains(section, "project rule") {
		t.Errorf("an excluded project file is still in the prompt: %q", section)
	}
	if !strings.Contains(section, "my own rule") {
		t.Errorf("the operator's own file went with it: %q", section)
	}
}

// The skills follow the same grant: /clear kept the list startup had
// discovered, so a skill installed since was not offered and a project
// skill the re-check excluded still was.
func TestRegrantRediscoversTheSkills(t *testing.T) {
	home, proj := regrantProject(t)
	global := filepath.Join(home, ".config", "lagent", "skills")
	project := filepath.Join(proj, trustpin.SkillsDir)
	writeSkill(t, global, "notes")
	writeSkill(t, project, "release")
	grant := projectGrant{trusted: true}

	_, list, notes := regrant(proj, grant, nil)
	if got := skillNames(list); got != "global:notes project:release" || len(notes) != 0 {
		t.Fatalf("skills = %q, notes %v", got, notes)
	}

	// Installed while the session ran.
	writeSkill(t, global, "minutes")
	_, list, _ = regrant(proj, grant, list)
	if got := skillNames(list); got != "global:minutes global:notes project:release" {
		t.Errorf("after an install: skills = %q", got)
	}

	// The project skill's content changed since it was trusted.
	excluded := projectGrant{trusted: true, excluded: map[string]bool{trustpin.SkillsDir + "/release": true}}
	_, list, _ = regrant(proj, excluded, list)
	if got := skillNames(list); got != "global:minutes global:notes" {
		t.Errorf("an excluded project skill is still offered: skills = %q", got)
	}
	skills.CloseAll(list)
}

// The list a /clear replaces is closed, and the one it leaves is the
// one the process closes on exit: a skill's root is an open directory.
func TestRegrantClosesTheListItReplaces(t *testing.T) {
	home, proj := regrantProject(t)
	writeSkill(t, filepath.Join(home, ".config", "lagent", "skills"), "notes")

	_, old, _ := regrant(proj, projectGrant{trusted: true}, nil)
	if len(old) != 1 {
		t.Fatalf("skills = %q", skillNames(old))
	}
	if _, err := old[0].Body(skills.DefaultLimits()); err != nil {
		t.Fatalf("a discovered skill does not load: %v", err)
	}
	_, list, _ := regrant(proj, projectGrant{trusted: true}, old)
	defer skills.CloseAll(list)
	if _, err := old[0].Body(skills.DefaultLimits()); err == nil {
		t.Error("the replaced list was left open")
	}
	if _, err := list[0].Body(skills.DefaultLimits()); err != nil {
		t.Errorf("the new list does not load: %v", err)
	}
}

// The defect this file exists for was in the wiring, not in a function:
// /clear re-read the files and never handed the result to the agent,
// and no behavioural test reaches that closure without running a whole
// session. So the call site itself is pinned: after the pins are
// re-checked, /clear regrants and sets the system prompt from it, and
// the exit path closes the list /clear may have replaced.
func TestClearAppliesWhatItReread(t *testing.T) {
	// The session wiring since ADR-0054 (liveSession.clear), with the
	// receiver dropped.
	lines := strings.Split(wiringSource(t), "\n")
	find := func(from int, want string) int {
		for i := from; i < len(lines); i++ {
			if strings.Contains(lines[i], want) {
				return i
			}
		}
		return -1
	}
	re := find(0, "projectContext, skillsList, regrantNotes = regrant(projectDir, grant, skillsList)")
	if re < 3 {
		t.Fatal("/clear does not regrant: the call was not found in root.go")
	}
	// The grant it reads must be the one just re-checked. The
	// mid-session re-check is the non-interactive one, which discards
	// the prompt it cannot show.
	if before := strings.Join(lines[re-3:re], "\n"); !strings.Contains(before, "grant.excluded, pinNotes = checkPins(") || !strings.Contains(before, "io.Discard") {
		t.Errorf("root.go:%d: the regrant does not follow /clear's pin re-check", re+1)
	}
	if !strings.Contains(lines[re+1], "ag.SetSystem(composeSystem())") {
		t.Errorf("root.go:%d: the re-read instructions are not handed to the agent: %s", re+2, strings.TrimSpace(lines[re+1]))
	}
	if announce := find(re, "announceFacts()"); announce < 0 {
		t.Error("no facts message follows the regrant: the model would keep the old skill catalog")
	}
	// A closure that reads the variable at exit — a cleanup since
	// ADR-0054 — not a defer that evaluated it at startup.
	if find(0, "func() { skills.CloseAll(skillsList) }") < 0 {
		t.Error("the exit path does not close the skill list /clear leaves (a plain defer keeps the startup list)")
	}
}
