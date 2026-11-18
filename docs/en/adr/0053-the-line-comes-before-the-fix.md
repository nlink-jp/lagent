# ADR-0053: the line comes before the fix — what was built outside it since v0.27.4 is taken out

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-18) — implemented |
| Date | 2026-11-18 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The operator, reviewing gem-agent's fixes since v0.100.0 and their ports here: the fixes were asked for in answer to proposals that stated no threat and no frequency, so the requests themselves are to be judged again. gem-agent decided it in its ADR-0123; this is the port |
| Amends | ADR-0052 (§1 extended; §3, §4, §5 and the "Not changed" list of §6 withdrawn) |
| Supersedes | ADR-0049, ADR-0050, ADR-0051 |
| Relates to | gem-agent ADR-0123 (the same decision, with its reasoning in full), ADR-0011 §5–§6 (skills and trust), ADR-0024 (outside text is made inert), ADR-0042, ADR-0043, ADR-0044, ADR-0046, ADR-0047 |

## Context

ADR-0052 §1 drew the line: the sandbox is held against a command the model ran by mistake or was talked into running, not against a program written to race the runtime. What the operator had asked for was kept because it had been asked for, though each request answered a proposal that said how to fix something and not who could make it happen, how often, or what they would gain.

The code is the code gem-agent has, ported, and the reading of it is the same. The origins are: **A** the model led by text it read (held against); **B** a repository's content before it was trusted (held against, by the trust record); **C** the operator's own acts (correctness, not attack); **D** a program that races the runtime (not held against, ADR-0052 §1); **E** malware running as the operator (not held against). gem-agent ADR-0123 sets out each change against them.

**A correction.** ADR-0051 §8 said a link under the scratch probe's fixed name had `x` written through it. The probe's command runs in the read lane, so the write lands only where the read lane may already write.

## Decision

1. **A finding is judged before anything is built: under what conditions and how often it happens; who makes it happen; what they already have; what it newly gives them.** A fix is proposed with those four answers, "do not fix" among the options. A request that answered a proposal without them is judged again on them.

2. **A project skill is no longer compared when it is used** (ADR-0049 superseded). Origin C, and B through a pull: the upstream author already writes the project's code; the changed text reaches the model until the next `/clear` or start, which compare it; the commands it leads to still go through the lanes and the approval. Taken out: the compare at use, the question and its outcomes, the disable state, `tools.NeedsOperator` and the agent's handling of it, the question dialog's heading and opening choice, the plain prompt's word answer, `/skill` as a step of the turn, the skill-trust records, and the rule that a skill whose entry or resolved path holds a control or format character is not listed (the terminal streams and the TUI make text inert at their ingress, ADR-0024).

3. **A skill's digest is taken by the walk v0.27.4 shipped**, with its cap of 2,000 files — met by a file about to be added, as v0.29.0 fixed it, not by the next entry of any kind; the bounds on entries and depth and the handle per level are taken out. Compared on the ten trees gem-agent ADR-0123 lists, every pin and note is the same before and after — and the same digests gem-agent gives — except a skill nested 40 deep: v0.28.0–v0.30.0 cut it at 32 levels, and its whole tree is digested now, so a pin taken of it then asks once at the next start.

4. **Opens are the standard library's again** (ADR-0050 superseded), as in v0.27.4 — the trace file and the rulebook included. Taken out: the `bounded` open helpers and the test of the opens. A FIFO at a name makes an open wait until interrupted; git carries none. Kept: the file tools' own open, which does not wait, and the file-read child opening only the roots a call names (ADR-0047).

5. **The startup checks are v0.29.0's again** (ADR-0051 superseded, ADR-0052 §3 withdrawn): probes made exclusively under random names, the control the same command outside the sandbox, removal by path, the scratch probe under its fixed name, no time bound. The clipboard capture's file is a random name in the temporary directory again.

6. **The trust record on disk is the record.** The fallback that kept the session's pins when the policy file no longer held the project's, and `adoptPolicy` that kept them through the session's writes (both from v0.27.4), are taken out: the policy file is out of every lane's reach. A file that is gone or does not parse still leaves the session's copy as the record.

7. **Kept:** ADR-0042 (the approved cache out of the read lane's reach), ADR-0043 (a `!` command's output is data), ADR-0044 (the dialog carries the whole call), ADR-0046 and ADR-0047 (added directories), ADR-0052 §2 (the lanes' anchors resolved once), and v0.27.4's comparison of pins with the record on disk.

## Consequences

- No question about a changed skill; one changed during a session loads as it is now until `/clear` or the next start, which leave it out and name it.
- A FIFO at a name the runtime opens makes it wait until interrupted.
- README and README.ja, the manual and the architecture reference in both languages, and AGENTS.md change with the code.

**Accepted, and written down:** a skill changed during a session reaches the model until `/clear` or a start (C, or B through a pull); a FIFO at a name makes an open wait (an approved command, or outside the lanes); a probe replaced while a check runs (D); a skill tree past the file cap in depth or entries makes the start slow (B); a skill with more than 2,000 files, or a `SKILL.md` over 1 MiB, loads as before v0.28.0, and a change past the first 2,000 files, or past a file's first megabyte with its size kept, is not seen at the next start (B); a policy file that lost the project's pins loads its files as unpinned (the operator, or outside the lanes).

## Tests the design requires

- The digest of each kind of tree a pin distinguishes is the same by the walk taken back as by the walk taken out (compared once).
- The tests that held what is taken out go with it; `make check` passes.

## Alternatives considered

As in gem-agent ADR-0123: keeping the question without its surroundings, refusing at use without asking, keeping the look before the project's files are opened, and releasing first — each rejected for the reasons given there.
