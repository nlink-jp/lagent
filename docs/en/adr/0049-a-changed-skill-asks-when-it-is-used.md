# ADR-0049: a changed skill asks when it is used

| Field | Value |
|-------|-------|
| Status | **Proposed** (2026-11-17) |
| Date | 2026-11-17 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The review before v0.27.4 noted that a project skill is read from disk when it is used, not when it is checked. Operator: when it has changed, ask whether to trust it or disable it |
| Ports | gem-agent ADR-0119 |
| Relates to | ADR-0011 §5–§6 (skills: `/skill` injects the body; trust), ADR-0003 (the skill catalog rides the facts message), ADR-0016 §2 (a read asked about after it was refused), ADR-0032 (the model tier), ADR-0045 (a pipe without `-p`) |

## Context

A project skill is pinned by content (ADR-0011 §6), and the pin is compared when the list of skills is built: at startup and at `/clear`. The skill's body and supporting files are read from disk later, each time `load_skill` or `/skill` uses it. Nothing is compared then.

## Measurement

lagent v0.27.4, a real session against a local model. A project skill was trusted and pinned, the session started, and `SKILL.md` was rewritten. The note after a `!` command named the skill as changed. `/skill <name>` then sent the rewritten body to the model, which followed it. After a second rewrite, the model's own `load_skill` call returned the second text.

What changes the file mid-session is mostly the operator's own doing — a `! git pull`, an operator-lane command, an editor in another window; the write lane denies `.claude`, and an approved write re-pins the one name it wrote. But not only the operator: a skill entry that is a link to a directory elsewhere in the project is protected by name, so a write to the link's target is an ordinary write (measured for the file tools by the design review), and MCP servers run outside the sandbox. Either way nobody confirmed the content, and it is loaded as the operator's own instructions and sent unwrapped.

## Decision

1. Every read of a project skill compares first. `load_skill` (the body and the `file` form) and `/skill` digest that skill's directory and compare it with the record on disk (`recordedPolicy`). One implementation takes the digest for the pin and for this comparison, through an open root — the object the loader reads — so the two cannot disagree about an unchanged skill. Equal: the read runs as today. Different, or a listed skill the recorded set does not hold: the question of item 2. A directory that is gone is an error, not a question. With pins off, or with nothing recorded for the project, nothing is compared. A global skill is never compared.
2. The question is its own question in the gate. The read-only lift is the precedent for a question that is not "may this call run"; this one has its own answers. It names the skill by its validated name and shows its directory, says either that the content has changed since it was trusted or that the trusted record does not hold it, says the content should be looked at, and says what each answer does, including that trusting records the content for later sessions. It offers two answers, trust and disable, and nothing else. No standing answer reaches it: not the session allowlist, a `"never"` policy, auto-approve or the model tier. It does not consult the session's exclusion set either: a listed skill is one the session loaded. The wording does not say whose change it was.
3. Three outcomes, and the gate returns all three: trusted, disabled, unanswered. Every gate implements the question — the TUI, the stdin gate, the one-shot gate — and a gate that does not is read as unanswered. On the TUI, Esc, Ctrl+C and an interrupted turn are unanswered. On the stdin gate only the two answers count: a blank line or other text asks again, a closed input is unanswered, and an answer that arrives after the turn was interrupted is discarded.
4. Trust records what was asked about. The directory is digested again. If it differs from the digest that raised the question, nothing is recorded and the question is asked once more about the new state; a second difference refuses the read as unanswered and says the skill is still changing. Otherwise that digest is recorded for that one name under the policy file's lock — by a write of its own, which takes the digest it expects — and the name leaves the session's exclusion set. The read runs, and the directory is digested once more: if it differs, the result is discarded and the read is refused the same way. The pin stays; it is what was asked about.
5. Disable holds for the session. The answer is kept beside the grant under a lock, read by `load_skill` and `/skill` before anything else, and written only after the gate has answered, never from inside the tool. Later uses are refused without a question, each with a one-line notice to the operator. The answer is forgotten where the list of skills is rebuilt and only there. Undoing it inside the session takes `lagent trust --accept` from another terminal and `/clear`.
6. Unanswered refuses that one read. Nothing is recorded and nothing is disabled; the next use asks again.
7. Where nobody is at a terminal, nothing is asked. The predicate is the one the pin check uses at startup, so a one-shot run and a piped run both refuse the read, with a text of their own that names `lagent trust --accept` and does not send the model to the read tools for the same file.
8. `load_skill` asks the way a refused credential read does (ADR-0016 §2): the tool reports that the skill differs, the agent asks outside the floor that abandons a slow tool, and on trust the tool runs again. The digest and the dialog are never inside the tool's own run.
9. `/skill` asks before it injects, without a model round. The compare, the question and the read move to the turn's own goroutine and go through the same agent path as item 8, so a cancelled turn raises no dialog and the outcome is recorded the same way. Trust injects the body as ADR-0011 §5 decided; disable or unanswered ends the turn without a model call and says so.
10. What the model is told is fixed text. `load_skill` results are sent unwrapped, so every refusal is built from constants and the skill's validated name — never the directory entry's name or a filesystem error.
11. The directory cannot write the dialog. A skill whose directory entry holds a control character is not listed, with a note. The question shows the directory quoted, and its text is made inert on both screens.
12. Every outcome is recorded under a kind of its own: the question, the outcome and each refused repeat, with the skill's name. Not as a gate decision: a trust answer is not the approval of a call.
13. The `load_skill` description and the comment that calls it ungated are brought in line.

## What differs here

- The list of skills is rebuilt at `/clear` and nowhere else: there is no skills reload, `/mcp reload` re-checks the pins without rebuilding the list, and a `/clear` that clears in place because no new session could be opened rebuilds nothing. A disable answer is forgotten only where the list is rebuilt.
- A disabled skill stays in the catalog line of the facts message already sent (ADR-0003: that message is part of the conversation). The refusal is what tells the model otherwise. The system prompt, the tool set and the facts message stay byte-identical.
- The model tier (ADR-0032) does not judge a verdict only the operator may answer, so item 2 excludes it by the rule it already follows.
- There is no child agent, no telemetry and no rule learner; the items above name only what this runtime has.

## Consequences

- "Compare before consuming" holds for skills at the moment of use. After a `git pull` that touches a skill, the first use asks once, in the session, and the operator answers with one key.
- A read of a project skill costs one digest of its directory before and, after a trust answer, two more (bound: 2,000 files of 1 MiB). To be measured on the largest installed skill before the implementation is called done.
- This is not a one-dialog change. The gate's contract gains a question with a three-way result in every implementation; both screens gain a dialog; `/skill` stops being a text expansion and becomes a step of the turn; the pin's digest moves onto a root. The design review put it at the size of the read-only lift and the credential retry together. It was chosen with that known: what it buys is that a changed skill is never loaded unconfirmed and never needs another terminal.
- The question, its two labels, the hint and the notices go in the catalog in both languages. The refusals are model-facing and fixed.
- ADR-0011 §5 stands: `/skill` spends no model round.

## Tests the design requires

- One test enumerates the read sites (`load_skill` body, `load_skill` `file`, `/skill`) and fails if a site reads without comparing.
- One test holds the digest taken at use equal to the pin's digest for an unchanged skill, over the cases the pin distinguishes (links, a file past 1 MiB, more files than the cap).
- One test drives a changed skill, with pins on and recorded, to the question through the production wiring; a wire never connected looks the same as "equal".
- Each gate implementation has a test of the three outcomes it can return, including Esc, an interrupt, a blank line and a closed input.
- The three outcomes and the unattended refusal each have a test of what is recorded, what the model receives, what stays disabled and what the exclusion set holds afterwards.
- The window of item 4 is tested by changing the directory between the question and the answer, twice, and between the pin and the read.
- A directory entry with a control character is tested as not listed.

## Accepted, and written down

- The operator's own skill of the same name stays shadowed after disable, until `/clear`. ADR-0011 §6 brings it back at list time; doing it mid-turn means replacing the list under a running turn.
- The digest has limits: files past the 2,000th are not covered, a link is covered by its target string, and a file's content past 1 MiB by its size only.
- Scripts. If a script changes after the skill was loaded, the shell runs what is there, under the lane and approval rules of every command.
- Other readers. The read tools and an `@` reference can read a skill's files as data, wrapped. A body loaded before the change stays in the conversation.
- A lost record. When the policy file no longer holds the project's pins, a trust answer records a set holding that one name.
- `/clear` still leaves a changed file out and names it; instruction files have no later read to guard.
- A skill that keeps changing is refused after one more question. The operator asks for it again once the pull or the build has finished.

## Alternatives considered

- Refuse without asking, and let the operator re-trust from another terminal. Fail-closed and smaller, but it sends the operator out of the session to answer a question they can answer in it.
- The ordinary approval dialog with a reason line (the first draft). It offers "allow for this session" and "never ask again", which here would each act as trust plus a standing entry nothing shows; and an interrupted turn answers it no, which would disable a skill nobody decided to disable. The design review found both.
- `/skill` tells the model to call `load_skill` (the first draft). It spends a model round ADR-0011 §5 decided not to spend, and if the model does not make the call nothing is loaded and nothing is said.
- Take a piped line as the answer. ADR-0045 makes a pipe the operator's input, and the stdin gate would read the next line; the pin check at startup already treats the same run as one with nobody to ask.
- Keep the body read at list time. Supporting files and scripts are still read later, so the skill would be half old and half new.
