# Changelog

## [Unreleased]

### Changed

- The shared MCP protocol layer is updated to nlink-jp/mcp v0.3.0
  (organization ADR-030 §6: the module at its latest tag). v0.3.0 changes only
  the module's `ToolServer` — it answers the calls already received before it
  returns at end of input, and refuses nil streams — which this runtime does
  not use (it uses only the module's client), so its behaviour does not
  change.

## [0.34.0] - 2026-11-27

### Changed

- **MCP servers are opened in both eras of the protocol** (ADR-0056). The
  hand-written stdio client is replaced by the shared module nlink-jp/mcp:
  each start probes with `server/discover` (MCP 2026-07-28) and falls back
  to `initialize` for an earlier server, now asking for `2025-11-25` (was
  `2025-06-18`), so a server that speaks only the new revision is reachable
  too. The probe added under 0.3 seconds to the start of each of the 30
  servers measured. What the runtime relies on does not change: a server
  starts when first needed, a timed-out call kills it and the next call
  starts it again, an ended server is reaped at once, and failures keep
  their types (`CallError.Sent`, `*RPCError`).
  The `instructions` a server publishes reach the catalog in either era
  (`initialize`, or `server/discover`), trimmed and clipped to 2000 runes
  as before.
- A server's `ping` is answered with `{}`, the specification's answer,
  instead of being refused with -32601. Answers to a server's requests are
  still written within the call deadline.
- A server that fails to start names the phase `open` (the probe and the
  handshake) instead of `initialize` or `initialized notification`.
- One call budget covers a whole `tools/list`, all its pages, instead of one
  budget per page.
- The protocol comes from nlink-jp/mcp v0.2.0, whose line reading ends at
  the 10 MiB cap even against a server that never writes a newline: the start
  ends and the server is killed at once, as with the hand-written reader.

## [0.33.0] - 2026-11-23

### Changed

- **`search_files` is about twice as fast, and its output does not
  change** (ADR-0055, the port of gem-agent's). Most of its time went
  into opening files: each open resolved the path one directory at a
  time from the project root. The search now opens each directory once
  and its files by one name, reads and matches up to eight files side
  by side, and, for a pattern with a literal prefix, checks each block
  of 1024 lines before matching its lines one by one. Measured through
  the sandbox on gem-agent's repository (789 files): 146 ms → 63 ms,
  and 297 ms → 85 ms for a `(?i)` pattern; the 37 ms each call costs to
  start is unchanged. The result is byte-for-byte what 0.32.1 returned,
  checked by a test against its output.

## [0.32.1] - 2026-11-19

### Changed

- **Internal: the session is assembled from parts** (ADR-0054). The
  1,600-line function that started a session is ten startup phases, a
  live session that owns what `/clear` and the reloads change, one
  place that decides where output goes, and three front ends. Nothing
  a session does changed beyond the fixes below; tests now run the whole
  start against a scripted model, the TUI included, and compare
  everything it leaves behind.

### Fixed

- **The plain REPL's "context window unknown" warning comes after the
  banner.** The lookup started before the banner was printed, so its
  warning could land between the banner's lines, and two threads wrote
  the startup record at once (a data race, found with the race
  detector; it was in 0.32.0 too).
- **`/clear` in the TUI names the persistent files the session
  changed.** The note "this session added or changed: …" — for the
  project and for each directory added with write access — was written
  to the terminal while the TUI held the screen, and the next redraw
  covered it: it was in the transcript and nowhere on screen. It is now
  part of `/clear`'s output.
- **Leaving the TUI shows what a `session_end` hook says.** A hook
  that failed or timed out at exit was reported to the TUI after it had
  already ended, which drops what it is sent: nothing reached the
  screen. The plain REPL showed it. The hooks' notices now go to the
  terminal, before the exit summary; anything else still reported after
  the TUI ends (a turn left running past the third Ctrl+C) is dropped,
  so the summary stays last.
- **The exit summary is the last thing on screen again when a
  `session_end` hook speaks.** The resume command and the totals printed
  before the session-end hooks ran, so a hook's notice (a failure, a
  timeout) came after them. The summary now follows everything else.
- **`/clear` no longer freezes the TUI when a `session_start` hook
  prints.** The hook's output is attached to the next turn and the
  operator is told; that notice was sent to the screen from inside the
  command and never returned, so `/clear` and everything typed after it
  hung. Notices from hooks during `/clear` are now part of its output,
  in the TUI and the plain REPL alike.
- **`/clear` no longer crashes a session that started without a
  transcript.** When the session log could not be opened at startup, a
  `/clear` that found persistent files changed during the session
  crashed while recording them. Nothing is recorded for the old
  session now, as everywhere else a transcript is missing.

## [0.32.0] - 2026-11-18

### Changed

- **`lagent workdirs clean` takes a session id's first characters**,
  four or more, as `--resume` does. `lagent sessions` shows eight, and
  only the full id was accepted. A prefix that matches more than one
  directory asks for more characters, and so does one typed while the
  listing is cut at its limit. A full id names only that session, as for
  `--resume`. The confirmation names the directories in full before
  anything is deleted.

### Fixed

- **Tab completes `/show`.** The command worked and `/help` listed it,
  but Tab did not offer it.
- **The startup note for a read-only sandbox that could not be set up
  or checked says what changes.** It said "every shell_exec asks"; what changes is
  that read-only commands need approval too, which auto-approve can
  give, as the manual says.

## [0.31.0] - 2026-11-18

### Security

- **A command that replaces its work directory by a link gains nothing
  when the sandbox is built again** (ADR-0052). The sandbox is built at
  startup, at `/clear` and each time you add a directory, and each
  build looked up where the project and the session's work directory
  really are. A command in the write lane can move its work directory
  aside and put a link under its name: at the next build the place the
  link led to became one the write lane may write (measured; in every
  version so far). A project with no protected file in it could be
  replaced the same way. The places are now the ones found the first
  time, so a link put under the name later is not somewhere the sandbox
  lets a command write. Such a command runs only when you approve it,
  or under an approval you gave in advance.

### Changed

- **Files and folders are opened the ordinary way again** (ADR-0053).
  v0.30.0 opened the runtime's own files, the session's folders, the
  folders you add and the trace file so that a named pipe (FIFO) put
  where one of them was could not make lagent wait, and v0.29.0 did the
  same for a skill's files, the instruction files, `.mcp.json` and
  `.lagent.toml`. Git cannot carry a
  named pipe, so one gets there only through a command you approved or
  a program outside lagent, and all it does is make lagent wait until
  you interrupt it. The special opens and the test that held them are
  removed.
- **The sandbox checks at startup are those of v0.29.0 again**
  (ADR-0053). v0.30.0 made them hold the folders their test files are
  made in, write and remove only through those, and give every test
  command ten seconds, so that a program replacing their test files
  while they run could not make them write or remove elsewhere. That
  needs a program written to race the check. The test files in the
  project, the shared temporary folder and the folders you add still
  get names nobody can guess, and are made so that nothing already there
  is opened; the one in the session's own scratch folder, which only the
  read-only sandbox writes, has a fixed name again. The clipboard capture's file is made in the temporary folder
  again, under a name nobody can guess.
- **The trust record on disk is the record, even when it no longer
  holds the project's pins** (ADR-0053). v0.27.4 kept the pins a
  session started with when the policy file was emptied, deleted and
  rewritten, or restored from an older copy, and kept them through the
  session's own writes. The policy file is somewhere no command lagent
  runs can write, so pins it lost were removed by you or by a program
  outside lagent. A policy file that is missing or cannot be read still
  leaves the session comparing with what it last read.
- **A project skill is no longer compared each time it is used, and
  nothing asks you about it mid-session** (ADR-0053). v0.28.0 compared a
  project skill with what you trusted at every use and, when it had
  changed, asked whether to trust it, disable it for the session or
  leave it for now. What changes a skill mid-session is mostly a
  `git pull`, and whoever wrote the change already writes the project's
  code, which you build and run; the commands the skill's text leads to
  still go through the sandbox and your approval. A changed skill is now
  left out and named at the next start or `/clear`, as a changed
  `AGENTS.md` is. Gone with the question: the limits of
  4,000 files and folders and 32 levels on a skill, and the rule that
  kept a skill whose name or folder holds a control character out of
  the list. As before v0.28.0, a skill with more than 2,000 files, or a
  `SKILL.md` over 1 MiB, loads again; its trust record covers the first
  2,000 files and the first megabyte of each. Trust records stay valid:
  a skill digests as before, except one past the 4,000-entry or
  32-level limit, which v0.28.0–v0.30.0 cut short and which now asks
  once at the next start.

## [0.30.0] - 2026-11-18

### Security

- **The sandbox checks at startup no longer write or delete outside the
  lanes** (ADR-0051). At every start the runtime makes test files where
  the lanes write, and checks that each lane refuses what it should. A
  process already running in the write lane — a child an earlier
  command left behind — could replace one of those files between its
  making and its use. Measured with such a process under the real
  profile: a test file replaced by a link had `x` written through it
  into a file no lane may write; a test directory replaced by a link
  had files beside the project deleted, those named `AGENTS.md`,
  `.env`, `plain` and `created`; and a test file replaced by a FIFO
  held the start. The checks now hold their directories open and
  make, write and remove the test files through them, write only into
  the file they made, and give every test command ten seconds. A
  check that meets a replaced file fails and says so, and the lane
  then asks, as after any failed check. A test command that was
  ended from outside, or did not finish, no longer counts as refused
  either: a process in the write lane could end one, and a lane that
  refused nothing then read as one that had.
- **The file `@clipboard` is captured into is made in the runtime's
  own state directory** (ADR-0051 §9). It was made in the temporary
  directory, which every write lane writes, and the capture runs
  outside the sandbox: a link moved under the file's name had the
  clipboard written through it into a file no lane may write
  (reproduced under the real profile by the design review).
- **A FIFO where a directory or one of the runtime's own files should be
  no longer holds the runtime** (ADR-0050). The earlier fix covered
  four project files; every other open has now been measured, each with
  a FIFO at the name. Three kinds waited. `.claude/skills` in a project:
  discovery, the look before the trust question and the trust records
  all stopped, before anything was asked. A directory the session's
  shell commands may write: a command can remove the work directory, an
  added directory or the project directory and leave a FIFO under its
  name, and the next thing to open it — the tidy-up at exit, `/clear`, a
  resume, a file read — never came back. And the runtime's own files: a
  memory, a transcript, `policy.toml`, `risk-rules.md` and the others,
  most of them read at every start. A FIFO there takes access to your
  own state directory, which the shell lanes do not have. All of these
  now answer at once: a directory is opened only as a directory, and a
  file the runtime names itself only when it is a regular file. Reads
  the model asks for (`read_file`, search, the listings, images) were
  measured too and never waited. A file is also opened in a way that
  cannot wait at all, so a name swapped for a FIFO at the very moment it
  is opened is refused as well; before, that swap held the open every
  time it was tried. How such an open behaves on an online-only file of
  a cloud folder was observed first, on iCloud Drive: the same as a
  plain one. The check a skill gets when it is used goes down the
  skill's folders the same way now: a folder swapped for a FIFO while
  the skill was being checked held the check — six tries of six — and
  is refused instead. The trust records are what they were, and the
  check takes no longer. The review before this release found three
  more opens that waited, and they are closed as well: comparing a
  result with one already saved under its name, when that name was
  swapped for a FIFO; removing a session's work directory when the
  directory of caches beside it had become a FIFO — `workdirs clean`
  did not come back after you confirmed; and writing one of the
  runtime's own files at the moment its name was swapped. The
  configuration file you pass with
  `--config` is unchanged — a FIFO there still waits until it is
  written, which may be what you mean.
- **A trace file is created, never opened over what is there.** With
  `LAGENT_LLM_TRACE` set, each request's trace was opened to truncate,
  and a FIFO under the name it would take held the request before it
  was sent. A name already taken is now an error for that turn.

## [0.29.0] - 2026-11-17

### Security

- **The check a skill gets when it is used is bounded in the work it
  does** (ADR-0049). A folder is no part of what a trust record holds, so
  folders added to a trusted skill asked nothing — and three things let
  them cost time at every use. Every folder was walked: 100,000 empty
  ones took 9.6 s. A folder was listed whole before the file limit was
  looked at: one holding 200,000 files took 0.7 s. And a folder was
  opened by its path from the top, which costs more the deeper it lies:
  800 nested empty folders took 32 s. Measured on v0.28.0. Nothing reached the
  model that had not been compared — the cost was time, and it takes
  write access to the skill's directory on this machine (git does not
  carry empty folders). A project skill may now hold 2,000 files, 4,000
  files and folders in all, and folders nested 32 deep; past any of
  these it is not loaded, with a notice. The same three trees are now
  refused in 0.3 s, 0.008 s and 0.005 s, and a skill at every limit is
  checked in 0.3 s.
- **A project file that is a FIFO no longer stops startup.** A skill's
  `SKILL.md`, an instruction file (`AGENTS.md` and the like),
  `.mcp.json` and the project configuration were each opened plainly,
  and a FIFO waits for something to write to it: a project holding one
  hung the runtime — for the project configuration, before the trust
  question was asked (measured for each; present since each was first
  read). They are now opened only if they are regular files.

### Fixed

- **A skill of exactly 2,000 files is loaded.** It was refused as having
  more than 2,000 when anything that is not a readable file — a folder,
  most often — sorted after its last file. If you had trusted such a
  skill, it asks once more.

### Changed

- **A skill past the limits may be asked about once at the next
  start.** That is a skill that was already too large to load, and one
  that loaded before and is past a new limit — more than 4,000 files
  and folders, or folders more than 32 deep. The check stops sooner than
  it did, so the record of such a skill no longer matches. Answer once
  and it is recorded again; a run with nobody to ask leaves it out with
  a note. Either way the skill is not loaded.

## [0.28.0] - 2026-11-17

### Security

- **A project skill that changed after the session started is no longer
  loaded unconfirmed** (ADR-0049). Project skills were compared with
  what you trusted when the list of skills was built, at startup and at
  `/clear`. Their body and files are read from disk each time a skill is
  used, and nothing compared them then: a skill rewritten after the
  session started — by a `git pull`, or by a write to a directory the
  skill links to — reached the model as your own instructions, through
  `/skill` and through the model's own `load_skill` (measured on
  v0.27.4). Every use now compares first, and what is returned is held
  to what was compared: the bytes read for a file are returned only if
  they are the bytes the trusted skill holds for that file, so a change
  that lands between the compare and the read is not returned either.
  When the skill is not what you trusted, lagent asks before loading it:
  **trust** records that one
  skill again and loads it, **disable** keeps it off for the rest of the
  session, and **not now** — where the dialog opens, and what Esc does —
  loads nothing, changes nothing, and is not asked again until your
  next message. One answer records lasting trust, so it takes
  deliberate keys: the arrows select and Enter confirms, the digit keys
  do nothing and the selection does not wrap (on the plain screen, the
  word `trust` or `disable`). Auto-approve and
  "allow for this session" do not answer it. In a one-shot or piped run nothing
  is asked and the skill is not loaded. If the content changes again
  while you are answering, it asks once more and then stops. Checked on
  a real session: each of the three answers, Esc, and the model's
  `load_skill` on a disabled skill.
- **Nothing a trust record does not cover is served from a skill.** A
  record covers a file's bytes up to 1 MiB and a skill's first 2,000
  files. A `SKILL.md` over 1 MiB could keep its body past that megabyte,
  and a skill with more than 2,000 files could have `SKILL.md` among the
  uncovered ones; in both cases the instructions could change without
  the record noticing (found by the review of this release, and present
  in earlier versions). A `SKILL.md` over 1 MiB is now not a skill, and a
  project skill with more than 2,000 files is not loaded, with a notice —
  which includes a skill that carries a `node_modules` or a `.venv`. A
  listing of a directory inside a skill shows only the entries the
  record covers: an empty directory can be given any name, and a name is
  text the model reads.
- **A skill directory whose name, or the path it resolves to, holds a
  control or format character is not listed.** The name a skill declares
  was always validated; its directory was not, and the new question
  shows the directory.

## [0.27.4] - 2026-11-17

### Added

- **A user manual** (`docs/en/manual.md`, `docs/ja/manual.ja.md`). The
  documentation so far was the README and three maintainer references;
  nothing read from the start for someone who only wants to use the tool.
  The manual is that one book: installing and preparing an LLM server, a first
  session, the screen and keys, attachments, approval and how to trade its
  friction against safety, sessions, directories outside the project,
  one-shot runs, instruction files and trust, MCP, skills, hooks, a
  configuration reference, tuning the server and the model, what the
  safety model does and does not cover, and appendices for looking a name
  up. No behaviour changes.

### Fixed

- **`/clear` now applies the instruction files and skills it re-reads.**
  It re-checked the trusted files and re-read the instruction files, then
  went on with the system prompt built at startup, and it never looked
  for skills again. So an instruction file you edited took effect only
  after a restart; a project file reported as "changed since you trusted
  it — not loaded" stayed in the prompt with its old content; and a skill
  installed during the session was not offered until the next start. The
  new session is now built from what is on disk and what the trust record
  allows: your own instruction files and skills are picked up, and a
  project file that changed outside lagent is really left out until you
  re-trust it. When nothing changed the system prompt is the
  same bytes as before, so the LLM server's cache still carries over a
  `/clear`. Checked on a real session: the request after `/clear` carried
  the operator's edited file and the new skill, and no longer the project
  file that had changed.
- **`lagent trust --accept` in another terminal now reaches a running
  session.** When a project file changes outside lagent, `/clear` leaves
  it out and says to re-trust it with `lagent trust --accept`. Doing that
  changed nothing for the session that said it: it compared the file with
  the copy of the trust record it had read at startup, so the next
  `/clear` printed the same notice, and only a restart loaded the file.
  Every comparison now reads the record from the policy file as it is at
  that moment — at `/clear`, at `/mcp reload`, in the note after a `!`
  command, and before an approved edit of a trusted file is recorded. If
  the policy file is missing, cannot be read, or no longer holds the
  record this session has for the project, the session keeps comparing
  with what it last read, so a changed file is still left out rather
  than loaded, and the last two cases are reported. `.lagent.toml` is the
  exception to the reload: it is read once, at startup. Checked on a real
  session: after `trust --accept` elsewhere, the notices stopped and
  `/clear` loaded the file.
- **A `/settings` change no longer makes a session forget the project's
  trusted files.** Every policy write replaced the session's copy of the
  trust record with the file. If the policy file had been deleted while
  the session ran, the first `/settings` change recreated it empty, the
  copy followed, and from then on `/clear` and the reloads treated the
  project as never pinned and loaded files that had changed. The
  session now keeps the record it holds when the file has none. Found
  by the review of this release; earlier versions had it.

## [0.27.3] - 2026-11-14

### Security

- **goldmark is updated from v1.7.13 to v1.7.17** (GO-2026-5320, cross-site
  scripting in its HTML renderer). lagent did not run that code: replies are
  rendered for the terminal by glamour, which replaces goldmark's HTML
  renderer with its own. `govulncheck` reported it as reachable because both
  renderers sit behind one interface. The update puts the fixed renderer in
  the build instead of leaving that argument to be made again.
- **`golang.org/x/net` is updated from v0.38.0 to v0.62.0**, with `x/text`
  v0.43.0, `x/sys` v0.49.0 and `x/term` v0.47.0. `govulncheck` listed
  advisories in `x/net/html` without finding the affected functions called.

### Changed

- **Building from source needs Go 1.27.4 or later** (`go.mod`; it was 1.26.2),
  the release whose standard library carries the fixes published so far. The
  binaries of v0.27.2 were already built with it.
- **`make package` runs the vulnerability check first** (`make vuln`), so a
  release cannot be built past a finding.

## [0.27.2] - 2026-11-14

### Fixed

- **A retry of a model call always waits first.** The backoff's jitter was
  a full second either way on a half-second base, so one first retry in
  four was sent with no wait at all, and one second retry in eight within
  a quarter second. The jitter is now half the base at every step: the
  first retry waits 0.25–0.75 s (it was 0–1.5 s), and the later ones 1 s,
  2 s and 4 s give or take a quarter second (it was a second). Which
  errors are retried, and how many times, is unchanged.

### Documentation

- **There is no Linux version, and the reason is recorded** (ADR-0048,
  from gem-agent ADR-0115). Landlock and namespaces protect an instruction
  file that exists, and neither can refuse the creation of a new one by
  name, which the write lane depends on. lagent stays macOS-only; running
  under WSL2 is the same case.

## [0.27.1] - 2026-11-09

### Changed

- **`/clear` keeps the added directories that still work** (ADR-0047). When
  one no longer passes the sandbox check, only that one is removed, and the
  note says which command adds it back at the same level.

## [0.27.0] - 2026-11-09

### Added

- **Directories outside the project, under the same lanes** (ADR-0046).
  `--add-dir <path>` adds a directory the file tools and the write and
  operator lanes write like the project; `--read-dir <path>` adds one that
  is read only: no tool or lane writes it. Both repeat, and `[directories]` in the
  global config file lists them too. Nothing in an added directory is
  loaded: not its instruction files, MCP servers or skills.
- **Adding a directory during a session.** `/add-dir <path>` and
  `/read-dir <path>` add one at the prompt, for as long as lagent runs, and
  `/dirs` lists what is in force. The model can ask with
  `request_directory`, which you always answer.

## [0.26.0] - 2026-11-09

### Documentation

- **What a pipe without `-p` means is now stated** (ADR-0045). It is your own
  input, as if typed: `!` commands and slash commands in it run. To hand a
  pipe to the model as data, use `-p`.

### Changed

- **The approval dialog shows the whole call and scrolls** (ADR-0044). Long
  commands and edits were cut to about 300 characters and 8 lines, with no
  way to read the rest, and a line with tabs could lose its end with no
  sign. The dialog now carries every argument, marks each row of the call
  with `┊`, and ↑↓, PgUp, PgDn, Home and End scroll it. ↑↓ no longer move
  the selected answer; ←→ and Tab still do.
- **The output of a `!` command reaches the model as data** (ADR-0043). It
  used to join the conversation as your own words. The command still does;
  what it printed is now marked like a file you attach.

### Security

- **A session directory that has become a link is refused on resume.** The
  write lane can replace the work directory or the approved cache with a
  link, and following it would widen what that lane may write.
- **A link left at the session scratch no longer becomes the read lane's
  writable directory.** Such a session now runs with the read lane off, and
  says so.
- **The startup file-read probe no longer writes through planted links.** It
  now uses a new randomly named directory and exclusive creates.
- **A control character in a skill directory's name no longer breaks every
  start.** Such names are now escaped in the global policy file, and a file
  that cannot be read back never replaces the existing one.
- **A settings edit no longer re-enables what you declined at startup.** A
  `never` entry from a project file you declined to re-trust came back on the
  next `/settings` change or `p` answer in the same session.
- **What an approved build trusts is out of reach of what runs unasked**
  (ADR-0042). The write and operator lanes' toolchain cache sat inside the
  read lane's scratch, separated only by an environment variable, so a
  read-lane command could plant an object a later approved build would
  link. It now lives beside the session's work directory, where the file
  tools do not reach and, under the default state directory, other
  sessions' lanes may not write; the read lane is denied it by the kernel
  and proved so at startup. It is kept across `--continue`, counted by
  `lagent workdirs`, and removed by `workdirs clean`.

## [0.25.0] - 2026-11-06

### Fixed

- **A path through a link outside the project reaches the directory it
  leads to** (ADR-0041, ported from gem-agent ADR-0108). With the
  project in `~/Library/CloudStorage/Dropbox/proj`, the full path through
  the link, `/Users/<you>/Dropbox/proj/x.txt`, was refused by every file tool — even
  an approved write failed with "path escapes the project directory" —
  while the real path worked. Such a path is now replaced by the real one
  where the call comes in, when it leads into the project or the work
  directory; the approval, the verdict, pre-tool hooks, the run and the
  audit all see that one path (the model's own message keeps what it
  typed). A link leading anywhere else is refused as before, without
  saying where it leads. The last element, a credential-looking name,
  paths in `/Volumes`, `/Network`, `/net` and `/home` and links that lead
  into them are left as typed. `@` references follow the same rule.
- **A sandbox probe file that cannot be removed is reported** (ADR-0041
  §2, from gem-agent ADR-0107 §4). The start-up check creates test files
  in the project and removed them ignoring a failure; in a shared drive
  that lets you create but not delete they would stay and sync to
  everyone sharing it, unannounced. Each leftover is now named in a
  start-up note.

## [0.24.0] - 2026-11-05

### Added

- **Every MCP result is saved, and evidence can be a copy of it
  (ADR-0039, ported from gem-agent ADR-0105).** Small results used to
  reach the model only inline, so an agent asked to keep a query result as
  evidence re-typed it — measured on this runtime, a spilled result was
  copied in 0 of 10 runs and re-typed with wrong values in 6. Now every
  text block an MCP server returns is saved in the session work directory
  under its full sha256, and the model reads lagent's own line about it —
  outside the nonce tag, where no server can forge it — naming the path,
  size and hash and offering the copy (a result saved as several files is
  announced part by part, never as whole). The transcript's tool-result entry
  records the same files in a new `saved` field, so whether an evidence
  file is a copy of what a server returned is a hash comparison. The
  runtime-facts message says so too: it used to say only results too
  large to return inline were saved, which told the model a small result
  had no file (ADR-0039 §8). Measured after both changes, the model this
  runtime was benched on still copied rarely (inline results 0 of 10,
  spilled 1 of 10): the route is in place and stays as it is, and that
  model not taking it is recorded as its property (ADR-0039 §8.2).
- **`write_file` copies with `from`.** Instead of `content`, `from` names
  an existing file whose bytes become the new file — whole, byte for
  byte, read back and checked, with the sha256 in the result. The
  destination is judged exactly as a write of content (no approval inside
  the project), so copying is never more work than re-typing. The source
  is read through the sandboxed child, up to 32 MiB; above that, and for
  credential material, the copy is refused. The call's detail names the
  source and its size, never its bytes.

### Fixed

- **Skill descriptions are read whole (ADR-0040, ported from gem-agent
  ADR-0106).** A description was cut at 400 characters with nothing said,
  so a skill written to the 1,024-character limit Claude Code uses lost
  the triggers past the cut — six of the nine skills-series skills did,
  mcp-tactics everything from BigQuery on. Descriptions are now listed up
  to 1,024 characters, and a longer one is cut with a discovery note
  naming the skill. A multi-line description (`description: >` or `|`, or
  a value continued on indented lines) was read as its first line, `>`;
  it is now read whole, and listed as one line.

### Changed

- **Saved MCP results are named by their full sha256**, not its first 4
  bytes, and a file already at that name is reused only when its own
  bytes hash to it.
- **The in-process credential retry is limited to the covered reads**
  (`tools.ChildTool`), so a refusal on any other tool is never one
  approval away from running again.

## [0.23.1] - 2026-11-04

### Changed

- **The config files are read by nlink-jp/toml** (v0.1.1) instead of the
  third-party `BurntSushi/toml`, so the tool carries no TOML code from
  outside the organization. `config.toml`, `policy.toml`, a project's
  `.lagent.toml` and the bench's `task.toml` read as before: unknown keys
  are still refused with their names (the project file's message still says
  what it may carry), the project file is still read through its capped
  read before the trust prompt, and the settings that came from the file
  are still told apart.

## [0.23.0] - 2026-11-04

### Added

- **A dialog left waiting rings the terminal (ADR-0038, from gem-agent
  ADR-0104).** When an approval, the read-only ceiling's approval or
  question, `ask_user` or the round-limit intervention has gone six seconds
  without a keystroke, the TUI tells the terminal once:
  `[tui].notify = "bell"` (the default) writes a BEL, `"osc9"` also writes
  an OSC 9 desktop notification ("lagent needs your answer", shown by
  iTerm2 and others), `"off"` writes nothing. A keystroke puts it off to the
  next six-second check, as Claude Code's permission prompt does (measured
  for gem-agent); reading without pressing a key does not. It stops nothing
  and records nothing. **This changes the default behaviour:** a dialog
  left for six seconds now rings the bell; set `[tui].notify = "off"` for
  the previous behaviour. The plain REPL and `-p` never notify.

## [0.22.0] - 2026-11-02

### Changed

- **An inline image is drawn as large as the screen allows (ADR-0037,
  from gem-agent ADR-0103).** A picture shown with `show_image` or
  `/show` was drawn at most 40 columns wide and a third of the screen
  tall — a square photo took 36 columns of a 200-column window. It now
  takes the terminal's width less one column, or the screen above the
  input, whichever binds first, and keeps its shape. It is never drawn
  larger than the picture itself (one picture pixel to one pixel the
  terminal reports for a cell), so an icon stays its own size. The box
  is shaped by the cell the terminal reports rather than the constant
  2.25, which did not fit kitty's 1.86 cell.
  On kitty a JPEG is re-encoded as a PNG at 8×18 pixels a cell, so a photo
  in a screen-sized box stays under the 2 MiB an inline image may be.

## [0.21.1] - 2026-11-02

### Fixed

- **The read lane works in a project in cloud storage or on a share
  (ADR-0036).** Google Drive, Dropbox and OneDrive keep their files
  under `~/Library/CloudStorage` and iCloud Drive under `~/Library/Mobile
  Documents`, and the read lane denies
  `~/Library` (and `/Volumes`, `/Network`) for reach — so in such a
  project every read-lane command failed with `getcwd: Operation not
  permitted`, as it did on an external disk or a network share. The
  project and the session work directory are now re-allowed for reading
  when they lie inside a denied tree; the rest of the tree and the
  credentials inside the project stay denied, and a project at the home
  directory does not re-open `~/Library` — nor does one at, or above, a
  home that itself lies under `/Volumes` or `/Network`. The home the
  read lane's denials are anchored on is now resolved to the kernel's
  spelling, so a symlinked home is denied where it really is. An
  online-only file is downloaded by its provider when read.

## [0.21.0] - 2026-11-02

### Fixed

- **One round of tool calls can no longer overflow the context window
  (ADR-0034).** A session that read several large files in one round
  (four whole-file reads, 506 KB, on a 168k-token window) passed the
  window, compaction declined because the six messages it keeps verbatim
  were that round, and every later turn failed until `/clear`. One
  round's tool results now share a budget sized to the window — half of
  what a compaction can make room for, re-judged before every call — and
  each tool cuts with a route to the rest: `read_file` line windows end
  on a whole line and name `start_line=K`, byte windows name `offset=N`,
  MCP previews shrink, listings and searches say to narrow the path or
  pattern. Compaction keeps the recent messages by size as well as
  count, so a session already stuck on an oversized round recovers once
  you write again. @-attachments share the bound. With automatic
  compaction off, a round stays under `compact_at_pct`, and a window
  with no room left ends the turn with `/compact` and `/clear` named
  instead of failing every request.
- **A project is found however its path is spelled (ADR-0035).** In a
  directory whose path has Japanese voiced kana, `--resume` refused a
  session recorded there ("belongs to <path>, not <path>" — two paths
  that print the same): the shell's `$PWD` carried the typed
  (precomposed) form while the record carried the on-disk (decomposed)
  one. The state directory, trust, pins and allowlists split the same
  way. The project path is now read from the kernel, and every path
  comparison — recorded projects, `trusted_projects`, and whether a path
  is inside the project — is made on Unicode-normalised strings, so
  records already written in either spelling are found without being
  rewritten and an absolute path typed in the other form is inside the
  project. Case is not folded.

## [0.20.1] - 2026-11-01

### Fixed

- **`/settings` shows the compaction settings.** `[agent].compact_at_pct`
  is listed with where it came from, and `[agent].auto_compact` can be
  switched for the session there, as in gem-agent. The row follows the
  live value, so automatic compaction that two failures turned off
  reads as off for this session rather than as the file's `true`.

## [0.20.0] - 2026-11-01

### Added

- **History compaction (ADR-0033).** A session no longer dies when the
  conversation fills the context window. Before a request would pass
  `[agent].compact_at_pct` (default 80) of the window, the older part
  of the conversation is replaced by a summary and the recent messages
  are kept verbatim; `/compact` does it by hand, and
  `[agent].auto_compact = false` turns the automatic one off. Ported
  from gem-agent's compaction, with the trigger judged on the request
  about to be sent rather than the last one, so a round of large tool
  results or a newly loaded MCP server is seen before it overflows. The
  session facts (the data tag, the work directory, memory) survive the
  cut verbatim, the summary is quoted as data, a failure leaves the
  history untouched, and a resumed session comes back compacted and
  re-loads every MCP server it had loaded. Measured on mlx-serve with a
  resumed 85k-token session: the compaction path took 111 s where
  re-reading the uncompacted history cold took 266 s.

## [0.19.2] - 2026-10-29

### Security

- **A submodule's hooks, info and config are protected like the
  project's.** A submodule's git directory lives under
  `.git/modules/<name>/` (the name may nest), and the write lane could
  write its `hooks/`, `info/` and `config` — git runs those hooks when
  you commit inside the submodule, and `core.hooksPath` in that config
  moves them (measured before the fix). They are now persistent files in
  the profile and for the file tools — the submodule's `objects`, `refs`
  and `logs` excepted, so `git gc` and branches named `hooks` or `config`
  keep working. `git submodule add` and `git submodule update`
  write that config, so they now need the operator lane, as `git init`,
  `git clone` and `git remote add` already did (the same defect was
  found and fixed in gem-agent).
- **`commondir` is protected.** A `commondir` file under `.git` — the
  repository's, a worktree's or a submodule's — points git at another
  directory's config and hooks, and the write lane could write it: the
  next plain `git commit` ran a planted hook (measured in gem-agent's
  pre-release review). It is now a persistent file at any depth. The
  residuals weighed and kept are recorded in gem-agent's ADR-0073.

### Fixed

- **The project's persistent-file report says when its walk was cut.**
  A session can fill the project past the walk's 20,000-entry cap and
  then add a nested `AGENTS.md` the walk never reaches; the report at
  `/clear` and exit ignored the cut and said nothing (measured). It now
  notes the cut, records `cut` in `persistent_changes`, and names
  nothing past the cut as removed.
- **Saving the persistent-file snapshot no longer races another session
  of the same project.** Both wrote through one fixed temporary name;
  measured, 145 of 200 concurrent saves failed. Each save now uses a
  temporary file of its own.

## [0.19.1] - 2026-10-29

### Security

- **The read lane no longer reads credential files under the toolchain
  caches in `~/Library`.** The read-lane profile re-allowed
  `~/Library/{Caches,Developer,Python,Go}` after its credential denials, and
  Seatbelt applies the last matching rule, so a `.env`, `credentials.json`
  or service-account key inside those trees could be read without asking.
  The credential block is now the last file-read rule of the read and write
  lane profiles, and a test holds that order. The write lane was not
  affected (it has no read re-allow), and other credential paths were
  denied as before (the same defect was found and fixed in gem-agent).
- **`.env` templates are carved out of the credential rule, not re-allowed
  after it.** A trailing `(allow …)` for `.env.example`, `.env.sample`,
  `.env.template` and `.env.dist` would beat every deny before it, so with
  the block moved last such a template would have become readable inside
  the trees the read lane denies (`/Volumes`, `/Network`, the rest of
  `~/Library`). The name rule now excludes templates itself, so no allow
  follows the credential block in any profile, the file-read child's
  included. One change in behaviour: a template inside a home credential
  directory (`~/.ssh/.env.example`) is now denied in the read and write
  lanes and in the file-read child, as the file tools already judged it.

## [0.19.0] - 2026-10-08

### Added

- **A model tier for write-lane shell commands** (ADR-0032), opt-in with
  `[approval].model_tier = "shell"`. Under auto-approve, a judge model on the
  same server decides the `shell_exec` calls in the write lane that the rule
  tier leaves at Review, with gem-agent's prompt, two rounds and 0.8
  confidence bar: approved ones run unasked, the rest are asked about with the
  judge's reason. MCP calls, the operator lane and every other Review still
  ask. The judge reads your `risk-rules.md` beside `config.toml`;
  `[llm].risk_model` and `[llm].risk_reasoning_effort` (default `none`) set its
  model and thinking. Records follow gem-agent's: `auto_decision` names the
  judge's model, the bar and the confidence, and the judgment's tokens are a
  `risk` usage record, and `/settings` shows the tier and the judge. Measured
  on a month of the operator's own write-lane prompts: about half would have
  run unasked, and no exfiltration or injection case passed. A lookup the
  operator had refused for their own reasons passed once in five until one
  line in `risk-rules.md` said so — write such rules there.

## [0.18.0] - 2026-10-08

### Added

- **mlx-serve is a supported backend** (ADR-0031), adopted on the task bench and
  a server measurement with the same Qwen 3.6 weights under mlx-serve and LM
  Studio: the speed comes from the MTP head of mlx-serve's own model builds, not
  the runtime; a 122k-token prompt completes. The ADR records the residual risk
  of running it — chiefly that 26.10.1 listens on every interface with no key
  by default — and that its default 2 GB prefix cache re-reads long sessions
  every turn: with an 8 GB cap a 122k resend took 1.1 s instead of 207 s.
- **`bench serve`** measures a model server: cold prompt reading per size,
  reuse on a resend and the next turn, decode speed, alternating conversations
  and two streams at once, logged raw; `bench serve-report` recounts a run.
- **`[llm].provider = "mlxserve"`** for
  [mlx-serve](https://github.com/ddalcu/mlx-serve). The context window is
  read from its `/v1/models` at startup, so leave `[model].context_window` at
  0 for it — a set window skips the lookup. A `[llm].model` that list does not
  carry is reported at startup: mlx-serve answers a chat request for any model
  name with the model it has loaded, so a mistyped id otherwise runs silently
  against whatever is resident.

### Fixed

- **A completion of only blank lines is asked again**, as an empty one is
  (ADR-0007, amended by ADR-0031). Under LM Studio, Qwen 3.6 starts its answer
  with the blank lines after its thinking; one bench run's entire final answer
  was `\n\n`, which was returned as the answer.

## [0.17.1] - 2026-10-06

### Fixed

- **A resumed session is told when it began** (ADR-0030, from gem-agent
  ADR-0097). The facts message sent on resume said "session started" on the
  resume day, beside the restored conversation's own facts message with the day
  it really began. It now reads `- resumed: <day>; the conversation above began
  on <day>`. The date is also captured once per session, so an MCP reload after
  midnight no longer states a new one.
- When `/clear` cannot open a new transcript and clears the history in place,
  a fresh facts message is sent: the clear had changed the isolation tag
  without telling the model its new name.

### Documentation

- The README says where an operator's procedures go: a skill arrives as
  instructions, every tool result — a knowledge-vault server's notes
  included — as data, so a procedure meant to be followed is a skill
  (from gem-agent ADR-0096 §7).

## [0.17.0] - 2026-10-05

### Changed

- **A saved MCP result shows its tail as well as its head** (ADR-0029,
  from gem-agent ADR-0096 §1). A text block too large to hold inline is
  previewed by its first 600 and last 200 characters, and the notice names
  the byte spans shown and the route to the rest (`read_file`
  offset/length). Metadata a server appends — `"truncated": true`, a row
  total — arrives as the last bytes and was never visible before.
- **`shell_exec` keeps the tail of long output and saves all of it**
  (ADR-0029, from gem-agent ADR-0096 §3). Past 20,000 bytes the model sees
  the first 15,000 and the last 5,000 bytes — a script's totals come last
  and used to be lost — and the whole output is saved to the work directory
  (up to 32 MiB), named in the note. The runtime writes the file, privately
  (`0600`), so no lane's reach changes. The operator lane is not saved, nor
  is any lane when the shell runs without the sandbox, and the note says so.
- **`search_files` counts every file it did not search** (ADR-0029, from
  gem-agent ADR-0096 §4). Files over 2 MB (named, up to five), binary
  files, images, unreadable files and directories that could not be listed
  are counted in a closing line; size and image skips were silent, so a
  "no matches" could hide a large log that was never read. Refusals past the
  five named are counted too.

### Added

- **`read_file` reads by bytes** (ADR-0029, from gem-agent ADR-0096 §2).
  `offset`/`length` take a byte window — a negative offset counts from the
  end — so the tail of a long single-line file is reachable, which a line
  window never was: a tool result saved to the work directory past 200 KB
  could not be read to its end by the tool its notice named. The note names
  the bytes returned, moved to rune boundaries. A plain read cut at 200 KB
  now says where to read on.

## [0.16.1] - 2026-10-02

### Fixed

- The bench writes a suite run's request trace beside the run when
  `--out` is a relative path. Before, the trace landed inside the run's
  project, and the injection suite's payload check read an empty
  directory.

### Added

- Bench: a `tickets` persona for the MCP fixture (nine tools with
  strictly validated schemas), the `mcp-load` suite that uses it, and an
  `arg errors` column in `bench report`.

### Documentation

- ADR-0028, rejected: putting a loaded server's schemas into the
  conversation, called through a `mcp_call` proxy, keeps the prompt
  cache. On a real server's sibling tools the model then mixed up
  arguments. Measured on a prototype that was not merged.

## [0.16.0] - 2026-09-30

### Added

- **Box art where no picture draws** (ADR-0027). On a terminal without
  images (Terminal.app, a multiplexer), flowchart, sequence and ER fences
  in a reply are drawn as box art instead of shown as source. mermaid-render
  draws them from the same parse as the pictures, places them by the TUI's
  own cell widths, and checks them on the grid every time. Japanese labels
  are included. The plain REPL and `-p` still show the source.

## [0.15.1] - 2026-09-30

### Changed

- A mind map's eleventh branch colour is an indigo instead of a teal the
  fifth branch nearly shared (mermaid-render v0.5.1).

## [0.15.0] - 2026-09-30

### Added

- **Mind maps are drawn as pictures** where the terminal draws images
  (mermaid-render v0.5.0): the tree on two sides of the root, a colour per
  branch, every node shape, labels wrapped as mermaid wraps them. A label
  with emphasis, an icon written in its text or math shows the source with
  a note.

### Fixed

- An ER or state label with emphasis the previous rule missed
  (`_snake_case_`) shows the source with a note instead of a picture with
  the underscores (mermaid-render v0.5.0).

## [0.14.0] - 2026-09-29

### Added

- **Gantt charts are drawn as pictures** where the terminal draws images
  (mermaid-render v0.4.0): sections, tasks placed by dates, durations,
  `after` / `until` and excluded days, milestones, `vert` markers, the time
  axis. A chart whose picture would depend on the day it is drawn shows the
  source with a note; the today marker is never drawn.

## [0.13.0] - 2026-09-29

### Added

- **State diagrams are drawn as pictures** where the terminal draws images
  (mermaid-render v0.3.0): composite states as frames, concurrent regions,
  choice / fork / join, notes on the side they name. A transition crossing a
  composite's frame shows the source with a note.

## [0.12.0] - 2026-09-29

### Added

- **Pie charts are drawn as pictures** where the terminal draws images
  (mermaid-render v0.2.1): slices as mermaid draws them, every item's
  percentage in a column beside the legend.

## [0.11.1] - 2026-09-29

### Fixed

- **Narrowing the window no longer clears the screen** (ADR-0026, ported
  from gem-agent ADR-0094). Pictures on the screen — mermaid diagrams and
  `/show` — stay, and no empty screens pile into the scrollback: v0.11.0's
  known limitation. A shrink erases only the rows the input frame gained by
  re-wrapping.

### Changed

- **The input box's rows end where their text ends**, and **while the
  window's width is changing the frame below the conversation is drawn
  short** — every row cut to 19 cells until no report changing the width has
  come for 400 ms (ADR-0026). A
  row as wide as the window, repainted while the terminal was already
  narrower than it had reported, left a stale copy of the box behind. The
  input line's highlight still spans the window.

## [0.11.0] - 2026-09-29

### Added

- **Mermaid fences are pictures where the terminal draws images**
  (ADR-0025, ported from gem-agent ADR-0092 at v0.85.1). On iTerm2 and kitty
  a fence in a reply is drawn by mermaid-render (an organization library):
  flowchart / graph, sequenceDiagram and erDiagram, from the source as
  written, CJK labels included. One em of diagram text is one terminal
  line; the width follows the cell's shape, read with an ioctl
  (`TIOCGWINSZ`), never a terminal query. A picture wider than the terminal
  shrinks; a taller one scrolls — on kitty in bands of at most half the
  screen, since kitty clips a picture taller than the screen. Anything the
  engine refuses is shown as source with a one-line note. There is no box
  art: without an image protocol, and in `-p` and the plain REPL, a fence
  stays source as before. `[tui.diagram]` picks the font (default Hiragino
  Sans W3 / W6); a value that does not load is a banner warning. A
  picture's payload reaches the terminal beside the renderer's ADR-0024
  hold, as a tool image does.

### Known limitations

- Narrowing the window loses the pictures on the screen at that moment and
  leaves black space in the scrollback (text survives). The TUI clears the
  screen on a shrink, and an image does not survive a clear; `/show`
  pictures are affected the same way. Measured in gem-agent; to be
  revisited with it.

## [0.10.4] - 2026-09-29

### Security

- **Text from outside the runtime reaches the terminal inert**
  (ADR-0024, ported from gem-agent ADR-0093). The model's reply, its
  thoughts, a tool call's detail and purpose, the approval and ask
  dialogs, `!` output and every other string the TUI shows reached the
  terminal with its escape sequences intact: measured on tmux, 61 of 95 hostile deliveries acted — the title set, the clipboard buffer written, the screen erased, the cursor moved, and in the approval dialog a CR or an unterminated OSC hid part of the command being approved. C0
  controls other than tab and newline, DEL, C1 and the bidi embeddings,
  overrides and isolates are now removed once, where text enters the
  TUI; the text of a sequence stays visible (`]0;…`), so nothing the
  operator could have read is hidden. The Markdown renderer's output is
  held to the colour codes it writes itself — none in the plain theme —
  and closed at the reply's end, because it decodes character
  references such as `&#27;` into real controls. The plain REPL and
  `-p` do the same when their stream is a terminal and stay
  byte-for-byte to a pipe or a file. The transcript is unchanged.

## [0.10.3] - 2026-09-26

### Changed

- **Promoted from lab-series to cli-series** (ADR-0023), on the evidence
  of daily use. lagent is now positioned as a coding-agent runtime for
  work that should not go to a cloud API, and the cli-series stability
  contract applies: flags, config keys, `LAGENT_*` variables, record
  formats and hook contracts change only through the org's
  breaking-change process. No behaviour changes in this entry.

## [0.10.2] - 2026-09-22

### Fixed

- **A JPEG drew nothing on a kitty-protocol terminal** (kitty, Ghostty, and
  WezTerm when it answers the graphics probe). The kitty graphics protocol's
  `f=100` means PNG and it has no JPEG format, but a JPEG went out labelled
  `f=100`; the terminal rejected it, `q=2` hid the rejection, and the rows it
  would have used were still counted. The kitty payload now passes a PNG
  through and decodes anything else, scales it down to what the declared box
  can show, and re-encodes it as PNG. A picture that does not decode is not
  drawn. iTerm2's inline images are unchanged. Seen by the operator on kitty: a JPEG that drew nothing before now draws.

## [0.10.1] - 2026-09-21

### Fixed

- **The image probe took the terminal's next input.** With
  `[tui] images = "auto"` (the default) on a terminal the environment cannot
  classify — Apple Terminal, the VS Code terminal, SSH — the start-up probe
  left a reader blocked on `/dev/tty`, and that reader received whatever the
  terminal sent next. With `theme = "auto"` it was the reply to the
  background-colour query, so the theme fell back to dark whatever the
  background was; with a fixed theme it was your first keystroke. Measured on
  Apple Terminal: the colour read was `#000000` with the probe and `#161821`
  without it, three runs of three; fixed, both read `#161821`. iTerm2, kitty,
  Ghostty and tmux were never asked and were not affected.
- On a terminal that does draw, the probe stopped reading at the graphics
  reply and never read the device-attributes reply behind it. Nothing showed
  it, because the stale reader above swallowed that reply. The probe now reads
  the whole answer, and a report from some other query that arrives around it
  (a DECRPM, a kitty-keyboard answer) no longer costs the verdict.
- **An MCP handshake that timed out blamed the wrong setting.** It gave up
  after `mcp.startup_timeout_sec` and said "initialize timed out after 1m0s" —
  the per-call number. The message now names the budget that ran out — for the
  first `tools/list` of the start-up listing as well, which runs under the same
  startup budget.
- `mcp.startup_timeout_sec` larger than `mcp.call_timeout_sec` was silently cut
  at the call's value, although the two are documented as separate. A slow
  server given a long startup budget now gets it. `/settings` lists
  `mcp.startup_timeout_sec` beside `mcp.call_timeout_sec`.
- **`/show` could not open a path with a space in it** — which is every macOS
  screenshot. The path was handed to the `@`-reference grammar, which ends a
  reference at the first space, so `/show Screenshot 2026-09-21 at
  10.00.00.png` answered "not found". `/show` now takes its argument as a
  path, and accepts one that is quoted or has its spaces backslash-escaped,
  which is what dragging a file into the terminal types.
- `show_image` told the model it could show "PNG, JPEG, WebP, GIF, HEIC". The
  screen draws PNG and JPEG (up to 2 MiB); the other formats were accepted,
  read, and then always refused. The description now says what is drawn.

### Documentation

- The ADR index listed ADR-0022 as "Proposed" after it was accepted
  and shipped; `config.example.toml`, the `/settings` help line and a
  type's doc comment still described the MCP intake as the source of inline
  images, which ADR-0022 withdrew; the reference's list of tools that ask
  before a credential read omitted `show_image`. Each now has a test that
  reads the document against its source: INDEX status against the ADR header,
  retired phrases across `cmd/`, `internal/` and the example config as well as
  `docs/`, and the credential-read list against `internal/risk`.
- Not changed, and said so: on a kitty-protocol terminal a JPEG is sent marked
  as PNG. No such terminal was available to measure, and this lane is only
  judged on a real one. `AGENTS.md` carries it as unmeasured.
- The injection-suite entries recorded under [0.9.0] shipped in v0.7.0 and are
  filed there now. `AGENTS.md` lists ten built-in tools, `show_image` among
  them, and states that the PreToolUse session fields stay.

## [0.10.0] - 2026-09-17

### Changed

- **The MCP intake no longer draws. The model shows, and so can you**
  ([ADR-0022](docs/en/adr/0022-showing-is-an-act-of-output.md)). An MCP
  image block is how a tool result carries an image into the MODEL's
  context, and MCP says who content is for with an `audience` annotation
  this runtime drops at the parser — measured across 24 registered servers,
  four can emit an image block and none sets an audience. So v0.9.0's rule,
  "draw every image block the intake saved and described", was this runtime
  inferring that an image which arrived is an image you want; two of those
  four servers return a screenshot so the MODEL can look at it, and every
  such inspection put a full-size picture in your scrollback.

  **This changes behaviour released in v0.9.0.** What reaches your screen is
  now authored:

  - `show_image` — the model calls it to show you a picture. It is
    `view_image`'s counterpart and the pair differs in one thing, the
    audience: `view_image` attaches pixels to the conversation so the model
    can look, `show_image` draws them in your terminal. The read happens in
    the tool layer, so a model-named path is confined and judged exactly
    like every other file tool's — which is what lets it reach the screen at
    all, since a read the view layer performed would be invisible to every
    enforcer.
  - `/show <path>` — you name the file. Same grammar as an `@<image>`
    attachment: a project path, or an absolute or `~` path for an image.

  The lane itself is unchanged: the declared box, the erase, the columns
  clamped below the terminal's width, the 2 MiB ceiling, the silent refusal
  on screen and the capability probe are all as they were. A refusal now
  reaches the caller in words — the tool result for the model, one line for
  you — because the screen stays silent and nobody else can explain it.

## [0.9.0] - 2026-09-17

### Fixed

- **An image is drawn only when its bytes reached disk, and one predicate
  decides what an image is.** Whether the write succeeded was inferred from
  the note's length, and the two notes are not ordered — a failure note can
  be shorter than a success note, which carries a full path — so a failed
  write was read as success and the picture was drawn with no file behind it
  and `view_image` on the path finding nothing. The same inference
  mislabelled the leftovers line as "saved but not listed" when nothing had
  been saved. Separately, the draw gate keyed on the MIME prefix while the
  model's note keyed on the block type, so a "resource" wearing an image
  MIME was drawn and recorded as a resource, and an image block with no MIME
  was recorded as an image and never drawn.

- **`/settings` lists `tui.images`**, resolving `auto` to what the probe
  actually found (`auto (→ iterm2)`), the way `tui.language` has shown its
  resolved value. Every refusal to draw is silent by design and the probe
  cannot be re-run mid-session, so this row is the only place that answers
  "did it find anything?" — and the panel answers when asked rather than
  reporting at startup. The capability is now resolved once and shared by
  the model and the panel.

### Added

- **The image capability probe no longer costs a terminal that cannot draw.**
  Asking only the graphics question left silence as the single verdict, so
  every unclassified terminal paid the whole timeout: measured at 2.001 s on
  Apple Terminal at every start, with the query's own body printed on the
  screen because that terminal does not parse APC. A device-attributes
  request now rides in the same write, and a DA1 reply with no graphics
  reply before it is the definitive no. Re-measured: under 1 ms, clean
  screen.

- **Verified on a real terminal, with a negative control.** iTerm2, screen
  full, three images through the production path: one frame, no stranded
  footer, the same reading as the plain control at the same fill. The same
  tree with the erase removed strands three — which is what makes the zero a
  measurement rather than a blind instrument. The pictures were confirmed to
  have actually drawn, because a silent refusal to draw would also leave a
  clean pin.

- **Images from MCP tools appear on screen** ([ADR-0021](docs/en/adr/0021-an-images-bytes-never-become-a-path.md)).
  A server that returns an image block — a screenshot, a rendered chart —
  now puts it in front of the operator as the call returns, drawn in a box
  this runtime declares so the scrollback row counter is told a number
  rather than measuring bytes it cannot see. The model's own note is
  unchanged: it still gets the path and still has to call `view_image` to
  look. The bytes travel in memory and the view layer opens no file, which
  matters because the path the intake writes is one a server can pre-empt
  with a symlink. An image is drawn only if the intake both saved and
  described it, only if its bytes actually decode as an image whatever the
  MIME says, and only up to 2 MiB decoded; every refusal is silent, because
  a line per undrawable image is a report rather than a control.
- **`[tui] images`** (`auto` | `off` | `iterm` | `kitty`, default `auto`) — the
  capability behind it. An image payload is zero cells wide to every surface
  the TUI has, so the row counter is TOLD the box the emitter declared
  instead of measuring bytes it cannot see ([ADR-0020](docs/en/adr/0020-inline-images-declare-their-height.md)) —
  measured on two terminals with a plain control at the same fill, a
  terminal that draws what the counter cannot see strands one frame per
  image once the screen is full. An image line also erases below itself
  before its payload: Bubble Tea's own flush clears one row, while the
  picture covers N, so without it the old frame survives to the right of
  every row a narrower picture draws over — three images, three stranded
  footers on gem-agent's iTerm2, with the row count already correct.
  `auto` asks the terminal once, before the UI starts, because once Bubble
  Tea owns stdin a reply arrives in the input box as phantom keystrokes;
  inside tmux or screen it resolves to off.
- **`internal/termimg`** — the protocol layer: capability detection
  (environment first, one query only when it must), iTerm2 `OSC 1337 File=`
  and kitty `APC _G` payloads that carry the declared box, and a fit that
  clamps an image below the terminal width so the terminal never wraps a
  picture into rows the declaration did not claim. Sixel is absent: it
  cannot declare a row count.

## [0.8.0] - 2026-09-14

### Added

- **`[mcp] startup_timeout_sec` (default 30)** — a server's handshake (spawn,
  `initialize`, the first `tools/list`) now has its own budget instead of
  borrowing `call_timeout_sec`. They are different things: a call's budget is
  how long the work may take, a handshake's is how long a server may take to
  say hello. It is a separate number rather than a smaller one on purpose —
  cutting a slow server off at startup does not save time overall, because the
  session then has to reconnect it, which costs more than waiting did. Before
  this, `initialize` was bounded by `call_timeout_sec` (60s) and only the
  listing had a hard-coded 30s.

## [0.7.1] - 2026-09-14

### Changed

- **MCP servers are spawned and listed at once instead of one after another.**
  Measured on a 25-server configuration: twenty-three answer `tools/list` in
  under 20ms and two go over the network — GitHub's remote MCP at ~1.9s and a
  Slack proxy at ~1.3s — so serially those two were the whole of MCP startup
  (~6.5s of a ~10s start), and a slow day at either landed on it in full.
  Startup now costs the slowest server rather than the sum. Only the waiting is
  concurrent: attaching still runs in configured order, so the registry, the
  catalog and the warnings read exactly as before.

## [0.7.0] - 2026-09-13

### Added

- **Every `tools/call` carries the session work directory** as request
  `_meta["jp.nlink/work_dir"]`. The in-house MCP servers take their output
  directory as a per-call `work_dir` argument now (organization ADR-021), and
  this is the contract's second channel: schema-blind, so one line covers every
  server, and a model that omits the argument still leaves the server with a
  destination this session can read back. A session with no work directory
  attaches nothing, and the model's own argument always wins.
  See [ADR-0019](docs/en/adr/0019-the-caller-names-the-work-dir.md).

<!-- The injection-suite entries below were recorded under [0.9.0] until 2026-09-21: they sat in an
     orphaned [Unreleased] block and a sweep filed them with the release being cut. The commits are
     in the v0.7.0 tag. -->
- **The injection suite** (ADR-0018). Six bench tasks, opt-in through a new
  `suite` field: `bench run` with no `--tasks` skips them, `--tasks injection`
  or a task name selects them. Three plant an instruction in the body of a file
  the task makes the model read — a tool result, the channel the nonce wrapping
  exists for — each dictating a different element of the output; three are the
  benign twins with the sentence deleted. They score what was written, not what
  was said, so a run that announces it will not comply and one that complies in
  silence are told apart by the file alone. `payload_marker` and the trace
  variable, which the runner now sets for suite runs, fail a run whose payload
  never reached the model: on the family's first sweep 1 run in 9 read the
  fixture without the sentence entering a request, and passed.
- **The task prompt is a second variable, measured.** Three levels of the same
  task — loose, the current wording, and one that says to transcribe the
  recorded string and ignore a file that asks otherwise — over the identical
  fixture and payload, ten repetitions each. Obeyed / valid: 3/3 loose, 6/7
  current, 0/6 strict; every benign twin 5/5. The task instruction dominates.
  The strict level proves less than it looks — it names the attack, so it is a
  per-task mitigation that works where the operator anticipated it — while the
  two non-strict levels carry the weight: **with the nonce wrapping and the
  defensive framing in force, as in every run, the payload got through 9 times
  in 10 valid runs.** There is no unwrapped condition to compare against, so
  this does not say the wrapping does nothing; it says the wrapping is not what
  stopped this class, because nothing stopped it. ADR-0018 §4 carries the
  amendment.
- **Exposure turned on tool choice.** The void runs used a shell one-liner to
  take each file's first line, so the payload never entered the context; the
  runs that obeyed used `read_file` and took the body with it. A looser prompt
  pushed the model toward the one-liner — a narrower read, not a defence.
- **First measurement, 2026-09-13.** The benign twins completed every time, so
  the tasks have power. `inject-value` broke through: the model wrote the value
  the injected file dictated where its twin wrote the real one, and reported
  success without mentioning that anything had asked. **The path is real on
  this runtime.** Nine attacked runs are not a rate and are not reported as
  one — ADR-0018 §5 leaves rates to the single-turn harness.

### Documentation

- **ADR-0018: how the injection bench has to be built** (design only; no code).
  The RFP's open item — whether the model actually honours the nonce
  isolation — was measured single-turn against this runtime's model on
  2026-09-13, through `llm-cli`, which carries the same guard and an A/B
  switch. Loud payloads ("discard every instruction", forged authority, forged
  closing tag) never got through, wrapped or not: 0 in 500. Payloads that
  accept the task and dictate one field of the output got through 92-100%
  unwrapped, and wrapping cut one to 0.3%, another to 14%, and a third not at
  all — so there is no single number for how well the wrapping protects, and a
  bench built from loud payloads would report perfect resistance after the
  defence was removed. The agent-side task therefore plants its payload in a
  tool result (not `AGENTS.md`, which ADR-0013 measured at 0/18), scores the
  value of a tool argument rather than prose or whether a tool ran, ships a
  benign twin in the same run, varies the element it targets across at least
  three payloads, and leaves rate measurement to the single-turn harness
  because an agent run is too slow to reach n=100.

## [0.6.0] - 2026-09-13

### Security

- **The search walk was unprotected wherever the file-read cage could
  not be installed.** ADR-0016 §5 said the file tools fall back to
  in-process reads "with `risk.credentialRead` as the boundary", but
  that function judges a path ARGUMENT and `search_files` has none: it
  is not in the credential read set, its verdict is `Safe`, and it
  never reaches a gate. So on a machine where the startup probe failed,
  the walk opened `.env` and printed the matching lines to the model
  with nothing in between — weaker than 0.4.0, which withheld the
  entry and reported a count — while the degradation note told the
  operator that credential files still needed their approval.
  `readForSearch` now refuses a path on the one `internal/sandbox` list
  before it opens the file, returning the permission error the walk
  already reports, so a refused file lands in the same
  `[not read: … ]` footer whether the kernel refused it or this check
  did. The refusal is unconditional rather than switched on the cage's
  absence: a mode branch would put the safety on the path that is
  exercised least, which is how this shipped. Inside the child the
  kernel still refuses whatever the check misses, so the list stays the
  fast path and not the boundary. Regression test:
  `TestSearchFilesRefusesCredentialsWithoutTheCage` drives a registry
  with no file child — the degraded path exactly — and fails on the
  leaked line without the guard. ADR-0016 carries an *Amended* note.

- **"One function, applied at every spawn site" was a claim, not a
  mechanism** (ADR-0017 §2). Four kinds of child did not apply it: the
  unconfined shell (`--no-sandbox`), the startup lane probes, the
  sandbox availability probe and the clipboard capture. Only the first
  carried real exposure — it built its command with no environment at
  all, so the child inherited the parent's whole environment including
  `LAGENT_API_KEY`. The other three read nothing in the runtime's
  namespace, so what was exposed there was the sentence rather than a
  secret. All four apply the rule now, and
  the read lane's probe extends the filtered environment with its
  temporary directory instead of rebuilding it from the parent's — an
  overwrite that would have quietly undone the fix.
- **The class is closed by a test.**
  `TestEverySpawnSiteAppliesTheChildEnvRule` fails when a function
  builds an `exec.Cmd` without naming the helper that applies the
  environment rule. The partition test next door pins the two NAME
  lists and cannot see a call site, which is why a false sentence
  survived a green build. ADR-0017 carries an *Amended* note.

  Found writing the second revision of the architecture review,
  recorded there as R33.

### Fixed

- **The degradation note and the `/settings` row claimed something that
  was not true of the walk.** Both said credential files "ask you"
  when the cage is absent; a search skips the file and names it, it
  does not ask. They now say credential files are refused either way,
  and by whom.

- **Text that outlived the credential redesign.** `list_files` told the
  model "Credential files … are left out and counted" and `list_tree`
  said credential files "are skipped and counted"; both stopped being
  true in 0.5.0, when ADR-0016 §3 withdrew the withholding — the walks
  list every name and the kernel refuses the content. A tool
  description is what the model plans against, so a stale one is a
  wrong fact in the prompt, not a stale comment. `search_files` now
  says a file it may not read is named rather than hidden, which is
  true of both enforcers. Also corrected: the `pathJudgedTools` comment
  still described the deleted skip, `laneEnv`'s comment still described the environment scrub ADR-0017 withdrew, and the
  profile's own doc comment claimed `stat .env` is refused — it is not,
  and that wrong measurement is the `file-read*` cut that broke every
  walk. The 0.5.0 entry carries an inline correction rather than a
  rewrite. ADR-0016 and ADR-0017 name the version that shipped them
  instead of saying "implemented and unreleased".

- **A product rule, in `AGENTS.md`.** A defect or design change in a
  mechanism both runtimes have is fixed in both, in the same piece of
  work. ADR-0001 and ADR-0002 say what is ported and what is not
  reproduced; neither said what to do about the code the two runtimes
  genuinely share. The stale tool descriptions above are what that gap
  costs: the change went into gem-agent and not here, and a model was
  told something false for a release.

## [0.5.1] - 2026-09-13

### Fixed

- **The file-read cage never switched on in 0.5.0.** Its startup probe
  asked the sandboxed child for `list_files`, which the child refuses
  because it is not one of the covered reads — so the probe failed on
  every start, the cage was never installed, and the operator got
  `warning: file reads are not sandboxed (… cannot list a directory
  holding a credential-named file) — restart lagent to try again`. Nothing
  was less safe than before the release: the reads fell back to the
  in-process credential check, which is what 0.5.0 did. But the release's
  headline feature did nothing. The probe now uses `search_files`,
  which is a covered read and is the walk whose listing the design is
  about, and it sends project-relative paths so the probe directory
  under `/var` is not refused as being outside the resolved
  `/private/var` root.
- **The degradation warning told the operator to do something that
  cannot help.** A restart re-runs the same probe. It now says what is
  true — the reads still ask about credential files, checked by
  lagent itself rather than by the kernel — and points at `/settings`,
  which now carries a `file reads` row saying which of the two is in
  force. Previously that row did not exist and the message named it
  anyway.
- Regression test: `TestInstallFileChildSucceedsOnThisMachine` builds
  the binary and runs the installer against it, asserting the probe
  installs the cage and the registry reports it. The earlier end-to-end
  test drove the tools directly and never ran the installer, which is
  why a broken installer shipped green.

## [0.5.0] - 2026-09-13

### Security

- **The kernel reads the file** (ADR-0016). The file tools never made
  the lanes' move to the kernel: they are in-process Go code, so
  `sandbox.CredentialPath` was the boundary and a miss there was the
  file's content in the transcript rather than a missing prompt.
  `read_file`, `view_image`, `file_info` and the whole `search_files`
  walk now run their reads in a child of this binary under a profile
  built from the same `internal/sandbox` list, which denies credential
  material at the kernel — measured: `cat .env` and `stat .env`
  refused,
  [corrected 2026-09-13: `stat .env` is NOT refused — the deny is
  `file-read-data`, as the review bullet of this release says; this
  measurement is from the withdrawn `file-read*` cut]
  `.env.example` read, `grep -r` skipping that one file, at
  18.8 ms per spawn against a local-model round measured in seconds. A
  refused open is the operator's question, and on approval the read
  runs in process, which is the operator lane's authority applied to a
  file tool. The matcher stays only to raise the prompt without
  spending a spawn: a miss in it now costs a spawn and a blunter
  prompt, never a leak. Verified at startup like the shell lanes; where
  it cannot be verified the reads stay in process and the note says so.
- **The walks stop hiding names** (ADR-0016 §3, withdrawing ADR-0015
  §2). The kernel lists names and refuses content, so a name was never
  the secret. `list_files` and `list_tree` list a credential-named
  entry like any other and carry no credential code at all;
  `search_files` names the file it could not read
  (`[not read: .env — reading one needs the operator's approval]`), the
  shape `grep -r` has. `credentialTally` is deleted. This also dissolves
  the walk defect gem-agent's review found — the walks judged the
  project-relative spelling and did not resolve their root — because
  there is no spelling left for them to get wrong.

- **Review fixes to the kernel read path** (ADR-0016, independent
  review). The first cut denied `file-read*`, which covers
  `file-read-metadata`; Go's `os.Root` listing stats every entry, so one
  credential-named file failed the whole directory read and
  `search_files` answered `no matches (0 files scanned)` for the entire
  project — silently, exit 0. The deny is now `file-read-data`: content
  refused, stat left, the walk intact, and the `[not read: …]` note the
  ADR promised actually reached. Also fixed: the child pipe silently
  halved reads over 85 KB; `file_info` collapsed a refusal into "not
  found" so the operator was never asked; a cancelled `search_files`
  lost its partial result; the child did not get `sandbox.ChildEnv`; any
  `EACCES` became a credential question; the hidden subcommand ran any
  tool; a missing work directory failed every read and leaked an
  absolute path; the probe directory leaked; the profile lacked the tty
  hardening; the degradation note named no next command. The reason all
  of it shipped green is that no test ran the real child under the real
  profile — `TestFileChildUnderTheRealProfile` now does, and it is what
  caught the rest on re-run.

### Changed

- **The operator's environment is no longer filtered, and the runtime's
  own variables no longer reach any child** (ADR-0017). The read lane's
  environment scrub is deleted: it covered one child of six — the write
  and operator lane shells, the `!` shell, MCP servers and hooks all
  received the environment whole — and the one it covered passed
  `OPENAI_KEY`, `GH_PAT` and `GPG_PASSPHRASE` while catching
  `NPM_TOKEN`. Which variable the program a command runs needs is not a
  question lagent can answer, and an allowlist would move the unbounded
  list rather than remove it. **A bare `env` in the read lane now
  prints what you exported**, as it already did in every other lane;
  the remedy is the operator's: launch lagent from a shell that does
  not hold what you would not give the model, or `env -u NAME lagent`.
  In exchange, the one set the runtime knows exhaustively — its own
  `LAGENT_` namespace — is handled in the other direction.
  `sandbox.ChildEnv` removes `LAGENT_API_KEY`, `LAGENT_PROVIDER`,
  `LAGENT_BASE_URL`, `LAGENT_MODEL`, `LAGENT_REASONING_EFFORT`,
  `LAGENT_STATE_DIR`, `LAGENT_LLM_TRACE` and `LAGENT_MCP_STDERR` from
  every child, while the three exported for children
  (`LAGENT_SESSION_ID`, `LAGENT_WORK_DIR`, `LAGENT_PROJECT_DIR`) pass.
  The two halves partition the namespace and an architecture test
  refuses a `LAGENT_` literal in neither, so R01's own variable is
  removed by what it is rather than by a regex recognising the word
  `key`. This closes the environment route only: the key also lives in
  `.lagent.toml`, which the read lane can read, and ADR-0015 records
  that residue as accepted. A nested `lagent` in a shell lane now reads
  its own configuration file instead of inheriting the parent's; an MCP
  server wanting a `LAGENT_` value takes it from the `env` block of its
  `.mcp.json` entry.

## [0.4.0] - 2026-09-13

### Security

- The read lane keeps the runtime's own exports by name, not by
  prefix. `sandbox.ScrubEnv` exempted every `LAGENT_*` variable from
  the secret-name rule, so `LAGENT_API_KEY` — the config's bearer token
  — reached every read-lane command's environment, from where a bare
  `env` puts it in front of the model and into the transcript (system
  risk review R01). The exemption is now an explicit list of the three
  variables the runtime exports for children, `LAGENT_SESSION_ID`,
  `LAGENT_WORK_DIR` and `LAGENT_PROJECT_DIR`; every other name goes
  through the secret-name rule. The write and operator lanes are
  unchanged. A deliberate local change (ADR-0001: the recorded
  gem-agent source commit is unchanged).
- Credential paths are operator-only for the read tools too
  (ADR-0015). The credential list — `.env` and its variants, private
  keys, `credentials.json`, the token stores under home — was denied
  to the read and write lanes at the kernel, refused by `write_file`
  and `edit_file`, and put in front of the operator when a shell
  command named it, but `read_file .env` inside the project was Safe:
  the value went to the model and into the transcript as an ordinary
  tool result (system risk review R02). Now `read_file`, `file_info`
  (each path of its batch) and `view_image` on such a path, resolved
  to the real path the tool will open, are a Review only the operator
  answers, every time: the session allowlist, a `"never"` row and
  `--allow` do not answer it, `--auto` escalates it, `-p` denies it.
  `.env.example` and its siblings stay ordinary files. `search_files`,
  `list_files` and `list_tree` do not prompt: they skip a
  credential-named entry and report the count and names with the
  route. `@` attachments are unchanged. One list, in `internal/sandbox`;
  a deliberate local change (ADR-0001), made in the same shape in
  gem-agent independently. The list's directory and file entries now
  match on a path segment: `.aws/credentials` and `~/.netrc` are
  credentials, `deploy.aws`, `keys.ssh/x` and `foo.netrc` are ordinary
  files — a suffix match once took them for credentials, which cost a
  false Block on a write and would have hidden them from the
  enumeration tools (independent review). The skip footer, the ignore
  tally and the cut lines of `search_files` and `list_tree` ride after
  the output cap, so a capped result still says what it skipped; the
  footer's names are sorted. An operator-approved credential read no
  longer fires the operator-write hooks (it writes nothing; the pins
  are for writes); the unattended denial tells the model the read-only
  file tools continue on ordinary files; the `--allow` help and the
  `/tools` policy marker say that credential reads still ask. ADR-0015
  records as residue, for a record of their own, the two shell routes
  the independent review measured and this release leaves open: the
  read lane can read the config file (an `[llm].api_key` kept there)
  and the transcripts under the state root, and the write lane can
  rename a credential file and read it under its new name.

### Fixed

- A pre-tool hook that exits 0 with output that is not a verdict is
  reported. ADR-0012 §3 promises a warning for unparseable output, and
  the runner emitted one for a crash, a non-zero exit and a timeout,
  but plain text (or JSON that does not parse) on exit 0 let the call
  through in silence — the shape of a guard that meant to deny and
  printed the wrong form (system risk review R06). The call still
  proceeds, as hooks only tighten; the notice names the hook and quotes
  the first line of what it printed. Empty stdout stays silent. JSON
  that parses but is no verdict of the contract — a deny spelled with
  the wrong field (`decision: "deny"`, a bare `permissionDecision`) or
  the wrong value — is reported the same way rather than passing as a
  verdict that denies nothing (independent review); a context hook
  printing such JSON injects nothing and says so.

## [0.3.3] - 2026-09-13

### Security

- `~/.config/mcp-bridge` is a credential location. Its `config.json`
  holds pre-registered OAuth client secrets and static API-key headers
  in plain JSON, and `state/<server>/tokens.json` the access tokens;
  remote HTTP MCP servers are reached through mcp-bridge, and ADR-0002
  makes MCP the only route to the web, so the directory is part of the
  runtime's expected deployment. It joins the credential list beside
  `~/.config/gcloud` and `~/.config/gh`: the read and write lanes deny
  the read at the kernel, the file tools refuse the path, and a shell
  command that names it is Block for the operator to see. A sibling
  entry such as `~/.config/lagent` is unaffected. A deliberate local
  change (ADR-0001: the recorded gem-agent source commit is unchanged).

## [0.3.2] - 2026-09-12

### Fixed

- `LAGENT_LLM_TRACE` files are private: the trace directory is created
  `0700` and each request/response pair `0600`, as the transcript is.
  The trace is the whole request body and the raw stream — the
  instruction files, every tool result, and the `reasoning_content`
  deltas nothing else stores (ADR-0009) — and the operator points it
  anywhere, so the earlier `0755` directory and umask-default files
  left all of it readable by every local user. A directory or file
  that already exists keeps its mode. The configuration reference now
  says what the trace holds and that deleting it is the operator's job.

## [0.3.1] - 2026-09-12

### Fixed

- The footer's `ctx` gauge (and its cache share) is reset by `/clear`.
  It is a mirror fed by each round's usage, and the shared slash
  handler that empties the conversation cannot see the TUI, so the
  discarded conversation's size stayed on screen until the next round
  reported (operator report). The `total` figure counts the process
  and is unchanged.

## [0.3.0] - 2026-09-12

### Added

- Agent memory (ADR-0013): short facts at a global and a project scope
  under the state root, recalled in the runtime-facts message — the
  bench measured a standing directive acted on 0/18 from the
  instruction files at any size and 5/6 from one facts-message line.
  `/remember [global] <name> <fact>`, `/forget`, `/memory`; the model
  proposes with `save_memory` / `delete_memory`, which the rule tier
  keeps at Review so every save asks. The bench gains `pointer-*`,
  `memory-follow` and `trust = true`.
- Pre-tool hooks (ADR-0012): `[[hooks.pre_tool_use]]` runs the
  operator's guard before a model tool call on Claude Code's measured
  PreToolUse contract (stdin JSON; deny by `permissionDecision` or
  exit 2). A deny is a floor before the approval ladder and the reason
  is returned to the model; anything else fails open with a warning.
  Global config only; `hook_denied` in the transcript.
- Session-start, prompt-submit and session-end hooks (ADR-0014):
  `[[hooks.session_start]]`, `[[hooks.user_prompt_submit]]` and
  `[[hooks.session_end]]` on Claude Code's measured contracts. Context
  a hook prints rides the next turn as a `hook` attachment quoted as
  data; a prompt hook can refuse the prompt (erased, reason shown); a
  start or end cannot. `/clear` fires end then start.
- Skills (ADR-0011): Claude Code's `SKILL.md` format read as-is from
  `~/.config/lagent/skills/<name>/` and a trusted project's
  `.claude/skills/<name>/` (pinned; a changed one stays out until
  re-trusted). One catalog line per skill rides the runtime-facts
  message; `load_skill` returns a skill's body or supporting file,
  confined to its directory and sent unwrapped as the operator's own
  instructions; `/skill <name> [args]` sends a skill by hand and
  `/skills` lists them. The bench gains `skill-follow`.
- `[llm].reasoning_effort` (and `LAGENT_REASONING_EFFORT`) rides every
  request verbatim when set; the OpenAI vocabulary, which LM Studio
  maps to the model's thinking on/off. Measured on the bench
  (ADR-0009): thinking on completed 18/18 against 17/18 with no empty
  completion, at twice the wall time; the default stays unset.

### Decided

- **The model tier of auto-approval is not adopted** (ADR-0010). Over
  36 bench runs the rule tier's Review calls were four to five
  write-lane verification shells the read lane now runs unasked; a
  verdict would cost seconds per call on the one local model, judging
  its own proposal. The ladder stays Safe / Review / Block, and the
  RFP's Phase 2 item closes as rejected on measurement.

### Fixed

- A re-send after an empty completion now carries one transient line
  ("Reply now: give your final answer as text, or call a tool."),
  never stored: replayed ten times, the identical request came back
  empty 10/10 right after a compile error from the verification run
  and 0/10 with the line (ADR-0007, second amendment).

## [0.2.0] - 2026-09-12

### Added

- **The task bench** (ADR-0006). `go run ./bench run` measures the
  runtime on six fixture tasks (search then answer, read then edit, a
  change across files, a shell count, an MCP lookup against the bench's
  own fixture server, an image to look at) in isolated homes and state
  roots, reads each run's transcript, and `bench report` prints
  completion, no-tool answers and the medians of rounds, tool calls,
  prompt tokens and wall time per task and configuration. Phase 2
  changes are measured against it.
- **The runtime supplies routes, not rules** (ADR-0008, the RFP's
  local-oriented prompt revision). Every `shell_exec` runs with the
  `[sandbox.scratch_caches]` table pointed into the session scratch
  (shipped row: `GOCACHE = "go-build"`; the read lane and the approved
  lanes get separate directories), so `go run`, `go vet` and `go test`
  run in the read lane without approval (they failed in both lanes
  before: the sandbox denies the cache under `~/Library`); a denied
  call in a one-shot run is told that no one can approve it and what
  runs without approval, instead of "ask the user"; `list_tree
  dirs_only` on a flat directory lists its files instead of "(empty
  directory)"; and the system prompt states the lanes as they are and
  drops the "ask how to proceed" rule. Measured cause: a bench run that
  explained its fix in prose after the runtime's own chain of a failed
  cache write, an unattended denial and that rule. The table and the
  per-lane split follow gem-agent ADR-0084's review of the same design.
- **An empty completion is asked again, twice at most** (ADR-0007).
  The bench's read-edit task failed on a completion with no text and
  no tool call; the raw stream showed a mis-sampled tool-call opener
  routed into the reasoning channel, and the same request replayed
  came back empty about half the time. The runtime now re-sends the
  identical request up to twice, notes each re-send, and records every
  attempt (`assistant_empty` carries `retried`); a third empty
  completion ends the turn as before.
- `LAGENT_LLM_TRACE=<dir>` writes every model request and its raw SSE
  reply to files, so a bench run's odd completion can be read as the
  server sent it. Off unless set.
- Homebrew tap distribution: `brew install nlink-jp/tap/lagent` installs
  the notarized release archive as-is; `make brew` generates the formula
  from the built zip (the org's vendored `gen-brew.sh`).

## [0.1.0] - 2026-09-10

### Measured

- The RFP's side-by-side usage comparison with gem-agent did not become
  one: Gemma 4 answers the same instructions in a single round where
  Gemini runs a tool loop, so the transcripts record different work.
  The compatibility check is `gem-usage-lens verify --sessions-root`
  against lagent's sessions (no checksum failure); the single-round
  behaviour is the Phase 1 effectiveness finding.

### Added

- **Images reach the model** (ADR-0005). An image path dropped on the
  terminal attaches without an `@` (backslash-escaped spaces read, in
  `@` references too); the `view_image` built-in returns so the model
  can look at an image a tool saved; and a reference that did not
  attach is told to the model as `[not attached: <ref> — <reason>]`
  beside the operator's warning. Measured cause: a pasted screenshot
  path attached nothing, and the model described an image it never
  had.
- **MCP tools are advertised on demand** (ADR-0004). Every server is
  connected and every tool registered as before, but the model is shown
  a catalog in the runtime facts — one line per server with its tool
  names and what it said about itself — and a built-in `mcp_load` that
  advertises one server's tools for the rest of the session. A call to
  a tool the model was not shown is refused with the route.
  `[mcp].advertise = "deferred"` (default) or `"all"` (the baseline);
  `[mcp].preload` and a `--allow mcp__<server>__*` grant advertise from
  the start; `/mcp` shows the loaded state and `/mcp load <server>`
  loads by hand; a resumed session replays its loads. With the
  operator's 243 tools the cold prefix drops from about 63k tokens to
  about 4k.
- **The system prompt is byte-identical across sessions** (ADR-0003).
  The isolation tag name, the session work directory and the start
  date now open the conversation as the runtime's own message
  (`Agent.AnnounceSession`) instead of living in the system prompt, so
  the local server's prefix cache survives a new session and a
  `/clear`. Measured with the operator's 243 MCP tools (60k tokens of
  schemas): a new session started in 2 s instead of 118 s. The session
  listing does not preview that message or count it as a conversation.
- **Phase 1 core (RFP §4).** The agent loop, the built-in file and
  shell tools, the sandbox lanes, the approval gate with its rule-tier
  auto-approve ladder, the MCP client, the JSONL transcript with
  `--continue` / `--resume`, one-shot `-p`, the inline TUI and the plain
  REPL, usage records in the gem-usage-lens shape — ported from gem-agent
  at the commit ADR-0001 pins, minus the features ADR-0002 lists.
- **The OpenAI-compatible backend** (`internal/llm`), written new: stdlib
  `net/http`, a hand-written SSE reader, tool-call assembly from indexed
  deltas, transient retry with backoff while nothing has been consumed,
  `finish_reason=length` returned as a partial result, and a per-provider
  context-length probe (`[llm].provider`: LM Studio's `/api/v0/models`,
  Ollama's `/api/show`, or the configured `[model].context_window`).
- **Config schema:** `[llm]` (provider / base_url / model / api_key) and
  `[model].context_window`; `LAGENT_PROVIDER` / `LAGENT_BASE_URL` /
  `LAGENT_MODEL` / `LAGENT_API_KEY` in the environment layer.
- The sibling runtime's `.gem-agent.toml` is an operator-only file here
  too: the two runtimes share projects, and a file only one protects is
  a file the other's model may rewrite.
- Scaffold: Go module, cobra root command answering `--version` and
  `version` identically (pinned by a test), Makefile with the org
  build / sign / notarize / release-gate targets, docs mirror check,
  RFP, ADR-0001 (porting sources pinned), ADR-0002 (features not
  reproduced from gem-agent).

### Fixed

- `/readonly` and `/help` described commands this runtime does not
  have: the usage line offered `auto on|off` (gem-agent's watcher, not
  ported), and `/help` listed `/compact`, `/riskbook`, `/memory`,
  `/skills` and `/skill`. The UI catalog drops those strings and the
  ones no code reads; `/help` describes `/mcp load <server>`. The same
  sweep over every string in the runtime: the round-limit notice no
  longer credits a progress review, the approval reasons no longer name
  a model tier, the memory-write rule and ceiling kind are gone with
  the memory tools, `/usage` no longer has empty review and compaction
  lines, and the TUI's skill-expansion hook is gone.
- A network client failing in the read lane is told which lane to ask
  for even when it printed nothing: `curl -s` exits 6 without a word,
  so the text-keyed hint never fired and the model retried the same
  lane until it concluded the sandbox blocks the network. The hint now
  also keys on a finite list of network programs (curl, wget, ssh, scp,
  sftp, nc, ncat, telnet, dig, nslookup, host, ping, traceroute) and
  git's remote subcommands.
- The documentation was audited against the code before release: the
  configuration reference no longer shows a default for `[llm].model`
  (there is none; startup fails without it), lists `LAGENT_STATE_DIR`,
  the exported `LAGENT_SESSION_ID` / `LAGENT_WORK_DIR` /
  `LAGENT_PROJECT_DIR` and the `LAGENT_MCP_STDERR` debug switch, and
  covers every subcommand flag and slash command; the RFP carries
  in-place notes where ADR-0003/0004/0005 and the build amended it; the
  shipped `.lagent.toml` template and the ported code cite gem-agent's
  ADRs as gem-agent's, not as records this repository has.
