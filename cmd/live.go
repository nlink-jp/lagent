package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nlink-jp/lagent/internal/agent"
	"github.com/nlink-jp/lagent/internal/config"
	"github.com/nlink-jp/lagent/internal/hooks"
	"github.com/nlink-jp/lagent/internal/mcp"
	"github.com/nlink-jp/lagent/internal/mcpfilter"
	"github.com/nlink-jp/lagent/internal/session"
	"github.com/nlink-jp/lagent/internal/skills"
	"github.com/nlink-jp/lagent/internal/tools"
	"github.com/nlink-jp/lagent/internal/trustpin"
	"github.com/nlink-jp/lagent/internal/uitext"
	"github.com/nlink-jp/lagent/internal/workdir"
)

// The state holders (ADR-0054 §1, §2.2): what /clear or a reload
// replaces lives in a holder, shared by pointer, so whatever was built
// before the change and must follow it reads through the holder — as it
// read through a closure over runREPL's locals before.

// transcript is the session's log: the state directory it lives under,
// the logger in use and its identity. /clear replaces the logger.
type transcript struct {
	sessionDir    string
	sessionDirErr error
	// sessionLog is the logger as the agent sees it: nil, not a nil
	// *session.Logger, when there is none.
	sessionLog  agent.SessionLog
	curLog      *session.Logger
	sessionPath string
	sessionID   string
}

// workArea is the session work directory; "" when there is none. /clear
// rotates it.
type workArea struct {
	workDir string
}

// grantHolder is what the project's trust lets this session load; every
// pin check rewrites its excluded set.
type grantHolder struct {
	grant projectGrant
}

// persistentState is the persistent-file snapshot and where it is kept
// between sessions.
type persistentState struct {
	persistentSnap    map[string]string
	persistentPath    string
	persistentPathErr error
}

// projectContent is what the session read from the project and its own
// stores: the instruction files, the skills and the memory store.
type projectContent struct {
	projectContext string
	skillsList     []skills.Skill
	memStore       memoryStore
}

// mcpSet is the connected MCP servers and the filter they were connected
// under.
type mcpSet struct {
	mcpClients []*mcp.Client
	mcpSummary []string
	mcpInv     mcpInventory
	mcpFilter  mcpfilter.Filter
}

// liveSession owns the holders after startup and the parts its methods
// call into. Its methods are the session's post-startup mutators; they run
// between turns only — a slash command cannot run while a turn is in
// flight — so one writer at a time is the rule, and nothing locks.
type liveSession struct {
	*transcript
	*workArea
	*grantHolder
	*persistentState
	*projectContent
	*mcpSet

	ctx            context.Context
	version        string
	cfg            *config.Config
	msgs           *uitext.Messages
	stderr         io.Writer
	projectDir     string
	projectTrusted bool
	policyFile     *config.PolicyFile
	policyPath     string
	projectCfg     *config.ProjectConfig
	registry       *tools.Registry
	shellExec      *liveExec
	sandboxOn      bool
	granter        *dirGranter
	watch          *addedWatch
	adv            *mcpAdvertiser
	backend        modelClient
	surf           *surface
	hookRunner     *hooks.Runner
	hookSession    hooks.Session
	dates          sessionDates
	ag             *agent.Agent
	settings       *settingsStore
}

func newLiveSession() *liveSession {
	return &liveSession{
		transcript:      &transcript{},
		workArea:        &workArea{},
		grantHolder:     &grantHolder{},
		persistentState: &persistentState{},
		projectContent:  &projectContent{},
		mcpSet:          &mcpSet{},
	}
}

// reportPersistent says on out what this session added, changed or
// removed among the project's persistent files, and records it. The
// caller names out: at exit the terminal, at /clear the command's
// output — under the TUI a write to the startup stream is covered by
// the next redraw (ADR-0054, Found 4).
func (sess *liveSession) reportPersistent(reason string, out io.Writer, log func(kind string, data any) error) {
	now, a, c, r, cut := persistentReport(sess.persistentSnap, func() (map[string]string, bool) { return trustpin.Snapshot(sess.projectDir) })
	if cut {
		fmt.Fprintf(out, "note: change detection covers the first %d files under the project only\n", trustpin.WalkEntries)
	}
	if len(a)+len(c)+len(r) > 0 || cut {
		if len(a)+len(c)+len(r) > 0 {
			fmt.Fprintf(out, sess.msgs.PersistentSessionFmt+"\n", describeSnapshotDiff(a, c, r))
		}
		if log != nil {
			_ = log("persistent_changes", map[string]any{"reason": reason, "added": a, "changed": c, "removed": r, "cut": cut})
		}
	}
	sess.persistentSnap = now
	if sess.persistentPathErr == nil {
		_ = savePersistentSnapshot(sess.persistentPath, now)
	}
}

func (sess *liveSession) mcpStatus() []string {
	return sess.mcpInv.summaryLinesWith(sess.adv.Loaded)
}

// The system prompt is composed in ONE place so a skills reload
// rebuilds exactly what startup built (gem-agent ADR-0039). It says nothing
// about diagrams on any surface (gem-agent ADR-0063): fence rendering is a
// view-layer concern the model is never told about, and both a
// prohibition and a format instruction were measured steering the
// model away from the behavior its own prior already had.
func (sess *liveSession) composeSystem() string {
	return buildSystemPrompt(sess.projectDir, sess.projectContext)
}

func (sess *liveSession) sessionEndHooks(reason string) {
	if sess.hookRunner == nil || !sess.hookRunner.HasSessionEnd() {
		return
	}
	sess.hookRunner.SessionEnd(sess.ctx, sess.hookSession, reason)
}

// announceFacts sends the facts as they stand. One place, because each
// caller's copy of this list would be one more to forget a root in.
func (sess *liveSession) announceFacts() {
	sess.granter.factsStale.Store(false)
	sess.ag.AnnounceSession(sessionFacts(sess.workDir, sess.dates, append(append(sess.mcpInv.catalogLines(sess.adv), skills.CatalogLines(sess.skillsList)...), sess.memStore.factsLines()...), sess.registry.AddedDirs()...))
}

// announceGrant is called where a turn starts: a directory granted
// since the last turn reaches the model in a fresh facts message
// (ADR-0046), which appends and leaves the earlier prefix cached.
func (sess *liveSession) announceGrant() {
	if sess.granter.factsStale.Load() {
		sess.announceFacts()
	}
}

// Session-start hooks (ADR-0014 §4): the output rides the next
// turn's user message as a data attachment — never the system
// prompt (ADR-0003), never the typed input.
func (sess *liveSession) sessionStartHooks(source string) {
	if sess.hookRunner == nil || !sess.hookRunner.HasSessionStart() {
		return
	}
	out := sess.hookRunner.SessionStart(sess.ctx, sess.hookSession, source)
	if out == "" {
		return
	}
	sess.ag.AttachData("session_start", agent.HookAttachmentKind, out)
	sess.surf.hookNotice(fmt.Sprintf("session_start hook (%s) attached %d bytes of context as data for the next turn", source, len(out)))
}

// --- in-session integration reload (gem-agent ADR-0039) ---
// Both closures reuse the startup code paths and the startup trust
// verdict — a reload can never widen what the trust gate allowed.
// They run only between turns (slash commands cannot be queued), so
// reassigning the captured variables is single-writer safe.
// reconnectMCP is /mcp reload; /clear calls it after its own pin
// check, so recheck is false there and the note is not printed twice.
func (sess *liveSession) reconnectMCP(recheck bool) string {
	if !sess.cfg.MCP.Enabled {
		return sess.msgs.MCPDisabled
	}
	sess.registry.RemoveByPrefix("mcp__")
	for _, c := range sess.mcpClients {
		c.Close()
	}
	// Warnings go into the command output: under the TUI a stderr
	// write would corrupt the display.
	var warn bytes.Buffer
	// A changed .mcp.json is re-checked against its pin first
	// (gem-agent ADR-0074); mid-session there is nobody at a prompt, so a
	// change is left out and named.
	if recheck {
		var pinNotes []string
		sess.grant.excluded, pinNotes = checkPins(sess.cfg, sess.policyFile, sess.policyPath, sess.projectDir, sess.projectTrusted, false, nil, &warn, sess.msgs)
		for _, n := range pinNotes {
			fmt.Fprintf(&warn, "%s\n", n)
		}
	}
	sess.mcpClients, sess.mcpSummary, sess.mcpInv = connectMCPServers(sess.ctx, sess.cfg, sess.projectDir, sess.version, sess.registry, &warn, sess.grant, sess.mcpFilter)
	sess.adv.setInventory(sess.mcpInv)
	sess.ag.RefreshTools()
	mcpTools := 0
	for _, t := range sess.registry.List() {
		if strings.HasPrefix(t.Name, "mcp__") {
			mcpTools++
		}
	}
	// The panel reads this snapshot; only a panel-driven reload used
	// to refresh it, so a server added to .mcp.json and picked up by
	// `/mcp reload` had no row and could not be excluded until some
	// other toggle healed it (pre-release review).
	sess.settings.inv = sess.mcpInv
	if sess.sessionLog != nil {
		_ = sess.sessionLog.Log("mcp_reload", map[string]any{
			"servers": len(sess.mcpClients), "tools": mcpTools})
	}
	var b strings.Builder
	fmt.Fprintf(&b, sess.msgs.MCPReloadedFmt, len(sess.mcpClients), mcpTools)
	for _, s := range sess.mcpSummary {
		b.WriteString("  " + s + "\n")
	}
	if warn.Len() > 0 {
		b.WriteString(warn.String())
	}
	return b.String()
}

func (sess *liveSession) reloadMCP() string {
	out := sess.reconnectMCP(true)
	// The catalog changed with the server set: the model is told
	// through a fresh facts message (ADR-0003's lane), never through
	// the system prompt.
	sess.announceFacts()
	return out
}

func (sess *liveSession) loadMCP(server string) (string, bool) {
	out, err := sess.adv.Load(server, sess.registry)
	if err != nil {
		return err.Error() + "\n", true
	}
	sess.ag.RefreshTools()
	return out, false
}

// The panel writes an exclusion for one server and then asks for
// this: the filter is re-derived from the files it just changed, and
// that one server is reconnected under it (gem-agent ADR-0039 + gem-agent ADR-0077 §3).
// Rebuilt rather than patched so the panel and the runtime read the
// same three files in the same order the next start will. One
// server, not the set: the edit names one, and reconnecting all of
// them killed and respawned every process on an arrow key, inside
// the TUI's event loop, so the keys typed meanwhile queued behind it
// (post-release field report).
func (sess *liveSession) reconnectServer(server string) (mcpfilter.Filter, mcpInventory, string) {
	f, err := mcpfilter.Build(sess.cfg.MCP.Exclude, policyScope(sess.policyFile), sess.projectCfg.MCP.Exclude)
	if err != nil {
		// Saved but unusable: keep the running set and say so
		// rather than dropping every exclusion on the floor.
		return sess.mcpFilter, sess.mcpInv, "not applied: " + err.Error()
	}
	sess.mcpFilter = f
	var running *mcp.Client
	others := make([]*mcp.Client, 0, len(sess.mcpClients))
	for _, c := range sess.mcpClients {
		if c.Name() == server {
			running = c
			continue
		}
		others = append(others, c)
	}
	// Only what went wrong goes back to the panel: an arrow key is
	// not a command that asked for an inventory, and the warnings
	// are still the operator's answer to "why did nothing appear"
	// (pre-release review).
	var warn bytes.Buffer
	timeout := time.Duration(sess.cfg.MCP.CallTimeoutSec) * time.Second
	startupTimeout := time.Duration(sess.cfg.MCP.StartupTimeoutSec) * time.Second
	start := func() mcpServer {
		return mcp.NewStdio(server, sess.mcpInv.configs[server], timeout, startupTimeout, sess.version, sess.registry.WorkDir())
	}
	// The interface hides the type; a nil *mcp.Client inside it
	// would not compare equal to nil, so the running client is
	// passed only when there is one.
	var runningServer mcpServer
	if running != nil {
		runningServer = running
	}
	kept := reconnectMCPServer(sess.ctx, server, runningServer, start, startupTimeout, sess.registry, &warn, sess.mcpFilter, &sess.mcpInv)
	if kept != nil {
		others = append(others, kept.(*mcp.Client))
	}
	sess.mcpClients = others
	sess.mcpSummary = sess.mcpInv.summaryLines()
	sess.adv.setInventory(sess.mcpInv)
	sess.ag.RefreshTools()
	sess.settings.inv = sess.mcpInv
	mcpTools := mcpToolCount(sess.registry)
	if sess.sessionLog != nil {
		_ = sess.sessionLog.Log("mcp_reload", map[string]any{
			"servers": len(sess.mcpClients), "tools": mcpTools, "server": server})
	}
	return sess.mcpFilter, sess.mcpInv, reloadWarnings(warn.String())
}

// resolveWindow settles the model's context window and hands it to
// everyone who needs it: the footer displays it. It runs in the
// background — the provider lookup must never delay the first
// prompt. A provider that does not answer leaves the window unknown
// and says so once; [model].context_window is the remedy.
func (sess *liveSession) resolveWindow() {
	tokens := sess.cfg.Model.ContextWindow
	if tokens <= 0 {
		mctx, cancel := context.WithTimeout(sess.ctx, 15*time.Second)
		defer cancel()
		w, err := sess.backend.ContextWindow(mctx)
		if err != nil {
			sess.surf.notice(fmt.Sprintf(sess.msgs.ContextWindowUnknownFmt, err))
		}
		tokens = w
	}
	sess.ag.SetContextWindow(tokens)
	sess.surf.contextWindow(tokens)
}

// /clear starts a new session: the old transcript is closed where
// the conversation ended and stays resumable by its id; a new id,
// transcript and work directory take over, exported to children
// like the first. If a new transcript cannot be opened the
// conversation is cleared in place, as before, and the operator is
// told.
func (sess *liveSession) clear() string {
	var notes []string
	defer sess.surf.collectHookNotices(&notes)()
	note := func(format string, a ...any) { notes = append(notes, fmt.Sprintf(format, a...)) }
	// The output is the slash command's: warnings first, then the
	// MCP reconnection report (the same text /mcp reload prints).
	var extra strings.Builder
	render := func() string {
		var b strings.Builder
		for _, n := range notes {
			fmt.Fprintf(&b, "[⚠ %s]\n", n)
		}
		b.WriteString(extra.String())
		return b.String()
	}
	// The new transcript is opened before anything ends, so a
	// failure leaves the session it found: cleared in place, the
	// operator told. An unresolved state root must not be handed
	// to Open.
	if sess.sessionDirErr != nil {
		sess.ag.Reset()
		// Cleared in place: the dates start over (the conversation the
		// resume line named is gone), and Reset changed the isolation
		// tag, which only a fresh facts message tells the model.
		sess.dates = sessionDates{Start: time.Now()}
		sess.announceFacts()
		note("history cleared; a new session could not be started (%v) — the conversation continues in this session", sess.sessionDirErr)
		return render()
	}
	newLog, err := openSessionLog(sess.sessionDir, "", sess.projectDir, sess.cfg.LLM.Model, sess.version)
	if err != nil {
		sess.ag.Reset()
		// Cleared in place: the dates start over (the conversation the
		// resume line named is gone), and Reset changed the isolation
		// tag, which only a fresh facts message tells the model.
		sess.dates = sessionDates{Start: time.Now()}
		sess.announceFacts()
		note("history cleared; a new session could not be started (%v) — the conversation continues in this session", err)
		return render()
	}
	// The old session's end hook runs before the new transcript
	// takes over (ADR-0014 §5), then the audit event under the old id.
	// From here to the runner's rebuild the session's identity, its
	// transcript and its work directory change: a grant — a late one
	// from an abandoned call included — waits, and one from the old
	// session is dropped by the epoch advanced now (ADR-0046).
	sess.granter.beginClear()
	sess.sessionEndHooks("clear")
	// The reports join the command's output: /clear runs inside the
	// TUI, where a line written to the terminal is covered by the next
	// redraw (ADR-0054, Found 4).
	sess.reportPersistent("clear", &extra, recordTo(sess.curLog))
	sess.watch.report("clear", &extra, recordTo(sess.curLog))
	sess.ag.Restart(newLog)
	// The new session re-checks the pins: a file that changed during
	// the old one is left out, and the system prompt and the skill
	// list are rebuilt from what the grant allows. Unchanged files
	// give the same prompt bytes, so the server's prefix cache
	// survives the clear (ADR-0003); the skill catalog rides the
	// facts message announced below.
	var pinNotes, regrantNotes []string
	sess.grant.excluded, pinNotes = checkPins(sess.cfg, sess.policyFile, sess.policyPath, sess.projectDir, sess.projectTrusted, false, nil, io.Discard, sess.msgs)
	notes = append(notes, pinNotes...)
	sess.projectContext, sess.skillsList, regrantNotes = regrant(sess.projectDir, sess.grant, sess.skillsList)
	sess.ag.SetSystem(sess.composeSystem())
	notes = append(notes, regrantNotes...)
	if sess.curLog != nil {
		_ = sess.curLog.Close()
	}
	sess.curLog = newLog
	sess.sessionLog = newLog
	sess.sessionPath, sess.sessionID = newLog.Path(), newLog.ID()
	sess.hookSession = hooks.Session{ID: sess.sessionID, TranscriptPath: sess.sessionPath, CWD: sess.projectDir}
	if len(sess.registry.AddedDirs()) > 0 {
		// The roots stay in force across /clear — startup, slash and
		// requested alike — so the new transcript records them too
		// (ADR-0046).
		_ = newLog.Log("added_directories", sess.granter.record())
	}
	if err := session.Export(sess.sessionID); err != nil {
		note("cannot set %s: %v", session.EnvVar, err)
	}
	if sess.workDir != "" {
		workdir.RemoveIfEmpty(sess.workDir)
	}
	sess.workDir = ""
	if dir, err := workdir.Ensure(sess.projectDir, sess.sessionID); err != nil {
		note("session work directory unavailable: %v", err)
	} else {
		sess.workDir = dir
	}
	// Exported or UNSET: the MCP servers reconnected below inherit
	// the environment, and the old directory must not survive in it.
	if err := exportWorkDir(sess.workDir); err != nil {
		note("cannot set %s: %v", workdir.EnvVar, err)
	}
	// Every consumer of the work directory follows it: the file
	// tools' second root, the sandbox profile, the MCP intake (it
	// reads the registry), and the model — told through a fresh
	// runtime-facts message, never through the system prompt, which
	// stays byte-identical so the server's prefix cache survives
	// the clear.
	notes = append(notes, rotateWorkDir(sess.registry, sess.shellExec, sess.sandboxOn, sess.projectDir, sess.workDir, sess.cfg.Sandbox.ReadLaneDenyExec, sess.cfg.Sandbox.Caches(), sess.cfg.Sandbox.ReadLanePrompts, trustpin.Parents(sess.projectDir, sess.persistentSnap), sess.registry.AddedDirs(), sess.granter.forget)...)
	// The rotate above ran under the grant lock: a late grant cannot
	// swap in a runner built against the old work directory.
	sess.granter.endClear()
	// The MCP servers — spawned at startup with the old id in their
	// environment and arguments — are reconnected the way /mcp
	// reload does, so a server keeping per-session state sees the
	// session the environment reports. A cleared session starts
	// unloaded, preloads aside (ADR-0004).
	sess.adv.Reset()
	if sess.cfg.MCP.Enabled {
		extra.WriteString(sess.reconnectMCP(false))
	}
	// A new session: a new start day, and no longer a resumed one.
	sess.dates = sessionDates{Start: time.Now()}
	// Told last, once everything it names is in place: the work
	// directory and the catalog ride one fresh facts message.
	sess.announceFacts()
	// A cleared conversation is a fresh start to the operator's
	// scripts (ADR-0014 §2): its output rides the first new turn.
	sess.sessionStartHooks("clear")
	return render()
}

// turn runs one turn of the conversation. A grant since the last turn is
// announced first, as a fresh facts message (ADR-0003's lane) — between
// turns, never during one. Every front end starts its turns here
// (ADR-0054 §1); under -p no grant can happen, so the announcement is
// idle.
func (sess *liveSession) turn(ctx context.Context, input string, out func(string)) error {
	sess.announceGrant()
	_, err := sess.ag.Run(ctx, input, out)
	return err
}

// slash runs a slash command against the live session: the memory
// commands first, then the rest — what the TUI and the plain REPL each
// did with the same arguments. slashOutput keeps its parameters, which
// its tests hand only what each command needs.
func (sess *liveSession) slash(input string, as assembled) (output string, isErr bool, quit bool) {
	if out, isErr, handled := memorySlash(input, sess.memStore); handled {
		return out, isErr, false
	}
	return slashOutput(input, sess.ag, sess.registry, sess.mcpStatus(), sess.skillsList,
		as.reloads,
		func() string { return usageReport(sess.ag, sess.cfg.LLM.Model) },
		appVersion, sess.msgs, sess.clear)
}
