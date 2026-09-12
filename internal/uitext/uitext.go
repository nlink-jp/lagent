// Package uitext holds the operator-facing UI strings in two complete
// catalogs, Japanese and English (gem-agent ADR-0029). One struct field per
// string keeps the two languages covering the same surface: the
// completeness test fails on any field left empty in either catalog,
// which is the mechanism that stops the historical one-string-at-a-time
// language drift from ever re-accumulating.
//
// Deliberately NOT here (gem-agent ADR-0029 §3): banner labels and "warning:"
// lines (grep-stable log output), cobra --help, model-facing text, and
// Go error chains.
//
// A notice the agent writes mid-turn IS here (gem-agent ADR-0079). Until v0.72.0
// the agent never read this package, so the same event printed Japanese
// when the operator asked for it and English when the runtime decided.
//
// The line between the two is authorship, not the presence of a next
// command: lagent composes the sentence, so it is cataloged, and it
// may quote a cause verbatim — the frame is translated, the cause
// arrives in whatever language it was written in. What stays English is
// a sentence that IS a returned error, because its innards come from
// libraries and a translated frame around them is the mixing §3
// removed. Every operator line should carry a next command; that is a
// rule about wording, and it was never a rule about language.
//
// Ported from gem-agent internal/uitext at be7609980022e38314268c58ca94a6517e6f5d28 (v0.74.0), ADR-0001.
package uitext

import "strings"

// Lang is a resolved UI language.
type Lang string

const (
	JA Lang = "ja"
	EN Lang = "en"
)

// Resolve turns the configured [tui].language value into a Lang.
// "auto" (and anything unrecognized — config validation rejects it
// earlier) follows the POSIX message-catalog convention: the first
// non-empty of LC_ALL, LC_MESSAGES, LANG decides, and only a "ja"
// prefix selects Japanese — "C" and "POSIX" mean English.
func Resolve(configured string, getenv func(string) string) Lang {
	switch configured {
	case "ja":
		return JA
	case "en":
		return EN
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(key); v != "" {
			if strings.HasPrefix(v, "ja") {
				return JA
			}
			return EN
		}
	}
	return EN
}

// For returns the catalog for lang. The pointer is shared and the
// catalogs are never mutated.
func For(lang Lang) *Messages {
	if lang == JA {
		return &ja
	}
	return &en
}

// Messages is one language's worth of interactive chrome. Every field
// must be non-empty in both catalogs (enforced by test); fields whose
// name ends in Fmt are fmt.Sprintf patterns and both catalogs must use
// the same verbs in the same order.
type Messages struct {
	// --- approval dialog (TUI) ---
	ApprovalTitleFmt  string // dialog title: %s = tool name
	ApproveAllow      string // dialog answer: allow once
	ApproveDeny       string // dialog answer: deny
	ApproveDenyReason string // dialog answer: deny with a typed reason (gem-agent ADR-0060)
	ApproveAlways     string // dialog answer: allow for the session
	ApprovePersist    string // dialog answer: persist never-ask (gem-agent ADR-0009 §5)
	ApprovalHint      string // key help under the dialog
	// ApprovalHintNoStanding is the same help for a call no standing
	// answer may settle: the two keys it drops are the two the dialog
	// does not offer.
	ApprovalHintNoStanding string
	// Reason field (gem-agent ADR-0060): the label above the input, its
	// placeholder, and the key help while it is open.
	ApprovalReasonPrompt      string
	ApprovalReasonPlaceholder string
	ApprovalReasonHint        string
	// ApprovalHiddenFmt warns that %d detail lines were clipped
	// (gem-agent ADR-0021: never approve what you have not seen).
	ApprovalHiddenFmt string
	// PurposePrefix marks the model's declared purpose (gem-agent ADR-0047), and
	// PurposeNone stands in its place when the model declared none —
	// "it did not say" and "there is nothing to say" must not look the
	// same on an approval prompt.
	PurposePrefix   string
	PurposeNone     string
	VerdictApproved string
	VerdictDenied   string
	// VerdictDeniedReasonFmt echoes a reasoned denial: %s = the reason
	// (clipped for display; the transcript keeps it whole).
	VerdictDeniedReasonFmt string
	VerdictAlways          string
	VerdictPersist         string
	// AutoApprovedFmt echoes an unattended approval: tier, reason.
	AutoApprovedFmt string
	CtrlCHint       string // "(interrupt with Ctrl+C)" while running

	// --- input chrome (TUI) ---
	Placeholder  string // empty input box hint
	QueueRefused string // ! and / cannot be queued mid-turn (gem-agent ADR-0021)
	QueuedPrefix string // prefix before an echoed queued message
	// QueueHandback explains a queued message returning unsent after a
	// failed or interrupted turn (gem-agent ADR-0007).
	QueueHandback string
	Interrupted   string // "(interrupted)" marker
	ErrorPrefix   string // prefix before a turn/shell error
	Bye           string // parting word on quit

	// --- settings panel (TUI) ---
	SettingsHint            string // key help in the /settings title row
	SettingsTitle           string // panel title
	SettingsMoreAboveFmt    string // "… %d more above" scroll marker
	SettingsMoreBelowFmt    string // "… %d more below" scroll marker
	SettingsImmutable       string // a read-only row was activated
	SettingsTooShort        string // the terminal cannot hold the panel
	SettingsSavedTo         string // label before the policy scope
	SettingsScopeGlobal     string // the global policy file
	SettingsScopeProjectFmt string // the project policy file (%s = project dir)
	SettingsUnavailable     string // /settings in a mode without the panel
	NoOutput                string // a shell command printed nothing

	// --- running-status chrome (TUI, gem-agent ADR-0033) ---
	StatusThinking     string
	StatusInterrupting string
	StatusToolWait     string
	StatusRunningFmt   string // %s = tool name
	StatusShellFmt     string // %s = command (clipped)
	// HeartbeatFmt: elapsed, chunk count, seconds since last chunk.
	HeartbeatFmt string
	// StallFmt: seconds with no data — the connection may be dead. It
	// does NOT name Ctrl+C: CtrlCHint renders right after it, and the
	// duplicate pushed the real hint off an 80-column terminal.
	StallFmt string
	// RetryFmt: attempt, max, cause token (429/503/error), wait seconds.
	RetryFmt string
	// ThoughtPrefix marks a live thought-summary line.
	ThoughtPrefix string
	// InterruptStuckWarn: the second Ctrl+C while already
	// interrupting — the next one quits (gem-agent ADR-0034 §3).
	InterruptStuckWarn string
	// AskTitleFmt / AskHint: the ask_user dialog (gem-agent ADR-0036).
	AskTitleFmt string // %s = the model's question
	AskHint     string
	// AskHiddenFmt discloses %d wrapped question lines the box could
	// not show (review round 3 — never answer what you have not read).
	AskHiddenFmt string
	// Round-limit intervention (gem-agent ADR-0040): the dialog question, the
	// review verdict shown as evidence, and the two answers.
	RoundLimitAskFmt    string // %d rounds used, %d hard cap, %s evidence
	RoundLoopAskFmt     string // %s repeated call, %s evidence
	RoundRecentCallsFmt string // %s = the turn's recent calls, one per line
	// ContextWindowUnknownFmt: the provider did not answer the
	// context-length lookup; %s = the error. The next command is the
	// config key.
	ContextWindowUnknownFmt string
	RoundContinue           string
	RoundStop               string

	// --- exit summary (cmd) ---
	// Printed once, on the way out — the last thing in the scrollback
	// answers "how do I get back to this?". Skipped when there was no
	// conversation: a resume hint for an empty session would be wrong.
	// ExitSessionFmt: %s = session id (twice).
	ExitSessionFmt string
	// ExitUsageFmt: rounds, prompt tokens, output tokens.
	ExitUsageFmt string
	// ExitAbandonedFmt: %d = tool calls the gem-agent ADR-0065 floor abandoned
	// that are still running at exit — their effect may land after
	// the process is gone, so the operator hears it.
	ExitAbandonedFmt string

	// --- slash command feedback (cmd) ---
	Help    string // the full /help text
	AutoOn  string
	AutoOff string
	// AutoUsage: /auto takes on|off as well as toggling, the same
	// grammar /readonly uses. An ignored argument is how `/auto on`
	// came to toggle instead.
	AutoUsage string
	// ReadOnlyOn/Off is the ceiling. They describe the state, not a
	// transition, and
	// only the states that constrain explain themselves: OFF is the
	// default, so "this session may change things" said something
	// obvious in a way that read as a puzzle (operator report).
	// ReadOnlyOn/Off describe the state, not a transition:
	// /readonly shows the current state as well as setting one, so a
	// transition verb ("…に戻りました") is a lie on the showing path.
	// Operator report, 2026-09-09.
	// The lift question (gem-agent ADR-0080 §4) is a mode change, not a tool
	// approval: its own title, its own consequence line, and only
	// y/n/N, because "allow for this session" and "always allow" are
	// answers to a question nobody asked here.
	CeilingLiftTitle       string
	CeilingLiftConsequence string
	CeilingLiftHint        string
	CeilingShellFmt        string
	// CeilingUnboundedReason explains why a call the ceiling cannot
	// bound is asked about anyway, every time.
	CeilingUnboundedReason string
	CeilingStateFmt        string
	ReadOnlyOn             string
	// ReadOnlyOnUnconfined is the same state under --no-sandbox, where
	// the refusal still reaches the file tools and a write- or
	// operator-declaring shell call but nothing bounds a command that
	// declares the read lane — so the plain sentence would promise what
	// only the lanes can give (independent review).
	ReadOnlyOnUnconfined string
	ReadOnlyOff          string
	ReadOnlyUsage        string
	// CeilingRefusedAgainFmt: %s = the tool. The lift dialog is asked
	// once a turn — a model that keeps trying is not worth re-asking
	// about. But the refusals kept happening with nothing on screen, so
	// in one-shot only the first denial was ever printed and in the TUI
	// the operator watched the model stall for no stated reason
	// (independent review). This is the line the suppressed dialog owes
	// them.
	CeilingRefusedAgainFmt string
	HistoryCleared         string

	// TranscriptFailedFmt: the cause. What reached the disk still
	// resumes; what follows the failure does not.
	TranscriptFailedFmt string
	// TruncatedFmt: why generation stopped early.
	TruncatedFmt string
	// EmptyRetried: the model returned nothing and the same request
	// is being sent once more (ADR-0007).
	EmptyRetried string
	// RemoteFaultFmt: server, tool, identical failures in a row.
	RemoteFaultFmt string
	// RoundLimitContinuedFmt / RoundLoopContinuedFmt: the progress
	// review extended the turn without asking. Two strings, not one:
	// the round limit and the loop guard are different events, and a
	// loop waved through announced as a round count told the operator
	// nothing about the repeat that triggered it. Neither names the
	// hard cap — it is not the number that stops them next, and
	// "round 20 of 200" read as 180 rounds of headroom when the next
	// check was at 30.
	RoundLimitContinuedFmt string // %d = the limit that fired
	RoundLoopContinuedFmt  string // %s = the repeated call
	// UnknownCommandFmt: %s = the input that matched no command.
	UnknownCommandFmt string
	MCPNone           string // /mcp with nothing connected
	// Integration reload results (gem-agent ADR-0039).
	MCPDisabled    string // /mcp reload while [mcp].enabled=false / --mcp off
	MCPReloadedFmt string // fmt: servers (int), tools (int)

	// --- startup safety (gem-agent ADR-0023, cmd) ---
	// TrustHeaderFmt opens the first-run prompt: project dir.
	TrustHeaderFmt string
	// TrustItem*Fmt describe what the project provides, naming what
	// each item implies (a server entry is a child process).
	TrustItemInstructionsFmt string // %s = file names
	TrustItemMCPFmt          string // %d = server count
	TrustItemSkillsFmt       string // %d = skill count
	TrustQuestion            string // the [y/N] question
	// Content pins (gem-agent ADR-0074).
	PinRecordedFmt         string // %d = files recorded, %s = their names
	PinNonePending         string // no pins yet, non-interactive: loaded as before
	PinChangeFmt           string // %s name, %s kind (PinKindChanged/PinKindAdded), %s size — one described change
	PinKindChanged         string // the kind word of a changed file
	PinKindAdded           string // the kind word of a new file
	PinChangedFmt          string // %s = PinChangeFmt result; the prompt line
	PinQuestion            string // the [y/N] question for one changed file
	PinNotLoadedFmt        string // %s = PinChangeFmt result — not loaded
	PinAcceptedFmt         string // %s = name
	PinRemovedFmt          string // %s = name — gone since trusted; pin kept
	PinPendingFmt          string // %s = PinChangeFmt results after an operator command
	PinStaleWriteFmt       string // %s = name — had drifted before the approved write; not re-pinned
	PersistentSinceLastFmt string // %s = comma list — changed since the previous session
	PersistentSessionFmt   string // %s = comma list — added/changed by this session
	// TrustDeclinedFmt is the banner note after declining: policy path.
	TrustDeclinedFmt string
	TrustUndecided   string // non-interactive, undecided: ran bare
	// Broad-root gate (gem-agent ADR-0023 §1). Reasons name what projectDir is.
	ReasonFSRoot        string
	ReasonHome          string
	ReasonHomeAncestor  string
	BroadRootPromptFmt  string // %s dir, %s reason
	BroadRootRefusedFmt string // non-interactive refusal: %s dir, %s reason
	BroadRootAbortFmt   string // declined: %s dir
}

// BroadReason maps a broadRoot key ("root", "home", "home-ancestor")
// to its localized description.
func (m *Messages) BroadReason(key string) string {
	switch key {
	case "root":
		return m.ReasonFSRoot
	case "home":
		return m.ReasonHome
	case "home-ancestor":
		return m.ReasonHomeAncestor
	}
	return key
}

var en = Messages{
	ApprovalTitleFmt:          "approval required: %s",
	ApproveAllow:              "allow (y)",
	ApproveDeny:               "deny (n)",
	ApproveDenyReason:         "deny with reason (N)",
	ApproveAlways:             "allow this session (a)",
	ApprovePersist:            "allow permanently (p)",
	ApprovalHint:              "←→/Tab select · Enter confirm · y/n/N/a/p direct · Esc denies",
	ApprovalHintNoStanding:    "←→/Tab select · Enter confirm · y/n/N direct · Esc denies",
	ApprovalReasonPrompt:      "deny reason:",
	ApprovalReasonPlaceholder: "why this call should not run, or what to do instead…",
	ApprovalReasonHint:        "Enter send · empty Enter denies without a reason · Esc back",
	ApprovalHiddenFmt:         "⚠ +%d lines hidden — do not approve without seeing all of it (deny, then inspect)",
	PurposePrefix:             "↪ ",
	PurposeNone:               "(no purpose declared)",
	VerdictApproved:           "approved",
	VerdictDenied:             "denied",
	VerdictDeniedReasonFmt:    "denied — reason: %s",
	VerdictAlways:             "approved (always this session)",
	VerdictPersist:            "approved (and this tool will not ask again)",
	AutoApprovedFmt:           "  ↳ auto-approved (%s): %s",
	CtrlCHint:                 "  (Ctrl+C interrupts)",

	Placeholder:   "message…  Enter send · Ctrl+J newline · /help · !shell",
	QueueRefused:  "⚠ ! and / commands cannot run mid-turn — interrupt with Ctrl+C first (your input is preserved)",
	QueuedPrefix:  "⏎ queued: ",
	QueueHandback: "⚠ the queued message was not sent — the turn did not finish. It is back in the input box",
	Interrupted:   "(interrupted)",
	ErrorPrefix:   "✗ error: ",
	Bye:           "bye",

	SettingsHint:            "  ↑↓ select · ←→ change · Enter open/close a server · s scope · Esc close",
	SettingsTitle:           "settings",
	SettingsMoreAboveFmt:    "  … %d more above",
	SettingsMoreBelowFmt:    "  … %d more below",
	SettingsImmutable:       "this setting cannot change mid-session — edit the config file and restart",
	SettingsTooShort:        "  terminal too short — resize, or edit the config file directly",
	SettingsSavedTo:         "  policy changes are saved to: ",
	SettingsScopeGlobal:     "global (~/.config/lagent/policy.toml)",
	SettingsScopeProjectFmt: "this project only — %s",
	SettingsUnavailable:     "✗ settings are unavailable in this mode — edit ~/.config/lagent/config.toml",
	NoOutput:                "(no output)",

	StatusThinking:          "thinking…",
	StatusInterrupting:      "interrupting…",
	StatusToolWait:          "waiting for the tool…",
	StatusRunningFmt:        "running %s",
	StatusShellFmt:          "shell: %s",
	HeartbeatFmt:            "%s · %d chunks · last %ds",
	StallFmt:                "no data for %ds — the stream may be stalled",
	RetryFmt:                "retry %d/%d (%s) — waiting %ds",
	ThoughtPrefix:           "✦ ",
	InterruptStuckWarn:      "⚠ tool ignores cancel — one more Ctrl+C quits lagent (the session is saved)",
	AskTitleFmt:             "question: %s",
	AskHint:                 "←→/Tab select · 1-9 pick directly · Enter confirm · Esc declines",
	AskHiddenFmt:            "⚠ +%d lines hidden — enlarge the terminal, or Esc to decline (the model can re-ask briefly)",
	RoundLimitAskFmt:        "round limit reached: %d rounds used (hard cap %d)%s. Continue?",
	RoundLoopAskFmt:         "possible loop: the same call keeps repeating (%s)%s. Continue?",
	RoundRecentCallsFmt:     "\nrecent calls:\n  %s\n",
	ContextWindowUnknownFmt: "context window unknown (%v) — set [model].context_window to show the gauge",
	RoundContinue:           "continue",
	RoundStop:               "stop here",

	ExitSessionFmt:   "session %s — resume: lagent -c (or --resume %s)",
	ExitUsageFmt:     "%d rounds · prompt %s · output %s",
	ExitAbandonedFmt: "%d abandoned tool call(s) still running — an effect may still land after this exit",

	Help: `commands:
  /help      show this help
  /tools     list tools and each one's current approval gate
  /mcp       list connected MCP servers with their loaded state (/mcp load <server> advertises one, /mcp reload reconnects)
  /auto      auto-approve: on|off, or bare to toggle (shift+tab too)
  /readonly  read-only: on|off · bare shows the state
  /settings  view and edit settings, with provenance
  /skills    list installed skills
  /skill <name> [args]   invoke a skill directly
  /memory    list memories (facts recalled in every session)
  /remember [global] <name> <fact>   save a memory (project scope unless global)
  /forget [global] <name>            remove a memory
  /usage     token statement for this session
  /version   version and platform
  /clear     reset the conversation
  /quit      exit (/exit and Ctrl+D too)

attach:
  @<path>      a project file or directory (Tab completes)
  @<image>     images may also use absolute or ~ paths (@~/Desktop/shot.png)
  @clipboard   the clipboard image

shell:
  !<command>   run directly — sandboxed, no approval, output shared with the model

keys:
  Enter send · up/down history · Ctrl+C interrupt/clear · Ctrl+D quit
  Ctrl+J or a trailing \ inserts a newline; a multi-line paste stays one message
  typing during a turn queues the text (! and / cannot be queued)
  approval dialog: arrows/Tab select · Enter confirm · y/n/N/a/p direct (N = deny with a reason)
`,
	AutoOn:                 "auto-approve: ON — safe changes run unattended; risky ones still ask\n",
	AutoOff:                "auto-approve: OFF — every change asks\n",
	AutoUsage:              "usage: /auto on|off (no argument toggles)\n",
	CeilingLiftTitle:       "Lift read-only?",
	CeilingLiftConsequence: "Yes lifts read-only for the rest of this session.",
	CeilingLiftHint:        "←→/Tab to choose · Enter to answer · y/n/N · Esc refuses",
	CeilingShellFmt:        "this session is capped at the %s lane, and the command declared %s",
	CeilingUnboundedReason: "read-only is on, and this tool runs on another server the ceiling cannot bound — so no standing answer applies to it, and none is created here",
	CeilingStateFmt:        "this session is capped at the %s lane, and this tool changes state outside it",
	ReadOnlyOn:             "read-only mode: ON — nothing outside the session scratch changes; /readonly off lifts it\n",
	ReadOnlyOnUnconfined:   "read-only mode: ON — but the sandbox is off, so a shell command declaring the read lane is bounded by nothing; /readonly off lifts the rest\n",
	ReadOnlyOff:            "read-only mode: OFF\n",
	ReadOnlyUsage:          "usage: /readonly on|off · no argument shows the state\n",
	CeilingRefusedAgainFmt: "%s refused: read-only is still on. You declined to lift it this turn, so this one was not asked",
	HistoryCleared:         "history cleared — the next message starts a fresh conversation\n",

	TranscriptFailedFmt:    "session transcript write failed (%s) — recording stopped, so this session can no longer be resumed in full; restart lagent to record again",
	TruncatedFmt:           "the response was cut off mid-generation (%s) — ask for the rest, or narrow the request",
	EmptyRetried:           "the model returned an empty response — sending the same request again",
	RemoteFaultFmt:         "MCP server %q: %s failed %d times in a row with the same error — /mcp reload, or fix the server",
	RoundLimitContinuedFmt: "round limit reached at %d rounds — continued at your request",
	RoundLoopContinuedFmt:  "the same call repeated (%s) — continued at your request",
	UnknownCommandFmt:      "unknown command %q — /help lists commands\n",
	MCPNone:                "no MCP servers connected — define them in ~/.config/lagent/mcp.json (global) or the project's .mcp.json (project; wins name collisions)\n",
	MCPDisabled:            "MCP is disabled for this session ([mcp].enabled=false or --mcp off) — restart to enable it\n",
	MCPReloadedFmt:         "mcp reloaded: %d server(s), %d tool(s)\n",

	TrustHeaderFmt:           "\nnew project: %s\nthis project provides:\n",
	TrustItemInstructionsFmt: "%s (loaded as your instructions)",
	TrustItemMCPFmt:          ".mcp.json (%d server(s) — will be started)",
	TrustItemSkillsFmt:       ".claude/skills/ (%d skill(s) — loaded as your instructions)",
	TrustQuestion:            "trust this project? These files will be treated as YOUR instructions and its MCP servers will run. [y/N]: ",
	PinRecordedFmt:           "project trust: %d file(s) recorded as trusted: %s",
	PinNonePending:           "project trust: no trusted files recorded yet — start interactively once, or run `lagent trust --accept`",
	PinChangeFmt:             "%s %s %s",
	PinKindChanged:           "changed",
	PinKindAdded:             "added",
	PinChangedFmt:            "\n%s since you trusted it.",
	PinQuestion:              "trust the new content? [y/N]: ",
	PinNotLoadedFmt:          "project trust: %s since you trusted it — not loaded; re-trust with `lagent trust --accept` or at an interactive start",
	PinAcceptedFmt:           "project trust: %s re-trusted",
	PinRemovedFmt:            "project trust: %s was removed since you trusted it",
	PinPendingFmt:            "project trust: %s since you trusted it — not re-trusted; `lagent trust --accept` or the next interactive start",
	PinStaleWriteFmt:         "project trust: %s had changed before this write — not re-trusted; asks at the next interactive start",
	PersistentSinceLastFmt:   "note: changed since your previous session: %s",
	PersistentSessionFmt:     "note: this session added or changed: %s",
	TrustDeclinedFmt:         "project trust: declined — the project's own files are not loaded (edit %s to be asked again)",
	TrustUndecided:           "project trust: undecided — the project's own files are not loaded; start interactively once to decide",
	ReasonFSRoot:             "the filesystem root",
	ReasonHome:               "your home directory",
	ReasonHomeAncestor:       "an ancestor of your home directory",
	BroadRootPromptFmt:       "\n⚠ %s is %s.\nFile tools and sandboxed shell writes would span this ENTIRE tree.\nstart anyway? [y/N]: ",
	BroadRootRefusedFmt:      "refusing to start in %s (%s): file tools and shell writes would span this entire tree; run interactively to confirm, or start in a project directory",
	BroadRootAbortFmt:        "not starting in %s — cd into a project directory first",
}

var ja = Messages{
	ApprovalTitleFmt:          "承認が必要です: %s",
	ApproveAllow:              "許可 (y)",
	ApproveDeny:               "拒否 (n)",
	ApproveDenyReason:         "理由を添えて拒否 (N)",
	ApproveAlways:             "このセッション中は許可 (a)",
	ApprovePersist:            "今後も許可 (p)",
	ApprovalHint:              "←→/Tab 選択 · Enter 決定 · y/n/N/a/p 直接指定 · Esc 拒否",
	ApprovalHintNoStanding:    "←→/Tab 選択 · Enter 決定 · y/n/N 直接指定 · Esc 拒否",
	ApprovalReasonPrompt:      "拒否理由:",
	ApprovalReasonPlaceholder: "拒否する理由や、代わりにすべきこと…",
	ApprovalReasonHint:        "Enter 送信 · 空 Enter は理由なし拒否 · Esc で戻る",
	ApprovalHiddenFmt:         "⚠ +%d 行が省略されています — 全体を見るまで承認しないでください（拒否して確認できます）",
	PurposePrefix:             "↪ ",
	PurposeNone:               "（理由の申告なし）",
	VerdictApproved:           "許可しました",
	VerdictDenied:             "拒否しました",
	VerdictDeniedReasonFmt:    "拒否しました — 理由: %s",
	VerdictAlways:             "許可しました（このセッション中は常に）",
	VerdictPersist:            "許可しました（このツールは今後確認しません）",
	AutoApprovedFmt:           "  ↳ 自動承認 (%s): %s",
	CtrlCHint:                 "  (Ctrl+C で中断)",

	Placeholder:   "メッセージ…  Enter 送信 · Ctrl+J 改行 · /help · !shell",
	QueueRefused:  "⚠ ! と / のコマンドは実行中には送れません — Ctrl+C で中断してから実行してください（入力は残っています）",
	QueuedPrefix:  "⏎ 予約: ",
	QueueHandback: "⚠ 予約したメッセージは送信されませんでした — ターンが正常に終了しなかったため、入力欄に戻しています",
	Interrupted:   "（中断）",
	ErrorPrefix:   "✗ エラー: ",
	Bye:           "bye",

	SettingsHint:            "  ↑↓ 選択 · ←→ 変更 · Enter サーバーを開閉 · s スコープ · Esc 閉じる",
	SettingsTitle:           "設定",
	SettingsMoreAboveFmt:    "  … 上に %d 件",
	SettingsMoreBelowFmt:    "  … 下に %d 件",
	SettingsImmutable:       "この設定はセッション中に変更できません — 設定ファイルを編集して再起動してください",
	SettingsTooShort:        "  端末の高さが足りません — 広げるか、設定ファイルを直接編集してください",
	SettingsSavedTo:         "  ポリシーの保存先: ",
	SettingsScopeGlobal:     "グローバル (~/.config/lagent/policy.toml)",
	SettingsScopeProjectFmt: "このプロジェクトのみ — %s",
	SettingsUnavailable:     "✗ このモードでは設定パネルを使えません — ~/.config/lagent/config.toml を編集してください",
	NoOutput:                "(出力なし)",

	StatusThinking:          "思考中…",
	StatusInterrupting:      "中断中…",
	StatusToolWait:          "ツールの完了待ち…",
	StatusRunningFmt:        "実行中 %s",
	StatusShellFmt:          "shell: %s",
	HeartbeatFmt:            "%s · %d チャンク · 最終 %d 秒前",
	StallFmt:                "%d 秒間データなし — 接続が失速している可能性",
	RetryFmt:                "リトライ %d/%d (%s) — %d 秒待機",
	ThoughtPrefix:           "✦ ",
	InterruptStuckWarn:      "⚠ ツールがキャンセルに応答しません — もう一度 Ctrl+C で lagent を終了します（セッションは保存済み）",
	AskTitleFmt:             "質問: %s",
	AskHint:                 "←→/Tab 選択 · 1-9 で即決定 · Enter 決定 · Esc 回答しない",
	AskHiddenFmt:            "⚠ +%d 行が非表示 — 端末を広げるか、Esc で辞退（モデルは短く聞き直せます）",
	RoundLimitAskFmt:        "ラウンド上限に到達: %d ラウンド消費（絶対上限 %d）%s。続行しますか？",
	RoundLoopAskFmt:         "ループの疑い: 同一コールが反復しています（%s）%s。続行しますか？",
	RoundRecentCallsFmt:     "\n直近のコール:\n  %s\n",
	ContextWindowUnknownFmt: "コンテキスト窓が不明（%v）— [model].context_window を設定するとゲージが出ます",
	RoundContinue:           "続行",
	RoundStop:               "ここで停止",

	ExitSessionFmt:   "セッション %s — 再開: lagent -c（または --resume %s）",
	ExitUsageFmt:     "%d ラウンド · prompt %s · output %s",
	ExitAbandonedFmt: "放棄したツール呼び出し %d 件がまだ実行中 — 終了後に効果が及ぶことがあります",

	Help: `コマンド:
  /help      このヘルプ
  /tools     ツール一覧と各ツールの現在の承認ゲート
  /mcp       接続中の MCP サーバー一覧とロード状態（/mcp load <server> で 1 台を広告、/mcp reload で再接続）
  /auto      auto-approve: on|off、引数なしで切替（shift+tab でも可）
  /readonly  読み取り専用: on|off・引数なしで状態を表示
  /settings  設定の表示と編集（出所つき）
  /skills    インストール済みスキル一覧
  /skill <name> [args]   スキルを直接起動
  /memory    メモリ一覧（毎セッション想起される事実）
  /remember [global] <name> <fact>   メモリを保存（global 指定が無ければ project）
  /forget [global] <name>            メモリを削除
  /usage     このセッションのトークン明細
  /version   バージョンとプラットフォーム
  /clear     会話履歴をリセット
  /quit      終了（/exit・Ctrl+D でも可）

添付:
  @<パス>      プロジェクト内のファイル/ディレクトリ（Tab 補完）
  @<画像>      画像は絶対パス・~ パスも可（@~/Desktop/shot.png）
  @clipboard   クリップボードの画像

シェル:
  !<コマンド>   直接実行 — sandbox 下・承認なし・出力はモデルと共有

キー:
  Enter 送信 · ↑↓ 履歴 · Ctrl+C 中断/クリア · Ctrl+D 終了
  改行は Ctrl+J か行末 \ + Enter。複数行ペーストは 1 メッセージのまま
  実行中の入力は次メッセージとして予約（! と / は予約不可）
  承認ダイアログ: ←→/Tab 選択 · Enter 決定 · y/n/N/a/p 直接（N = 理由を添えて拒否）
`,
	AutoOn:                 "auto-approve: ON — 安全な変更は無人で実行します。危険なものは引き続き確認します\n",
	AutoOff:                "auto-approve: OFF — すべての変更で確認します\n",
	AutoUsage:              "使い方: /auto on|off（引数なしで切り替え）\n",
	CeilingLiftTitle:       "read-only を解除しますか",
	CeilingLiftConsequence: "「はい」はこのセッションの残りで read-only を解除します",
	CeilingLiftHint:        "←→/Tab 選択 · Enter 決定 · y/n/N 直接指定 · Esc 拒否",
	CeilingShellFmt:        "このセッションは %s レーンに抑えられていますが、このコマンドは %s を宣言しています",
	CeilingUnboundedReason: "read-only が ON ですが、このツールは上限が縛れない別サーバーで動きます。そのため継続的な許可は効かず、ここでの回答も残りません",
	CeilingStateFmt:        "このセッションは %s レーンに抑えられていますが、このツールはスクラッチの外の状態を変更します",
	ReadOnlyOn:             "読み取り専用モード: ON — スクラッチの外は変更しません。解除は /readonly off\n",
	ReadOnlyOnUnconfined:   "読み取り専用モード: ON — ただし sandbox が off のため、read レーンを宣言したシェルコマンドは何にも縛られません。残りの解除は /readonly off\n",
	ReadOnlyOff:            "読み取り専用モード: OFF\n",
	ReadOnlyUsage:          "使い方: /readonly on|off・引数なしで状態を表示\n",
	CeilingRefusedAgainFmt: "%s を拒否: 読み取り専用モードのままです。このターンで解除しないと答えたため、確認は出していません",
	HistoryCleared:         "履歴をクリアしました — 次のメッセージから新しい会話が始まります\n",

	TranscriptFailedFmt:    "セッション記録の書き込みに失敗しました（%s）。記録が停止したため、このセッションは完全な形では再開できません。記録を再開するには lagent を起動し直してください",
	TruncatedFmt:           "応答が生成途中で打ち切られました（%s）— 続きを求めるか、要求を絞ってください",
	EmptyRetried:           "モデルが空の応答を返しました — 同じ要求をもう一度送ります",
	RemoteFaultFmt:         "MCP サーバー %q: %s が同じエラーで %d 回連続して失敗しました — /mcp reload、またはサーバー側を修正してください",
	RoundLimitContinuedFmt: "ラウンド上限 %d に達しました — あなたの指示で継続しました",
	RoundLoopContinuedFmt:  "同じ呼び出しが繰り返されました（%s）— あなたの指示で継続しました",
	UnknownCommandFmt:      "未知のコマンド %q — /help に一覧があります\n",
	MCPNone:                "MCP サーバー未接続 — ~/.config/lagent/mcp.json（グローバル）またはプロジェクトの .mcp.json（プロジェクト側が名前衝突で優先）で定義します\n",
	MCPDisabled:            "MCP はこのセッションでは無効です（[mcp].enabled=false または --mcp off）— 有効化するには再起動してください\n",
	MCPReloadedFmt:         "MCP を再接続しました: %d サーバー・%d ツール\n",

	TrustHeaderFmt:           "\n新しいプロジェクト: %s\nこのプロジェクトの提供物:\n",
	TrustItemInstructionsFmt: "%s（あなたへの指示として読み込まれます）",
	TrustItemMCPFmt:          ".mcp.json（サーバー %d 件 — 起動されます）",
	TrustItemSkillsFmt:       ".claude/skills/（スキル %d 件 — あなたへの指示として読み込まれます）",
	TrustQuestion:            "このプロジェクトを信用しますか？ これらのファイルはあなたへの指示として扱われ、MCP サーバーが起動します。 [y/N]: ",
	PinRecordedFmt:           "project trust: %d 件を信用済みとして記録: %s",
	PinNonePending:           "project trust: 信用済みファイルは未記録です — 一度対話起動するか `lagent trust --accept` を実行",
	PinChangeFmt:             "%s が%s %s",
	PinKindChanged:           "変更されました",
	PinKindAdded:             "追加されました",
	PinChangedFmt:            "\n信用した時点から %s。",
	PinQuestion:              "新しい内容を信用しますか？ [y/N]: ",
	PinNotLoadedFmt:          "project trust: 信用した時点から %s — 読み込みません。`lagent trust --accept` か対話起動で再信用",
	PinAcceptedFmt:           "project trust: %s を再信用しました",
	PinRemovedFmt:            "project trust: %s は信用した時点から削除されています",
	PinPendingFmt:            "project trust: 信用した時点から %s — 再信用していません。`lagent trust --accept` か次の対話起動で",
	PinStaleWriteFmt:         "project trust: %s はこの書込の前から変わっていました — 再信用していません。次の対話起動で確認",
	PersistentSinceLastFmt:   "note: 前回のセッション以降に変更: %s",
	PersistentSessionFmt:     "note: このセッションが追加・変更: %s",
	TrustDeclinedFmt:         "project trust: 拒否 — このプロジェクト自身のファイルは読み込みません（再確認するには %s を編集）",
	TrustUndecided:           "project trust: 未決定 — このプロジェクト自身のファイルは読み込みません。一度対話起動して決めてください",
	ReasonFSRoot:             "ファイルシステムのルート",
	ReasonHome:               "ホームディレクトリ",
	ReasonHomeAncestor:       "ホームディレクトリの祖先",
	BroadRootPromptFmt:       "\n⚠ %s は %s です。\nファイルツールとサンドボックス内シェルの書き込みが、このツリー全体に及びます。\nこのまま起動しますか？ [y/N]: ",
	BroadRootRefusedFmt:      "%s（%s）では起動を拒否します: ファイルツールとシェル書き込みがツリー全体に及びます。対話モードで確認するか、プロジェクトディレクトリで起動してください",
	BroadRootAbortFmt:        "%s では起動しません — まずプロジェクトディレクトリに cd してください",
}
