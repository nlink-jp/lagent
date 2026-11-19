package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/nlink-jp/lagent/internal/agent"
	"github.com/nlink-jp/lagent/internal/approve"
	"github.com/nlink-jp/lagent/internal/config"
	"github.com/nlink-jp/lagent/internal/hooks"
	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/mcpfilter"
	"github.com/nlink-jp/lagent/internal/memory"
	"github.com/nlink-jp/lagent/internal/mention"
	"github.com/nlink-jp/lagent/internal/policy"
	"github.com/nlink-jp/lagent/internal/repl"
	"github.com/nlink-jp/lagent/internal/sandbox"
	"github.com/nlink-jp/lagent/internal/session"
	"github.com/nlink-jp/lagent/internal/skills"
	"github.com/nlink-jp/lagent/internal/termimg"
	"github.com/nlink-jp/lagent/internal/tools"
	"github.com/nlink-jp/lagent/internal/trustpin"
	"github.com/nlink-jp/lagent/internal/tui"
	"github.com/nlink-jp/lagent/internal/uitext"
	"github.com/nlink-jp/lagent/internal/workdir"
)

// runSettings is what the run resolved before anything of the project is
// read: where the config came from and the session's shape.
type runSettings struct {
	cfgPath      string
	oneShot      bool
	autoOn       bool
	ceiling      sandbox.Ceiling
	initialInput string
	ladder       *interruptLadder
}

// admission is what the project was admitted with: its policy, and the
// note a `!` command's output carries.
type admission struct {
	home             string
	approvalPolicy   policy.Policy
	projectMayLoosen bool
	policyNotes      []policy.Note
	afterShell       func() string
}

// opened is the transcript openTranscript put in place, and what a resume
// restores. prepareWorkDir takes it: the work directory is keyed by the
// session id, so it cannot be made before the transcript is open.
type opened struct {
	tr            *transcript
	resumedID     string
	resumedFrom   time.Time
	restored      []llm.Message
	restoredLoads []string
}

// loaded is what loadProject read beyond the holders: the instruction
// files' labels for the banner and the operator's screen.
type loaded struct {
	contextLabels []string
	screen        *tui.Screen
}

// models is the model tier (ADR-0032): its backend, model, rulebook and
// the notes it raised. The main client is the session's backend.
type models struct {
	riskBackend llm.Backend
	riskModel   string
	rulebook    string
	tierNotes   []string
}

// frontEnd is the session's face: TUI or not, and what goes with it.
type frontEnd struct {
	useTUI  bool
	images  termimg.Protocol
	gate    agent.Approver
	tuiGate *tui.Gate
	reader  *repl.Reader
}

// toolsRegistered is what registerTools returns, and only it:
// assembleSession takes it, because the agent caches the tool
// declarations when it is built and every tool must be registered by
// then.
type toolsRegistered struct {
	onRoundLimit func(ctx context.Context, info agent.RoundLimitInfo) bool
}

// assembled is what the front ends need of the assembled session.
type assembled struct {
	reloads      slashReloads
	settingsData tui.SettingsData
}

// The startup phases (ADR-0054 §2.1). Each takes what it needs from the
// phases before it as arguments and returns what the phases after it
// read; the order runREPL calls them in is today's order, so every
// warning prints where it printed before.

func resolveRun(sess *liveSession, cmd *cobra.Command, fl runFlags, args []string) (runSettings, error) {
	var err error
	// --- config ---
	cfgPath := fl.config
	if cfgPath == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return runSettings{}, err
		}
		cfgPath = p
	}
	oneShot := fl.prompt != ""
	roFlag, err := readOnlyOverride(fl.writable, fl.readOnly)
	if err != nil {
		return runSettings{}, err
	}
	sess.cfg, err = config.LoadWithOverrides(cfgPath, config.Overrides{
		Model: fl.model, MCP: fl.mcp, Auto: fl.auto, ReadOnly: roFlag})
	if err != nil {
		return runSettings{}, err
	}
	// The first interactive turn, from the positional argument
	// (gem-agent ADR-0064). Never combined with -p: the two select different
	// session shapes, and ambiguity is refused, not resolved.
	initialInput, err := firstMessage(args, oneShot)
	if err != nil {
		return runSettings{}, err
	}
	// Everything downstream reads this one effective value, never the
	// raw config field.
	autoOn := effectiveAuto(sess.cfg.Agent.AutoApprove, oneShot, fl.auto)
	// The ceiling this run starts with, resolved once for the same
	// reason (gem-agent ADR-0080 §1).
	ceiling := effectiveCeiling(sess.cfg.Agent)
	// UI language, resolved once (gem-agent ADR-0029): the chrome that follows —
	// prompts, TUI, slash output — is built with it.
	uiLang := uitext.Resolve(sess.cfg.TUI.Language, os.Getenv)
	sess.msgs = uitext.For(uiLang)
	// The escape ladder for turns outside the TUI (gem-agent ADR-0065 §3): the
	// TUI has its own three-press exit; the plain REPL and one-shot
	// mode get the same one here. The quit skips the deferred flushes
	// on purpose — the transcript is per event, and the warning that
	// preceded it says so.
	ladder := &interruptLadder{
		Interrupting: func() { fmt.Fprintln(cmd.ErrOrStderr(), sess.msgs.StatusInterrupting) },
		Warn:         func() { fmt.Fprintln(cmd.ErrOrStderr(), sess.msgs.InterruptStuckWarn) },
		Quit: func() {
			fmt.Fprintln(cmd.ErrOrStderr(), sess.msgs.Bye)
			os.Exit(130)
		},
	}

	return runSettings{cfgPath: cfgPath, oneShot: oneShot, autoOn: autoOn, ceiling: ceiling, initialInput: initialInput, ladder: ladder}, nil
}

func admitProject(sess *liveSession, cmd *cobra.Command, fl runFlags, rs runSettings) (admission, error) {
	var err error
	// --- project directory ---
	sess.projectDir, err = currentProject()
	if err != nil {
		return admission{}, err
	}

	// --- startup safety (gem-agent ADR-0023) ---
	// Both gates read one unbuffered line from stdin before the REPL/TUI
	// takes over. One-shot mode counts as non-interactive even on a TTY:
	// a scripted -p must behave deterministically.
	interactive := !rs.oneShot && term.IsTerminal(int(os.Stdin.Fd()))
	home, _ := os.UserHomeDir()
	if reason := broadRoot(sess.projectDir, home); reason != "" {
		if err := confirmBroadRoot(reason, sess.projectDir, interactive, os.Stdin, cmd.ErrOrStderr(), sess.msgs); err != nil {
			return admission{}, err
		}
	}

	sess.policyPath = config.PolicyPath(rs.cfgPath)
	sess.policyFile, err = config.LoadPolicyFile(sess.policyPath)
	if err != nil {
		return admission{}, err
	}

	// --- first-run project trust (gem-agent ADR-0023): does this project's own
	// instruction files / .mcp.json / skills get loaded at all? ---
	var trustNote string
	sess.projectTrusted, trustNote = resolveProjectTrust(
		sess.cfg, sess.policyFile, sess.policyPath, sess.projectDir, interactive, os.Stdin, cmd.ErrOrStderr(), sess.msgs)
	if trustNote != "" {
		fmt.Fprintf(sess.stderr, "%s\n", trustNote)
	}
	// Content pins (gem-agent ADR-0074): trust was given to a directory; what is
	// loaded is content, and content that changed since asks again.
	// Decided before anything of the project is read — .lagent.toml
	// below included.
	sess.grant = projectGrant{trusted: sess.projectTrusted}
	var pinNotes []string
	sess.grant.excluded, pinNotes = checkPins(sess.cfg, sess.policyFile, sess.policyPath, sess.projectDir, sess.projectTrusted, interactive, os.Stdin, cmd.ErrOrStderr(), sess.msgs)
	for _, n := range pinNotes {
		fmt.Fprintf(sess.stderr, "%s\n", n)
	}

	// --- per-tool approval policy (gem-agent ADR-0008) ---
	// The project half may tighten freely and may only loosen where the
	// operator trusted this directory and its content is what they
	// trusted: a checked-out repository must not be able to switch the
	// gate off.
	var projectMayLoosen bool
	sess.projectCfg, projectMayLoosen, err = loadProjectConfig(sess.cfg, sess.projectDir, sess.grant)
	if err != nil {
		return admission{}, err
	}
	// The MCP tool filter (gem-agent ADR-0077): what this session does not have.
	// Built once — the reload re-derives what it removes, but the files
	// it is built from are config, and config changes need a restart
	// like every other key. A malformed entry refuses to start: an
	// exclusion that does not do what it says is worse than none, and
	// falling back to "exclude nothing" would silently hand back the
	// write functions the operator took away.
	sess.mcpFilter, err = mcpfilter.Build(sess.cfg.MCP.Exclude, policyScope(sess.policyFile), sess.projectCfg.MCP.Exclude)
	if err != nil {
		return admission{}, err
	}
	mergedTools := map[string]string{}
	for k, v := range sess.cfg.Approval.Tools {
		mergedTools[k] = v
	}
	// The machine-owned file wins: a change made through /settings must
	// not be silently overridden by the hand-written config (gem-agent ADR-0009).
	for k, v := range sess.policyFile.ForProject(sess.projectDir) {
		mergedTools[k] = v
	}
	// --allow entries sit above both files (flags > machine-owned >
	// hand-written, the order the config system already declares) and
	// compile into the same policy build — so the project tighten, the
	// Block floor, hooks, and the bare-"*" ban all hold without knowing
	// the flag exists (gem-agent ADR-0053 §2).
	if err := applyAllowFlag(mergedTools, fl.allow); err != nil {
		return admission{}, err
	}
	// Learned command rules are parsed but NOT applied (gem-agent ADR-0049 §3):
	// /learn is withdrawn, nothing displays or manages those entries, and
	// invisible standing permissions are the state the withdrawal exists
	// to end. Ignoring them only ever tightens. The note tells the
	// operator where the entries live.
	approvalPolicy, policyNotes, err := policy.Build(
		mergedTools, sess.projectCfg.Approval.Tools, nil, projectMayLoosen)
	if err != nil {
		return admission{}, err
	}
	if n := len(sess.policyFile.CommandsFor(sess.projectDir)); n > 0 {
		fmt.Fprintf(sess.stderr, "note: %s has %d obsolete [projects.<dir>.commands] entries — ignored; delete them to silence this note\n", sess.policyPath, n)
	}

	// The persistent-file snapshot (gem-agent ADR-0074 §3/§4): what this session
	// starts from, compared with what the previous session left, and
	// compared again at the end so what the session added or changed
	// is said — the parent directories it names are denied by name in
	// the write lane.
	var persistentCut bool
	sess.persistentSnap, persistentCut = trustpin.Snapshot(sess.projectDir)
	if persistentCut {
		fmt.Fprintf(sess.stderr, "note: change detection covers the first %d files under the project only\n", trustpin.WalkEntries)
	}
	sess.persistentPath, sess.persistentPathErr = persistentStateFile(sess.projectDir)
	if sess.persistentPathErr == nil {
		if prev := loadPersistentSnapshot(sess.persistentPath); prev != nil {
			if a, c, r := trustpin.SnapshotDiff(prev, sess.persistentSnap); len(a)+len(c)+len(r) > 0 {
				fmt.Fprintf(sess.stderr, sess.msgs.PersistentSinceLastFmt+"\n", describeSnapshotDiff(a, c, r))
			}
		}
		_ = savePersistentSnapshot(sess.persistentPath, sess.persistentSnap)
	}
	// What a `!` command changed in the files later sessions trust is
	// said after its output.
	afterShell := func() string {
		return pinChangesNote(sess.cfg, sess.policyFile, sess.policyPath, sess.projectDir, sess.projectTrusted, sess.msgs)
	}

	return admission{home: home, approvalPolicy: approvalPolicy, projectMayLoosen: projectMayLoosen, policyNotes: policyNotes, afterShell: afterShell}, nil
}

func openTranscript(sess *liveSession, cl *cleanups, fl runFlags) (opened, error) {
	// --- session transcript: the log, and the resume source (gem-agent ADR-0005) ---
	if fl.continueLast && fl.resume != "" {
		return opened{}, fmt.Errorf("--continue and --resume name different sessions; use one")
	}
	sess.sessionDir, sess.sessionDirErr = session.DefaultDir()

	var restored []llm.Message
	var restoredLoads []string
	resumedID := ""
	var resumedFrom time.Time // when a resumed conversation began (ADR-0030)
	if fl.continueLast || fl.resume != "" {
		if sess.sessionDirErr != nil {
			return opened{}, fmt.Errorf("cannot resume: %w", sess.sessionDirErr)
		}
		meta, err := resolveResume(sess.sessionDir, sess.projectDir, sess.cfg.LLM.Model, fl.resume)
		if err != nil {
			return opened{}, err
		}
		resumedID = meta.ID
		resumedFrom = meta.Started
		// restored is loaded below, AFTER Reopen holds the flock.
	}

	// A broken log warns; a session that cannot be recorded is still a
	// session, and degrading beats refusing to start. A broken *resume*
	// is fatal, though — the operator asked for that history, and
	// continuing without it silently would be worse than stopping.

	sess.sessionPath = "(disabled)"
	sess.sessionID = ""
	// curLog is the transcript in use; /clear replaces it (gem-agent ADR-0071 §2),
	// so the exit-time close reads the variable, not a snapshot.

	cl.push("close the transcript", func() {
		if sess.curLog != nil {
			_ = sess.curLog.Close()
		}
	})
	if sess.sessionDirErr != nil {
		fmt.Fprintf(sess.stderr, "warning: session log disabled: %v\n", sess.sessionDirErr)
	} else {
		lg, err := openSessionLog(sess.sessionDir, resumedID, sess.projectDir, sess.cfg.LLM.Model, sess.version)
		switch {
		case err != nil && resumedID != "":
			return opened{}, fmt.Errorf("cannot append to session %s: %w", resumedID, err)
		case err != nil:
			fmt.Fprintf(sess.stderr, "warning: session log disabled: %v\n", err)
		default:
			sess.curLog = lg
			sess.sessionLog = lg
			sess.sessionPath = lg.Path()
			sess.sessionID = lg.ID()
			// Exported before any MCP server starts, so a registration
			// line can hand the session to a server that keeps
			// per-session state (gem-agent ADR-0069 addendum 2).
			if err := session.Export(sess.sessionID); err != nil {
				fmt.Fprintf(sess.stderr, "warning: cannot set %s: %v\n", session.EnvVar, err)
			}
			if resumedID != "" {
				// Under the flock (Reopen holds it): the file we read
				// is exactly the file we will append to, with no
				// window for another process's tail (review round 2).
				history, resumeNotes, err := loadResumedHistory(lg, resumedID)
				if err != nil {
					return opened{}, err
				}
				for _, n := range resumeNotes {
					fmt.Fprintf(sess.stderr, "warning: %s\n", n)
				}
				restored = history
				// Same file, same lock: the servers the session loaded,
				// including any whose mcp_load a compaction summarised
				// out of the restored history (ADR-0033).
				if restoredLoads, err = session.CallArgs(lg.Path(), MCPLoadName, "server"); err != nil {
					return opened{}, err
				}
			}
		}
	}

	return opened{tr: sess.transcript, resumedID: resumedID, resumedFrom: resumedFrom, restored: restored, restoredLoads: restoredLoads}, nil
}

func prepareWorkDir(sess *liveSession, cl *cleanups, op opened) *workArea {
	// --- session work directory (gem-agent ADR-0058) ---
	// Everything the session produces outside the project lands here: an
	// MCP result too large to hold in context, binary a server returned,
	// scratch a shell command wrote. It has to exist BEFORE the sandbox
	// profile is built (it is a writable root) and before the MCP
	// servers start, because ${LAGENT_WORK_DIR} in an mcp.json args
	// entry is expanded at load time from the process environment.
	//
	// That ordering is why the session block above was moved ahead of
	// this one: the directory is keyed by session id, so a resume lands
	// back in the directory its earlier self used.
	// The project directory is the third fact a child sees (gem-agent ADR-0071
	// §3), beside the session id and the work directory.
	if err := os.Setenv(workdir.ProjectEnvVar, sess.projectDir); err != nil {
		fmt.Fprintf(sess.stderr, "warning: cannot set %s: %v\n", workdir.ProjectEnvVar, err)
	}
	sess.workDir = ""
	cl.push("remove the fallback scratch", removeFallbackScratch)
	cl.push("remove the work directory if empty", func() {
		if sess.workDir != "" {
			workdir.RemoveIfEmpty(sess.workDir)
		}
	})
	if op.tr.sessionID != "" {
		if dir, err := workdir.Ensure(sess.projectDir, op.tr.sessionID); err != nil {
			// A missing work directory degrades the session (oversized
			// MCP results get truncated with a note); it does not stop
			// the session from starting.
			fmt.Fprintf(sess.stderr, "warning: session work directory unavailable: %v\n", err)
			// An inherited value (a nested launch) must not stand in.
			_ = os.Unsetenv(workdir.EnvVar)
		} else {
			sess.workDir = dir
			// Exported, not passed: this is what puts the path in front
			// of shell_exec's child, every MCP server (internal/mcp
			// inherits os.Environ), and every hook, without any of them
			// needing to know lagent's layout.
			if err := exportWorkDir(sess.workDir); err != nil {
				fmt.Fprintf(sess.stderr, "warning: cannot set %s: %v\n", workdir.EnvVar, err)
			}
			// Gated on bytes, not on the count (gem-agent ADR-0078 §4): two empty
			// leftovers are not an accumulation, and a line reading "0B"
			// asks the operator to look at nothing.
			// `more` means the scan was cut, so bytes is a lower bound:
			// a truncated sweep whose partial sum is under the floor
			// would suppress the note the floor exists to make
			// meaningful (pre-release review).
			if dirs, bytes, more, err := workdir.Sweep(sess.projectDir, op.tr.sessionID); err == nil &&
				(bytes >= workDirNoteFloor || (more && bytes > 0)) {
				plus := ""
				if more {
					plus = "+" // the startup scan was cut: a lower bound
				}
				fmt.Fprintf(sess.stderr, "note: %d%s earlier session work dir(s) hold %s%s — delete them with 'lagent workdirs clean'\n",
					dirs, plus, humanBytes(bytes), plus)
			}
		}
	}

	return sess.workArea
}

func buildConfinement(sess *liveSession, fl runFlags, rs runSettings, adm admission, wa *workArea) error {
	var err error
	// --- shell execution strategy (gem-agent ADR-0001 defense-in-depth) ---
	sess.sandboxOn = sess.cfg.Sandbox.Enabled && !fl.noSandbox
	if sess.sandboxOn {
		// Fail closed (gem-agent ADR-0073 §5): a sandbox that cannot apply here
		// (a nested Seatbelt) must not degrade silently into unconfined
		// execution — the operator says --no-sandbox, or nothing runs.
		if err := sandbox.Available(); err != nil {
			return fmt.Errorf("the sandbox cannot be applied here (%v); every shell command would run unconfined — pass --no-sandbox to accept that explicitly", err)
		}
	}
	// The operator's added directories (ADR-0046): judged by the one
	// rule before anything uses them, and a refusal stops the start —
	// the operator typed the path and must hear it did not apply.
	added, skippedDirs, err := resolveAddedDirs(sess.cfg, sess.projectDir, wa.workDir, rs.cfgPath, fl.addDir, fl.readDir)
	if err != nil {
		return err
	}
	for _, s := range skippedDirs {
		fmt.Fprintf(sess.stderr, sess.msgs.AddedSkippedFmt+"\n", s)
	}
	if len(added) > 0 {
		_ = sess.log("added_directories", addedDirsRecord(added))
	}
	// Persistent files in the write roots are watched: what changed
	// since the previous session is said now, what this session changes
	// at /clear and at exit.
	sess.watch = newAddedWatch(added, sess.projectDir, sess.stderr, sess.msgs)
	execFn, enforcement, laneNotes, err := buildExecFn(sess.sandboxOn, sess.projectDir, wa.workDir, sess.cfg.Sandbox.ReadLaneDenyExec, sess.cfg.Sandbox.Caches(), trustpin.Parents(sess.projectDir, sess.persistentSnap), rootsOf(added))
	if err != nil {
		return err
	}
	if sess.cfg.Sandbox.ReadLanePrompts {
		// The operator's opt-out (gem-agent ADR-0073 §5): read-lane commands keep
		// their cage and their prompt.
		enforcement.ReadLane = false
	}
	for _, n := range laneNotes {
		fmt.Fprintf(sess.stderr, "warning: %s\n", n)
	}
	// The strategy is swapped when /clear rotates the work directory
	// (gem-agent ADR-0071 §2): the sandbox profile names the directory, so a
	// profile built at startup denied every write to the new one.
	sess.shellExec = &liveExec{fn: execFn}

	sess.registry, err = tools.New(sess.projectDir, nil, time.Duration(sess.cfg.Agent.ShellTimeoutSec)*time.Second)
	if err != nil {
		return err
	}
	// The lanes (gem-agent ADR-0073): a read-lane command runs without approval
	// only where the read lane's denials were verified on this machine;
	// with the sandbox off every shell call is the operator's alone.
	sess.registry.SetLaneExec(sess.shellExec.run, enforcement)
	if wa.workDir != "" {
		// The file tools get the work directory as a second root, so a
		// result the intake saved is one read_file can read back. Without
		// it the model sees paths it cannot open and routes around the
		// built-ins with shell redirection, which is less reviewable.
		if err := sess.registry.UseWorkDir(wa.workDir); err != nil {
			fmt.Fprintf(sess.stderr, "warning: session work directory not readable by the file tools: %v\n", err)
		}
	}
	if err := sess.registry.AddDirs(rootsOf(added)); err != nil {
		return err
	}
	// Mid-session grants (ADR-0046): the operator's /add-dir and
	// /read-dir and the model's request_directory share this one path.
	sess.granter = &dirGranter{
		registry: sess.registry, shellExec: sess.shellExec, sandboxOn: sess.sandboxOn,
		projectDir: realOrSelf(sess.projectDir), workDir: func() string { return wa.workDir },
		denyExec: sess.cfg.Sandbox.ReadLaneDenyExec, caches: sess.cfg.Sandbox.Caches(), readLanePrompts: sess.cfg.Sandbox.ReadLanePrompts,
		parents: func() []string { return trustpin.Parents(sess.projectDir, sess.persistentSnap) },
		base:    rootBaseContext(rs.cfgPath), watch: sess.watch, sources: map[string]string{}, msgs: sess.msgs,
		log: sess.log,
	}
	for _, a := range added {
		sess.granter.sources[a.Dir] = a.Source
	}
	if !rs.oneShot {
		// Only where a human answers: under -p it could only be denied.
		if err := registerRequestDirectory(sess.registry, sess.granter); err != nil {
			return err
		}
	}
	// The file tools' reads run in a child under a profile that denies
	// the credential list at the kernel (ADR-0016), proven on this
	// machine before it is trusted. Where it cannot be proven the reads
	// stay in process with the matcher as the boundary, and the note
	// says so rather than letting the banner imply a cage.
	if probeDir, doneProbe := fileProbeDir(wa.workDir); probeDir != "" {
		note := installFileChild(sess.registry, adm.home, probeDir)
		doneProbe()
		if note != "" {
			fmt.Fprintf(sess.stderr, "warning: %s\n", note)
		}
	} else {
		fmt.Fprintf(sess.stderr, "warning: file reads are not sandboxed (no probe directory)\n")
	}

	return nil
}

func loadProject(sess *liveSession, cl *cleanups, fl runFlags) (loaded, error) {
	// --- project instruction files (drop-in: AGENTS.md and friends,
	// including ancestor directories, exactly as other agents read them)
	// Read here, with the skills, before any process the project names
	// (an MCP server) starts: the pins were digested moments ago, and
	// the gap between digest and read is kept to this (gem-agent ADR-0074).
	var contextLabels, contextNotes []string
	sess.projectContext, contextLabels, contextNotes = loadInstructions(sess.projectDir, sess.grant)
	for _, n := range contextNotes {
		fmt.Fprintf(sess.stderr, "warning: instruction file %s\n", n)
	}
	// --- skills (ADR-0011): Claude Code's format from lagent's own
	// global directory and the trusted project's .claude/skills ---
	var skillNotes []string
	sess.skillsList, skillNotes = discoverSkills(sess.projectDir, sess.grant)
	cl.push("close the skills' roots", func() { skills.CloseAll(sess.skillsList) }) // reads the variable: /clear replaces it
	for _, n := range skillNotes {
		fmt.Fprintf(sess.stderr, "warning: skills: %s\n", n)
	}
	// load_skill is registered whether or not any skill was found, so
	// the tool set — and the cached prefix — does not depend on what the
	// operator has installed.
	if err := registerSkillTool(sess.registry, func() []skills.Skill { return sess.skillsList }); err != nil {
		return loaded{}, err
	}
	// --- memory (ADR-0013): two scopes under the state root; recall
	// rides the facts message, the operator writes with /remember, the
	// model proposes through gated tools ---
	sess.memStore = memoryStore{Project: sess.projectDir}
	if base, err := memory.DefaultDir(); err == nil {
		sess.memStore.Base = base
	} else {
		fmt.Fprintf(sess.stderr, "warning: memory is off: %v\n", err)
	}
	if err := registerMemoryTools(sess.registry, sess.memStore); err != nil {
		return loaded{}, err
	}
	if _, memNotes := sess.memStore.load(); len(memNotes) > 0 {
		for _, n := range memNotes {
			fmt.Fprintf(sess.stderr, "warning: memory: %s\n", n)
		}
	}

	// --- MCP servers from the project's .mcp.json (drop-in) ---
	// screen is the route to the operator's screen, late-bound because
	// the program does not exist yet (ADR-0021 §1's plumbing, which
	// ADR-0022 keeps while withdrawing the intake as its source). What
	// sends through it now is show_image, which the model calls, and
	// /show, which the operator types. Unbound it is inert.
	screen := tui.NewScreen()
	sess.mcpClients, sess.mcpSummary, sess.mcpInv = connectMCPServers(sess.ctx, sess.cfg, sess.projectDir, sess.version, sess.registry, sess.stderr, sess.grant, sess.mcpFilter)
	cl.push("close the MCP clients", func() {
		for _, c := range sess.mcpClients {
			c.Close()
		}
	})
	// What the model is shown of them (ADR-0004): every server's tools
	// are registered; only a loaded server's are advertised. A --allow
	// grant naming a server is a preload — the operator's declaration
	// that this run needs it.
	sess.adv = newMCPAdvertiser(sess.cfg.MCP.Advertise == "all", sess.cfg.MCP.Preload, fl.allow)
	sess.adv.setInventory(sess.mcpInv)

	return loaded{contextLabels: contextLabels, screen: screen}, nil
}

func buildModels(sess *liveSession, rs runSettings) (models, error) {
	var err error
	// --- LLM backend ---
	sess.backend, err = newModelClient(sess.cfg, sess.cfg.LLM.Model)
	if err != nil {
		return models{}, err
	}
	sess.backend.SetReasoningEffort(sess.cfg.LLM.ReasoningEffort)

	// --- the model tier (ADR-0032): its own backend on the same server,
	// because the judge's reasoning_effort is its own even on the main
	// model; the operator's rulebook beside config.toml.
	var riskBackend llm.Backend
	var rulebook string
	var tierNotes []string
	riskModel := sess.cfg.LLM.RiskModel
	if riskModel == "" {
		riskModel = sess.cfg.LLM.Model
	}
	if sess.cfg.Approval.ModelTier == "shell" {
		rb, err := newModelClient(sess.cfg, riskModel)
		if err != nil {
			return models{}, err
		}
		rb.SetReasoningEffort(sess.cfg.LLM.RiskReasoningEffort)
		if err := checkRiskModel(sess.ctx, rb, sess.cfg.LLM.Provider, sess.cfg.LLM.RiskModel); err != nil {
			return models{}, err
		}
		riskBackend = rb
		text, clipped, err := config.LoadRulebook(rs.cfgPath)
		switch {
		case err != nil:
			tierNotes = append(tierNotes, fmt.Sprintf("risk rulebook not loaded, judging without it: %v", err))
		case clipped:
			tierNotes = append(tierNotes, fmt.Sprintf("%s is longer than %d characters; the judge sees the first %d", config.RulebookPath(rs.cfgPath), config.RulebookCap, config.RulebookCap))
		}
		rulebook = text
	}

	return models{riskBackend: riskBackend, riskModel: riskModel, rulebook: rulebook, tierNotes: tierNotes}, nil
}

func chooseFrontEnd(sess *liveSession, cmd *cobra.Command, rs runSettings) frontEnd {
	// The TUI needs a real terminal on both ends (gem-agent ADR-0002); piped use
	// falls back to the plain line REPL so scripts and smoke pipelines
	// keep working.
	useTUI := !rs.oneShot &&
		term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))

	// One shared buffered reader for the plain REPL and its approval
	// gate: bufio.NewReader returns an existing *bufio.Reader unchanged,
	// so both components drain the same buffer and no typed-ahead input
	// is stranded in a second one. (The TUI reads the terminal itself.)
	stdin := bufio.NewReader(cmd.InOrStdin())
	// The inline-image capability is resolved ONCE, here: before
	// tea.NewProgram (ADR-0020 §7), and only for a session that will have
	// a UI — the probe writes escapes to the terminal and a one-shot or a
	// piped run has none to write to. The settings panel answers with it
	// too, so it cannot be resolved twice or answered differently in the
	// two places.
	images := termimg.None
	if useTUI {
		images = resolveImages(sess.cfg.TUI.Images)
	}
	var gate agent.Approver
	var tuiGate *tui.Gate
	switch {
	case rs.oneShot:
		// One-shot runs non-interactively (stdin may be a pipe): a
		// blocking approval prompt would hang, so mutating tools are
		// denied outright. Read-only pipelines still work.
		gate = denyGate{out: sess.stderr}
	case useTUI:
		tuiGate = tui.NewGate()
		gate = tuiGate
	default:
		gate = approve.New(stdin, sess.stderr)
	}
	reader := repl.NewReader(stdin, sess.stderr)

	// Where operator-facing output goes (ADR-0054 §2.3): the TUI once
	// prog runs, stderr until then and without one.
	sess.surf = &surface{w: sess.stderr, msgs: sess.msgs, oneShot: rs.oneShot, plainAsk: plainAsk(stdin, cmd.ErrOrStderr())}
	// Turn observability (gem-agent ADR-0033): stream heartbeat, retries, and
	// thought summaries reach the TUI when one is running; the plain
	// REPL and one-shot mode stay quiet — their output goes to pipes.
	sess.backend.SetObserver(func(ev llm.StreamEvent) {
		if ev.Kind == "thought" && !sess.cfg.TUI.ShowThoughts {
			return
		}
		sess.surf.stream(ev)
	})

	return frontEnd{useTUI: useTUI, images: images, gate: gate, tuiGate: tuiGate, reader: reader}
}

func registerTools(sess *liveSession, rs runSettings) (toolsRegistered, error) {
	// --- ask_user: a structured mid-turn choice (gem-agent ADR-0036) ---
	// Registered before agent.New (declarations are cached there). The
	// asker picks its mode at call time: one-shot has nobody to ask,
	// the TUI shows a dialog, the plain REPL reads a number. The same
	// asker carries the round-limit dialog (gem-agent ADR-0040).
	// The plain-REPL asker reads the SHARED stdin reader: a second
	// bufio.Reader over the same fd strands typed-ahead input in one
	// buffer while the other blocks (AGENTS.md gotcha; review round 3).
	askOperator := sess.surf.ask
	if err := registerAskTool(sess.registry, askOperator); err != nil {
		return toolsRegistered{}, err
	}
	// mcp_load (ADR-0004): registered before agent.New, which caches the
	// declarations; the refresh closure reads `ag` lazily — the tool can
	// only run inside ag.Run, so the pointer is set by then.

	if err := registerMCPLoadTool(sess.registry, sess.adv, func() { sess.ag.RefreshTools() }); err != nil {
		return toolsRegistered{}, err
	}

	// --- round-limit intervention dialog ---
	// nil in one-shot mode: nobody to ask, and no model review to
	// decide in their place, so the checkpoint stops there (fail-closed
	// inside the agent). The dialog shows the turn's recent calls: the
	// evidence a reviewer would have read, for the operator to read.
	var onRoundLimit func(ctx context.Context, info agent.RoundLimitInfo) bool
	if !rs.oneShot {
		onRoundLimit = func(rctx context.Context, info agent.RoundLimitInfo) bool {
			recent := info.RecentCalls
			if len(recent) > 8 {
				recent = recent[len(recent)-8:]
			}
			evidence := ""
			if len(recent) > 0 {
				evidence = fmt.Sprintf(sess.msgs.RoundRecentCallsFmt, clipRunes(strings.Join(recent, "\n  "), 600))
			}
			var q string
			if info.Trigger == "loop" {
				q = fmt.Sprintf(sess.msgs.RoundLoopAskFmt, clipRunes(info.Detail, 80), evidence)
			} else {
				q = fmt.Sprintf(sess.msgs.RoundLimitAskFmt, info.Rounds, info.Cap, evidence)
			}
			idx, err := askOperator(rctx, q, []string{sess.msgs.RoundContinue, sess.msgs.RoundStop})
			return err == nil && idx == 0
		}
	}

	return toolsRegistered{onRoundLimit: onRoundLimit}, nil
}

// assembleHooks builds the operator's hooks and the two the agent calls,
// and pushes the session-end hooks onto the teardown.
func assembleHooks(sess *liveSession, cl *cleanups) (preToolHook func(ctx context.Context, name string, args map[string]any) (bool, string), promptHook agent.PromptHook) {
	// --- pre-tool hooks (ADR-0012): the operator's floor before the
	// ladder. What a hook learns about the session: the transcript path
	// only while a log is actually being written. /clear re-points it.
	sess.hookSession = hooks.Session{ID: sess.sessionID, CWD: sess.projectDir}
	if sess.sessionID != "" {
		sess.hookSession.TranscriptPath = sess.sessionPath
	}
	// Hook notices are collected while a slash command runs the hooks
	// (surface.hookNotice says why: ADR-0054, Found 1).
	hookNotify := sess.surf.hookNotice

	if len(sess.cfg.Hooks.PreToolUse)+len(sess.cfg.Hooks.SessionStart)+len(sess.cfg.Hooks.UserPromptSubmit)+len(sess.cfg.Hooks.SessionEnd) > 0 {
		sess.hookRunner = hooks.New(hooks.Hooks{
			PreToolUse:       hookEntries(sess.cfg.Hooks.PreToolUse),
			SessionStart:     hookEntries(sess.cfg.Hooks.SessionStart),
			UserPromptSubmit: hookEntries(sess.cfg.Hooks.UserPromptSubmit),
			SessionEnd:       hookEntries(sess.cfg.Hooks.SessionEnd),
		}, hookNotify)
	}
	if sess.hookRunner != nil && sess.hookRunner.HasPreToolUse() {
		preToolHook = func(ctx context.Context, name string, args map[string]any) (bool, string) {
			return sess.hookRunner.Pre(ctx, sess.hookSession, name, args)
		}
	}
	// The context hooks (ADR-0014): a prompt hook sees the typed input
	// before the turn starts; session-start output rides the next turn
	// as a data attachment; session end runs on exit and on /clear.
	if sess.hookRunner != nil && sess.hookRunner.HasPromptSubmit() {
		promptHook = func(ctx context.Context, input string) (string, bool, string) {
			return sess.hookRunner.PromptSubmit(ctx, sess.hookSession, input)
		}
	}
	cl.push("run the session-end hooks", func() { sess.sessionEndHooks("exit") })
	return preToolHook, promptHook
}

// assembleAgent builds the agent over everything startup assembled, and
// gives it the history a resume restores.
func assembleAgent(sess *liveSession, rs runSettings, adm admission, op opened, md models, fe frontEnd, rg toolsRegistered, writes *writeGuard, preToolHook func(ctx context.Context, name string, args map[string]any) (bool, string), promptHook agent.PromptHook) {
	notice := sess.surf.notice
	sess.ag = agent.New(agent.Options{
		// resolveWindow below always answers, a configured window included.
		WindowLookup: true,
		// The operator's language for the notices the agent writes
		// mid-turn (gem-agent ADR-0029 §3: they are chrome, not error chains).
		Msgs: sess.msgs,
		// Accounting only (gem-agent ADR-0057): the model name that goes into
		// this session's usage records.
		Model:          sess.cfg.LLM.Model,
		Backend:        sess.backend,
		Registry:       sess.registry,
		Gate:           fe.gate,
		Log:            sess.sessionLog,
		System:         sess.composeSystem(),
		MaxTurns:       sess.cfg.Agent.MaxTurns,
		Policy:         adm.approvalPolicy,
		Advertise:      sess.adv.Advertise,
		PreToolHook:    preToolHook,
		PromptHook:     promptHook,
		ClipboardImage: clipboardImage,
		BeforeOperatorWrite: func(tc llm.ToolCall) {
			if name := pinNameForWrite(sess.projectDir, tc); name != "" {
				writes.begin(tc, name, repinnable(sess.policyFile, sess.policyPath, sess.projectDir, name, sess.grant.excluded))
			}
		},
		OnOperatorWrite: func(tc llm.ToolCall) {
			// The operator approved a write into the files later
			// sessions trust: that content — the one file they saw
			// written — is now what they trust (gem-agent ADR-0074 §1), provided
			// the file was still what its pin records when the write
			// began; a file that had drifted before is left for the
			// next start to ask about. An operator-lane command shows
			// its text, not its effect on those files, so it re-pins
			// nothing; what now differs is named and the next start
			// asks.
			if name := pinNameForWrite(sess.projectDir, tc); name != "" {
				if !writes.end(tc, name) {
					notice(fmt.Sprintf(sess.msgs.PinStaleWriteFmt, name))
					return
				}
				if err := repinName(sess.policyPath, sess.projectDir, name, sess.policyFile, sess.grant.excluded); err != nil {
					notice(fmt.Sprintf("could not refresh the trust pin of %s: %v", name, err))
				}
				return
			}
			if n := pinChangesNote(sess.cfg, sess.policyFile, sess.policyPath, sess.projectDir, sess.projectTrusted, sess.msgs); n != "" {
				notice(n)
			}
		},
		OnToolCall: func(tc llm.ToolCall) {
			// Describe, not CallDetail+CallPurpose: only the agent knows
			// which tools it added the purpose field to, and a tool that
			// publishes an argument of that name must keep it visible
			// among the arguments (gem-agent ADR-0047 §2).
			detail, purpose := sess.ag.Describe(tc)
			sess.surf.toolCall(tc.Name, detail, purpose)
		},
		// The tool-finished signal (review round 3): the TUI's stall
		// detector re-arms on this, never on stream chunks — a risk or
		// progress review's side-stream used to look like "the tool
		// returned" and produced false stall warnings.
		OnToolDone: func(tc llm.ToolCall) { sess.surf.toolDone(tc.Name) },
		OnAttach: func(atts []mention.Attachment, problems []mention.Problem) {
			lines := make([]string, 0, len(atts))
			for _, a := range atts {
				lines = append(lines, fmt.Sprintf("attached %s: %s (%d bytes)", a.Kind, a.Ref, a.Bytes))
			}
			notes := make([]string, 0, len(problems))
			for _, p := range problems {
				notes = append(notes, fmt.Sprintf("@%s: %s", p.Ref, p.Reason))
			}
			sess.surf.attached(lines, notes)
		},
		OnNotice: notice,
		OnUsage:  sess.surf.usage,
		// The round limit is an intervention ladder on the main loop.
		RoundReview:  true,
		OnRoundLimit: rg.onRoundLimit,
		Unattended:   rs.oneShot,
		// A skill body is the operator's own instruction file (ADR-0011
		// §3): sent unwrapped, and the only tool that is.
		InstructionTools: []string{skills.ToolName},
		AutoApprove:      rs.autoOn,
		Ceiling:          rs.ceiling,
		ModelTier:        md.riskBackend != nil,
		RiskBackend:      md.riskBackend,
		RiskModel:        md.riskModel,
		Rulebook:         md.rulebook,

		// ADR-0033: the history is compacted before a request would fill
		// the window, one-shot included.
		AutoCompact:  sess.cfg.Agent.AutoCompact,
		CompactAtPct: sess.cfg.Agent.CompactAtPct,

		OnAutoDecision: func(tc llm.ToolCall, d agent.AutoDecision) {
			if !d.Approved {
				return // the escalation shows up in the approval prompt
			}
			sess.surf.autoApproved(tc.Name, d.Reason, d.Tier.String())
		},
	})
	if len(op.restored) > 0 {
		sess.ag.SetHistory(op.restored)
		// The servers the earlier session loaded are loaded again, so
		// it sees what it saw (ADR-0004).
		if replayLoads(op.restoredLoads, sess.adv, sess.registry) > 0 {
			sess.ag.RefreshTools()
		}
	}
}

func assembleSession(sess *liveSession, cmd *cobra.Command, cl *cleanups, rs runSettings, adm admission, op opened, ld loaded, md models, fe frontEnd, rg toolsRegistered) assembled {
	// writes pairs the agent's before/after hooks around an
	// operator-approved write (gem-agent ADR-0074 §1).
	writes := &writeGuard{}
	// notice puts a one-line warning in front of the operator wherever
	// they are: the TUI's note strip when it runs, stderr otherwise.
	notice := sess.surf.notice
	sess.granter.notice = notice
	for _, n := range md.tierNotes {
		notice(n)
	}
	preToolHook, promptHook := assembleHooks(sess, cl)
	assembleAgent(sess, rs, adm, op, md, fe, rg, writes, preToolHook, promptHook)
	// The session's dates are captured once (ADR-0030): a facts message
	// re-sent mid-session states the same day; /clear captures a new one.
	sess.dates = sessionDates{Start: time.Now(), ResumedFrom: op.resumedFrom}
	// The per-session facts open the conversation (after SetHistory: the
	// isolation tag is fresh, and the restored history's own opening
	// message named the old one). The MCP catalog rides with them.
	sess.announceFacts()
	if op.resumedID != "" {
		sess.sessionStartHooks("resume")
	} else {
		sess.sessionStartHooks("startup")
	}

	// Exit summary (operator request): every interactive exit route —
	// /quit, Ctrl+C, Ctrl+D — ends with the resume hint and the cost
	// line as the last thing in the scrollback. A final cleanup: after
	// everything else, the session-end hooks and what they say included
	// (ADR-0054, Found 3). One-shot mode stays clean for pipelines.
	if !rs.oneShot {
		out := cmd.ErrOrStderr()
		cl.pushFinal("print the exit summary", func() {
			for _, l := range exitSummary(sess.ag.Usage(), sess.sessionID, sess.msgs) {
				fmt.Fprintln(out, l)
			}
		})
	}

	// showImage is the one door to the operator's screen (ADR-0022). Both
	// routes use it — show_image, which the model calls, and /show, which
	// the operator types — so a refusal reads the same either way, and
	// the reason is returned rather than printed: the screen stays silent
	// on refusal (ADR-0020), and the caller is the one who can say
	// something useful about it.
	showImage := func(data []byte, mime string) error {
		if fe.images == termimg.None {
			return fmt.Errorf("this terminal cannot draw inline images (see [tui].images and /settings)")
		}
		if _, _, ok := termimg.Measure(data); !ok {
			return fmt.Errorf("not a drawable image, or larger than %d bytes decoded", termimg.MaxBytes)
		}
		if !ld.screen.Image(data, mime) {
			return fmt.Errorf("this session has no screen (one-shot or non-interactive)")
		}
		return nil
	}
	sess.registry.SetShowImage(showImage)

	// showPath is /show: the OPERATOR names the file, so it is admitted
	// under the rules of an `@<image>` attachment — a project path, or an
	// absolute or ~ path for an image (ADR-0005) — rather than under the
	// tools' project confinement. The rules, not the grammar: see newShowPath.
	showPath := newShowPath(sess.projectDir, showImage)

	sess.settings = &settingsStore{
		cfg: sess.cfg, projectCfg: sess.projectCfg, mayLoosen: adm.projectMayLoosen, policyFile: sess.policyFile,
		policyPath: sess.policyPath, projectDir: sess.projectDir,
		registry: sess.registry, ag: sess.ag, current: adm.approvalPolicy,
		filter: sess.mcpFilter, inv: sess.mcpInv, images: fe.images,
	}
	settingsData := sess.settings.data()

	sess.settings.reloadMCP = sess.reconnectServer

	// One value for both surfaces, so a closure added to one cannot be
	// missed in the other and read as an unknown command there.
	addDirSlash := func(rest string, write bool) (string, bool) {
		note, err := sess.granter.grant(context.Background(), rest, write, "slash", sess.granter.currentEpoch(), nil)
		if err != nil {
			return err.Error(), true
		}
		return note, false
	}
	reloads := slashReloads{mcp: sess.reloadMCP, load: sess.loadMCP, show: showPath, addDir: addDirSlash, dirs: sess.granter.listing}

	cl.push("report the project's persistent files", func() {
		sess.reportPersistent("exit", sess.stderr, sess.log)
	})
	// Cleanups run last-first: at exit the added roots are reported before
	// the project.
	cl.push("report the added roots' persistent files", func() {
		sess.watch.report("exit", sess.stderr, sess.log)
	})

	return assembled{reloads: reloads, settingsData: settingsData}
}
