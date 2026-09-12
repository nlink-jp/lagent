# ADR-0012: Pre-tool hooks are the operator's control outside the model

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-12 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: hooks belong here too — when the model runs wild they are, with the sandbox, the defence that does not depend on the model listening |

## Context

The sandbox decides what a lane may touch: the read lane cannot write
the project, no lane reaches the credential paths, an operator-only
file always asks. It cannot refuse a command by its shape. `gofmt -w .`
in the write lane the operator just approved, or under `--auto` where
the rule tier calls an in-project write Safe, rewrites the whole
workspace inside every boundary the sandbox draws. The organization
runs a Claude Code `PreToolUse` guard against exactly that class
(recursive in-place rewrites aimed at relative paths), on the stated
ground that a written rule cannot prevent a lapse of attention, so the
control has to live outside the agent. gem-agent carries the same guard
(gem-agent ADR-0044) so that it survives a fallback; lagent, until now,
did not, and it is the runtime whose model is the least steerable by
prose (ADR-0008: a rule in the prompt is not a control here).

What this runtime has already measured bears on the shape:

- **A denial with its reason steers the model.** Under ADR-0008 an
  unattended denial names the route and the bench shows the model
  taking it. A hook's reason returned as the tool result is the same
  mechanism.
- **A refused call repeated is caught by the round ladder.** The same
  call three times in a row escalates or, unattended, stops the turn.
  A hook that refuses a call and a model that insists therefore meet
  the loop guard within three rounds, which is the "runs wild" case
  the operator named, closed without a model tier.
- **Tool results are data** (RFP §3). A hook is a configured command
  whose words no one reviews at a prompt, so its reason ships wrapped
  like any result, not unwrapped like a skill (ADR-0011).

gem-agent's hook contract was measured, not read from documentation:
the installed guard reads `tool_input.command` from Claude Code's
PreToolUse stdin JSON and answers with
`hookSpecificOutput.permissionDecision: "deny"` on stdout, exit 0.
`shell_exec` names its argument `command` here as there, so the org's
guard runs on a lagent payload unchanged.

## Decision

1. **One event, from lagent's own config, global only.**
   `[[hooks.pre_tool_use]]` entries in `config.toml` carry a `matcher`,
   a `command` and an optional `timeout_sec`. `~/.claude/settings.json`
   is not read (ADR-0011's principle: the format is drop-in, the
   location is not shared). A project file cannot register a hook: a
   cloned repository would run an arbitrary command on every tool call.
   Claude Code's other events are not implemented; the mechanism takes
   them when a consumer appears (gem-agent ADR-0069 added two for
   agent-board), and that is a record of its own.
2. **A deny is a floor.** The hook runs at the top of tool dispatch,
   after the advertise check and before the decision ladder: neither
   auto-approve, a `"never"` row, `--allow` nor the session allowlist
   can overrule it. The reason goes to the model as the tool result,
   wrapped as data; the transcript records `hook_denied` with the
   reason. Hooks guard the model's calls only: the operator's `!`
   escape does not pass through them.
3. **Both verdict forms; anything else fails open with a notice.** A
   hook denies by stdout JSON (`hookSpecificOutput.permissionDecision:
   "deny"` or the older `decision: "block"`) or by exit code 2 with
   stderr as the reason. Non-zero exit, unparseable output and a
   timeout let the call proceed to the ladder and put one warning in
   front of the operator: hooks only ever tighten, and a broken guard
   must not brick the runtime. Hooks run unsandboxed, in the project
   directory, with a ten-second default timeout, in their own process
   group so the timeout kills what they started, with both streams
   bounded as they arrive.
4. **Matchers speak both vocabularies.** An exact tool name, a
   `|`-alternation or `*`, matched against lagent's name and its Claude
   Code alias (`Bash`, `Write`, `Edit`, `Read`), so a hooks block
   copied from Claude Code settings works without renaming. The
   payload's `tool_name` is lagent's real name.
5. **The payload is Claude Code's PreToolUse shape** with the session
   identity every Claude Code event carries: `hook_event_name`,
   `session_id`, `transcript_path` (empty when the log is disabled),
   `cwd`, `tool_name`, `tool_input`. `tool_input` is the call's
   arguments as the model sent them, minus the declared purpose (the
   hook is a judge; the proposer's justification is not evidence). For
   `shell_exec` that includes `access`, the lane — a guard here can key
   on the lane as well as on the command.

## Consequences

- The org's rewrite guard and delete rules hold on this runtime,
  enforced by the same script files Claude Code and gem-agent run.
  Registration is one block in lagent's config pointing at the same
  file; nothing is copied.
- Every model tool call pays one matcher comparison; a matched call
  pays one process spawn. Unconfigured, nothing runs.
- A hook can deny everything (a visible denial of service) and can
  approve nothing.
- An unattended run whose call a hook refused stops on the loop guard
  if the model insists — the fail-closed ceiling ADR-0010 already
  states for this runtime, now with the hook's reason in the
  transcript.
- `internal/hooks` is a port of gem-agent's package (source commit in
  its doc comment, ADR-0001) cut to the one event; `agent.Options`
  gains `PreToolHook`. One `[hooks]` section in the config reference
  and the shipped template; no settings-panel row — a control with a
  toggle is a bypass.

## Alternatives considered

- **Read `~/.claude/settings.json`.** Rejected: the runtime's guards
  would change whenever Claude Code's settings do, and a settings file
  carries far more than hooks. The same principle as ADR-0011.
- **All of Claude Code's events at once.** Rejected: one event has a
  demonstrated consumer; the others are dead weight until one appears,
  and each has its own delivery rule to decide (gem-agent ADR-0069 §4
  took a page to place context hooks' output).
- **A prompt rule ("never run a recursive rewrite on `.`").** Rejected
  by ADR-0008 and by the guard's own rationale: a rule the model must
  remember fails exactly when it stops listening. The control must not
  need the model's cooperation.
- **A pattern in the rule tier's Block floor.** Rejected: the rule tier
  reads no command text for intent (gem-agent ADR-0073; the lane
  decides), and its Block list is the runtime's finite floor. The
  operator's guards are the operator's, in the operator's language,
  and change without a release.
