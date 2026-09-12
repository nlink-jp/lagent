# ADR-0015: Credential paths are operator-only for the read tools too

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-13 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | System risk review of 2026-09-13, finding R02 (high): the shell lanes deny the credential list at the kernel and the write tools refuse it, but `read_file .env` inside the project was Safe — the value went to the model and into the transcript as an ordinary tool result |
| Relates to | ADR-0001 (a deliberate local change; gem-agent makes the same change independently, in the same shape), gem-agent ADR-0073 (the lanes: only the operator lane reads credentials), gem-agent ADR-0072 §4 (an operator-only Review), ADR-0008 (a skip names the route), ADR-0010 (no model tier: the operator is the only judge) |
| Amended by | as built, 2026-09-13: the residue under Consequences is accepted, not scheduled — the operator's decision; closing it would reopen the rule-growth gem-agent ADR-0076 measured and withdrew |

## Context

The runtime keeps one credential list, in `internal/sandbox`: the
home-relative directories (`.ssh`, `.aws`, `.config/gcloud`,
`.config/mcp-bridge`, …), the home-relative files (`.netrc`,
`.git-credentials`, …) and the names that are secrets wherever they
sit (`.env` and its variants, `id_rsa` and the other private keys,
`credentials.json`, service-account files), with one re-allow for the
committed templates `.env.example`, `.env.sample`, `.env.template` and
`.env.dist`. Until this record it was enforced at three points:

- the Seatbelt profiles: the read lane and the write lane deny
  `file-read*` on every entry; the operator lane does not, because
  reading credentials is the operator's (gem-agent ADR-0073);
- the rule tier, for `write_file` and `edit_file`: a credential path
  is Block;
- the shell Block floor: a command that names one asks the operator
  in every lane.

The read tools consulted none of it. `read_file`, `search_files`,
`view_image` and `file_info` check the roots — the project and the
session work directory, through `os.Root` — and nothing else, so
`read_file .env` on a project's own `.env` was "read-only tool: Safe":
it ran unasked under every mode, and the file's content became a tool
result, which is to say part of the model's prompt and a line of the
transcript. `search_files API_KEY` returned the matching line of the
same file. The review's chapter 10 states the confusion that let this
stand: the write tools' Block for credential paths reads as "the file
tools refuse credential paths", and it must not be read that way.

What the runtime already has for the answer:

- **An operator-only Review** (gem-agent ADR-0072 §4) is a verdict no
  standing answer may give: the session allowlist, a `"never"` policy
  row and a `--allow` grant are refused it, `--auto` escalates it, and
  one-shot (`-p`) denies it because there is nobody to ask. The write
  tools use it for the files later sessions trust; the shell tool uses
  it for the operator lane. `Agent.gated` already routes a floor
  verdict on a non-mutating call to the gate (a `sudo` declared in the
  read lane is the precedent), so the machinery needs no new path.
- **The real path.** The write tools' verdict is taken on the real path
  the tool will open, so a link named `notes.md` pointing at `AGENTS.md`
  is an `AGENTS.md` write. A link named `notes.txt` pointing at `.env`
  is the same hole for a read.
- **Every skip is reported** (gem-agent ADR-0052): the enumeration
  tools already count what they leave out — ignored directories,
  oversize files, capped listings — and say so in a bracketed line
  with the way to see more.

`@` attachments are the operator's own typing and reach the model
without any gate; they are not the model's reach and are not in
question here. `load_skill` reads files inside a discovered skill
directory unasked (ADR-0011) and is not on the list either: a skill
directory is the operator's own placement, pinned by content, and a
credential file found there was put there by the operator. Nor is the
content of files the list does not name: a token inside `config.yaml`
is read like any other file. The list is a finite set of names shared
by every enforcer, not a detector.

## Decision

1. **A single-file read tool on a credential path is an operator-only
   Review.** `read_file`, `view_image` and `file_info` (its batch form
   path by path — one credential among twenty is a credential read)
   whose path, resolved to the real path the tool will open, matches
   `sandbox.CredentialPath` are Review with `OperatorOnly`: must-prompt
   in every mode. The session allowlist, a `"never"` row and `--allow`
   do not answer it; `--auto` escalates it; `-p` denies it. The
   template re-allow applies as in the lanes: `.env.example` is an
   ordinary file. This mirrors the lanes exactly — the read lane and
   the write lane cannot read the list, the operator lane can, and the
   operator lane is the one verdict only the operator approves.
2. **The enumeration tools skip and say so; they never prompt.**
   `search_files`, `list_files` and `list_tree` leave out an entry —
   file or directory, judged by its project-relative path — that the
   list names, and report the count and up to five names in the shape
   of every other skip, with the route: `read_file` on one asks the
   operator. A search that silently found nothing in `.env` would be a
   false negative; a listing that hid `.env` without saying so would
   be a false "not here"; a prompt per credential file in a walk would
   be a dozen questions the model did not ask.
3. **One list, one decision point.** `internal/sandbox` keeps the list;
   the profile, the write tools' Block, the shell floor, the read
   tools' Review and the enumeration skip all read it, and a new
   credential location is added there and nowhere else. The verdict is
   taken in `Agent.decide` through `risk.Classify`, the single decision
   point `internal/archtest` pins; `risk.PathJudged` names the tools
   whose path arguments `decide` resolves to real paths first, so the
   read tools join the write tools on that list rather than growing a
   second one.
4. **`@` attachments stay as they are.** The operator typed the path;
   the operator is the gate.

## Consequences

- Through the file tools, a project's `.env` reaches the model only on
  the operator's yes, each time, interactively; the residue below names
  the two shell routes that remain. An unattended run is refused it, as it is
  refused the write lane — a one-shot workflow that needs `.env`
  content has no route by design, the ceiling ADR-0010 states for every
  Review call. The one-shot denial on stderr names the reason; the
  model receives the standing unattended denial.
- `search_files` results never carry a line from a credential file; the
  footer says how many files were skipped and names them. `list_tree`
  and `list_files` say what they left out; a directory holding only
  credential files lists as its footer, not as "empty".
- `file_info` on a credential file asks although it returns no content:
  a hash is an oracle for a guessed secret, and the rule is about the
  path the tool opens, not what it returns — one rule, no per-tool
  exception to reason about.
- The tool descriptions say it — a route (ADR-0008), so the model
  expects the question rather than retrying through the shell, where
  the floor would ask again.
- The rule tier gains the read tools' path check and `PathJudged`;
  `Agent.decide` resolves `path` and `paths` for every path-judged
  tool; `internal/tools` gains the skip and its footer. No config key,
  no settings row: a credential read with a toggle is a bypass.
- Not a DLP: a secret in a file the list does not name is read like any
  file, as chapter 10 of the review says. Unchanged.
- **Residue, recorded and accepted** (independent review of this
  record, measured under `sandbox-exec`; the operator's decision,
  2026-09-13). Closing either route means growing rules over a
  directory that mixes secrets and ordinary settings — the class
  gem-agent ADR-0076 measured and withdrew, and the knowledge base
  records as "do not split such a directory with a deny" — so neither
  is scheduled; both stay named here so that nobody reads the file
  tools' gate as covering them. (1) The read lane can
  read `~/.config/lagent/config.toml` — and with it an `[llm].api_key`
  kept in the file rather than the environment — and the transcripts
  under the state root, so an approved `read_file .env` of one session
  is a line a later session's unasked `cat` can return; neither path
  is on the credential list. Closing it means denying the config path
  and the `sessions/` and `memory/` subtrees under the state root in
  the read and write lanes while the work directory under the same
  root stays writable, and deciding whether a key may live in the file
  at all. (2) The write lane denies reading a
  credential name but not renaming it: `mv .env notes.txt && cat
  notes.txt` in one approved write-lane command reads it, and
  `read_file notes.txt` afterwards is an ordinary read; the text floor
  catches a literal `.env`, not `.en?`. Closing it is a `file-write*`
  deny on the credential names in the write lane, which also stops
  `cp .env.example .env` there. Keep secrets out of the working copy
  and off the state root's machine account, as chapter 10 of the
  review recommends; that is the control for both.

## Alternatives considered

- **Block, as the write tools do.** Rejected: Block and an operator-only
  Review are the same floor at the gate, but they say different things
  in the record and on the prompt. Writing a credential file is never
  what the model should do; reading one is sometimes exactly what the
  operator wants — the operator lane exists for it — and the verdict
  the operator lane carries is the honest name for the question.
- **Refuse outright, no prompt.** Rejected: the operator lane's rule is
  that the operator decides; a read tool that could never be approved
  would send the model to the shell for a read the floor then asks
  about anyway — two prompts for one question.
- **Skip in the read tools too** (`read_file` returns "skipped").
  Rejected for the same reason: a read the operator would have approved
  becomes impossible.
- **Prompt in `search_files`.** Rejected: a search is a walk; one
  prompt per credential file is many questions the model did not ask,
  and the walk should stay unasked and the content unread. The skip
  with a footer keeps both.
- **Judge the spelling, not the real path.** Rejected: the write tools
  already judge the real name (a link named `notes.md` → `AGENTS.md`),
  and a link named `notes.txt` → `.env` is the same hole.
- **Inspect content for secrets.** Rejected: an unbounded domain. The
  list is finite, documented and shared by every enforcer; a detector
  would be a second rule with its own misses.
